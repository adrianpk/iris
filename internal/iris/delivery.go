package iris

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
)

type receiveParams struct {
	conversationParams
	ConsumerID  string `json:"consumer_id"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	WaitSeconds int    `json:"wait_seconds,omitempty"`
}

type consumerPosition struct {
	SchemaVersion     int    `json:"schema_version"`
	ConversationID    string `json:"conversation_id"`
	ProjectID         string `json:"project_id"`
	ConsumerID        string `json:"consumer_id"`
	AckedSequence     int64  `json:"acked_sequence"`
	DeliveredSequence int64  `json:"delivered_sequence"`
}

type receivePage struct {
	consumerPosition
	Messages   []message `json:"messages"`
	NextCursor *string   `json:"next_cursor"`
	Truncated  bool      `json:"truncated"`
}

type deliveryCursor struct {
	SchemaVersion  int    `json:"schema_version"`
	ConversationID string `json:"conversation_id"`
	ProjectID      string `json:"project_id"`
	ConsumerID     string `json:"consumer_id"`
	AfterSequence  int64  `json:"after_sequence"`
}

type receiveOutcome struct {
	page    receivePage
	problem error
}

func prepareReceive(params receiveParams) (receiveParams, error) {
	if params.ConsumerID == "" {
		params.ConsumerID = "manual"
	}

	if params.Limit == 0 {
		params.Limit = 50
	}

	if !validID(params.ConversationID) || !validText(params.ConsumerID, 128) || params.Limit < 1 || params.Limit > 100 || len(params.Cursor) > 1024 || params.WaitSeconds < 0 || params.WaitSeconds > 60 {
		return params, problem("invalid_request", "invalid receive scope, consumer, page or wait limit")
	}

	return params, nil
}

func loadConsumer(ctx context.Context, tx *sql.Tx, position consumerPosition) (consumerPosition, error) {
	position.SchemaVersion = 1

	err := tx.QueryRowContext(ctx, "SELECT acked_sequence,delivered_sequence FROM consumer_cursors WHERE conversation_id=? AND project_id=? AND consumer_id=?", position.ConversationID, position.ProjectID, position.ConsumerID).Scan(&position.AckedSequence, &position.DeliveredSequence)
	if !errors.Is(err, sql.ErrNoRows) {
		return position, err
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM consumer_cursors WHERE conversation_id=? AND project_id=?", position.ConversationID, position.ProjectID).Scan(&count); err != nil {
		return position, err
	}

	if count >= 64 {
		return position, problem("limit_exceeded", "conversation/project consumer limit reached")
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO consumer_cursors VALUES (?,?,?,0,0)", position.ConversationID, position.ProjectID, position.ConsumerID)

	return position, err
}

func cursorAfter(encoded string, position consumerPosition, highest int64) (int64, error) {
	if encoded == "" {
		return position.AckedSequence, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)

	var cursor deliveryCursor

	if err != nil {
		return 0, problem("invalid_cursor", "invalid cursor encoding")
	}

	if err := decodeJSON(raw, &cursor); err != nil {
		return 0, problem("invalid_cursor", "invalid cursor fields")
	}

	if !cursor.matches(position) {
		return 0, problem("invalid_cursor", "cursor belongs to another scope or version")
	}

	if cursor.AfterSequence < 0 || cursor.AfterSequence > highest || cursor.AfterSequence > position.DeliveredSequence {
		return 0, problem("invalid_cursor", "cursor position is outside delivered history")
	}

	return cursor.AfterSequence, nil
}

func encodeCursor(position consumerPosition, after int64) (string, error) {
	raw, err := json.Marshal(deliveryCursor{SchemaVersion: 1, ConversationID: position.ConversationID, ProjectID: position.ProjectID, ConsumerID: position.ConsumerID, AfterSequence: after})

	return base64.RawURLEncoding.EncodeToString(raw), err
}

func (cursor deliveryCursor) matches(position consumerPosition) bool {
	return cursor.SchemaVersion == 1 && cursor.ConversationID == position.ConversationID && cursor.ProjectID == position.ProjectID && cursor.ConsumerID == position.ConsumerID
}
