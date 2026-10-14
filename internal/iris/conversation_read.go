package iris

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func readConversation(ctx context.Context, tx *sql.Tx, id string) (conversation, error) {
	result := conversation{SchemaVersion: 1, ID: id}

	var (
		created, activity, expires int64
		terminal                   sql.NullInt64
	)

	err := tx.QueryRowContext(ctx, `SELECT c.pair_ordinal,c.topic,c.state,c.created_at,c.last_activity_at,c.terminal_at,i.code,i.expires_at
        FROM conversations c JOIN invitations i ON i.conversation_id=c.id WHERE c.id=?`, id).
		Scan(&result.Ordinal, &result.Topic, &result.State, &created, &activity, &terminal, &result.InvitationCode, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation{}, problem("not_found", "conversation is not retained")
	}

	if err != nil {
		return conversation{}, err
	}

	result.CreatedAt, result.LastActivityAt, result.InvitationExpiresAt = milliseconds(created), milliseconds(activity), milliseconds(expires)
	if terminal.Valid {
		at := milliseconds(terminal.Int64)
		result.TerminalAt = &at
	}

	result.Participants, err = readParticipants(ctx, tx, id)
	if err != nil {
		return conversation{}, err
	}

	if len(result.Participants) != 2 {
		return conversation{}, problem("internal_error", "conversation participants are incomplete")
	}

	result.Label = fmt.Sprintf("%s ↔ %s #%04d", result.Participants[0].Alias, result.Participants[1].Alias, result.Ordinal)
	result.InitialMessage, err = readInitialMessage(ctx, tx, id, result.Topic)

	return result, err
}

func readParticipants(ctx context.Context, tx *sql.Tx, id string) (result []participant, resultErr error) {
	rows, err := tx.QueryContext(ctx, "SELECT project_id,alias_snapshot,joined_at FROM participants WHERE conversation_id=? ORDER BY project_id", id)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	result = []participant{}

	for rows.Next() {
		var (
			value  participant
			joined sql.NullInt64
		)
		if err := rows.Scan(&value.ProjectID, &value.Alias, &joined); err != nil {
			return nil, err
		}

		if joined.Valid {
			at := milliseconds(joined.Int64)
			value.JoinedAt = &at
		}

		result = append(result, value)
	}

	return result, rows.Err()
}

func readInitialMessage(ctx context.Context, tx *sql.Tx, id, topic string) (result message, resultErr error) {
	result = message{SchemaVersion: 1, ConversationID: id, Topic: topic, Recipients: []string{}}

	var created int64

	err := tx.QueryRowContext(ctx, `SELECT id,sequence,sender_project_id,sender_alias,created_at,kind,body,reply_to,correlation_id,idempotency_key
        FROM messages WHERE conversation_id=? AND sequence=1`, id).
		Scan(&result.ID, &result.Sequence, &result.SenderProjectID, &result.SenderAlias, &created, &result.Kind, &result.Body, &result.ReplyTo, &result.CorrelationID, &result.IdempotencyKey)
	if err != nil {
		return message{}, err
	}

	result.CreatedAt = milliseconds(created)

	rows, err := tx.QueryContext(ctx, "SELECT project_id FROM message_recipients WHERE message_id=? ORDER BY project_id", result.ID)
	if err != nil {
		return message{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	for rows.Next() {
		var recipient string
		if err := rows.Scan(&recipient); err != nil {
			return message{}, err
		}

		result.Recipients = append(result.Recipients, recipient)
	}

	return result, rows.Err()
}
