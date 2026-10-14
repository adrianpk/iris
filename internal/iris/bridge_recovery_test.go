package iris

import (
	"testing"
)

// A completed result survives store reopen and lease takeover, then publishes once without another invocation.
func TestBridgeRecordedRecovery(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	claim, input := invokingBridge(t, f)
	params := completionReference(claim, input)

	value, err := f.store.recordBridgeResult(t.Context(), params)
	if err != nil || value.State != "result_recorded" {
		t.Fatalf("record=%+v %v", value, err)
	}

	assertBridgePosition(t, f, claim, 0, 2)
	assertRecordedRetry(t, f, params)
	clock.advance(bridgeLease)

	reopened, err := openStore(t.Context(), f.config)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := reopened.close(); err != nil {
			t.Error(err)
		}
	})

	reopened.now = clock.now
	f.store = reopened

	recovered := mustClaimBridge(t, f, f.beta)
	if recovered.PendingResult == nil || recovered.Binding.ID != claim.Binding.ID || recovered.Binding.TurnsUsed != 1 {
		t.Fatalf("recovery=%+v", recovered)
	}

	if _, err := f.store.completeBridge(t.Context(), params); !hasCode(err, "worker_uncertain") {
		t.Fatalf("stale owner=%v", err)
	}

	assertRecoveredPublication(t, f, claim, recovered)
}

func assertRecordedRetry(t *testing.T, f deliveryFixture, params bridgeCompletion) {
	t.Helper()

	if _, err := f.store.recordBridgeResult(t.Context(), params); err != nil {
		t.Fatal(err)
	}

	params.Answer.Body = "changed result"
	if _, err := f.store.recordBridgeResult(t.Context(), params); !hasCode(err, "idempotency_conflict") {
		t.Fatalf("changed result=%v", err)
	}

	if _, err := f.store.completeBridge(t.Context(), params); !hasCode(err, "idempotency_conflict") {
		t.Fatalf("changed publication=%v", err)
	}
}

// Recovery claims preserve unsafe invocation checkpoints and cannot invoke again or share an owner.
func TestBridgeInvocationRecovery(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	claim, _ := invokingBridge(t, f)

	clock.advance(bridgeLease)

	recovered, err := f.store.recoverBridgeClaim(t.Context(), closeReference(f.beta, f.conversation.ID))
	if err != nil || recovered.Binding.State != "invoking" || recovered.Binding.TurnID == nil || recovered.Binding.TurnsUsed != 1 {
		t.Fatalf("recovery=%+v %v", recovered, err)
	}

	if recovered.OwnerToken == claim.OwnerToken {
		t.Fatal("owner generation reused")
	}

	params := bridgeCheckpoint{bridgeOwner: bridgeOwner{BindingID: recovered.Binding.ID, OwnerToken: recovered.OwnerToken}, Action: "invoke", InputMessageID: *recovered.Binding.InputMessageID}
	if _, err := f.store.checkpointBridge(t.Context(), params); !hasCode(err, "worker_uncertain") {
		t.Fatalf("unsafe replay=%v", err)
	}

	if _, err := f.store.recoverBridgeClaim(t.Context(), closeReference(f.beta, f.conversation.ID)); !hasCode(err, "server_busy") {
		t.Fatalf("concurrent recovery=%v", err)
	}
}

// Lost publication confirmation retains durable ack state; a later owner does not replay committed input.
func TestBridgeLostCompletion(t *testing.T) {
	f := newDeliveryFixture(t)
	claim, input := invokingBridge(t, f)

	params := completionReference(claim, input)
	if _, err := f.store.completeBridge(t.Context(), params); err != nil {
		t.Fatal(err)
	}

	if _, err := f.store.checkpointBridge(t.Context(), bridgeCheckpoint{bridgeOwner: params.bridgeOwner, Action: "failure", ErrorCode: "worker_uncertain"}); err != nil {
		t.Fatal(err)
	}

	replacement := mustClaimBridge(t, f, f.beta)

	value, err := f.store.receiveBridge(t.Context(), bridgeReceiveParams{bridgeOwner: bridgeOwner{BindingID: replacement.Binding.ID, OwnerToken: replacement.OwnerToken}})
	if err != nil || value.Page.AckedSequence != 1 || value.Page.Messages[0].Kind != "system" {
		t.Fatalf("replay=%+v %v", value, err)
	}

	assertBridgePosition(t, f, claim, 1, 3)
}

// A recorded result is recoverable over real Unix IPC without starting any model client.
func TestBridgeRecordedWorker(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)

	claim, input := invokingBridge(t, f)
	if _, err := f.store.recordBridgeResult(t.Context(), completionReference(claim, input)); err != nil {
		t.Fatal(err)
	}

	clock.advance(bridgeLease)

	recovered := mustClaimBridge(t, f, f.beta)
	cfg, _ := testService(t, f.store.handle)

	worker := bridgeWorker{app: NewApp(nil, nil), config: cfg, claim: recovered, binding: recovered.Binding, owner: bridgeOwner{BindingID: recovered.Binding.ID, OwnerToken: recovered.OwnerToken}}
	if err := worker.recover(t.Context()); err != nil {
		t.Fatal(err)
	}

	if worker.client != nil || worker.binding.State != "acked" {
		t.Fatalf("unexpected model client/state=%+v", worker.binding)
	}

	assertBridgePosition(t, f, claim, 1, 3)
}

// An invocation without a confirmed turn cannot be reconstructed from a different or guessed turn.
func TestBridgeUnconfirmedRecovery(t *testing.T) {
	f := newDeliveryFixture(t)

	claim, _ := invokingBridge(t, f)
	if _, err := f.store.db.ExecContext(t.Context(), "UPDATE bridge_bindings SET turn_id=NULL WHERE id=?", claim.Binding.ID); err != nil {
		t.Fatal(err)
	}

	binding, err := f.store.inspectBridge(t.Context(), closeReference(f.beta, f.conversation.ID))
	if err != nil {
		t.Fatal(err)
	}

	worker := bridgeWorker{binding: binding}
	if err := worker.recover(t.Context()); !hasCode(err, "worker_uncertain") {
		t.Fatalf("unconfirmed=%v", err)
	}

	assertBridgePosition(t, f, claim, 0, 2)
}

func assertRecoveredPublication(t *testing.T, f deliveryFixture, claim, recovered bridgeClaim) {
	t.Helper()

	completion := recovered.PendingResult.completion(bridgeOwner{BindingID: recovered.Binding.ID, OwnerToken: recovered.OwnerToken})

	result, err := f.store.completeBridge(t.Context(), completion)
	if err != nil || result.State != "acked" || result.ResultMessageID == nil {
		t.Fatalf("complete=%+v %v", result, err)
	}

	assertBridgePosition(t, f, claim, 1, 3)

	if _, err := f.store.completeBridge(t.Context(), completion); err != nil {
		t.Fatal(err)
	}

	assertBridgePosition(t, f, claim, 1, 3)
}
