package iris

import (
	"context"
	"database/sql"
)

func (s *store) selectedStatistics(ctx context.Context, params statsParams) (statistics, error) {
	if params.AllProjects {
		if params.Project != "" {
			return statistics{}, problem("invalid_request", "all_projects and project cannot be combined")
		}

		return s.statistics(ctx)
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return statistics{}, err
	}

	result := statistics{SchemaVersion: 1, Ready: true, Projects: 1}
	err = s.write(ctx, func(tx *sql.Tx) error {
		selected, err := s.selectProjectTx(ctx, tx, plan)
		if err != nil {
			return err
		}

		result.ProjectID = &selected.ID

		return tx.QueryRowContext(ctx, `SELECT
            (SELECT max(version) FROM schema_migrations),
            (SELECT count(*) FROM participants WHERE project_id=?),
            (SELECT count(*) FROM messages m WHERE EXISTS
                (SELECT 1 FROM participants p WHERE p.conversation_id=m.conversation_id AND p.project_id=?))`, selected.ID, selected.ID).Scan(&result.StorageSchemaVersion, &result.Conversations, &result.Messages)
	})

	return result, err
}
