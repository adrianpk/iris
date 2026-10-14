package iris

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
)

type historyShow struct {
	SchemaVersion int                 `json:"schema_version"`
	Conversation  historyConversation `json:"conversation"`
	Messages      []message           `json:"messages"`
	NextCursor    *string             `json:"next_cursor"`
	Truncated     bool                `json:"truncated"`
}

func (s *store) showConversation(ctx context.Context, params historyParams) (historyShow, error) {
	if params.ConversationID == "" && (len(params.Pair) != 2 || params.Ordinal <= 0) {
		return historyShow{}, problem("invalid_request", "show requires a conversation ID")
	}

	plan, err := s.planHistory(ctx, params)
	if err != nil {
		return historyShow{}, err
	}

	result := historyShow{SchemaVersion: 1}
	err = s.write(ctx, func(tx *sql.Tx) error {
		resolved, err := s.resolveHistory(ctx, tx, plan)
		if err != nil {
			return err
		}

		result, err = readHistoryShow(ctx, tx, resolved)

		return err
	})

	return result, err
}

func readHistoryShow(ctx context.Context, tx *sql.Tx, plan historyPlan) (historyShow, error) {
	cursor := plan.params.Cursor
	plan.params.Cursor = ""

	ids, err := historyIDs(ctx, tx, plan)
	if err != nil {
		return historyShow{}, err
	}

	if len(ids) != 1 {
		return historyShow{}, problem("not_found", "conversation is not visible in this scope")
	}

	value, err := readHistoryConversation(ctx, tx, ids[0])
	if err != nil {
		return historyShow{}, err
	}

	scope, err := plan.cursorScope()
	if err != nil {
		return historyShow{}, err
	}

	after, err := historySequence(cursor, scope, value.MessageCount)
	if err != nil {
		return historyShow{}, err
	}

	messages, last, err := readHistoryMessages(ctx, tx, value.ID, after, value.MessageCount, plan.params.Limit)

	result := historyShow{SchemaVersion: 1, Conversation: value, Messages: messages, Truncated: last < value.MessageCount}
	if err == nil && result.Truncated {
		result.NextCursor, err = encodeHistorySequence(scope, last)
	}

	return result, err
}

type historySequenceCursor struct {
	Version int    `json:"schema_version"`
	Scope   string `json:"scope"`
	After   int64  `json:"after_sequence"`
}

func historySequence(encoded, scope string, highest int64) (int64, error) {
	if encoded == "" {
		return 0, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)

	var cursor historySequenceCursor
	if err != nil || decodeJSON(raw, &cursor) != nil || cursor.Version != 1 || cursor.Scope != scope || cursor.After < 0 || cursor.After > highest {
		return 0, problem("invalid_cursor", "cursor belongs to another history selection")
	}

	return cursor.After, nil
}

func encodeHistorySequence(scope string, after int64) (*string, error) {
	raw, err := json.Marshal(historySequenceCursor{Version: 1, Scope: scope, After: after})
	cursor := base64.RawURLEncoding.EncodeToString(raw)

	return &cursor, err
}

func historyMessageIDs(ctx context.Context, tx *sql.Tx, id string, after, highest int64, limit int) (ids []string, resultErr error) {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM messages WHERE conversation_id=? AND sequence>? AND sequence<=? ORDER BY sequence LIMIT ?", id, after, highest, limit)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func readHistoryMessages(ctx context.Context, tx *sql.Tx, id string, after, highest int64, limit int) ([]message, int64, error) {
	ids, err := historyMessageIDs(ctx, tx, id, after, highest, limit)
	if err != nil {
		return nil, after, err
	}

	result := []message{}
	size := 0

	for _, id := range ids {
		value, err := readMessage(ctx, tx, id)
		if err != nil {
			return nil, after, err
		}

		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, after, err
		}

		if size+len(encoded) > maxResponseBytes-(1<<16) {
			break
		}

		size += len(encoded) + 1

		result = append(result, value)
		after = value.Sequence
	}

	return result, after, nil
}
