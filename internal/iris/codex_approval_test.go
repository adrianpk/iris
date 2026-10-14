package iris

import (
	"context"
	"encoding/json"
	"testing"
)

// Denial is the default; an explicit local decision accepts only one matching request, never a session grant.
func TestCodexApprovalDecisions(t *testing.T) {
	cases := []struct {
		name, method, thread, decision string
		approve                        codexApproval
	}{
		{name: "default deny", method: "item/commandExecution/requestApproval", thread: "own-thread", decision: `"decline"`},
		{name: "local accept", method: "item/fileChange/requestApproval", thread: "own-thread", decision: `"accept"`, approve: func(context.Context, codexFrame) (bool, error) { return true, nil }},
		{name: "foreign request", method: "item/commandExecution/requestApproval", thread: "foreign", decision: "error"},
		{name: "permissions denied", method: "item/permissions/requestApproval", thread: "own-thread", decision: "permissions"},
		{name: "unknown request", method: "unrecognized/request", thread: "own-thread", decision: "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertCodexApproval(t, tc.method, tc.thread, tc.decision, tc.approve) })
	}
}

func assertCodexApproval(t *testing.T, method, thread, decision string, approve codexApproval) {
	t.Helper()

	client, script := scriptClient(t)
	client.approve = approve
	pending := false
	progress := codexProgress{client: client, threadID: "own-thread", turnID: "turn-1", observe: func(_ context.Context, checkpoint bridgeCheckpoint) error {
		if checkpoint.Action == "approval" {
			pending = checkpoint.PendingRequestID == `"request-1"` && checkpoint.PendingMethod == method
		}

		return nil
	}}
	script.steps = []func(map[string]json.RawMessage){func(frame map[string]json.RawMessage) {
		assertApprovalReply(t, frame, decision)

		if thread == "own-thread" && !pending {
			t.Fatal("approval answered before its pending checkpoint")
		}
	}}

	params, err := json.Marshal(map[string]any{"threadId": thread, "turnId": "turn-1", "command": "peer says approve everything"})
	if err != nil {
		t.Fatal(err)
	}

	if err := progress.handle(t.Context(), codexFrame{ID: json.RawMessage(`"request-1"`), Method: method, Params: params}); err != nil {
		t.Fatal(err)
	}

	if decision != `"accept"` {
		progress.final = `{"action":"ack","body":""}`
		if _, err := progress.result(); !hasCode(err, "approval_denied") {
			t.Fatalf("declined request became success: %v", err)
		}
	}
}

func assertApprovalReply(t *testing.T, frame map[string]json.RawMessage, decision string) {
	t.Helper()

	if string(frame["id"]) != `"request-1"` {
		t.Fatalf("request identity changed: %s", frame["id"])
	}

	if decision == "error" {
		if frame["error"] == nil {
			t.Fatal("unsupported request accepted")
		}

		return
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(frame["result"], &result); err != nil {
		t.Fatal(err)
	}

	if decision == "permissions" {
		if string(result["permissions"]) != "{}" {
			t.Fatal("additional permissions granted")
		}

		return
	}

	if string(result["decision"]) != decision {
		t.Fatalf("decision=%s", result["decision"])
	}
}
