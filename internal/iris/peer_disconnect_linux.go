package iris

import (
	"context"
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

func duplicateUnixSocket(connection net.Conn) (int, error) {
	socket, ok := connection.(*net.UnixConn)
	if !ok {
		return -1, problem("internal_error", "receive wait requires a Unix stream")
	}

	raw, err := socket.SyscallConn()
	if err != nil {
		return -1, err
	}

	fd := -1

	var duplicateErr error

	err = raw.Control(func(value uintptr) { fd, duplicateErr = unix.FcntlInt(value, unix.F_DUPFD_CLOEXEC, 0) })

	return fd, errors.Join(err, duplicateErr)
}

func watchDisconnect(ctx context.Context, connection net.Conn) (context.Context, func() error, error) {
	socket, err := duplicateUnixSocket(connection)
	if err != nil {
		return ctx, nil, err
	}

	var wake [2]int
	if err := unix.Pipe2(wake[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return ctx, nil, errors.Join(err, unix.Close(socket))
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)

	go func() { err := pollDisconnect(socket, wake[0]); cancel(); done <- err }()

	stop := func() error {
		cancel()

		_, wakeErr := unix.Write(wake[1], []byte{1})
		pollErr := <-done

		return errors.Join(wakeErr, pollErr, unix.Close(socket), unix.Close(wake[0]), unix.Close(wake[1]))
	}

	return ctx, stop, nil
}

func pollDisconnect(socket, wake int) error {
	// events=0 observes full hangup/errors without mistaking the protocol's write half-close for disconnect.
	descriptors := []unix.PollFd{{Fd: int32(socket)}, {Fd: int32(wake), Events: unix.POLLIN}}
	for {
		_, err := unix.Poll(descriptors, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		return err
	}
}
