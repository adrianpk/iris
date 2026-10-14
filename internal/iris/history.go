package iris

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type historyParams struct {
	projectSelection
	AllProjects    bool   `json:"all_projects"`
	ConversationID string `json:"conversation_id,omitempty"`
	State          string `json:"state,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
}

type historyConversation struct {
	SchemaVersion  int           `json:"schema_version"`
	ID             string        `json:"conversation_id"`
	Ordinal        int64         `json:"ordinal"`
	Label          string        `json:"label"`
	Topic          string        `json:"topic"`
	State          string        `json:"state"`
	CreatedAt      time.Time     `json:"created_at"`
	LastActivityAt time.Time     `json:"last_activity_at"`
	TerminalAt     *time.Time    `json:"terminal_at"`
	Participants   []participant `json:"participants"`
	MessageCount   int64         `json:"message_count"`
	BodyBytes      int64         `json:"body_bytes"`
}

type historyPage struct {
	SchemaVersion int                   `json:"schema_version"`
	Conversations []historyConversation `json:"conversations"`
	NextCursor    *string               `json:"next_cursor"`
	Truncated     bool                  `json:"truncated"`
}

type historyPlan struct {
	params    historyParams
	selection *selectionPlan
	projectID string
}

func (s *store) planHistory(ctx context.Context, params historyParams) (historyPlan, error) {
	if params.Limit == 0 {
		params.Limit = 50
	}

	plan := historyPlan{params: params}
	if params.Limit < 1 || params.Limit > 100 || len(params.Cursor) > 2048 {
		return plan, problem("invalid_request", "invalid history page bounds")
	}

	if err := validateHistorySelection(params); err != nil {
		return plan, err
	}

	if params.AllProjects {
		if params.Project != "" || params.CWD != "" {
			return plan, problem("invalid_request", "all-projects cannot be combined with project selection")
		}

		return plan, nil
	}

	selection, err := s.planSelection(ctx, params.projectSelection, false)
	plan.selection = &selection

	return plan, err
}

func (s *store) resolveHistory(ctx context.Context, tx *sql.Tx, plan historyPlan) (historyPlan, error) {
	if plan.selection == nil {
		return plan, nil
	}

	selected, err := s.selectProjectTx(ctx, tx, *plan.selection)
	plan.projectID = selected.ID

	return plan, err
}

func readHistoryConversation(ctx context.Context, tx *sql.Tx, id string) (historyConversation, error) {
	result := historyConversation{SchemaVersion: 1, ID: id}

	var (
		created, activity int64
		terminal          sql.NullInt64
	)

	err := tx.QueryRowContext(ctx, `SELECT pair_ordinal,topic,state,created_at,last_activity_at,terminal_at,message_count,body_bytes FROM conversations WHERE id=?`, id).Scan(&result.Ordinal, &result.Topic, &result.State, &created, &activity, &terminal, &result.MessageCount, &result.BodyBytes)
	if err != nil {
		return result, err
	}

	result.CreatedAt, result.LastActivityAt = milliseconds(created), milliseconds(activity)
	if terminal.Valid {
		at := milliseconds(terminal.Int64)
		result.TerminalAt = &at
	}

	result.Participants, err = readParticipants(ctx, tx, id)
	if err != nil {
		return result, err
	}

	if len(result.Participants) != 2 {
		return result, problem("internal_error", "conversation participants are incomplete")
	}

	result.Label = fmt.Sprintf("%s ↔ %s #%04d", result.Participants[0].Alias, result.Participants[1].Alias, result.Ordinal)

	return result, nil
}

func validateHistorySelection(params historyParams) error {
	if params.ConversationID != "" && !validID(params.ConversationID) {
		return problem("invalid_request", "invalid conversation ID")
	}

	switch params.State {
	case "", "pending", "active", "closed", "expired", "live":
	default:
		return problem("invalid_request", "invalid conversation state")
	}

	return nil
}
