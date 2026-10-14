package iris

import (
	"context"
)

type statistics struct {
	SchemaVersion        int   `json:"schema_version"`
	Ready                bool  `json:"ready"`
	StorageSchemaVersion int   `json:"storage_schema_version"`
	Projects             int64 `json:"projects"`
	Conversations        int64 `json:"conversations"`
	Messages             int64 `json:"messages"`
}

type statsParams struct {
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

	if req.Operation != "stats" {
		return failure("unknown_operation", "unsupported operation", false)
	}

	var params statsParams
	if err := decodeJSON(req.Params, &params); err != nil {
		return failure("invalid_request", "invalid operation parameters", false)
	}

	if !params.AllProjects {
		return failure("project_not_found", "explicit all_projects scope is required for service statistics", false)
	}

	result, err := s.statistics(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return failure("cancelled", "request cancelled", false)
		}

		return failure("storage_unavailable", "cannot read service storage", false)
	}

	return success(result)
}
