package iris

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

func callService(ctx context.Context, cfg config, req request) (reply response, resultErr error) {
	if err := checkSocket(cfg.socket); err != nil {
		return response{}, unavailable(err)
	}

	dialer := net.Dialer{Timeout: time.Duration(cfg.settings.RequestTimeoutSeconds) * time.Second}

	connection, err := dialer.DialContext(ctx, "unix", cfg.socket)
	if err != nil {
		return response{}, unavailable(err)
	}

	defer func() {
		if err := connection.Close(); !errors.Is(err, net.ErrClosed) {
			resultErr = errors.Join(resultErr, err)
		}
	}()

	stop := closeOnCancel(ctx, connection)
	defer func() {
		resultErr = errors.Join(resultErr, stop())
		if ctx.Err() != nil {
			resultErr = ctx.Err()
		}
	}()

	if err := setClientDeadline(ctx, cfg, connection); err != nil {
		return response{}, unavailable(err)
	}

	if err := writeFrame(connection, req, maxRequestBytes); err != nil {
		if errors.Is(err, errFrameLimit) {
			return response{}, problem("limit_exceeded", "encoded request exceeds the protocol limit")
		}
		return response{}, unavailable(err)
	}

	unix, ok := connection.(*net.UnixConn)
	if !ok {
		return response{}, fmt.Errorf("expected Unix stream connection")
	}

	if err := unix.CloseWrite(); err != nil {
		return response{}, unavailable(err)
	}

	return readResponse(connection)
}

func readResponse(connection net.Conn) (response, error) {
	data, err := readFrame(connection, maxResponseBytes)
	if err != nil {
		return response{}, unavailable(err)
	}

	var reply response
	if err := decodeJSON(data, &reply); err != nil {
		return response{}, unavailable(err)
	}

	if err := validateResponse(reply); err != nil {
		return response{}, unavailable(err)
	}

	if reply.Error != nil {
		return response{}, reply.Error
	}

	return reply, nil
}

func unavailable(cause error) *serviceError {
	return &serviceError{Code: "storage_unavailable", Message: "cannot reach Iris service; start iris serve or check runtime permissions", cause: cause}
}

func setClientDeadline(ctx context.Context, cfg config, connection net.Conn) error {
	deadline := time.Now().Add(time.Duration(cfg.settings.RequestTimeoutSeconds) * time.Second)
	if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}

	return connection.SetDeadline(deadline)
}

func validateResponse(reply response) error {
	if reply.Protocol != protocolVersion || reply.SchemaVersion != 1 {
		return fmt.Errorf("invalid response version")
	}

	if reply.OK {
		if reply.Error != nil || len(reply.Result) == 0 {
			return fmt.Errorf("invalid success envelope")
		}

		return nil
	}

	if reply.Error == nil || len(reply.Result) != 0 {
		return fmt.Errorf("invalid error envelope")
	}

	if reply.Error.Code == "" || reply.Error.Message == "" {
		return fmt.Errorf("incomplete error envelope")
	}

	return nil
}
