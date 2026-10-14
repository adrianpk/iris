package iris

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type server struct {
	config config
	handle func(context.Context, request) response
}

func closeOnCancel(ctx context.Context, closer io.Closer) func() error {
	closed := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { closed <- closer.Close() })

	return func() error {
		if stop() {
			return nil
		}

		err := <-closed
		if errors.Is(err, net.ErrClosed) {
			return nil
		}

		return err
	}
}

func (s *server) serve(ctx context.Context, listener *net.UnixListener) (resultErr error) {
	ctx, cancel := context.WithCancel(ctx)
	stop := closeOnCancel(ctx, listener)

	var workers sync.WaitGroup

	defer func() { cancel(); resultErr = errors.Join(resultErr, stop()); workers.Wait() }()

	slots := make(chan struct{}, s.config.settings.MaxConnections)

	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return err
		}

		select {
		case slots <- struct{}{}:
			workers.Go(func() {
				defer func() { <-slots }()

				if err := s.connection(ctx, connection); err != nil {
					return
				}
			})
		default:
			if err := rejectBusy(connection); err != nil {
				continue
			}
		}
	}
}

func rejectBusy(connection net.Conn) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, connection.Close()) }()

	if err := connection.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		return err
	}

	if _, err := readFrame(connection, maxRequestBytes); err != nil {
		return err
	}

	return writeFrame(connection, failure("server_busy", "connection limit reached", true), maxResponseBytes)
}

func (s *server) connection(ctx context.Context, connection net.Conn) (resultErr error) {
	defer func() {
		if err := connection.Close(); !errors.Is(err, net.ErrClosed) {
			resultErr = errors.Join(resultErr, err)
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := closeOnCancel(ctx, connection)
	defer func() { resultErr = errors.Join(resultErr, stop()) }()

	deadline := time.Now().Add(time.Duration(s.config.settings.RequestTimeoutSeconds) * time.Second)
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}

	data, err := readFrame(connection, maxRequestBytes)
	if err != nil {
		if errors.Is(err, errFrameLimit) {
			return writeFrame(connection, failure("limit_exceeded", "request exceeds wire limit", false), maxResponseBytes)
		}

		return writeFrame(connection, failure("invalid_request", "invalid bounded JSON frame", false), maxResponseBytes)
	}

	var req request
	if err := decodeJSON(data, &req); err != nil {
		return writeFrame(connection, failure("invalid_request", "invalid JSON request", false), maxResponseBytes)
	}

	return s.respond(ctx, connection, req)
}

func (s *server) respond(ctx context.Context, connection net.Conn, req request) (resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, requestDuration(s.config, req))
	defer cancel()

	deadline, _ := ctx.Deadline()
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}

	if waitsForMessages(req) {
		watched, stop, err := watchDisconnect(ctx, connection)
		if err != nil {
			return err
		}

		ctx = watched

		defer func() { resultErr = errors.Join(resultErr, stop()) }()
	}

	reply := s.handle(ctx, req)
	if err := writeFrame(connection, reply, maxResponseBytes); err != nil {
		if errors.Is(err, errFrameLimit) {
			return writeFrame(connection, failure("limit_exceeded", "response exceeds wire limit", false), maxResponseBytes)
		}

		return err
	}

	return nil
}
