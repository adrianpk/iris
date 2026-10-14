package iris

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func privateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create private directory: %w", err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect private directory: %w", err)
	}

	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 || !ownedByUser(info) {
		return fmt.Errorf("directory must be owned by the current user with private permissions")
	}

	return nil
}

func ownedByUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)

	return ok && stat.Uid == uint32(os.Geteuid())
}

func privateRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || !ownedByUser(info) {
		return fmt.Errorf("file must be regular, user-owned and private")
	}

	return nil
}

func createPrivateFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if errors.Is(err, os.ErrExist) {
		return privateRegularFile(path)
	}

	if err != nil {
		return err
	}

	return file.Close()
}
