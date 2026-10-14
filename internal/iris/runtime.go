package iris

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func acquireRuntime(cfg config) (*os.File, error) {
	if err := checkPrivateDirectory(filepath.Dir(cfg.runtimeRoot)); err != nil {
		return nil, err
	}

	if err := privateDirectory(cfg.runtimeRoot); err != nil {
		return nil, err
	}

	path := cfg.socket + ".lock"
	if err := createPrivateFile(path); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.Join(fmt.Errorf("service already owns this runtime: %w", err), file.Close())
	}

	// Closing releases the lock. Keep its inode to prevent competing lock generations.
	return file, nil
}

func listenUnix(path string) (*net.UnixListener, error) {
	if err := removeStaleSocket(path); err != nil {
		return nil, err
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}

	if err := os.Chmod(path, 0o600); err != nil {
		return nil, errors.Join(err, listener.Close())
	}

	return listener, nil
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSocket == 0 || !ownedByUser(info) || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("refuse to replace an unsafe socket path")
	}

	connection, err := net.DialTimeout("unix", path, 100*time.Millisecond)
	if err == nil {
		return errors.Join(fmt.Errorf("socket already has a listener"), connection.Close())
	}

	if !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Remove(path)
}

func checkSocket(path string) error {
	if err := checkPrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 || !ownedByUser(info) {
		return fmt.Errorf("socket must be user-owned with mode 0600")
	}

	return nil
}
