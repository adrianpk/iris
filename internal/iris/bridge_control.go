package iris

import (
	"context"
	"database/sql"
)

type bridgeCheckpoint struct {
	bridgeOwner
	Action string `json:"action"`
}

func (s *store) checkpointBridge(ctx context.Context, params bridgeCheckpoint) (bridgeBinding, error) {
	if params.Action != "stop" {
		return bridgeBinding{}, problem("invalid_request", "invalid bridge checkpoint action")
	}

	var value bridgeRecord

	err := s.write(ctx, func(tx *sql.Tx) error {
		var err error

		value, err = s.ownedBridge(ctx, tx, params.bridgeOwner)
		if err != nil {
			return err
		}

		if value.State != "waiting" && value.State != "acked" {
			return problem("worker_uncertain", "invocation cannot be released as safely stopped")
		}

		if _, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET state='stopped',owner_token=NULL,lease_until=NULL,updated_at=? WHERE id=?", s.now().UnixMilli(), value.ID); err != nil {
			return err
		}

		value, err = readBridge(ctx, tx, "b.id=?", value.ID)

		return err
	})

	return value.bridgeBinding, err
}

func (s *store) handleBridge(ctx context.Context, req request) response {
	switch req.Operation {
	case "bridge.claim":
		return typedOperation(ctx, req, s.claimBridge)
	case "bridge.renew":
		return typedOperation(ctx, req, s.renewBridge)
	case "bridge.checkpoint":
		return typedOperation(ctx, req, s.checkpointBridge)
	case "bridge.status":
		return typedOperation(ctx, req, s.inspectBridge)
	case "bridge.stop":
		return typedOperation(ctx, req, s.stopBridge)
	}

	return failure("unknown_operation", "unsupported bridge operation", false)
}

func (s *store) inspectBridge(ctx context.Context, params conversationParams) (bridgeBinding, error) {
	return s.bridgeStatus(ctx, params, false)
}

func (s *store) stopBridge(ctx context.Context, params conversationParams) (bridgeBinding, error) {
	return s.bridgeStatus(ctx, params, true)
}

func typedOperation[T, R any](ctx context.Context, req request, execute func(context.Context, T) (R, error)) response {
	var params T
	if err := decodeJSON(req.Params, &params); err != nil {
		return failure("invalid_request", "invalid operation parameters", false)
	}

	value, err := execute(ctx, params)

	return operationResponse(ctx, value, err)
}
