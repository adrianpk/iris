package iris

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type codexScript struct {
	t      *testing.T
	frames chan codexRead
	steps  []func(map[string]json.RawMessage)
}

func (script *codexScript) Write(data []byte) (int, error) {
	script.t.Helper()

	var frame map[string]json.RawMessage
	if err := json.Unmarshal(data, &frame); err != nil {
		script.t.Fatal(err)
	}

	if len(script.steps) == 0 {
		script.t.Fatalf("unexpected protocol write: %s", data)
	}

	next := script.steps[0]
	script.steps = script.steps[1:]

	next(frame)

	return len(data), nil
}

func (script *codexScript) Close() error { return nil }

func scriptClient(t *testing.T) (*codexClient, *codexScript) {
	t.Helper()

	script := &codexScript{t: t, frames: make(chan codexRead, 16)}

	return &codexClient{input: script, frames: script.frames}, script
}

func (script *codexScript) push(raw string) {
	script.t.Helper()

	var frame codexFrame
	if err := json.Unmarshal([]byte(raw), &frame); err != nil {
		script.t.Fatal(err)
	}

	script.frames <- codexRead{frame: frame}
}

// The installed protocol handshake creates only a project-bound thread and extracts completed final output.
func TestCodexThreadAndTurn(t *testing.T) {
	client, script := scriptClient(t)
	binding := bridgeBinding{ProjectPath: "/fixture/project", Options: defaultBridgeOptions()}
	script.steps = []func(map[string]json.RawMessage){
		func(frame map[string]json.RawMessage) {
			assertCodexMethod(t, frame, "initialize")
			script.push(`{"id":1,"result":{}}`)
		},
		func(frame map[string]json.RawMessage) { assertCodexMethod(t, frame, "initialized") },
		func(frame map[string]json.RawMessage) {
			assertCodexMethod(t, frame, "thread/start")

			if !strings.Contains(string(frame["params"]), `"cwd":"/fixture/project"`) || !strings.Contains(string(frame["params"]), `"approvalPolicy":"never"`) {
				t.Fatalf("thread policies=%s", frame["params"])
			}

			script.push(`{"id":2,"result":{"thread":{"id":"own-thread","cwd":"/fixture/project"}}}`)
		},
		func(frame map[string]json.RawMessage) {
			assertCodexMethod(t, frame, "turn/start")
			script.push(`{"method":"turn/started","params":{"threadId":"own-thread","turn":{"id":"turn-1"}}}`)
			script.push(`{"id":3,"result":{"turn":{"id":"turn-1"}}}`)
			pushCodexResult(script)
		},
	}

	if err := client.initialize(t.Context()); err != nil {
		t.Fatal(err)
	}

	thread, err := client.openThread(t.Context(), binding)
	if err != nil {
		t.Fatal(err)
	}

	binding.ThreadID = &thread

	result, err := client.runTurn(t.Context(), binding, "one conversation", ignoreCodexCheckpoint)
	if err != nil || result.Answer.Body != "verified response" || result.Usage == nil || *result.Usage != 42 {
		t.Fatalf("result=%+v, error=%v", result, err)
	}
}

func assertCodexMethod(t *testing.T, frame map[string]json.RawMessage, method string) {
	t.Helper()

	if string(frame["method"]) != fmt.Sprintf("%q", method) {
		t.Fatalf("method=%s, want %s", frame["method"], method)
	}
}

func ignoreCodexCheckpoint(context.Context, bridgeCheckpoint) error { return nil }

func pushCodexResult(script *codexScript) {
	script.push(`{"method":"item/completed","params":{"threadId":"other-thread","turnId":"other-turn","item":{"type":"agentMessage","text":"cross-conversation"}}}`)
	script.push(`{"method":"item/completed","params":{"threadId":"own-thread","turnId":"turn-1","item":{"type":"commandExecution","text":"tool output"}}}`)
	script.push(`{"method":"item/completed","params":{"threadId":"own-thread","turnId":"turn-1","item":{"type":"agentMessage","phase":"analysis","text":"private analysis"}}}`)
	script.push(`{"method":"item/completed","params":{"threadId":"own-thread","turnId":"turn-1","item":{"type":"agentMessage","phase":"final_answer","text":"{\"action\":\"reply\",\"body\":\"verified response\"}"}}}`)
	script.push(`{"method":"thread/tokenUsage/updated","params":{"threadId":"own-thread","turnId":"turn-1","tokenUsage":{"total":{"totalTokens":42}}}}`)
	script.push(`{"method":"turn/completed","params":{"threadId":"own-thread","turn":{"id":"turn-1","status":"completed"}}}`)
}

// Only an explicitly bound thread can be resumed; failed or foreign turns never become successful results.
func TestCodexFailures(t *testing.T) {
	cases := []struct{ name, event, code string }{
		{"failed", `{"method":"turn/completed","params":{"threadId":"own-thread","turn":{"id":"turn-1","status":"failed"}}}`, "worker_failed"},
		{"interrupted", `{"method":"turn/completed","params":{"threadId":"own-thread","turn":{"id":"turn-1","status":"interrupted"}}}`, "worker_interrupted"},
		{"foreign turn", `{"method":"turn/completed","params":{"threadId":"own-thread","turn":{"id":"unowned","status":"completed"}}}`, "worker_uncertain"},
		{"tool failed", `{"method":"item/completed","params":{"threadId":"own-thread","turnId":"turn-1","item":{"type":"commandExecution","status":"failed"}}}`, "worker_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := scriptClient(t)
			progress := codexProgress{client: client, threadID: "own-thread", turnID: "turn-1"}

			var frame codexFrame
			if err := json.Unmarshal([]byte(tc.event), &frame); err != nil {
				t.Fatal(err)
			}

			if err := progress.handle(t.Context(), frame); !hasCode(err, tc.code) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

// Resumption cannot replace a binding with an unrelated thread returned by the server.
func TestCodexResumeIdentity(t *testing.T) {
	for _, returned := range []string{"own-thread", "unrelated"} {
		t.Run(returned, func(t *testing.T) {
			client, script := scriptClient(t)
			thread := "own-thread"
			binding := bridgeBinding{ThreadID: &thread, ProjectPath: "/fixture/project", Options: defaultBridgeOptions()}
			script.steps = []func(map[string]json.RawMessage){func(frame map[string]json.RawMessage) {
				assertCodexMethod(t, frame, "thread/resume")

				if !strings.Contains(string(frame["params"]), `"threadId":"own-thread"`) {
					t.Fatalf("resume=%s", frame["params"])
				}

				script.push(fmt.Sprintf(`{"id":1,"result":{"thread":{"id":%q,"cwd":"/fixture/project"}}}`, returned))
			}}

			_, err := client.openThread(t.Context(), binding)
			if returned == thread && err != nil {
				t.Fatal(err)
			}

			if returned != thread && !hasCode(err, "worker_uncertain") {
				t.Fatalf("foreign resume=%v", err)
			}
		})
	}
}
