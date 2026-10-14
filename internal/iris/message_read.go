package iris

import (
	"context"
	"database/sql"
	"errors"
)

func readMessage(ctx context.Context, tx *sql.Tx, id string) (result message, resultErr error) {
	result = message{SchemaVersion: 1, Recipients: []string{}}

	var created int64

	err := tx.QueryRowContext(ctx, `SELECT m.id,m.conversation_id,m.sequence,m.sender_project_id,m.sender_alias,m.created_at,m.kind,m.body,c.topic,m.reply_to,m.correlation_id,m.idempotency_key
 FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE m.id=?`, id).
		Scan(&result.ID, &result.ConversationID, &result.Sequence, &result.SenderProjectID, &result.SenderAlias, &created, &result.Kind, &result.Body, &result.Topic, &result.ReplyTo, &result.CorrelationID, &result.IdempotencyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return message{}, problem("not_found", "message is not retained")
	}

	if err != nil {
		return message{}, err
	}

	result.CreatedAt = milliseconds(created)

	rows, err := tx.QueryContext(ctx, "SELECT project_id FROM message_recipients WHERE message_id=? ORDER BY project_id", id)
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
