package iris

import (
	"context"
	"database/sql"
)

type bridgeCheckpoint struct {
	bridgeOwner
	Action           string `json:"action"`
	TurnID           string `json:"turn_id,omitempty"`
	ThreadID         string `json:"thread_id,omitempty"`
	PendingRequestID string `json:"pending_request_id,omitempty"`
	PendingMethod    string `json:"pending_method,omitempty"`
	InputMessageID   string `json:"input_message_id,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
}

func (s *store) checkpointBridge(ctx context.Context, params bridgeCheckpoint) (bridgeBinding, error) {
	var (
		value   bridgeRecord
		outcome error
	)

	err := s.write(ctx, func(tx *sql.Tx) error {
		var err error

		value, err = s.ownedBridge(ctx, tx, params.bridgeOwner)
		if err != nil {
			return err
		}

		active, err := s.checkBridgeLifecycle(ctx, tx, value, params.Action)
		if err != nil {
			return err
		}

		if !active {
			outcome = problem("conversation_inactive", "bridge work is stopping")

			return nil
		}

		if err := s.updateBridgeCheckpoint(ctx, tx, value, params); err != nil {
			return err
		}

		value, err = readBridge(ctx, tx, "b.id=?", value.ID)

		return err
	})
	if err != nil {
		return bridgeBinding{}, err
	}

	if outcome != nil {
		s.notify(value.ConversationID)
	}

	return value.bridgeBinding, outcome
}

func (s *store) handleBridge(ctx context.Context, req request) response {
	switch req.Operation {
	case "bridge.recover":
		return typedOperation(ctx, req, s.recoverBridgeClaim)
	case "bridge.record":
		return typedOperation(ctx, req, s.recordBridgeResult)
	case "bridge.receive":
		return typedOperation(ctx, req, s.receiveBridge)
	case "bridge.complete":
		return typedOperation(ctx, req, s.completeBridge)
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
	case "invoke":
		return s.invokeBridge(ctx, tx, value, params.InputMessageID)
	case "failure":
		return s.failBridge(ctx, tx, value, params.ErrorCode)
	case "stop":
		return s.releaseBridge(ctx, tx, value)
	case "thread":
		return s.bindBridgeThread(ctx, tx, value, params.ThreadID)
	case "approval", "running":
		return s.checkpointApproval(ctx, tx, value, params)
	case "turn":
		return s.bindBridgeTurn(ctx, tx, value, params.TurnID)
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

func (s *store) bindBridgeTurn(ctx context.Context, tx *sql.Tx, value bridgeRecord, turn string) error {
	if value.State != "invoking" || !validText(turn, 128) {
		return problem("worker_uncertain", "turn does not match an invoking binding")
	}

	if value.TurnID != nil && *value.TurnID != turn {
		return problem("worker_uncertain", "binding already has a different turn")
	}

	_, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET turn_id=?,updated_at=? WHERE id=?", turn, s.now().UnixMilli(), value.ID)

	return err
}

func (s *store) checkBridgeLifecycle(ctx context.Context, tx *sql.Tx, value bridgeRecord, action string) (bool, error) {
	if action == "failure" || action == "stop" {
		return true, nil
	}

	active, err := s.bridgeActive(ctx, tx, value)

	return active && value.StopRequestedAt == nil, err
}
