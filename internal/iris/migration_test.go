package iris

import (
	"os"
	"strings"
	"testing"
)

// An interrupted upgrade rolls back DDL and ledger entries, then can be retried safely.
func TestMigrationUpgradeRollback(t *testing.T) {
	s, _ := testStore(t)

	migrations := append(initialMigrations(), migration{2, "CREATE TABLE upgrade_probe (id INTEGER PRIMARY KEY)"})

	broken := append(append([]migration(nil), migrations...), migration{3, "CREATE TABLE temporary_probe (id INTEGER); INVALID SQL"})
	if err := s.migrate(t.Context(), broken); err == nil {
		t.Fatal("invalid upgrade passed")
	}

	var version int
	if err := s.db.QueryRowContext(t.Context(), "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 1 {
		t.Fatalf("version=%d error=%v", version, err)
	}

	var tables int
	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM sqlite_master WHERE name IN ('upgrade_probe','temporary_probe')").Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("partial tables=%d error=%v", tables, err)
	}

	if err := s.migrate(t.Context(), migrations); err != nil {
		t.Fatal(err)
	}

	if err := s.migrate(t.Context(), migrations); err != nil {
		t.Fatal("repeat migration:", err)
	}

	if err := s.db.QueryRowContext(t.Context(), "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 2 {
		t.Fatalf("version=%d error=%v", version, err)
	}
}

// Applied checksum changes, newer schemas and version gaps fail closed.
func TestMigrationCompatibility(t *testing.T) {
	cases := []struct {
		name       string
		statement  string
		migrations []migration
	}{
		{"checksum", "UPDATE schema_migrations SET checksum='changed'", initialMigrations()},
		{"future", "INSERT INTO schema_migrations VALUES (2,'future',0)", initialMigrations()},
		{"gap", "", []migration{{version: 2, sql: initialSchema}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := testStore(t)
			if tc.statement != "" {
				if _, err := s.db.ExecContext(t.Context(), tc.statement); err != nil {
					t.Fatal(err)
				}
			}

			if err := s.migrate(t.Context(), tc.migrations); err == nil {
				t.Fatal("incompatible schema accepted")
			}
		})
	}
}

// The embedded migration remains byte-identical to the accepted schema contract.
func TestMigrationMatchesContract(t *testing.T) {
	model, err := os.ReadFile("../../ops/default/spec/iris-model.md")
	if err != nil {
		t.Fatal(err)
	}

	_, schema, found := strings.Cut(string(model), "```sql\n")
	if !found {
		t.Fatal("documented schema missing")
	}

	schema, _, found = strings.Cut(schema, "```")
	if !found || schema != initialSchema {
		t.Fatal("embedded migration differs from accepted schema")
	}
}
