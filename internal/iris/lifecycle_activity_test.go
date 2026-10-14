package iris

import (
	"sync"
	"testing"
	"time"
)

// New content renews inactivity; identical send/call/join retries and maintenance preserve its timestamp.
func TestLifecycleSignificantActivity(t *testing.T) {
	for _, kind := range []string{"message", "status"} {
		t.Run(kind, func(t *testing.T) {
			f := newDeliveryFixture(t)
			clock := useLifecycleClock(f)
			f.store.policy.InactivitySeconds = 5

			clock.advance(4 * time.Second)

			key := "activity-key"
			params := sendReference(f.alpha, f.conversation.ID, "new content")
			params.Kind, params.IdempotencyKey = kind, &key
			mustSend(t, f.store, params)

			activity := clock.now()
			clock.advance(4 * time.Second)
			mustSend(t, f.store, params)
			mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "initial"))
			mustJoin(t, f.store, joinParams{projectSelection: projectSelection{Project: f.beta.ID}, Code: f.conversation.InvitationCode})

			if count, err := f.store.expireBatch(t.Context()); err != nil || count != 0 {
				t.Fatalf("premature expiration=%d, %v", count, err)
			}

			assertConversationActivity(t, f.store, f.conversation.ID, activity)
			clock.advance(time.Second)
			assertSingleExpiration(t, f.store, f.conversation.ID)
		})
	}
}

// Join, send and maintenance serialize at the exact deadline and append one terminal event per conversation.
func TestLifecycleExpirationRaces(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	f.store.policy.InvitationSeconds, f.store.policy.InactivitySeconds = 1, 1
	pending := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "racing expiry"))

	clock.advance(time.Second)

	var group sync.WaitGroup

	results := make(chan error, 3)

	group.Go(func() {
		_, err := f.store.send(t.Context(), sendReference(f.alpha, f.conversation.ID, "late content"))
		if !hasCode(err, "conversation_inactive") {
			results <- err
		}
	})
	group.Go(func() {
		_, err := f.store.join(t.Context(), joinParams{projectSelection: projectSelection{Project: f.beta.ID}, Code: pending.InvitationCode})
		if !hasCode(err, "invitation_expired") {
			results <- err
		}
	})
	group.Go(func() {
		_, err := f.store.expireBatch(t.Context())
		if err != nil {
			results <- err
		}
	})
	group.Wait()
	close(results)

	for err := range results {
		t.Fatalf("deadline race accepted work or failed: %v", err)
	}

	assertSingleExpiration(t, f.store, f.conversation.ID)
	assertSingleExpiration(t, f.store, pending.ID)
}
