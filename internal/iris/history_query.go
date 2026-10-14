package iris

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type historyCursor struct {
	Version int    `json:"schema_version"`
	Scope   string `json:"scope"`
	Created int64  `json:"created_at"`
	ID      string `json:"conversation_id"`
}

func (plan historyPlan) query() (string, []any) {
	clauses, args := plan.searchClauses()
	clauses = append(clauses, "1=1")

	if plan.projectID != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM participants p WHERE p.conversation_id=c.id AND p.project_id=?)")
		args = append(args, plan.projectID)
	}

	if plan.params.ConversationID != "" {
		clauses = append(clauses, "c.id=?")
		args = append(args, plan.params.ConversationID)
	}

	if plan.params.State == "live" {
		clauses = append(clauses, "c.state IN ('pending','active')")
	} else if plan.params.State != "" {
		clauses = append(clauses, "c.state=?")
		args = append(args, plan.params.State)
	}

	return "SELECT c.id FROM conversations c WHERE " + strings.Join(clauses, " AND "), args
}

func (plan historyPlan) cursorScope() (string, error) {
	query, args := plan.query()

	return jsonDigest(struct {
		Query string
		Args  []any
	}{query, args})
}

func (plan historyPlan) pageQuery() (string, []any, error) {
	query, args := plan.query()
	if plan.params.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(plan.params.Cursor)

		var cursor historyCursor

		scope, scopeErr := plan.cursorScope()
		if err != nil || scopeErr != nil || decodeJSON(raw, &cursor) != nil || cursor.Version != 1 || cursor.Scope != scope || !validID(cursor.ID) {
			return "", nil, problem("invalid_cursor", "cursor belongs to another history selection")
		}

		query += " AND (c.created_at,c.id)>(?,?)"

		args = append(args, cursor.Created, cursor.ID)
	}

	query += " ORDER BY c.created_at,c.id LIMIT ?"

	args = append(args, plan.params.Limit+1)

	return query, args, nil
}

func historyIDs(ctx context.Context, tx *sql.Tx, plan historyPlan) (ids []string, resultErr error) {
	query, args, err := plan.pageQuery()
	if err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, query, args...)
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

func (s *store) listConversations(ctx context.Context, params historyParams) (historyPage, error) {
	plan, err := s.planHistory(ctx, params)
	if err != nil {
		return historyPage{}, err
	}

	result := historyPage{SchemaVersion: 1, Conversations: []historyConversation{}}
	err = s.write(ctx, func(tx *sql.Tx) error {
		resolved, err := s.resolveHistory(ctx, tx, plan)
		if err != nil {
			return err
		}

		ids, err := historyIDs(ctx, tx, resolved)
		if err != nil {
			return err
		}

		return fillHistoryPage(ctx, tx, resolved, ids, &result)
	})

	return result, err
}

func fillHistoryPage(ctx context.Context, tx *sql.Tx, plan historyPlan, ids []string, result *historyPage) error {
	for index, id := range ids {
		if index == plan.params.Limit {
			result.Truncated = true

			break
		}

		value, err := readHistoryConversation(ctx, tx, id)
		if err != nil {
			return err
		}

		if plan.params.Text != "" {
			value.Matches, value.MatchesTruncated, err = readHistoryMatches(ctx, tx, id, plan.params.Text)
			if err != nil {
				return err
			}
		}

		result.Conversations = append(result.Conversations, value)
	}

	if !result.Truncated {
		return nil
	}

	scope, err := plan.cursorScope()
	if err != nil {
		return err
	}

	last := result.Conversations[len(result.Conversations)-1]
	raw, err := json.Marshal(historyCursor{Version: 1, Scope: scope, Created: last.CreatedAt.UnixMilli(), ID: last.ID})
	cursor := base64.RawURLEncoding.EncodeToString(raw)
	result.NextCursor = &cursor

	return err
}
