package iris

import (
	"context"
	"database/sql"
)

type bridgeCheckpoint struct {
	bridgeOwner
	Action   string `json:"action"`
	TurnID   string `json:"turn_id,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
}

func (s *store) checkpointBridge(ctx context.Context, params bridgeCheckpoint) (bridgeBinding, error) {
	var value bridgeRecord

	err := s.write(ctx, func(tx *sql.Tx) error {
		var err error

		value, err = s.ownedBridge(ctx, tx, params.bridgeOwner)
		if err != nil {
			return err
		}

		if err := s.updateBridgeCheckpoint(ctx, tx, value, params); err != nil {
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

func (s *store) updateBridgeCheckpoint(ctx context.Context, tx *sql.Tx, value bridgeRecord, params bridgeCheckpoint) error {
	switch params.Action {
	case "stop":
		return s.releaseBridge(ctx, tx, value)
	case "thread":
		return s.bindBridgeThread(ctx, tx, value, params.ThreadID)
	case "turn":
		if value.State != "invoking" || !validText(params.TurnID, 128) {
			return problem("worker_uncertain", "turn does not match an invoking binding")
		}

		if value.TurnID != nil && *value.TurnID != params.TurnID {
			return problem("worker_uncertain", "binding already has a different turn")
		}

		_, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET turn_id=?,updated_at=? WHERE id=?", params.TurnID, s.now().UnixMilli(), value.ID)

		return err
	}

	return problem("invalid_request", "invalid bridge checkpoint action")
}

func (s *store) releaseBridge(ctx context.Context, tx *sql.Tx, value bridgeRecord) error {
	if value.State != "waiting" && value.State != "acked" {
		return problem("worker_uncertain", "invocation cannot be released as safely stopped")
	}

	_, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET state='stopped',owner_token=NULL,lease_until=NULL,updated_at=? WHERE id=?", s.now().UnixMilli(), value.ID)

	return err
}

func (s *store) bindBridgeThread(ctx context.Context, tx *sql.Tx, value bridgeRecord, thread string) error {
	if !validText(thread, 128) || (value.ThreadID != nil && *value.ThreadID != thread) {
		return problem("worker_uncertain", "cannot replace the bound Codex thread")
	}

	if value.State != "waiting" && value.State != "acked" {
		return problem("worker_uncertain", "cannot bind a thread during invocation")
	}

	_, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET thread_id=?,updated_at=? WHERE id=?", thread, s.now().UnixMilli(), value.ID)

	return err
}
