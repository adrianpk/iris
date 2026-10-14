package iris

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
)

var errSchemaMismatch = errors.New("schema mismatch")

//go:embed migrations/001_initial.sql
var initialSchema string

//go:embed migrations/002_bridge_progress.sql
var bridgeProgressSchema string

//go:embed migrations/003_bridge_result.sql
var bridgeResultSchema string

const storageSchemaVersion = 3

type migration struct {
	version int
	sql     string
}

type migrationRecord struct {
	version  int
	checksum string
}

func initialMigrations() []migration {
	return []migration{{version: 1, sql: initialSchema}, {version: 2, sql: bridgeProgressSchema}, {version: 3, sql: bridgeResultSchema}}
}

func migrationChecksum(text string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
}

func (s *store) migrate(ctx context.Context, migrations []migration) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		history, err := migrationHistory(ctx, tx)
		if err != nil {
			return err
		}

		if err := verifyMigrationHistory(history, migrations); err != nil {
			return err
		}

		for _, next := range migrations[len(history):] {
			if _, err := tx.ExecContext(ctx, next.sql); err != nil {
				return fmt.Errorf("apply migration %d: %w", next.version, err)
			}

			if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations VALUES (?,?,?)", next.version, migrationChecksum(next.sql), s.now().UTC().UnixMilli()); err != nil {
				return fmt.Errorf("record migration %d: %w", next.version, err)
			}
		}

		return nil
	})
}

func migrationHistory(ctx context.Context, tx *sql.Tx) ([]migrationRecord, error) {
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&exists); err != nil {
		return nil, fmt.Errorf("inspect migration ledger: %w", err)
	}

	if exists == 0 {
		var tables int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
			return nil, err
		}

		if tables != 0 {
			return nil, fmt.Errorf("%w: unversioned nonempty database", errSchemaMismatch)
		}

		return nil, nil
	}

	return readMigrationHistory(ctx, tx)
}

func readMigrationHistory(ctx context.Context, tx *sql.Tx) (history []migrationRecord, resultErr error) {
	rows, err := tx.QueryContext(ctx, "SELECT version,checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	for rows.Next() {
		var record migrationRecord
		if err := rows.Scan(&record.version, &record.checksum); err != nil {
			return nil, err
		}

		history = append(history, record)
	}

	return history, rows.Err()
}

func verifyMigrationHistory(history []migrationRecord, migrations []migration) error {
	if len(history) > len(migrations) {
		return fmt.Errorf("%w: database schema is newer than this executable", errSchemaMismatch)
	}

	for index, next := range migrations {
		if next.version != index+1 {
			return fmt.Errorf("migration versions must be consecutive from 1")
		}
	}

	for index, record := range history {
		if record.version != migrations[index].version || record.checksum != migrationChecksum(migrations[index].sql) {
			return fmt.Errorf("%w: migration ledger or checksum mismatch at version %d", errSchemaMismatch, record.version)
		}
	}

	return nil
}
