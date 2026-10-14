package iris

import (
	"context"
	"encoding/json"
	"time"
)

const (
	maxPruneCandidates   = 1000
	maxPrunePreviews     = 64
	prunePreviewLifetime = 120 * time.Second
)

type pruneParams struct {
	projectSelection
	AllProjects    bool       `json:"all_projects"`
	ConversationID string     `json:"conversation_id,omitempty"`
	State          string     `json:"state,omitempty"`
	Before         *time.Time `json:"terminal_before,omitempty"`
}

type prunePlan struct {
	params    pruneParams
	selection *selectionPlan
}

type pruneCandidate struct {
	ConversationID string    `json:"conversation_id"`
	State          string    `json:"state"`
	Revision       int64     `json:"revision"`
	TerminalAt     time.Time `json:"terminal_at"`
	MessageCount   int64     `json:"message_count"`
	BodyBytes      int64     `json:"body_bytes"`
}

type prunePreview struct {
	SchemaVersion int              `json:"schema_version"`
	Token         string           `json:"confirmation_token"`
	ExpiresAt     time.Time        `json:"expires_at"`
	Cutoff        time.Time        `json:"eligibility_cutoff"`
	ProjectID     *string          `json:"project_id"`
	Candidates    []pruneCandidate `json:"candidates"`
	Messages      int64            `json:"message_count"`
	BodyBytes     int64            `json:"body_bytes"`
}

type pruneSnapshot struct {
	ID         string `json:"conversation_id"`
	Revision   int64  `json:"revision"`
	TerminalAt int64  `json:"terminal_at"`
}

type pruneConfirmation struct {
	Token string `json:"confirmation_token"`
}

type pruneResult struct {
	SchemaVersion   int      `json:"schema_version"`
	ConversationIDs []string `json:"conversation_ids"`
	Messages        int64    `json:"message_count"`
	BodyBytes       int64    `json:"body_bytes"`
}

func (s *store) planPrune(ctx context.Context, params pruneParams) (prunePlan, error) {
	plan := prunePlan{params: params}
	if params.ConversationID != "" && !validID(params.ConversationID) {
		return plan, problem("invalid_request", "invalid prune conversation ID")
	}

	if params.State != "" && params.State != "closed" && params.State != "expired" {
		return plan, problem("invalid_request", "prune state must be closed or expired")
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

func pruneSelectionDigest(values []pruneCandidate) (string, error) {
	snapshots := make([]pruneSnapshot, len(values))
	for index, value := range values {
		snapshots[index] = pruneSnapshot{ID: value.ConversationID, Revision: value.Revision, TerminalAt: value.TerminalAt.UnixMilli()}
	}

	return jsonDigest(snapshots)
}

func jsonDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return textDigest(string(encoded)), nil
}
