package iris

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func bridgeReference(owner project, id string) bridgeClaimParams {
	return bridgeClaimParams{conversationParams: closeReference(owner, id), Options: defaultBridgeOptions()}
}

func mustClaimBridge(t *testing.T, f deliveryFixture, owner project) bridgeClaim {
	t.Helper()

	value, err := f.store.claimBridge(t.Context(), bridgeReference(owner, f.conversation.ID))
	if err != nil {
		t.Fatal(err)
	}

	return value
}

// A binding has one renewable owner and keeps project context and its private consumer isolated.
func TestBridgeOwnership(t *testing.T) {
	f := newDeliveryFixture(t)

	claim := mustClaimBridge(t, f, f.beta)
	if claim.Binding.ProjectPath != *f.beta.Path || claim.Binding.ProjectID != f.beta.ID || claim.Binding.ThreadID != nil {
		t.Fatalf("binding=%+v", claim.Binding)
	}

	if _, err := f.store.claimBridge(t.Context(), bridgeReference(f.beta, f.conversation.ID)); !hasCode(err, "server_busy") {
		t.Fatalf("duplicate owner=%v", err)
	}

	other := mustClaimBridge(t, f, f.alpha)
	if other.Binding.ID == claim.Binding.ID || other.Binding.ConsumerID == claim.Binding.ConsumerID {
		t.Fatal("bindings share worker progress")
	}

	assertBridgePublicStatus(t, f, claim)
	assertBridgeStop(t, f, claim)
}

func assertBridgePublicStatus(t *testing.T, f deliveryFixture, claim bridgeClaim) {
	t.Helper()

	status, err := f.store.inspectBridge(t.Context(), closeReference(f.beta, f.conversation.ID))
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(status)
	if err != nil || strings.Contains(string(encoded), claim.OwnerToken) || strings.Contains(string(encoded), "owner_token") {
		t.Fatalf("status leaks ownership: %s, %v", encoded, err)
	}
}

func assertBridgeStop(t *testing.T, f deliveryFixture, claim bridgeClaim) {
	t.Helper()

	owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}
	if _, err := f.store.stopBridge(t.Context(), closeReference(f.beta, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	value, err := f.store.renewBridge(t.Context(), owner)
	if err != nil || value.StopRequestedAt == nil {
		t.Fatalf("stop request=%+v, %v", value, err)
	}

	value, err = f.store.checkpointBridge(t.Context(), bridgeCheckpoint{bridgeOwner: owner, Action: "stop"})
	if err != nil || value.State != "stopped" || value.LeaseUntil != nil {
		t.Fatalf("stopped=%+v, %v", value, err)
	}

	if _, err := f.store.renewBridge(t.Context(), owner); !hasCode(err, "worker_uncertain") {
		t.Fatalf("released token accepted: %v", err)
	}

	restarted := mustClaimBridge(t, f, f.beta)
	if restarted.Binding.ID != claim.Binding.ID || restarted.OwnerToken == claim.OwnerToken {
		t.Fatal("restart did not keep binding and replace ownership")
	}
}

// Concurrent claims serialize at SQLite; expiry permits safe idle takeover but rejects stale mutations.
func TestBridgeConcurrentClaim(t *testing.T) {
	f := newDeliveryFixture(t)

	var group sync.WaitGroup

	results := make(chan error, 2)

	for range 2 {
		group.Go(func() {
			_, err := f.store.claimBridge(t.Context(), bridgeReference(f.beta, f.conversation.ID))
			results <- err
		})
	}

	group.Wait()
	close(results)

	var winners, losers int

	for err := range results {
		if err == nil {
			winners++
		} else if hasCode(err, "server_busy") {
			losers++
		} else {
			t.Fatal(err)
		}
	}

	if winners != 1 || losers != 1 {
		t.Fatalf("winners=%d, losers=%d", winners, losers)
	}
}

// Expired invoking work never gets silently replayed by a new owner.
func TestBridgeExpiredLease(t *testing.T) {
	f := newDeliveryFixture(t)
	now := time.Now().UTC()
	f.store.now = func() time.Time { return now }
	claim := mustClaimBridge(t, f, f.beta)
	owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}
	now = now.Add(bridgeLease)

	if _, err := f.store.renewBridge(t.Context(), owner); !hasCode(err, "worker_uncertain") {
		t.Fatalf("expired lease accepted: %v", err)
	}

	replacement := mustClaimBridge(t, f, f.beta)
	if replacement.OwnerToken == claim.OwnerToken {
		t.Fatal("expired token reused")
	}

	if _, err := f.store.db.ExecContext(t.Context(), "UPDATE bridge_bindings SET state='invoking',lease_until=? WHERE id=?", now.UnixMilli(), claim.Binding.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := f.store.claimBridge(t.Context(), bridgeReference(f.beta, f.conversation.ID)); !hasCode(err, "worker_uncertain") {
		t.Fatalf("uncertain execution replayed: %v", err)
	}
}
