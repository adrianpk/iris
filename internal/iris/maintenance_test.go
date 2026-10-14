package iris

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type lifecycleClock struct{ millis atomic.Int64 }

func (clock *lifecycleClock) now() time.Time              { return milliseconds(clock.millis.Load()) }
func (clock *lifecycleClock) advance(delta time.Duration) { clock.millis.Add(delta.Milliseconds()) }

func useLifecycleClock(f deliveryFixture) *lifecycleClock {
	clock := &lifecycleClock{}
	clock.millis.Store(f.conversation.LastActivityAt.UnixMilli())

	f.store.now = clock.now

	return clock
}

func startMaintenance(t *testing.T, s *store) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- s.maintain(ctx) }()

	t.Cleanup(func() {
		cancel()

		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("maintenance did not stop")
		}
	})
}

func awaitConversationState(t *testing.T, s *store, id, want string) {
	t.Helper()

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		var state string
		if err := s.db.QueryRowContext(t.Context(), "SELECT state FROM conversations WHERE id=?", id).Scan(&state); err != nil {
			t.Fatal(err)
		}

		if state == want {
			return
		}

		select {
		case <-deadline.C:
			t.Fatalf("state=%s, want %s", state, want)
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		case <-ticker.C:
		}
	}
}

// Expiration runs independently of an agent request and wakes blocked consumers only after commit.
func TestMaintenanceExpiration(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	f.store.policy.InactivitySeconds = 1
	f.store.policy.InvitationSeconds = 1
	pending := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "pending expiration"))
	params := receiveReference(f.beta, f.conversation.ID, "waiting")

	page := mustReceive(t, f.store, params)
	for _, input := range page.Messages {
		mustAck(t, f.store, ackReference(f.beta, input, "waiting"))
	}

	params.WaitSeconds = 30
	done := make(chan error, 1)

	go func() { _, err := f.store.receive(t.Context(), params); done <- err }()

	awaitWaiterCount(t, f.store, 1)
	startMaintenance(t, f.store)
	clock.advance(time.Second)
	f.store.wakeMaintenance()
	awaitConversationState(t, f.store, f.conversation.ID, "expired")
	awaitConversationState(t, f.store, pending.ID, "expired")

	select {
	case err := <-done:
		if !hasCode(err, "conversation_inactive") {
			t.Fatalf("waiting receive=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("expiration did not wake receive")
	}

	assertConversationActivity(t, f.store, f.conversation.ID, f.conversation.LastActivityAt)
	assertSingleExpiration(t, f.store, f.conversation.ID)
}

func assertSingleExpiration(t *testing.T, s *store, id string) {
	t.Helper()

	if _, err := s.expireBatch(t.Context()); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM messages WHERE conversation_id=? AND body='Conversation expired.'", id).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expiration events=%d", count)
	}
}

// Nearest-deadline scheduling expires an active conversation without a request or an injected wakeup.
func TestMaintenanceTimer(t *testing.T) {
	f := newDeliveryFixture(t)
	f.store.policy.InactivitySeconds = 1
	startMaintenance(t, f.store)
	awaitConversationState(t, f.store, f.conversation.ID, "expired")
	assertSingleExpiration(t, f.store, f.conversation.ID)
}

// Startup reconciliation drains more than one bounded batch and retains every terminal transcript.
func TestMaintenanceReconciliation(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)

	f.store.policy.InvitationSeconds = 1
	for index := range maintenanceBatchSize + 1 {
		mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, fmt.Sprintf("overdue-%d", index)))
		clock.advance(time.Second)
	}

	if err := f.store.reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM conversations WHERE state='expired'").Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != maintenanceBatchSize+1 {
		t.Fatalf("reconciled=%d", count)
	}
}

// A failed terminal event rolls back state and bridge shutdown markers and emits no notification.
func TestMaintenanceRollback(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	f.store.policy.InactivitySeconds = 1
	claim := mustClaimBridge(t, f, f.alpha)

	subscription, err := f.store.events.subscribe(f.conversation.ID, f.alpha.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer f.store.events.remove(subscription)

	if _, err := f.store.db.ExecContext(t.Context(), `CREATE TRIGGER reject_expiration BEFORE INSERT ON messages WHEN NEW.body='Conversation expired.' BEGIN SELECT RAISE(ABORT,'injected event failure'); END`); err != nil {
		t.Fatal(err)
	}

	clock.advance(time.Second)

	if _, err := f.store.expireBatch(t.Context()); err == nil {
		t.Fatal("injected terminal event failure succeeded")
	}

	assertExpirationRollback(t, f, claim)

	select {
	case <-subscription.ready:
		t.Fatal("rollback emitted a terminal notification")
	default:
	}
}

func assertExpirationRollback(t *testing.T, f deliveryFixture, claim bridgeClaim) {
	t.Helper()

	var (
		state   string
		stopped sql.NullInt64
	)

	if err := f.store.db.QueryRowContext(t.Context(), "SELECT state FROM conversations WHERE id=?", f.conversation.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}

	if err := f.store.db.QueryRowContext(t.Context(), "SELECT stop_requested_at FROM bridge_bindings WHERE id=?", claim.Binding.ID).Scan(&stopped); err != nil {
		t.Fatal(err)
	}

	if state != "active" || stopped.Valid {
		t.Fatalf("state=%s, stopped=%+v", state, stopped)
	}
}
