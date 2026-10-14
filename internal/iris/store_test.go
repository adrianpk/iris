package iris

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testStore(t *testing.T) (*store, config) {
	t.Helper()

	root := t.TempDir()

	cfg, err := configFromEnvironment(environment{home: root, dataHome: filepath.Join(root, "data"), runtimeDir: filepath.Join(root, "run")})
	if err != nil {
		t.Fatal(err)
	}

	s, err := openStore(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := s.close(); err != nil {
			t.Error(err)
		}
	})

	return s, cfg
}

func insertTestProject(ctx context.Context, tx *sql.Tx, alias string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO projects VALUES (?,NULL,?,0,NULL)", strings.Repeat(alias[:1], 32), alias)

	return err
}

// A committed row and the immutable migration ledger survive store reopening.
func TestStoreReopen(t *testing.T) {
	s, cfg := testStore(t)

	ctx := t.Context()
	if err := s.write(ctx, func(tx *sql.Tx) error { return insertTestProject(ctx, tx, "alpha") }); err != nil {
		t.Fatal(err)
	}

	if err := s.close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := openStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.close(); err != nil {
			t.Error(err)
		}
	}()

	var alias string
	if err := reopened.db.QueryRowContext(ctx, "SELECT alias FROM projects").Scan(&alias); err != nil || alias != "alpha" {
		t.Fatalf("alias=%q error=%v", alias, err)
	}

	var (
		version  int
		checksum string
	)
	if err := reopened.db.QueryRowContext(ctx, "SELECT version,checksum FROM schema_migrations").Scan(&version, &checksum); err != nil {
		t.Fatal(err)
	}

	if version != 1 || checksum != migrationChecksum(initialSchema) {
		t.Fatal("migration ledger changed")
	}
}

// Required SQLite pragmas, indexes and restrictive files are effective in the driver.
func TestStoreConfiguration(t *testing.T) {
	s, cfg := testStore(t)
	for _, tc := range []struct{ name, want string }{
		{"foreign_keys", "1"}, {"journal_mode", "wal"}, {"synchronous", "2"}, {"busy_timeout", "5000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value string
			if err := s.db.QueryRowContext(t.Context(), "PRAGMA "+tc.name).Scan(&value); err != nil || value != tc.want {
				t.Fatalf("value=%q error=%v", value, err)
			}
		})
	}

	var count int
	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM sqlite_master WHERE type='index' AND name='participant_project'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("index count=%d error=%v", count, err)
	}

	for _, path := range []string{cfg.database, cfg.database + "-wal", cfg.database + "-shm"} {
		if err := privateRegularFile(path); err != nil {
			t.Fatal(err)
		}
	}
}

// Foreign-key and transaction failures never publish partially committed state.
func TestStoreRollback(t *testing.T) {
	s, _ := testStore(t)
	ctx := t.Context()
	stopped := errors.New("stop transaction")

	err := s.write(ctx, func(tx *sql.Tx) error {
		if err := insertTestProject(ctx, tx, "alpha"); err != nil {
			return err
		}

		return stopped
	})
	if !errors.Is(err, stopped) {
		t.Fatalf("rollback error=%v", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM projects").Scan(&count); err != nil || count != 0 {
		t.Fatalf("row count=%d error=%v", count, err)
	}

	err = s.write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO pair_sequences VALUES ('missing-a','missing-b',0)")

		return err
	})
	if err == nil {
		t.Fatal("foreign key accepted missing projects")
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	if err := s.write(cancelled, func(*sql.Tx) error { t.Fatal("cancelled transaction executed"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error=%v", err)
	}
}

// Independent database handles serialize immediate transactions without losing updates.
func TestStoreConcurrentWrites(t *testing.T) {
	first, cfg := testStore(t)

	second, err := openStore(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := second.close(); err != nil {
			t.Error(err)
		}
	}()

	ctx := t.Context()
	if err := first.write(ctx, func(tx *sql.Tx) error {
		if err := insertTestProject(ctx, tx, "alpha"); err != nil {
			return err
		}

		if err := insertTestProject(ctx, tx, "beta"); err != nil {
			return err
		}

		_, err := tx.ExecContext(ctx, "INSERT INTO pair_sequences VALUES (?,?,0)", strings.Repeat("a", 32), strings.Repeat("b", 32))

		return err
	}); err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	for i := range 24 {
		workers.Go(func() {
			s := []*store{first, second}[i%2]
			if err := s.write(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, "UPDATE pair_sequences SET last_ordinal=last_ordinal+1")

				return err
			}); err != nil {
				t.Error(err)
			}
		})
	}

	workers.Wait()

	var ordinal int
	if err := first.db.QueryRowContext(ctx, "SELECT last_ordinal FROM pair_sequences").Scan(&ordinal); err != nil || ordinal != 24 {
		t.Fatalf("ordinal=%d error=%v", ordinal, err)
	}
}

// Refuse symlink or broadly readable files rather than following or rewriting them.
func TestStorePrivateFiles(t *testing.T) {
	for _, kind := range []string{"directory", "database", "symlink", "sidecar"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()

			cfg, err := configFromEnvironment(environment{home: root, dataHome: root, runtimeDir: root})
			if err != nil {
				t.Fatal(err)
			}

			if err := privateDirectory(cfg.dataRoot); err != nil {
				t.Fatal(err)
			}

			prepareUnsafeStore(t, cfg, kind)

			if s, err := openStore(t.Context(), cfg); err == nil {
				if err := s.close(); err != nil {
					t.Error(err)
				}

				t.Fatal("unsafe path accepted")
			}
		})
	}
}

func prepareUnsafeStore(t *testing.T, cfg config, kind string) {
	t.Helper()

	switch kind {
	case "directory":
		if err := os.Chmod(cfg.dataRoot, 0o755); err != nil {
			t.Fatal(err)
		}
	case "database":
		if err := os.WriteFile(cfg.database, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		if err := os.Symlink(filepath.Join(t.TempDir(), "target"), cfg.database); err != nil {
			t.Fatal(err)
		}
	case "sidecar":
		if err := os.WriteFile(cfg.database+"-wal", nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
