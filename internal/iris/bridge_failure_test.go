package iris

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

// Safe setup uses exactly three delayed retries and never retries a rejected or uncertain turn invocation.
func TestBridgeStartupRetries(t *testing.T) {
	cases := []struct {
		name    string
		failure error
		calls   int
	}{
		{"capacity", problem("provider_capacity", "capacity"), 4},
		{"setup disconnect", problem("worker_uncertain", "disconnected before invocation"), 4},
		{"configuration", problem("worker_failed", "missing executable"), 1},
		{"denied authority", problem("approval_denied", "denied"), 1},
		{"cancelled", context.Canceled, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			delays := []time.Duration{}

			err := retryBridgeStartup(t.Context(), func(context.Context) error { calls++; return test.failure }, func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil })
			if !errors.Is(err, test.failure) || calls != test.calls {
				t.Fatalf("calls=%d error=%v", calls, err)
			}

			if test.calls == 4 && !reflect.DeepEqual(delays, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}) {
				t.Fatalf("delays=%v", delays)
			}
		})
	}
}

// Expired invoking ownership is visibly uncertain without mutating or acknowledging the prior input.
func TestBridgeUncertainStatus(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	claim, _ := invokingBridge(t, f)

	clock.advance(bridgeLease)

	status, err := f.store.inspectBridge(t.Context(), closeReference(f.beta, f.conversation.ID))
	if err != nil || status.State != "uncertain" || status.ErrorCode == nil || *status.ErrorCode != "worker_uncertain" || status.RecoveryHint == nil {
		t.Fatalf("status=%+v %v", status, err)
	}

	assertBridgePosition(t, f, claim, 0, 2)
}

// Failed, interrupted and capacity outcomes remain distinct and cannot be silently reclaimed or acknowledged.
func TestBridgeFailedRecovery(t *testing.T) {
	for _, test := range []struct{ code, state string }{{"worker_failed", "failed"}, {"worker_interrupted", "interrupted"}, {"provider_capacity", "capacity"}, {"worker_uncertain", "uncertain"}} {
		t.Run(test.code, func(t *testing.T) {
			f := newDeliveryFixture(t)
			claim, _ := invokingBridge(t, f)
			owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}

			value, err := f.store.checkpointBridge(t.Context(), bridgeCheckpoint{bridgeOwner: owner, Action: "failure", ErrorCode: test.code})
			if err != nil || value.State != test.state {
				t.Fatalf("failure=%+v %v", value, err)
			}

			if _, err := f.store.claimBridge(t.Context(), bridgeReference(f.beta, f.conversation.ID)); !hasCode(err, test.code) {
				t.Fatalf("unsafe replay=%v", err)
			}

			assertBridgePosition(t, f, claim, 0, 2)
		})
	}
}

// Definitive protocol overload stays capacity; a lost turn-start confirmation stays uncertain and is never retried.
func TestCodexRejectedInvocation(t *testing.T) {
	client, script := scriptClient(t)
	script.steps = []func(map[string]json.RawMessage){func(frame map[string]json.RawMessage) {
		assertCodexMethod(t, frame, "turn/start")
		script.push(`{"id":1,"error":{"code":-32001,"message":"overloaded"}}`)
	}}
	thread := "own-thread"

	_, err := client.runTurn(t.Context(), bridgeBinding{ThreadID: &thread, ProjectPath: "/fixture/project", Options: defaultBridgeOptions()}, "untrusted data", ignoreCodexCheckpoint)
	if !hasCode(err, "provider_capacity") || !errors.Is(err, errCodexRejected) {
		t.Fatalf("rejection=%v", err)
	}

	if len(script.steps) != 0 {
		t.Fatal("unexpected invocation retry")
	}
}

// Cancellation during backoff ends promptly; retry exhaustion cannot acknowledge an input.
func TestBridgeRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := bridgeRetryPause(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("pause=%v", err)
	}
}

// A recorded decline survives owner replacement and cannot be converted into success by a completed provider transcript.
func TestBridgeDeclinedRecovery(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	claim, input := invokingBridge(t, f)

	owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}
	for _, params := range []bridgeCheckpoint{
		{bridgeOwner: owner, Action: "approval", TurnID: "owned-turn", PendingRequestID: "denied-request", PendingMethod: "item/commandExecution/requestApproval"},
		{bridgeOwner: owner, Action: "running", TurnID: "owned-turn", ErrorCode: "approval_denied"},
	} {
		if _, err := f.store.checkpointBridge(t.Context(), params); err != nil {
			t.Fatal(err)
		}
	}

	clock.advance(bridgeLease)

	recovered, err := f.store.recoverBridgeClaim(t.Context(), closeReference(f.beta, f.conversation.ID))
	if err != nil {
		t.Fatal(err)
	}

	completion := completionReference(recovered, input)
	if _, err := f.store.recordBridgeResult(t.Context(), completion); !hasCode(err, "approval_denied") {
		t.Fatalf("denied result=%v", err)
	}

	if _, err := f.store.completeBridge(t.Context(), completion); !hasCode(err, "approval_denied") {
		t.Fatalf("denied completion=%v", err)
	}

	assertBridgePosition(t, f, claim, 0, 2)
}
