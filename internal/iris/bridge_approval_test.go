package iris

import "testing"

// Pending approval identity persists while connected, and only its current owner and turn can clear it.
func TestBridgePendingApproval(t *testing.T) {
	f := newDeliveryFixture(t)

	claim := mustClaimBridge(t, f, f.beta)
	if _, err := f.store.db.ExecContext(t.Context(), "UPDATE bridge_bindings SET state='invoking',turn_id='turn-1' WHERE id=?", claim.Binding.ID); err != nil {
		t.Fatal(err)
	}

	pending := bridgeCheckpoint{bridgeOwner: bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}, Action: "approval", TurnID: "turn-1", PendingRequestID: `"request-1"`, PendingMethod: "item/fileChange/requestApproval"}

	value, err := f.store.checkpointBridge(t.Context(), pending)
	if err != nil || value.State != "approval_required" || value.PendingRequestID == nil || *value.PendingRequestID != pending.PendingRequestID {
		t.Fatalf("pending=%+v, error=%v", value, err)
	}

	assertApprovalCleared(t, f, pending)
}

func assertApprovalCleared(t *testing.T, f deliveryFixture, pending bridgeCheckpoint) {
	t.Helper()

	pending.Action, pending.TurnID = "running", "foreign"
	if _, err := f.store.checkpointBridge(t.Context(), pending); !hasCode(err, "worker_uncertain") {
		t.Fatalf("foreign turn cleared approval: %v", err)
	}

	pending.TurnID = "turn-1"

	value, err := f.store.checkpointBridge(t.Context(), pending)
	if err != nil || value.State != "invoking" || value.PendingRequestID != nil || value.PendingMethod != nil {
		t.Fatalf("running=%+v, error=%v", value, err)
	}
}
