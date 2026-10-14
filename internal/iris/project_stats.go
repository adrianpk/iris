package iris

import (
	"context"
	"database/sql"
)

func (s *store) selectedStatistics(ctx context.Context, params statsParams) (statistics, error) {
	if params.AllProjects {
		if params.Project != "" || params.CWD != "" {
			return statistics{}, problem("invalid_request", "all_projects and project cannot be combined")
		}

		return s.statistics(ctx)
	}

	plan, err := s.planSelection(ctx, params.projectSelection, false)
	if err != nil {
		return statistics{}, err
	}

	var result statistics

	err = s.write(ctx, func(tx *sql.Tx) error {
		selected, err := s.selectProjectTx(ctx, tx, plan)
		if err != nil {
			return err
		}

		result, err = s.readStatistics(ctx, tx, &selected.ID)

		return err
	})

	return result, err
}
