package iris

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Read-only recovery accepts only the recorded completed turn and its final answer, never another invocation.
func TestCodexRecordedRecovery(t *testing.T) {
	thread, turn := "own-thread", "own-turn"
	binding := bridgeBinding{ThreadID: &thread, TurnID: &turn, ProjectPath: "/fixture/project"}
	client, script := scriptClient(t)
	script.steps = []func(map[string]json.RawMessage){func(frame map[string]json.RawMessage) {
		assertCodexMethod(t, frame, "thread/read")
		script.push(`{"id":1,"result":` + storedCodexFixture("completed", turn, "/fixture/project", "completed") + `}`)
	}}

	value, err := client.readResult(t.Context(), binding)
	if err != nil || value.Answer.Body != "recovered result" || value.TurnID != turn || value.Usage != nil {
		t.Fatalf("result=%+v %v", value, err)
	}

	if len(script.steps) != 0 {
		t.Fatal("recovery skipped inspection")
	}
}

func storedCodexFixture(status, turn, cwd, toolStatus string) string {
	return fmt.Sprintf(`{"thread":{"id":"own-thread","cwd":%q,"turns":[{"id":%q,"status":%q,"items":[{"type":"commandExecution","status":%q,"exitCode":0},{"type":"agentMessage","phase":"commentary","text":"not final"},{"type":"agentMessage","phase":"final_answer","text":"{\"action\":\"reply\",\"body\":\"recovered result\"}"}]}]}}`, cwd, turn, status, toolStatus)
}

// Failed, interrupted, incomplete, foreign and mismatched thread history never becomes successful work.
func TestCodexRecoveryFailures(t *testing.T) {
	thread, turn := "own-thread", "own-turn"
	binding := bridgeBinding{ThreadID: &thread, TurnID: &turn, ProjectPath: "/fixture/project"}

	cases := []struct{ name, status, turn, cwd, tool, code string }{
		{"unfinished", "inProgress", turn, binding.ProjectPath, "completed", "worker_uncertain"},
		{"interrupted", "interrupted", turn, binding.ProjectPath, "completed", "worker_interrupted"},
		{"failed", "failed", turn, binding.ProjectPath, "completed", "worker_failed"},
		{"foreign turn", "completed", "foreign", binding.ProjectPath, "completed", "worker_uncertain"},
		{"foreign cwd", "completed", turn, "/fixture/other", "completed", "worker_uncertain"},
		{"failed tool", "completed", turn, binding.ProjectPath, "failed", "worker_failed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := recoverCodexResult(json.RawMessage(storedCodexFixture(test.status, test.turn, test.cwd, test.tool)), binding)
			if !hasCode(err, test.code) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
