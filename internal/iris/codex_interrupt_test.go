package iris

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// A turn deadline interrupts only the owned turn and requires a terminal confirmation before reporting interruption.
func TestCodexOwnedInterruption(t *testing.T) {
	for _, confirmed := range []bool{true, false} {
		name := "uncertain"
		if confirmed {
			name = "confirmed"
		}

		t.Run(name, func(t *testing.T) { assertCodexInterruption(t, confirmed) })
	}
}

func assertCodexInterruption(t *testing.T, confirmed bool) {
	t.Helper()

	client, script := scriptClient(t)
	thread := "own-thread"
	binding := bridgeBinding{ThreadID: &thread, ProjectPath: "/fixture/project", Options: defaultBridgeOptions()}
	script.steps = []func(map[string]json.RawMessage){
		func(frame map[string]json.RawMessage) {
			assertCodexMethod(t, frame, "turn/start")
			script.push(`{"id":1,"result":{"turn":{"id":"turn-1"}}}`)
		},
		func(frame map[string]json.RawMessage) {
			assertCodexMethod(t, frame, "turn/interrupt")

			if confirmed {
				script.push(`{"id":2,"result":{}}`)
				script.push(`{"method":"turn/completed","params":{"threadId":"own-thread","turn":{"id":"turn-1","status":"interrupted"}}}`)
			} else {
				script.push(`{"id":2,"error":{"code":-1,"message":"interruption unavailable"}}`)
			}
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	_, err := client.runTurn(ctx, binding, "bounded input", ignoreCodexCheckpoint)

	code := "worker_uncertain"
	if confirmed {
		code = "worker_interrupted"
	}

	if !hasCode(err, code) {
		t.Fatalf("error=%v, want %s", err, code)
	}
}
