package iris

import (
	"context"
	"strings"
)

type statistics struct {
	SchemaVersion        int     `json:"schema_version"`
	ProjectID            *string `json:"project_id"`
	Ready                bool    `json:"ready"`
	StorageSchemaVersion int     `json:"storage_schema_version"`
	Projects             int64   `json:"projects"`
	Conversations        int64   `json:"conversations"`
	Messages             int64   `json:"messages"`
}

type statsParams struct {
	projectSelection
	AllProjects bool `json:"all_projects"`
}

func (s *store) statistics(ctx context.Context) (statistics, error) {
	result := statistics{SchemaVersion: 1, Ready: true}
	err := s.db.QueryRowContext(ctx, `SELECT
        (SELECT max(version) FROM schema_migrations),
        (SELECT count(*) FROM projects WHERE canonical_path IS NOT NULL),
        (SELECT count(*) FROM conversations),
        (SELECT count(*) FROM messages)`).Scan(&result.StorageSchemaVersion, &result.Projects, &result.Conversations, &result.Messages)

	return result, err
}

func (s *store) handle(ctx context.Context, req request) response {
	if req.Protocol == "" || req.Operation == "" {
		return failure("invalid_request", "protocol and operation are required", false)
	}

	if req.Protocol != protocolVersion {
		return failure("unsupported_protocol", "expected iris/v1", false)
	}

	if strings.HasPrefix(req.Operation, "projects.") {
		return s.handleProjects(ctx, req)
	}

	if req.Operation == "call" {
		return s.handleCall(ctx, req)
	}

	if req.Operation != "stats" {
		return failure("unknown_operation", "unsupported operation", false)
	}

	var params statsParams
	if err := decodeJSON(req.Params, &params); err != nil {
		return failure("invalid_request", "invalid operation parameters", false)
	}

	result, err := s.selectedStatistics(ctx, params)

	return operationResponse(ctx, result, err)
}
