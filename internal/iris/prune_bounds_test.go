package iris

import (
	"database/sql"
	"fmt"
	"testing"
)

// A complete preview is required; the cap cannot silently truncate an eligible selection.
func TestPruneCandidateBound(t *testing.T) {
	f, _ := retainedFixture(t)
	seedRetainedRows(t, f, maxPruneCandidates)

	if _, err := f.store.previewPrune(t.Context(), projectPruneParams(f)); !hasCode(err, "limit_exceeded") {
		t.Fatalf("oversized preview=%v", err)
	}

	params := projectPruneParams(f)
	params.State = "closed"

	preview := mustPreviewPrune(t, f.store, params)
	if len(preview.Candidates) != 1 || preview.Candidates[0].ConversationID != f.conversation.ID {
		t.Fatalf("narrowed preview=%+v", preview)
	}
}

func seedRetainedRows(t *testing.T, f deliveryFixture, count int) {
	t.Helper()

	err := f.store.write(t.Context(), func(tx *sql.Tx) error {
		var low, high string
		if err := tx.QueryRowContext(t.Context(), "SELECT project_low,project_high FROM conversations WHERE id=?", f.conversation.ID).Scan(&low, &high); err != nil {
			return err
		}

		for index := range count {
			id := fmt.Sprintf("%032x", index+1)

			_, err := tx.ExecContext(t.Context(), `INSERT INTO conversations (id,project_low,project_high,pair_ordinal,topic,state,created_at,last_activity_at,terminal_at) VALUES (?,?,?,?,'Synthetic retained history','expired',0,0,0)`, id, low, high, index+2)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(t.Context(), "INSERT INTO participants VALUES (?,?,?,0)", id, f.alpha.ID, f.alpha.Alias); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Confirmation metadata is bounded and expires, without removing conversation history.
func TestPrunePreviewBound(t *testing.T) {
	f, clock := retainedFixture(t)
	params := projectPruneParams(f)

	params.State = "expired"
	for range maxPrunePreviews {
		mustPreviewPrune(t, f.store, params)
	}

	if _, err := f.store.previewPrune(t.Context(), params); !hasCode(err, "limit_exceeded") {
		t.Fatalf("preview capacity=%v", err)
	}

	clock.advance(prunePreviewLifetime)
	mustPreviewPrune(t, f.store, params)
	assertRetainedConversation(t, f.store, f.conversation.ID)

	var count int
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM prune_previews").Scan(&count); err != nil || count != 1 {
		t.Fatalf("previews=%d, error=%v", count, err)
	}
}

// Project scope cannot prune a different pair's history; all-projects selection must be explicit.
func TestPruneProjectScope(t *testing.T) {
	f, _ := retainedFixture(t)
	gamma := registeredProject(t, f.store, "gamma")
	params := pruneParams{projectSelection: projectSelection{Project: gamma.ID}, ConversationID: f.conversation.ID}

	preview := mustPreviewPrune(t, f.store, params)
	if len(preview.Candidates) != 0 {
		t.Fatal("foreign project selected retained history")
	}

	params = pruneParams{AllProjects: true, ConversationID: f.conversation.ID}

	preview = mustPreviewPrune(t, f.store, params)
	if len(preview.Candidates) != 1 || preview.ProjectID != nil {
		t.Fatalf("all-projects=%+v", preview)
	}

	params.Project = gamma.ID
	if _, err := f.store.previewPrune(t.Context(), params); !hasCode(err, "invalid_request") {
		t.Fatalf("ambiguous scope=%v", err)
	}
}
