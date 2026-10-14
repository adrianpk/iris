package iris

import (
	"testing"
	"time"
)

// Every invocation rechecks persisted limits; rejection cannot consume or acknowledge its input.
func TestBridgeInvocationBudgets(t *testing.T) {
	cases := []struct {
		name   string
		adjust func(*testing.T, deliveryFixture, bridgeClaim)
	}{
		{"turns", func(t *testing.T, f deliveryFixture, c bridgeClaim) {
			t.Helper()

			if _, err := f.store.db.ExecContext(t.Context(), "UPDATE bridge_bindings SET turns_used=20 WHERE id=?", c.Binding.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"usage", func(t *testing.T, f deliveryFixture, c bridgeClaim) {
			t.Helper()

			if _, err := f.store.db.ExecContext(t.Context(), "UPDATE bridge_bindings SET usage_tokens=100000 WHERE id=?", c.Binding.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"time", func(t *testing.T, f deliveryFixture, c bridgeClaim) {
			t.Helper()

			if _, err := f.store.db.ExecContext(t.Context(), "UPDATE bridge_bindings SET started_at=? WHERE id=?", time.Now().Add(-time.Hour).UnixMilli(), c.Binding.ID); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDeliveryFixture(t)
			claim := mustClaimBridge(t, f, f.beta)
			input := mustBridgeInput(t, f, claim)

			owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}
			if _, err := f.store.checkpointBridge(t.Context(), bridgeCheckpoint{bridgeOwner: owner, Action: "thread", ThreadID: "owned"}); err != nil {
				t.Fatal(err)
			}

			tc.adjust(t, f, claim)

			if _, err := f.store.checkpointBridge(t.Context(), bridgeCheckpoint{bridgeOwner: owner, Action: "invoke", InputMessageID: input.ID}); !hasCode(err, "limit_exceeded") {
				t.Fatalf("budget=%v", err)
			}

			assertBridgePosition(t, f, claim, 0, 2)
		})
	}
}

// Informational and reachable terminal events are acknowledged without creating a thread or turn.
func TestBridgeInformationalInputs(t *testing.T) {
	f := newDeliveryFixture(t)

	claim := mustClaimBridge(t, f, f.alpha)
	if _, err := f.store.closeConversation(t.Context(), closeReference(f.alpha, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		input := mustBridgeInput(t, f, claim)
		params := bridgeCompletion{bridgeOwner: bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}, InputMessageID: input.ID, Answer: bridgeAnswer{Action: "ack"}}

		value, err := f.store.completeBridge(t.Context(), params)
		if err != nil || value.TurnsUsed != 0 || value.ThreadID != nil {
			t.Fatalf("informational result=%+v, error=%v", value, err)
		}
	}

	assertBridgePosition(t, f, claim, 3, 3)
}
