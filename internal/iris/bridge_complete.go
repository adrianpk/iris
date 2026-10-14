package iris

import (
	"context"
	"database/sql"
)

type bridgeCompletion struct {
	bridgeOwner
	InputMessageID string       `json:"input_message_id"`
	TurnID         string       `json:"turn_id"`
	Answer         bridgeAnswer `json:"answer"`
	UsageTokens    *int64       `json:"usage_tokens"`
}

func (params bridgeCompletion) validate() error {
	if !validID(params.InputMessageID) || (params.UsageTokens != nil && *params.UsageTokens < 0) {
		return problem("invalid_request", "invalid bridge completion identity or usage")
	}

	if params.Answer.Action == "ack" && params.Answer.Body == "" {
		return nil
	}

	if params.Answer.Action != "reply" || !validText(params.Answer.Body, maxMessageBodyBytes) {
		return problem("invalid_request", "invalid bridge final answer")
	}

	return nil
}

func (s *store) completeBridge(ctx context.Context, params bridgeCompletion) (bridgeBinding, error) {
	if err := params.validate(); err != nil {
		return bridgeBinding{}, err
	}

	var (
		result  bridgeRecord
		outcome error
	)

	err := s.write(ctx, func(tx *sql.Tx) error {
		value, err := s.ownedBridge(ctx, tx, params.bridgeOwner)
		if err != nil {
			return err
		}

		active, err := s.bridgeActive(ctx, tx, value)
		if err != nil {
			return err
		}

		outcome, err = s.completeBridgeTx(ctx, tx, value, params, active)
		if err != nil {
			return err
		}

		result, err = readBridge(ctx, tx, "b.id=?", value.ID)

		return err
	})
	if err != nil {
		return bridgeBinding{}, err
	}

	s.notify(result.ConversationID)

	return result.bridgeBinding, outcome
}

func (s *store) completeBridgeTx(ctx context.Context, tx *sql.Tx, value bridgeRecord, params bridgeCompletion, active bool) (error, error) {
	if isBridgeCompletionRetry(value, params) {
		return nil, checkBridgeRetry(ctx, tx, value, params)
	}

	if err := validateRecordedCompletion(value, params); err != nil {
		return nil, err
	}

	input, err := nextBridgeInput(ctx, tx, value, params.InputMessageID)
	if err != nil {
		return nil, err
	}

	if input.Kind == "message" && (!active || value.StopRequestedAt != nil) {
		return problem("conversation_inactive", "bridge cannot publish or acknowledge pending work after shutdown"), nil
	}

	if err := validateBridgeCompletion(value, params, input); err != nil {
		return nil, err
	}

	resultID, err := s.publishBridgeAnswer(ctx, tx, value, params, input)
	if err != nil {
		return nil, err
	}

	ack := ackParams{conversationParams: conversationParams{ConversationID: value.ConversationID}, ConsumerID: value.ConsumerID, MessageID: input.ID}
	if _, err := acknowledgeTx(ctx, tx, ack, value.ProjectID); err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, "UPDATE bridge_bindings SET state='acked',pending_result_json=NULL,input_message_id=?,result_message_id=?,turn_id=NULLIF(?,''),usage_tokens=CASE WHEN ? THEN ? ELSE usage_tokens END,usage_floor=max(usage_floor,COALESCE(?,0)),pending_request_id=NULL,pending_method=NULL,error_code=NULL,updated_at=? WHERE id=?", input.ID, resultID, params.TurnID, input.Kind == "message", params.UsageTokens, params.UsageTokens, s.now().UnixMilli(), value.ID)

	return nil, err
}

func validateBridgeCompletion(value bridgeRecord, params bridgeCompletion, input message) error {
	if input.Kind != "message" {
		return validateInformationalCompletion(value, params)
	}

	if err := checkBridgeAuthority(value.bridgeBinding); err != nil {
		return err
	}

	if (value.State != "invoking" && value.State != "result_recorded") || value.TurnID == nil || *value.TurnID != params.TurnID || value.InputMessageID == nil || *value.InputMessageID != input.ID {
		return problem("worker_uncertain", "completion does not match the owned invocation")
	}

	return nil
}

func checkBridgeRetry(ctx context.Context, tx *sql.Tx, value bridgeRecord, params bridgeCompletion) error {
	if !sameBridgeTurn(value, params.TurnID) {
		return problem("idempotency_conflict", "bridge completion has a different turn identity")
	}

	if value.ResultMessageID == nil && params.Answer.Action == "ack" {
		return nil
	}

	if value.ResultMessageID != nil && params.Answer.Action == "reply" {
		original, err := readMessage(ctx, tx, *value.ResultMessageID)
		if err != nil {
			return err
		}

		if original.Body == params.Answer.Body {
			return nil
		}
	}

	return problem("idempotency_conflict", "bridge completion was already recorded differently")
}

func (s *store) publishBridgeAnswer(ctx context.Context, tx *sql.Tx, value bridgeRecord, params bridgeCompletion, input message) (*string, error) {
	if params.Answer.Action == "ack" {
		return nil, nil
	}

	key := "bridge:" + value.ID + ":" + input.ID

	prepared, err := prepareSend(sendParams{conversationParams: conversationParams{ConversationID: value.ConversationID}, Body: params.Answer.Body, ReplyTo: &input.ID, IdempotencyKey: &key, Recipients: []string{*input.SenderProjectID}})
	if err != nil {
		return nil, err
	}

	outcome, err := s.sendTx(ctx, tx, prepared, project{ID: value.ProjectID, Path: &value.ProjectPath})
	if err != nil {
		return nil, err
	}

	if outcome.problem != nil {
		return nil, outcome.problem
	}

	return &outcome.value.ID, nil
}

func isBridgeCompletionRetry(value bridgeRecord, params bridgeCompletion) bool {
	return value.State == "acked" && value.InputMessageID != nil && *value.InputMessageID == params.InputMessageID
}

func validateInformationalCompletion(value bridgeRecord, params bridgeCompletion) error {
	if params.Answer.Action != "ack" || params.TurnID != "" || (value.State != "waiting" && value.State != "acked") {
		return problem("worker_uncertain", "informational input requires acknowledgment without invocation")
	}

	return nil
}

func sameBridgeTurn(value bridgeRecord, turn string) bool {
	if value.TurnID == nil {
		return turn == ""
	}

	return *value.TurnID == turn
}

func validateRecordedCompletion(value bridgeRecord, params bridgeCompletion) error {
	if value.State == "result_recorded" {
		return matchRecordedResult(value, params)
	}

	return nil
}
