package iris

import (
	"context"
	"errors"
	"time"
)

type bridgeWorker struct {
	app         *App
	config      config
	claim       bridgeClaim
	owner       bridgeOwner
	binding     bridgeBinding
	client      *codexClient
	threadReady bool
}

func (app *App) runBridgeOwner(ctx context.Context, cfg config, claim bridgeClaim) (bridgeBinding, error) {
	deadline := claim.Binding.StartedAt.Add(time.Duration(claim.Binding.Options.TotalSeconds) * time.Second)

	lifetime, end := context.WithDeadline(ctx, deadline)
	defer end()

	owned, cancel := context.WithCancelCause(lifetime)
	defer cancel(nil)

	worker := bridgeWorker{app: app, config: cfg, claim: claim, binding: claim.Binding, owner: bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}}
	lease := keepBridgeLease(owned, cfg, worker.owner, cancel)

	err := worker.resume(owned)
	if worker.client != nil {
		err = errors.Join(err, worker.client.close())
	}

	cancel(nil)

	leaseErr := <-lease
	if leaseErr != nil && !hasCode(leaseErr, "worker_stopped") {
		err = errors.Join(err, leaseErr)
	}

	return worker.finish(ctx, err)
}

func keepBridgeLease(ctx context.Context, cfg config, owner bridgeOwner, cancel context.CancelCauseFunc) <-chan error {
	done := make(chan error, 1)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				done <- nil

				return
			case <-ticker.C:
				value, err := serviceValue[bridgeBinding](ctx, cfg, "bridge.renew", owner)
				if err == nil && value.StopRequestedAt != nil {
					err = problem("worker_stopped", "bridge shutdown was requested")
				}

				if err != nil {
					cancel(err)

					done <- err

					return
				}
			}
		}
	}()

	return done
}

func (worker *bridgeWorker) run(ctx context.Context) error {
	for {
		value, err := serviceValue[bridgeDelivery](ctx, worker.config, "bridge.receive", bridgeReceiveParams{bridgeOwner: worker.owner, WaitSeconds: 30})
		if err != nil {
			return bridgeRunError(ctx, err)
		}

		if bridgeShouldStop(value) {
			return problem("worker_stopped", "bridge shutdown was requested")
		}

		if len(value.Page.Messages) == 0 {
			if !value.Active {
				return problem("conversation_inactive", "conversation is terminal")
			}

			continue
		}

		input := value.Page.Messages[0]
		if input.Kind == "message" && !value.Active {
			return problem("conversation_inactive", "pending work cannot run after closure")
		}

		if err := worker.consume(ctx, input); err != nil {
			return err
		}
	}
}

func bridgeRunError(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}

	return err
}

func (worker *bridgeWorker) finish(ctx context.Context, runErr error) (bridgeBinding, error) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	checkpoint := bridgeCheckpoint{bridgeOwner: worker.owner, Action: "stop"}
	if runErr != nil && !hasCode(runErr, "worker_stopped") {
		checkpoint.Action, checkpoint.ErrorCode = "failure", bridgeRunCode(runErr)
	}

	value, err := serviceValue[bridgeBinding](cleanup, worker.config, "bridge.checkpoint", checkpoint)

	if hasCode(runErr, "worker_stopped") {
		runErr = nil
	}

	return value, errors.Join(runErr, err)
}

func bridgeRunCode(err error) string {
	var failure *serviceError
	if errors.As(err, &failure) {
		if _, ok := bridgeFailureState(failure.Code); ok {
			return failure.Code
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "limit_exceeded"
	}

	if errors.Is(err, context.Canceled) {
		return "worker_interrupted"
	}

	return "worker_uncertain"
}

func bridgeShouldStop(value bridgeDelivery) bool {
	if !value.Stopping {
		return false
	}

	return value.Active || len(value.Page.Messages) == 0 || value.Page.Messages[0].Kind == "message"
}

func (worker *bridgeWorker) resume(ctx context.Context) error {
	if err := worker.recover(ctx); err != nil {
		return err
	}

	return worker.run(ctx)
}
