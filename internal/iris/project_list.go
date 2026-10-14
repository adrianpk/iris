package iris

import (
	"context"
	"errors"
)

type projectListParams struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type projectPage struct {
	SchemaVersion int       `json:"schema_version"`
	Projects      []project `json:"projects"`
	NextCursor    *string   `json:"next_cursor"`
	Truncated     bool      `json:"truncated"`
}

func (s *store) listProjects(ctx context.Context, params projectListParams) (result projectPage, resultErr error) {
	if params.Limit == 0 {
		params.Limit = 50
	}

	if params.Limit < 1 || params.Limit > 100 || len(params.Cursor) > 32 {
		return projectPage{}, problem("invalid_request", "invalid project page size or cursor")
	}

	rows, err := s.db.QueryContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE id>? ORDER BY id LIMIT ?", params.Cursor, params.Limit+1)
	if err != nil {
		return projectPage{}, err
	}

	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()

	result = projectPage{SchemaVersion: 1, Projects: []project{}}

	for rows.Next() {
		value, err := scanProject(rows)
		if err != nil {
			return projectPage{}, err
		}

		if len(result.Projects) == params.Limit {
			result.Truncated = true
			last := result.Projects[len(result.Projects)-1].ID
			result.NextCursor = &last

			break
		}

		result.Projects = append(result.Projects, value)
	}

	return result, rows.Err()
}
