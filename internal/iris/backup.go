package iris

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
)

func (s *store) backup(ctx context.Context, path string) (resultErr error) {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("backup path must be absolute")
	}

	if err := privateDirectory(filepath.Dir(path)); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create new backup: %w", err)
	}

	if err := file.Close(); err != nil {
		return err
	}

	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, os.Remove(path))
		}
	}()

	connection, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, connection.Close()) }()

	return connection.Raw(func(raw any) error {
		provider, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("SQLite driver does not expose backup")
		}

		operation, err := provider.NewBackup(path)
		if err != nil {
			return err
		}

		return copyBackup(ctx, operation)
	})
}

func copyBackup(ctx context.Context, operation *sqlite.Backup) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, operation.Finish()) }()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		more, err := operation.Step(128)
		if err != nil {
			return err
		}

		if !more {
			return nil
		}
	}
}
