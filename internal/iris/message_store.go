package iris

import (
	"context"
	"database/sql"
	"errors"
	"slices"
)

func resolveMessageRecipients(ctx context.Context, tx *sql.Tx, params sendParams, sender string) (sendParams, error) {
	participants, err := readParticipants(ctx, tx, params.ConversationID)
	if err != nil {
		return params, err
	}

	ids := []string{}
	for _, value := range participants {
		ids = append(ids, value.ProjectID)
	}

	if len(params.Recipients) == 0 {
		for _, id := range ids {
			if id != sender {
				params.Recipients = append(params.Recipients, id)
			}
		}
	}

	for _, id := range params.Recipients {
		if !slices.Contains(ids, id) {
			return params, problem("recipient_mismatch", "recipient is outside the conversation")
		}
	}

	return params, nil
}

func findSentMessage(ctx context.Context, tx *sql.Tx, value messageCreation) (message, bool, error) {
	if value.params.IdempotencyKey == nil {
		return message{}, false, nil
	}

	var id, digest string

	err := tx.QueryRowContext(ctx, "SELECT id,payload_digest FROM messages WHERE conversation_id=? AND sender_project_id=? AND idempotency_key=?", value.state.id, value.sender.ID, *value.params.IdempotencyKey).Scan(&id, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return message{}, false, nil
	}

	if err != nil {
		return message{}, false, err
	}

	if digest != value.digest {
		return message{}, true, problem("idempotency_conflict", "message key was used with different content")
	}

	original, err := readMessage(ctx, tx, id)

	return original, true, err
}

func (s *store) insertParticipantMessage(ctx context.Context, tx *sql.Tx, value messageCreation) (message, error) {
	if value.state.messageCount >= maxConversationMessages-1 || value.state.bodyBytes+int64(len(value.params.Body)) > maxConversationBodyBytes {
		return message{}, problem("limit_exceeded", "conversation message or body budget is exhausted")
	}

	if err := validateReply(ctx, tx, value.params); err != nil {
		return message{}, err
	}

	id, err := newID()
	if err != nil {
		return message{}, err
	}

	var alias string
	if err := tx.QueryRowContext(ctx, "SELECT alias_snapshot FROM participants WHERE conversation_id=? AND project_id=?", value.state.id, value.sender.ID).Scan(&alias); err != nil {
		return message{}, err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO messages VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, id, value.state.id, value.state.nextSequence, value.sender.ID, alias, s.now().UnixMilli(), value.params.Kind, value.params.Body, value.params.ReplyTo, value.params.CorrelationID, value.params.IdempotencyKey, value.digest)
	if err != nil {
		return message{}, err
	}

	for _, recipient := range value.params.Recipients {
		if _, err := tx.ExecContext(ctx, "INSERT INTO message_recipients VALUES (?,?,?)", value.state.id, id, recipient); err != nil {
			return message{}, err
		}
	}

	_, err = tx.ExecContext(ctx, `UPDATE conversations SET next_sequence=next_sequence+1,message_count=message_count+1,body_bytes=body_bytes+?,revision=revision+1,last_activity_at=? WHERE id=?`, len(value.params.Body), s.now().UnixMilli(), value.state.id)
	if err != nil {
		return message{}, err
	}

	return readMessage(ctx, tx, id)
}

func validateReply(ctx context.Context, tx *sql.Tx, params sendParams) error {
	if params.ReplyTo == nil {
		return nil
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM messages WHERE conversation_id=? AND id=?", params.ConversationID, *params.ReplyTo).Scan(&count); err != nil {
		return err
	}

	if count != 1 {
		return problem("not_found", "reply target is not in this conversation")
	}

	return nil
}
