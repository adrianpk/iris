package iris

import (
	"context"
	"errors"
	"time"
)

const bridgeStartupRetries = 3

// Only pre-invocation setup can retry. Once invoke is checkpointed, no turn is retried here.
func retryBridgeStartup(ctx context.Context, attempt func(context.Context) error, pause func(context.Context, time.Duration) error) error {
	for retries := 0; ; retries++ {
		err := attempt(ctx)
		if err == nil || retries == bridgeStartupRetries || !safeStartupRetry(err) {
			return err
		}

		if err := pause(ctx, time.Second<<retries); err != nil {
			return err
		}
	}
}

func safeStartupRetry(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return hasCode(err, "provider_capacity") || hasCode(err, "worker_uncertain")
}

func bridgeRetryPause(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (worker *bridgeWorker) setupCodex(ctx context.Context) (resultErr error) {
	defer func() {
		if resultErr != nil && worker.client != nil {
			resultErr = errors.Join(resultErr, worker.client.close())
			worker.client = nil
			worker.threadReady = false
		}
	}()

	if err := worker.connectCodex(ctx); err != nil {
		return err
	}

	setup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	thread, err := worker.client.openThread(setup, worker.binding)
	if err != nil {
		return err
	}

	err = worker.checkpoint(ctx, bridgeCheckpoint{Action: "thread", ThreadID: thread})
	worker.threadReady = err == nil

	return err
}
