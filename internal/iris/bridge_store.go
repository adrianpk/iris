package iris

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

func readBridge(ctx context.Context, reader projectReader, where string, args ...any) (bridgeRecord, error) {
	value := bridgeRecord{bridgeBinding: bridgeBinding{SchemaVersion: 1}}

	var (
		options          string
		lease, stopped   sql.NullInt64
		started, updated int64
	)

	err := reader.QueryRowContext(ctx, `SELECT b.id,b.conversation_id,b.project_id,p.canonical_path,b.consumer_id,b.thread_id,b.turn_id,b.input_message_id,b.result_message_id,b.state,b.limits_json,b.usage_tokens,b.owner_token,b.lease_until,b.updated_at,b.error_code,b.stop_requested_at,b.turns_used,b.started_at,b.pending_request_id,b.pending_method
 FROM bridge_bindings b JOIN projects p ON p.id=b.project_id WHERE `+where, args...).Scan(&value.ID, &value.ConversationID, &value.ProjectID, &value.ProjectPath, &value.ConsumerID, &value.ThreadID, &value.TurnID, &value.InputMessageID, &value.ResultMessageID, &value.State, &options, &value.UsageTokens, &value.ownerToken, &lease, &updated, &value.ErrorCode, &stopped, &value.TurnsUsed, &started, &value.PendingRequestID, &value.PendingMethod)
	if errors.Is(err, sql.ErrNoRows) {
		return value, problem("not_found", "bridge binding does not exist")
	}

	if err != nil {
		return value, err
	}

	if err := decodeJSON([]byte(options), &value.Options); err != nil {
		return value, err
	}

	value.StartedAt, value.UpdatedAt = milliseconds(started), milliseconds(updated)
	if lease.Valid {
		at := milliseconds(lease.Int64)
		value.LeaseUntil = &at
	}

	if stopped.Valid {
		at := milliseconds(stopped.Int64)
		value.StopRequestedAt = &at
	}

	if value.UsageTokens == nil {
		limitation := "provider usage has not been reported"
		value.UsageLimitation = &limitation
	}

	return value, nil
}

func (s *store) ownedBridge(ctx context.Context, tx *sql.Tx, owner bridgeOwner) (bridgeRecord, error) {
	if !validID(owner.BindingID) || !validID(owner.OwnerToken) {
		return bridgeRecord{}, problem("invalid_request", "invalid bridge owner identity")
	}

	value, err := readBridge(ctx, tx, "b.id=?", owner.BindingID)
	if err != nil {
		return value, err
	}

	if value.ownerToken == nil || *value.ownerToken != owner.OwnerToken || value.LeaseUntil == nil {
		return value, problem("worker_uncertain", "bridge ownership was lost")
	}

	if !s.now().Before(*value.LeaseUntil) {
		return value, problem("worker_uncertain", "bridge ownership lease has expired")
	}

	return value, nil
}

func (s *store) claimBridge(ctx context.Context, params bridgeClaimParams) (bridgeClaim, error) {
	if !validID(params.ConversationID) {
		return bridgeClaim{}, problem("invalid_request", "bridge requires a conversation ID")
	}

	if err := params.Options.validate(); err != nil {
		return bridgeClaim{}, err
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return bridgeClaim{}, err
	}

	var (
		result  bridgeClaim
		outcome bridgeOutcome
	)

	err = s.write(ctx, func(tx *sql.Tx) error {
		selected, err := s.selectProjectTx(ctx, tx, plan)
		if err != nil {
			return err
		}

		outcome, err = s.claimBridgeTx(ctx, tx, params, selected)
		if err != nil {
			return err
		}

		if outcome.problem == nil {
			result = bridgeClaim{SchemaVersion: 1, Binding: outcome.record.bridgeBinding, OwnerToken: *outcome.record.ownerToken}
			result.Conversation, err = readConversation(ctx, tx, params.ConversationID)
		}

		return err
	})
	if err != nil {
		return result, err
	}

	if outcome.problem != nil {
		s.notify(params.ConversationID)
	}

	return result, outcome.problem
}

func (s *store) claimBridgeTx(ctx context.Context, tx *sql.Tx, params bridgeClaimParams, selected project) (bridgeOutcome, error) {
	state, err := s.loadLifecycle(ctx, tx, params.ConversationID)
	if err != nil {
		return bridgeOutcome{}, err
	}

	if err := requireJoinedParticipant(ctx, tx, state.id, selected.ID); err != nil {
		return bridgeOutcome{}, err
	}

	if err := s.expireIfDue(ctx, tx, &state); err != nil {
		return bridgeOutcome{}, err
	}

	if state.state != "active" {
		return bridgeOutcome{problem: problem("conversation_inactive", "bridge requires an active conversation")}, nil
	}

	existing, err := readBridge(ctx, tx, "b.conversation_id=? AND b.project_id=?", state.id, selected.ID)
	if err != nil && !hasCode(err, "not_found") {
		return bridgeOutcome{}, err
	}

	if err := s.checkBridgeClaim(ctx, tx, existing); err != nil {
		return bridgeOutcome{}, err
	}

	if existing.ID == "" {
		return s.insertBridge(ctx, tx, params, selected)
	}

	return s.reclaimBridge(ctx, tx, params.Options, existing.ID)
}

func (s *store) reclaimBridge(ctx context.Context, tx *sql.Tx, options bridgeOptions, id string) (bridgeOutcome, error) {
	encoded, err := json.Marshal(options)
	if err != nil {
		return bridgeOutcome{}, err
	}

	token, err := newID()
	if err != nil {
		return bridgeOutcome{}, err
	}

	_, err = tx.ExecContext(ctx, `UPDATE bridge_bindings SET owner_token=?,lease_until=?,state='waiting',limits_json=?,updated_at=?,stop_requested_at=NULL,error_code=NULL,pending_request_id=NULL,pending_method=NULL WHERE id=?`, token, s.now().Add(bridgeLease).UnixMilli(), string(encoded), s.now().UnixMilli(), id)
	if err != nil {
		return bridgeOutcome{}, err
	}

	value, err := readBridge(ctx, tx, "b.id=?", id)

	return bridgeOutcome{record: value}, err
}
