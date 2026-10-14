package iris

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"

	"modernc.org/sqlite"
)

type store struct {
	db *sql.DB
}

func openStore(ctx context.Context, cfg config) (*store, error) {
	if err := privateDirectory(cfg.dataRoot); err != nil {
		return nil, err
	}

	if err := createPrivateFile(cfg.database); err != nil {
		return nil, fmt.Errorf("prepare SQLite database: %w", err)
	}

	if err := checkSidecars(cfg.database); err != nil {
		return nil, err
	}

	connector, err := sqlite.NewConnector(databaseDSN(cfg.database))
	if err != nil {
		return nil, fmt.Errorf("configure SQLite: %w", err)
	}

	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &store{db: db}

	if err := s.migrate(ctx, initialMigrations()); err != nil {
		return nil, errors.Join(err, db.Close())
	}

	return s, nil
}

func databaseDSN(path string) string {
	location := url.URL{Scheme: "file", Path: path}

	query := url.Values{}
	for _, pragma := range []string{"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(WAL)", "synchronous(FULL)"} {
		query.Add("_pragma", pragma)
	}

	query.Set("_txlock", "immediate")

	location.RawQuery = query.Encode()

	return location.String()
}

func checkSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := privateRegularFile(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unsafe SQLite sidecar: %w", err)
		}
	}

	return nil
}

func (s *store) close() error {
	return s.db.Close()
}

func (s *store) write(ctx context.Context, apply func(*sql.Tx) error) (resultErr error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()

	if err := apply(tx); err != nil {
		return err
	}

	return tx.Commit()
}
