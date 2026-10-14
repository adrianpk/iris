package iris

import (
	"context"
	"database/sql"
	"strings"
)

type statistics struct {
	SchemaVersion         int                `json:"schema_version"`
	ProjectID             *string            `json:"project_id"`
	Ready                 bool               `json:"ready"`
	StorageSchemaVersion  int                `json:"storage_schema_version"`
	Projects              int64              `json:"projects"`
	Conversations         int64              `json:"conversations"`
	Messages              int64              `json:"messages"`
	States                conversationStates `json:"states"`
	RetainedConversations int64              `json:"retained_conversations"`
	PruneEligible         int64              `json:"prune_eligible"`
	BodyBytes             int64              `json:"body_bytes"`
	Lifecycle             lifecyclePolicy    `json:"lifecycle"`
	Storage               storageStatistics  `json:"storage"`
}

type statsParams struct {
	projectSelection
	AllProjects bool `json:"all_projects"`
}

func (s *store) statistics(ctx context.Context) (statistics, error) {
	var result statistics

	err := s.write(ctx, func(tx *sql.Tx) error {
		var err error

		result, err = s.readStatistics(ctx, tx, nil)

		return err
	})

	return result, err
}

func (s *store) handle(ctx context.Context, req request) response {
	if req.Protocol == "" || req.Operation == "" {
		return failure("invalid_request", "protocol and operation are required", false)
	}

	if req.Protocol != protocolVersion {
		return failure("unsupported_protocol", "expected iris/v1", false)
	}

	return s.dispatch(ctx, req)
}

func (s *store) dispatch(ctx context.Context, req request) response {
	if strings.HasPrefix(req.Operation, "dump.") {
		return s.handleHistory(ctx, req)
	}

	if strings.HasPrefix(req.Operation, "prune.") {
		return s.handlePrune(ctx, req)
	}

	if strings.HasPrefix(req.Operation, "bridge.") {
		return s.handleBridge(ctx, req)
	}

	if strings.HasPrefix(req.Operation, "projects.") {
		return s.handleProjects(ctx, req)
	}

	switch req.Operation {
	case "conversations", "search", "show":
		return s.handleHistory(ctx, req)
	case "receive", "ack":
		return s.handleDelivery(ctx, req)
	case "send":
		return s.handleSend(ctx, req)
	case "call", "join", "close":
		return s.handleHandshake(ctx, req)
	case "stats":
		return s.handleStats(ctx, req)
	}

	return failure("unknown_operation", "unsupported operation", false)
}

func (s *store) handleStats(ctx context.Context, req request) response {
	var params statsParams
	if err := decodeJSON(req.Params, &params); err != nil {
		return failure("invalid_request", "invalid operation parameters", false)
	}

	result, err := s.selectedStatistics(ctx, params)

	return operationResponse(ctx, result, err)
}
