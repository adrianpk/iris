package iris

import (
	"testing"
	"time"
)

// Every terminal route preserves pair numbering while the next conversation starts without inherited worker context.
func TestLifecycleFreshConversation(t *testing.T) {
	for _, route := range []string{"closed", "expired", "pruned"} {
		t.Run(route, func(t *testing.T) { assertFreshLifecycleConversation(t, route) })
	}
}

func assertFreshLifecycleConversation(t *testing.T, route string) {
	t.Helper()

	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	old := bindHistoricalBridge(t, f)
	finishLifecycleRoute(t, f, clock, route)
	clock.advance(-time.Second)

	params := callFixture(t, f.store, f.alpha, f.beta.ID, "fresh-"+route)
	params.Body = "A fresh request for " + route
	fresh := mustCall(t, f.store, params)

	fresh = mustJoin(t, f.store, joinParams{projectSelection: projectSelection{Project: f.beta.ID}, Code: fresh.InvitationCode})
	if fresh.ID == f.conversation.ID || fresh.Ordinal != 2 || fresh.InitialMessage.Body != params.Body {
		t.Fatalf("fresh=%+v", fresh)
	}

	claim, err := f.store.claimBridge(t.Context(), bridgeReference(f.alpha, fresh.ID))
	if err != nil || claim.Binding.ThreadID != nil || claim.Binding.TurnsUsed != 0 || claim.Binding.ConsumerID == old.Binding.ConsumerID {
		t.Fatalf("new context=%+v, error=%v", claim.Binding, err)
	}

	page := mustReceive(t, f.store, receiveReference(f.beta, fresh.ID, "fresh-inspection"))
	if len(page.Messages) != 2 || page.Messages[0].ID != fresh.InitialMessage.ID {
		t.Fatalf("fresh history=%+v", page)
	}
}

func finishLifecycleRoute(t *testing.T, f deliveryFixture, clock *lifecycleClock, route string) {
	t.Helper()

	var err error

	switch route {
	case "expired":
		f.store.policy.InactivitySeconds = 1

		clock.advance(time.Second)

		_, err = f.store.expireBatch(t.Context())
	default:
		_, err = f.store.closeConversation(t.Context(), closeReference(f.alpha, f.conversation.ID))
	}

	if err != nil {
		t.Fatal(err)
	}

	if route == "pruned" {
		f.store.policy.RetentionSeconds = 1

		clock.advance(time.Second)

		preview := mustPreviewPrune(t, f.store, projectPruneParams(f))
		if _, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token}); err != nil {
			t.Fatal(err)
		}

		return
	}

	clock.advance(-time.Second)

	if _, err := f.store.receive(t.Context(), receiveReference(f.beta, f.conversation.ID, "historical")); !hasCode(err, "conversation_inactive") {
		t.Fatalf("terminal state reactivated after clock rollback: %v", err)
	}

	clock.advance(time.Second)
	assertRetainedConversation(t, f.store, f.conversation.ID)
}

func bindHistoricalBridge(t *testing.T, f deliveryFixture) bridgeClaim {
	t.Helper()

	old := mustClaimBridge(t, f, f.alpha)

	owner := bridgeOwner{BindingID: old.Binding.ID, OwnerToken: old.OwnerToken}
	if _, err := f.store.checkpointBridge(t.Context(), bridgeCheckpoint{bridgeOwner: owner, Action: "thread", ThreadID: "historical-owned-thread"}); err != nil {
		t.Fatal(err)
	}

	return old
}
