package iris

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
)

func listenService(path string) (*net.UnixListener, error) {
	active, err := parseActivation(os.Getpid(), os.Getenv("LISTEN_PID"), os.Getenv("LISTEN_FDS"), os.Getenv("LISTEN_FDNAMES"))
	if err != nil {
		return nil, err
	}

	if !active {
		return listenUnix(path)
	}

	// systemd retains the original socket; only this process's descriptor is closed.
	syscall.CloseOnExec(3)

	file := os.NewFile(3, "iris.socket")
	if file == nil {
		return nil, fmt.Errorf("inherited Iris socket is unavailable")
	}

	listener, listenErr := inheritedListener(path, file)
	if err := errors.Join(listenErr, file.Close()); err != nil {
		if listener != nil {
			err = errors.Join(err, listener.Close())
		}

		return nil, err
	}

	return listener, nil
}

func parseActivation(processID int, pid, count, names string) (bool, error) {
	if pid == "" && count == "" && names == "" {
		return false, nil
	}

	owner, err := strconv.Atoi(pid)
	if err != nil || owner != processID {
		return false, fmt.Errorf("LISTEN_PID must identify this Iris process")
	}

	if count != "1" || (names != "" && names != "iris") {
		return false, fmt.Errorf("expected one inherited descriptor named iris")
	}

	return true, nil
}

func inheritedListener(path string, file *os.File) (*net.UnixListener, error) {
	accepting, err := syscall.GetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_ACCEPTCONN)
	if err != nil {
		return nil, fmt.Errorf("inspect inherited socket: %w", err)
	}

	if accepting != 1 {
		return nil, fmt.Errorf("inherited descriptor must be a listening socket")
	}

	listener, err := net.FileListener(file)
	if err != nil {
		return nil, fmt.Errorf("adopt inherited Iris socket: %w", err)
	}

	unix, ok := listener.(*net.UnixListener)
	if !ok || listener.Addr().Network() != "unix" {
		return nil, errors.Join(fmt.Errorf("inherited socket must use Unix streams"), listener.Close())
	}

	unix.SetUnlinkOnClose(false)

	if unix.Addr().String() != path {
		return nil, errors.Join(fmt.Errorf("inherited socket path does not match Iris runtime"), unix.Close())
	}

	if err := checkSocket(path); err != nil {
		return nil, errors.Join(err, unix.Close())
	}

	return unix, nil
}
