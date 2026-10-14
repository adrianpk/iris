package iris

import (
	"context"
	"database/sql"
)

func (s *store) checkpointApproval(ctx context.Context, tx *sql.Tx, value bridgeRecord, params bridgeCheckpoint) error {
	if value.TurnID == nil || *value.TurnID != params.TurnID {
		return problem("worker_uncertain", "approval does not belong to the invoked turn")
	}

	if params.Action == "running" {
		if value.State != "approval_required" || (params.ErrorCode != "" && params.ErrorCode != "approval_denied") {
			return problem("worker_uncertain", "bridge has no pending approval")
		}

		_, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET state='invoking',pending_request_id=NULL,pending_method=NULL,error_code=CASE WHEN ?='approval_denied' THEN 'approval_denied' ELSE error_code END,updated_at=? WHERE id=?", params.ErrorCode, s.now().UnixMilli(), value.ID)

		return err
	}

	if value.State != "invoking" || !validText(params.PendingRequestID, 128) || !validText(params.PendingMethod, 128) {
		return problem("worker_uncertain", "invalid pending approval identity or state")
	}

	_, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET state='approval_required',pending_request_id=?,pending_method=?,updated_at=? WHERE id=?", params.PendingRequestID, params.PendingMethod, s.now().UnixMilli(), value.ID)

	return err
}
