package iris

import (
	"context"
	"database/sql"
	"encoding/json"
)

func (s *store) checkBridgeClaim(ctx context.Context, tx *sql.Tx, existing bridgeRecord) error {
	if existing.ownerToken != nil && existing.LeaseUntil != nil && s.now().Before(*existing.LeaseUntil) {
		return &serviceError{Code: "server_busy", Message: "bridge already has a live owner", Retryable: true}
	}

	switch existing.State {
	case "invoking", "approval_required", "uncertain":
		return problem("worker_uncertain", "prior invocation requires explicit recovery")
	case "failed", "capacity", "interrupted":
		return problem("worker_failed", "prior failed execution requires explicit recovery")
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM bridge_bindings WHERE owner_token IS NOT NULL AND lease_until>?", s.now().UnixMilli()).Scan(&count); err != nil {
		return err
	}

	if count >= 4 {
		return problem("limit_exceeded", "concurrent bridge limit reached")
	}

	return nil
}

func (s *store) insertBridge(ctx context.Context, tx *sql.Tx, params bridgeClaimParams, selected project) (bridgeOutcome, error) {
	id, err := newID()
	if err != nil {
		return bridgeOutcome{}, err
	}

	token, err := newID()
	if err != nil {
		return bridgeOutcome{}, err
	}

	consumer := "bridge:" + id
	if _, err := loadConsumer(ctx, tx, consumerPosition{ConversationID: params.ConversationID, ProjectID: selected.ID, ConsumerID: consumer}); err != nil {
		return bridgeOutcome{}, err
	}

	encoded, err := json.Marshal(params.Options)
	if err != nil {
		return bridgeOutcome{}, err
	}

	now := s.now().UnixMilli()

	_, err = tx.ExecContext(ctx, `INSERT INTO bridge_bindings
 (id,conversation_id,project_id,consumer_id,state,limits_json,owner_token,lease_until,updated_at,started_at)
 VALUES (?,?,?,?,'waiting',?,?,?,?,?)`, id, params.ConversationID, selected.ID, consumer, string(encoded), token, s.now().Add(bridgeLease).UnixMilli(), now, now)
	if err != nil {
		return bridgeOutcome{}, err
	}

	value, err := readBridge(ctx, tx, "b.id=?", id)

	return bridgeOutcome{record: value}, err
}

func (s *store) bridgeStatus(ctx context.Context, params conversationParams, stop bool) (bridgeBinding, error) {
	if !validID(params.ConversationID) {
		return bridgeBinding{}, problem("invalid_request", "bridge requires a conversation ID")
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return bridgeBinding{}, err
	}

	var result bridgeRecord

	err = s.write(ctx, func(tx *sql.Tx) error {
		selected, err := s.selectProjectTx(ctx, tx, plan)
		if err != nil {
			return err
		}

		if err := requireJoinedParticipant(ctx, tx, params.ConversationID, selected.ID); err != nil {
			return err
		}

		result, err = readBridge(ctx, tx, "b.conversation_id=? AND b.project_id=?", params.ConversationID, selected.ID)
		if err != nil {
			return err
		}

		result, err = s.requestBridgeStop(ctx, tx, result, stop)

		return err
	})
	if err == nil && stop {
		s.events.notify(params.ConversationID)
	}

	return result.bridgeBinding, err
}

func (s *store) renewBridge(ctx context.Context, owner bridgeOwner) (bridgeBinding, error) {
	var value bridgeRecord

	err := s.write(ctx, func(tx *sql.Tx) error {
		var err error

		value, err = s.ownedBridge(ctx, tx, owner)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET lease_until=?,updated_at=? WHERE id=?", s.now().Add(bridgeLease).UnixMilli(), s.now().UnixMilli(), value.ID); err != nil {
			return err
		}

		value, err = readBridge(ctx, tx, "b.id=?", value.ID)

		return err
	})

	return value.bridgeBinding, err
}

func (s *store) requestBridgeStop(ctx context.Context, tx *sql.Tx, value bridgeRecord, stop bool) (bridgeRecord, error) {
	if !stop || value.StopRequestedAt != nil {
		return value, nil
	}

	if _, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET stop_requested_at=?,updated_at=? WHERE id=?", s.now().UnixMilli(), s.now().UnixMilli(), value.ID); err != nil {
		return value, err
	}

	return readBridge(ctx, tx, "b.id=?", value.ID)
}
