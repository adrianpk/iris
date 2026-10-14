package iris

import (
	"context"
	"errors"
	"time"
)

type codexWriteDeadline interface{ SetWriteDeadline(time.Time) error }

// Real app-server stdin is a pollable pipe; cancellation must also bound a blocked write.
func (client *codexClient) writeContext(ctx context.Context, value any) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}

	pipe, ok := client.input.(codexWriteDeadline)
	if !ok {
		return client.write(value)
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}

	if err := pipe.SetWriteDeadline(deadline); err != nil {
		return err
	}

	interrupted := make(chan error, 1)

	stop := context.AfterFunc(ctx, func() { interrupted <- pipe.SetWriteDeadline(time.Now()) })
	defer func() {
		if !stop() {
			resultErr = errors.Join(resultErr, <-interrupted)
		}

		resultErr = errors.Join(resultErr, pipe.SetWriteDeadline(time.Time{}))
	}()

	err := client.write(value)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err != nil && ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}

	return err
}
