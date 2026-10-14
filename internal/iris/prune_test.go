package iris

import (
	"database/sql"
	"testing"
	"time"
)

func retainedFixture(t *testing.T) (deliveryFixture, *lifecycleClock) {
	t.Helper()

	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	f.store.policy.RetentionSeconds = 1
	mustReceive(t, f.store, receiveReference(f.beta, f.conversation.ID, "unacknowledged"))

	if _, err := f.store.closeConversation(t.Context(), closeReference(f.alpha, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	clock.advance(time.Second)

	return f, clock
}

func mustPreviewPrune(t *testing.T, s *store, params pruneParams) prunePreview {
	t.Helper()

	value, err := s.previewPrune(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func projectPruneParams(f deliveryFixture) pruneParams {
	return pruneParams{projectSelection: projectSelection{Project: f.alpha.ID}}
}

func retainedState(t *testing.T, f deliveryFixture) []int64 {
	t.Helper()

	var revision, activity, terminal, messages, bytes, ack int64
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT revision,last_activity_at,terminal_at,message_count,body_bytes FROM conversations WHERE id=?", f.conversation.ID).Scan(&revision, &activity, &terminal, &messages, &bytes); err != nil {
		t.Fatal(err)
	}

	if err := f.store.db.QueryRowContext(t.Context(), "SELECT acked_sequence FROM consumer_cursors WHERE consumer_id='unacknowledged'").Scan(&ack); err != nil {
		t.Fatal(err)
	}

	return []int64{revision, activity, terminal, messages, bytes, ack}
}

// Preview changes only confirmation metadata; exact confirmation deletes payload but preserves permanent allocations.
func TestPrunePreviewAndConfirm(t *testing.T) {
	f, _ := retainedFixture(t)
	preview := assertPrunePreviewReadOnly(t, f)

	result, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token})
	if err != nil || len(result.ConversationIDs) != 1 || result.ConversationIDs[0] != f.conversation.ID || result.Messages != preview.Messages || result.BodyBytes != preview.BodyBytes {
		t.Fatalf("result=%+v, error=%v", result, err)
	}

	assertPruneAllocations(t, f)

	if _, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token}); !hasCode(err, "preview_stale") {
		t.Fatalf("token reused: %v", err)
	}
}

func assertPruneAllocations(t *testing.T, f deliveryFixture) {
	t.Helper()

	for _, table := range []string{"conversations", "participants", "invitations", "messages", "message_recipients", "consumer_cursors", "bridge_bindings", "prune_previews", "prune_candidates"} {
		var count int
		if err := f.store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s count=%d, error=%v", table, count, err)
		}
	}

	for _, table := range []string{"projects", "pair_sequences", "invitation_code_tombstones", "call_requests"} {
		var count int
		if err := f.store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM "+table).Scan(&count); err != nil || count == 0 {
			t.Fatalf("allocation %s count=%d, error=%v", table, count, err)
		}
	}

	var original sql.NullString
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT conversation_id FROM call_requests WHERE request_key='initial'").Scan(&original); err != nil || original.Valid {
		t.Fatalf("request tombstone=%+v, error=%v", original, err)
	}
}

// A later-eligible conversation cannot be silently included in an earlier exact selection.
func TestPruneFrozenSelection(t *testing.T) {
	f, clock := retainedFixture(t)
	second := newRetainedConversation(t, f, "later terminal")
	pending := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "still pending"))

	preview := mustPreviewPrune(t, f.store, projectPruneParams(f))
	if len(preview.Candidates) != 1 {
		t.Fatalf("eligible=%+v", preview.Candidates)
	}

	clock.advance(time.Second)

	if _, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token}); err != nil {
		t.Fatal(err)
	}

	assertRetainedConversation(t, f.store, second.ID)
	assertRetainedConversation(t, f.store, pending.ID)

	fresh := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "after pruning"))
	if fresh.Ordinal != 4 || fresh.ID == f.conversation.ID || fresh.InvitationCode == f.conversation.InvitationCode {
		t.Fatalf("reused allocation=%+v", fresh)
	}
}

func newRetainedConversation(t *testing.T, f deliveryFixture, key string) conversation {
	t.Helper()

	value := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, key))
	mustJoin(t, f.store, joinParams{projectSelection: projectSelection{Project: f.beta.ID}, Code: value.InvitationCode})

	closed, err := f.store.closeConversation(t.Context(), closeReference(f.alpha, value.ID))
	if err != nil {
		t.Fatal(err)
	}

	return closed
}

func assertRetainedConversation(t *testing.T, s *store, id string) {
	t.Helper()

	if err := s.write(t.Context(), func(tx *sql.Tx) error { _, err := readConversation(t.Context(), tx, id); return err }); err != nil {
		t.Fatal(err)
	}
}

func assertPrunePreviewReadOnly(t *testing.T, f deliveryFixture) prunePreview {
	t.Helper()

	before := retainedState(t, f)

	preview := mustPreviewPrune(t, f.store, projectPruneParams(f))
	if len(preview.Candidates) != 1 || preview.Messages != 3 || preview.BodyBytes != before[4] {
		t.Fatalf("preview=%+v", preview)
	}

	after := retainedState(t, f)
	for index, value := range before {
		if after[index] != value {
			t.Fatal("preview mutated history or processing progress")
		}
	}

	return preview
}

// Retention includes the complete interval and becomes eligible exactly at terminal time plus policy.
func TestPruneRetentionBoundary(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)

	f.store.policy.RetentionSeconds = 1
	if _, err := f.store.closeConversation(t.Context(), closeReference(f.alpha, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	clock.advance(time.Second - time.Millisecond)

	preview := mustPreviewPrune(t, f.store, projectPruneParams(f))
	if len(preview.Candidates) != 0 {
		t.Fatal("retained history became eligible early")
	}

	clock.advance(time.Millisecond)

	preview = mustPreviewPrune(t, f.store, projectPruneParams(f))
	if len(preview.Candidates) != 1 {
		t.Fatal("retention boundary was ignored")
	}
}
