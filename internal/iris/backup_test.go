package iris

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// An online backup includes committed WAL data and reopens independently of its source.
func TestBackupReopen(t *testing.T) {
	s, _ := testStore(t)

	ctx := t.Context()
	if err := s.write(ctx, func(tx *sql.Tx) error { return insertTestProject(ctx, tx, "alpha") }); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "private")

	path := filepath.Join(root, "backup.db")
	if err := s.backup(ctx, path); err != nil {
		t.Fatal(err)
	}

	if err := privateRegularFile(path); err != nil {
		t.Fatal(err)
	}

	cfg := config{dataRoot: root, database: path}

	backup, err := openStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := backup.close(); err != nil {
			t.Error(err)
		}
	}()

	assertBackupContents(t, backup)

	if err := s.backup(ctx, path); err == nil {
		t.Fatal("existing backup overwritten")
	}
}

// Cancelled backup removes only its newly created destination and preserves the source.
func TestBackupCancellation(t *testing.T) {
	s, _ := testStore(t)
	path := filepath.Join(t.TempDir(), "private", "cancelled.db")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := s.backup(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("backup error=%v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial backup remains: %v", err)
	}

	if err := s.db.PingContext(t.Context()); err != nil {
		t.Fatal("source unavailable:", err)
	}
}

func assertBackupContents(t *testing.T, backup *store) {
	t.Helper()

	var alias, integrity string
	if err := backup.db.QueryRowContext(t.Context(), "SELECT alias FROM projects").Scan(&alias); err != nil || alias != "alpha" {
		t.Fatalf("alias=%q error=%v", alias, err)
	}

	if err := backup.db.QueryRowContext(t.Context(), "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q error=%v", integrity, err)
	}
}
