package iris

import (
	"context"
	"database/sql"
	"errors"
)

type ackParams struct {
	conversationParams
	ConsumerID string `json:"consumer_id"`
	MessageID  string `json:"message_id"`
}

func prepareAck(params ackParams) (ackParams, error) {
	if params.ConsumerID == "" {
		params.ConsumerID = "manual"
	}

	if !validID(params.ConversationID) || !validID(params.MessageID) || !validText(params.ConsumerID, 128) {
		return params, problem("invalid_request", "ack requires conversation, message and valid consumer IDs")
	}

	return params, nil
}

func (s *store) acknowledge(ctx context.Context, params ackParams) (consumerPosition, error) {
	params, err := prepareAck(params)
	if err != nil {
		return consumerPosition{}, err
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return consumerPosition{}, err
	}

	var position consumerPosition

	err = s.write(ctx, func(tx *sql.Tx) error {
		selected, err := s.selectProjectTx(ctx, tx, plan)
		if err != nil {
			return err
		}

		state, err := s.loadLifecycle(ctx, tx, params.ConversationID)
		if err != nil {
			return err
		}

		if err := requireJoinedParticipant(ctx, tx, state.id, selected.ID); err != nil {
			return err
		}

		if err := s.expireIfDue(ctx, tx, &state); err != nil {
			return err
		}

		position, err = acknowledgeTx(ctx, tx, params, selected.ID)

		return err
	})
	if err == nil {
		s.notify(params.ConversationID)
	}

	return position, err
}

func acknowledgeTx(ctx context.Context, tx *sql.Tx, params ackParams, projectID string) (consumerPosition, error) {
	position, err := loadConsumer(ctx, tx, consumerPosition{ConversationID: params.ConversationID, ProjectID: projectID, ConsumerID: params.ConsumerID})
	if err != nil {
		return position, err
	}

	var sequence int64

	err = tx.QueryRowContext(ctx, `SELECT m.sequence FROM messages m JOIN message_recipients r ON r.message_id=m.id
 WHERE m.id=? AND m.conversation_id=? AND r.project_id=?`, params.MessageID, params.ConversationID, projectID).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return position, problem("not_found", "message is not addressed to this conversation/project")
	}

	if err != nil {
		return position, err
	}

	if sequence <= position.AckedSequence {
		return position, nil
	}

	var next sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT min(m.sequence) FROM messages m JOIN message_recipients r ON r.message_id=m.id
 WHERE m.conversation_id=? AND r.project_id=? AND m.sequence>?`, params.ConversationID, projectID, position.AckedSequence).Scan(&next); err != nil {
		return position, err
	}

	if !next.Valid || next.Int64 != sequence || sequence > position.DeliveredSequence {
		return position, problem("ack_out_of_order", "ack must name the next addressed message delivered to this consumer")
	}

	position.AckedSequence = sequence
	_, err = tx.ExecContext(ctx, "UPDATE consumer_cursors SET acked_sequence=? WHERE conversation_id=? AND project_id=? AND consumer_id=?", sequence, params.ConversationID, projectID, params.ConsumerID)

	return position, err
}
