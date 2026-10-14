package iris

import (
	"testing"
	"time"
)

// Due expiration is durable even for an identical retry, which still returns the original message.
func TestSendExpiry(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := f.conversation.LastActivityAt
	f.store.now = func() time.Time { return clock }

	key := "before-expiry"
	params := sendReference(f.alpha, f.conversation.ID, "Original input")
	params.IdempotencyKey = &key
	original := mustSend(t, f.store, params)
	clock = clock.Add(defaultInactivityLifetime)

	retry := mustSend(t, f.store, params)
	if retry.ID != original.ID {
		t.Fatal("expiry changed committed retry")
	}

	assertHandshakeRows(t, f.store, f.conversation.ID, "expired", 4, 0)

	params.IdempotencyKey = nil
	if _, err := f.store.send(t.Context(), params); !hasCode(err, "conversation_inactive") {
		t.Fatalf("expired send=%v", err)
	}
}

// Storage quotas reject ordinary inserts before consuming the slot reserved for a terminal event.
func TestSendStorageBounds(t *testing.T) {
	for _, budget := range []string{"messages", "body"} {
		t.Run(budget, func(t *testing.T) {
			f := newDeliveryFixture(t)
			query := "UPDATE conversations SET message_count=? WHERE id=?"
			value := maxConversationMessages - 1

			if budget == "body" {
				query = "UPDATE conversations SET body_bytes=? WHERE id=?"
				value = maxConversationBodyBytes
			}

			if _, err := f.store.db.ExecContext(t.Context(), query, value, f.conversation.ID); err != nil {
				t.Fatal(err)
			}

			if _, err := f.store.send(t.Context(), sendReference(f.alpha, f.conversation.ID, "Body")); !hasCode(err, "limit_exceeded") {
				t.Fatalf("quota=%v", err)
			}

			mustClose(t, f.store, f.alpha, f.conversation.ID)
		})
	}
}

// Reads and acknowledgments preserve the last significant activity timestamp.
func TestDeliveryActivity(t *testing.T) {
	f := newDeliveryFixture(t)
	before := f.conversation.LastActivityAt
	f.store.now = func() time.Time { return before.Add(time.Hour) }
	page := mustReceive(t, f.store, receiveReference(f.beta, f.conversation.ID, "manual"))
	mustAck(t, f.store, ackReference(f.beta, page.Messages[0], "manual"))

	var stored int64
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT last_activity_at FROM conversations WHERE id=?", f.conversation.ID).Scan(&stored); err != nil || stored != before.UnixMilli() {
		t.Fatalf("reads renewed activity: %d error=%v", stored, err)
	}
}
