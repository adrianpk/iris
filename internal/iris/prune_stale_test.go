package iris

import (
	"database/sql"
	"testing"
	"time"
)

// Every stale condition rejects the whole selection and leaves its remaining history untouched.
func TestPruneStaleConfirmations(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, deliveryFixture, *lifecycleClock, prunePreview)
	}{
		{"changed revision", func(t *testing.T, f deliveryFixture, _ *lifecycleClock, _ prunePreview) {
			execPruneMutation(t, f, "UPDATE conversations SET revision=revision+1 WHERE id=?", f.conversation.ID)
		}},
		{"changed terminal", func(t *testing.T, f deliveryFixture, _ *lifecycleClock, _ prunePreview) {
			execPruneMutation(t, f, "UPDATE conversations SET terminal_at=terminal_at-1 WHERE id=?", f.conversation.ID)
		}},
		{"newly active", func(t *testing.T, f deliveryFixture, _ *lifecycleClock, _ prunePreview) {
			execPruneMutation(t, f, "UPDATE conversations SET state='active',terminal_at=NULL WHERE id=?", f.conversation.ID)
		}},
		{"missing candidate", func(t *testing.T, f deliveryFixture, _ *lifecycleClock, _ prunePreview) {
			execPruneMutation(t, f, "DELETE FROM conversations WHERE id=?", f.conversation.ID)
		}},
		{"policy changed", func(_ *testing.T, f deliveryFixture, _ *lifecycleClock, _ prunePreview) {
			f.store.policy.InvitationSeconds++
		}},
		{"expired token", func(_ *testing.T, _ deliveryFixture, clock *lifecycleClock, _ prunePreview) {
			clock.advance(prunePreviewLifetime)
		}},
		{"clock backward", func(_ *testing.T, _ deliveryFixture, clock *lifecycleClock, _ prunePreview) {
			clock.advance(-2 * time.Second)
		}},
		{"export lease", func(t *testing.T, f deliveryFixture, clock *lifecycleClock, _ prunePreview) {
			t.Helper()

			if err := f.store.write(t.Context(), func(*sql.Tx) error {
				f.store.exports["owned-export"] = exportLease{conversationID: f.conversation.ID, expiresAt: clock.now().Add(time.Minute)}

				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}},
		{"selection digest", func(t *testing.T, f deliveryFixture, _ *lifecycleClock, preview prunePreview) {
			execPruneMutation(t, f, "UPDATE prune_previews SET selection_digest='changed' WHERE token_digest=?", textDigest(preview.Token))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertStalePrune(t, tc.mutate) })
	}
}

func assertStalePrune(t *testing.T, mutate func(*testing.T, deliveryFixture, *lifecycleClock, prunePreview)) {
	t.Helper()

	f, clock := retainedFixture(t)
	second := newRetainedConversation(t, f, "second retained")

	clock.advance(time.Second)

	preview := mustPreviewPrune(t, f.store, projectPruneParams(f))
	if len(preview.Candidates) != 2 {
		t.Fatal("fixture did not preview both conversations")
	}

	mutate(t, f, clock, preview)

	if _, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token}); !hasCode(err, "preview_stale") {
		t.Fatalf("stale confirmation=%v", err)
	}

	assertRetainedConversation(t, f.store, second.ID)
}

func execPruneMutation(t *testing.T, f deliveryFixture, query string, args ...any) {
	t.Helper()

	if _, err := f.store.db.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatal(err)
	}
}

// A failure on the second deletion rolls back the first deletion, its cascades and token consumption.
func TestPruneDeletionRollback(t *testing.T) {
	f, clock := retainedFixture(t)
	second := newRetainedConversation(t, f, "second retained")

	clock.advance(time.Second)

	preview := mustPreviewPrune(t, f.store, projectPruneParams(f))
	id := preview.Candidates[1].ConversationID
	execPruneMutation(t, f, "CREATE TRIGGER reject_prune BEFORE DELETE ON conversations WHEN OLD.id='"+id+"' BEGIN SELECT RAISE(ABORT,'injected prune failure'); END")

	if _, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token}); err == nil {
		t.Fatal("injected deletion failure succeeded")
	}

	assertRetainedConversation(t, f.store, f.conversation.ID)
	assertRetainedConversation(t, f.store, second.ID)
	execPruneMutation(t, f, "DROP TRIGGER reject_prune")

	result, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token})
	if err != nil || len(result.ConversationIDs) != 2 {
		t.Fatalf("retry=%+v, error=%v", result, err)
	}
}
