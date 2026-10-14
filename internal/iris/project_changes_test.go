package iris

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Alias/path collisions fail rather than silently replacing another project's identity.
func TestProjectChangesRejectCollisions(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")

	beta := registeredProject(t, s, "beta")
	if _, err := s.registerProject(t.Context(), projectRegistration{Path: projectDirectory(t, "alpha")}); !hasCode(err, "alias_conflict") {
		t.Fatalf("registration collision=%v", err)
	}

	if _, err := s.renameProject(t.Context(), projectRename{ProjectID: alpha.ID, Name: beta.Alias}); !hasCode(err, "alias_conflict") {
		t.Fatalf("rename collision=%v", err)
	}

	if _, err := s.relocateProject(t.Context(), projectRelocation{ProjectID: alpha.ID, Path: *beta.Path}); !hasCode(err, "project_conflict") {
		t.Fatalf("relocation collision=%v", err)
	}

	if _, err := s.renameProject(t.Context(), projectRename{ProjectID: beta.ID, Name: alpha.ID}); err != nil {
		t.Fatal(err)
	}

	if _, err := selectTestProject(t.Context(), s, projectSelection{Project: alpha.ID}, false); !hasCode(err, "project_conflict") {
		t.Fatalf("ambiguous selector=%v", err)
	}
}

// Renaming and moving preserve stable IDs, registration time and participant snapshots.
func TestProjectHistoryAfterMove(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	beta := registeredProject(t, s, "beta")
	conversation := seedParticipantSnapshot(t, s, alpha, beta)

	renamed, err := s.renameProject(t.Context(), projectRename{ProjectID: alpha.ID, Name: "renamed"})
	if err != nil || renamed.ID != alpha.ID || renamed.Alias != "renamed" {
		t.Fatalf("rename=%+v error=%v", renamed, err)
	}

	newPath := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(*alpha.Path, newPath); err != nil {
		t.Fatal(err)
	}

	moved, err := s.relocateProject(t.Context(), projectRelocation{ProjectID: alpha.ID, Path: newPath})
	if err != nil {
		t.Fatal(err)
	}

	assertProjectMove(t, s, alpha, moved, newPath, conversation)
}

func assertProjectMove(t *testing.T, s *store, alpha, moved project, newPath, conversation string) {
	t.Helper()

	if moved.ID != alpha.ID || *moved.Path != newPath || !moved.RegisteredAt.Equal(*alpha.RegisteredAt) {
		t.Fatalf("move=%+v", moved)
	}

	var snapshot string
	if err := s.db.QueryRowContext(t.Context(), "SELECT alias_snapshot FROM participants WHERE conversation_id=? AND project_id=?", conversation, alpha.ID).Scan(&snapshot); err != nil || snapshot != "alpha" {
		t.Fatalf("snapshot=%q error=%v", snapshot, err)
	}

	selected, err := selectTestProject(t.Context(), s, projectSelection{CWD: newPath}, false)
	if err != nil || selected.ID != alpha.ID {
		t.Fatalf("discovery=%+v error=%v", selected, err)
	}
}

func seedParticipantSnapshot(t *testing.T, s *store, alpha, beta project) string {
	t.Helper()

	low, high := alpha.ID, beta.ID
	if low > high {
		low, high = high, low
	}

	conversation := strings.Repeat("f", 32)
	ctx := t.Context()

	err := s.write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO pair_sequences VALUES (?,?,1)", low, high); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO conversations
            (id,project_low,project_high,pair_ordinal,topic,state,created_at,last_activity_at)
            VALUES (?,?,?,1,'Snapshot','active',0,0)`, conversation, low, high); err != nil {
			return err
		}

		_, err := tx.ExecContext(ctx, "INSERT INTO participants VALUES (?,?,?,0)", conversation, alpha.ID, alpha.Alias)

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	return conversation
}
