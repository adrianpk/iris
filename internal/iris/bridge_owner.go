package iris

import (
	"context"
	"errors"
	"time"
)

func (app *App) runBridgeOwner(ctx context.Context, cfg config, claim bridgeClaim) (bridgeBinding, error) {
	owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var runErr error
	for runErr == nil {
		select {
		case <-ctx.Done():
			runErr = ctx.Err()
		case <-ticker.C:
			value, err := serviceValue[bridgeBinding](ctx, cfg, "bridge.renew", owner)
			if err != nil {
				runErr = err

				break
			}

			if value.StopRequestedAt != nil {
				return stopBridgeOwner(ctx, cfg, owner, nil)
			}
		}
	}

	return stopBridgeOwner(ctx, cfg, owner, runErr)
}

func stopBridgeOwner(ctx context.Context, cfg config, owner bridgeOwner, runErr error) (bridgeBinding, error) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	value, err := serviceValue[bridgeBinding](cleanup, cfg, "bridge.checkpoint", bridgeCheckpoint{bridgeOwner: owner, Action: "stop"})

	return value, errors.Join(runErr, err)
}
