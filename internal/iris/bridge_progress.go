package iris

import (
	"context"
	"database/sql"
	"time"
)

func (s *store) bridgeActive(ctx context.Context, tx *sql.Tx, value bridgeRecord) (bool, error) {
	state, err := loadLifecycle(ctx, tx, value.ConversationID)
	if err != nil {
		return false, err
	}

	if err := requireJoinedParticipant(ctx, tx, state.id, value.ProjectID); err != nil {
		return false, err
	}

	if err := s.expireIfDue(ctx, tx, &state); err != nil {
		return false, err
	}

	return state.state == "active", nil
}

func (s *store) invokeBridge(ctx context.Context, tx *sql.Tx, value bridgeRecord, input string) error {
	if value.State != "waiting" && value.State != "acked" {
		return problem("worker_uncertain", "bridge is already invoking work")
	}

	if value.ThreadID == nil {
		return problem("worker_uncertain", "bridge has no bound thread")
	}

	if err := s.bridgeBudget(value); err != nil {
		return err
	}

	message, err := nextBridgeInput(ctx, tx, value, input)
	if err != nil {
		return err
	}

	if message.Kind != "message" {
		return problem("invalid_request", "informational input cannot invoke a model")
	}

	_, err = tx.ExecContext(ctx, "UPDATE bridge_bindings SET state='invoking',input_message_id=?,result_message_id=NULL,turn_id=NULL,turns_used=turns_used+1,updated_at=?,error_code=NULL WHERE id=?", input, s.now().UnixMilli(), value.ID)

	return err
}

func (s *store) bridgeBudget(value bridgeRecord) error {
	if value.TurnsUsed >= value.Options.MaxTurns || !s.now().Before(value.StartedAt.Add(time.Duration(value.Options.TotalSeconds)*time.Second)) {
		return problem("limit_exceeded", "bridge turn or lifetime budget exhausted")
	}

	if value.UsageTokens != nil && *value.UsageTokens >= value.Options.MaxTokens {
		return problem("limit_exceeded", "bridge reported-token budget exhausted")
	}

	return nil
}

func nextBridgeInput(ctx context.Context, tx *sql.Tx, value bridgeRecord, input string) (message, error) {
	position, err := loadConsumer(ctx, tx, consumerPosition{ConversationID: value.ConversationID, ProjectID: value.ProjectID, ConsumerID: value.ConsumerID})
	if err != nil {
		return message{}, err
	}

	page, err := readDeliveryPage(ctx, tx, position, position.AckedSequence, 1)
	if err != nil {
		return message{}, err
	}

	if len(page.Messages) != 1 || page.Messages[0].ID != input || page.Messages[0].Sequence > position.DeliveredSequence {
		return message{}, problem("ack_out_of_order", "bridge input must be the next delivered addressed message")
	}

	return page.Messages[0], nil
}

func (s *store) failBridge(ctx context.Context, tx *sql.Tx, value bridgeRecord, code string) error {
	state, ok := bridgeFailureState(code)
	if !ok {
		return problem("invalid_request", "unsupported bridge failure code")
	}

	if state == "stopped" && (value.State == "invoking" || value.State == "approval_required") {
		state, code = "uncertain", "worker_uncertain"
	}

	_, err := tx.ExecContext(ctx, "UPDATE bridge_bindings SET state=?,error_code=?,owner_token=NULL,lease_until=NULL,updated_at=? WHERE id=?", state, code, s.now().UnixMilli(), value.ID)

	return err
}

func bridgeFailureState(code string) (string, bool) {
	switch code {
	case "worker_uncertain":
		return "uncertain", true
	case "provider_capacity":
		return "capacity", true
	case "worker_interrupted":
		return "interrupted", true
	case "approval_required":
		return "approval_required", true
	case "worker_failed", "approval_denied":
		return "failed", true
	case "limit_exceeded", "conversation_inactive", "worker_stopped":
		return "stopped", true
	}

	return "", false
}
