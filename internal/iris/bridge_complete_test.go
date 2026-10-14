package iris

import "testing"

func invokingBridge(t *testing.T, f deliveryFixture) (bridgeClaim, message) {
	t.Helper()

	claim := mustClaimBridge(t, f, f.beta)
	input := mustBridgeInput(t, f, claim)

	owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}
	for _, params := range []bridgeCheckpoint{
		{Action: "thread", ThreadID: "owned-thread"},
		{Action: "invoke", InputMessageID: input.ID},
		{Action: "turn", TurnID: "owned-turn"},
	} {
		params.bridgeOwner = owner
		if _, err := f.store.checkpointBridge(t.Context(), params); err != nil {
			t.Fatal(err)
		}
	}

	return claim, input
}

func mustBridgeInput(t *testing.T, f deliveryFixture, claim bridgeClaim) message {
	t.Helper()

	value, err := f.store.receiveBridge(t.Context(), bridgeReceiveParams{bridgeOwner: bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}})
	if err != nil {
		t.Fatal(err)
	}

	if len(value.Page.Messages) != 1 {
		t.Fatalf("page=%+v", value)
	}

	return value.Page.Messages[0]
}

func completionReference(claim bridgeClaim, input message) bridgeCompletion {
	return bridgeCompletion{bridgeOwner: bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}, InputMessageID: input.ID, TurnID: "owned-turn", Answer: bridgeAnswer{Action: "reply", Body: "technical result"}}
}

// Reply and processing acknowledgment commit together; retry cannot duplicate or replace the result.
func TestBridgeAtomicCompletion(t *testing.T) {
	f := newDeliveryFixture(t)
	claim, input := invokingBridge(t, f)
	params := completionReference(claim, input)

	if _, err := f.store.db.ExecContext(t.Context(), `CREATE TRIGGER reject_bridge_ack BEFORE UPDATE OF acked_sequence ON consumer_cursors WHEN NEW.acked_sequence>OLD.acked_sequence BEGIN SELECT RAISE(ABORT,'injected ack failure'); END`); err != nil {
		t.Fatal(err)
	}

	if _, err := f.store.completeBridge(t.Context(), params); err == nil {
		t.Fatal("injected acknowledgment failure succeeded")
	}

	assertBridgePosition(t, f, claim, 0, 2)

	if _, err := f.store.db.ExecContext(t.Context(), "DROP TRIGGER reject_bridge_ack"); err != nil {
		t.Fatal(err)
	}

	result, err := f.store.completeBridge(t.Context(), params)
	if err != nil || result.State != "acked" || result.ResultMessageID == nil {
		t.Fatalf("result=%+v, error=%v", result, err)
	}

	assertBridgePosition(t, f, claim, 1, 3)
	assertBridgeCompletionRetry(t, f, params, *result.ResultMessageID)
}

func assertBridgeCompletionRetry(t *testing.T, f deliveryFixture, params bridgeCompletion, id string) {
	t.Helper()

	value, err := f.store.completeBridge(t.Context(), params)
	if err != nil || value.ResultMessageID == nil || *value.ResultMessageID != id {
		t.Fatalf("retry=%+v, error=%v", value, err)
	}

	params.Answer.Body = "changed result"
	if _, err := f.store.completeBridge(t.Context(), params); !hasCode(err, "idempotency_conflict") {
		t.Fatalf("changed completion=%v", err)
	}
}

func assertBridgePosition(t *testing.T, f deliveryFixture, claim bridgeClaim, wantAck, wantMessages int64) {
	t.Helper()

	var ack, count int64
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT acked_sequence FROM consumer_cursors WHERE consumer_id=?", claim.Binding.ConsumerID).Scan(&ack); err != nil {
		t.Fatal(err)
	}

	if err := f.store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM messages WHERE conversation_id=?", f.conversation.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if ack != wantAck || count != wantMessages {
		t.Fatalf("acked=%d, messages=%d", ack, count)
	}
}

// Closing after invocation prevents both result publication and acknowledgment of unfinished work.
func TestBridgeCompletionAfterClose(t *testing.T) {
	f := newDeliveryFixture(t)

	claim, input := invokingBridge(t, f)
	if _, err := f.store.closeConversation(t.Context(), closeReference(f.alpha, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	if _, err := f.store.completeBridge(t.Context(), completionReference(claim, input)); !hasCode(err, "conversation_inactive") {
		t.Fatalf("closed completion=%v", err)
	}

	assertBridgePosition(t, f, claim, 0, 3)
}

// An owner token is checked in the same transaction as any attempt to publish or acknowledge.
func TestBridgeStaleCompletion(t *testing.T) {
	f := newDeliveryFixture(t)
	claim, input := invokingBridge(t, f)
	params := completionReference(claim, input)

	params.OwnerToken = "00000000000000000000000000000000"
	if _, err := f.store.completeBridge(t.Context(), params); !hasCode(err, "worker_uncertain") {
		t.Fatalf("stale completion=%v", err)
	}

	assertBridgePosition(t, f, claim, 0, 2)
}
