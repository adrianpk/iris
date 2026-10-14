package iris

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

type projectRename struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
}

type projectRelocation struct {
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
}

func validID(id string) bool {
	if len(id) != 32 || id != strings.ToLower(id) {
		return false
	}

	_, err := hex.DecodeString(id)

	return err == nil
}

func projectByID(ctx context.Context, reader projectReader, id string) (project, error) {
	value, err := scanProject(reader.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return project{}, problem("project_not_found", "project ID is not registered")
	}

	return value, err
}

func mutableProject(ctx context.Context, tx *sql.Tx, id string) (project, error) {
	value, err := projectByID(ctx, tx, id)
	if err != nil {
		return project{}, err
	}

	if value.Path == nil {
		return project{}, problem("project_conflict", "unregistered recipients cannot be renamed or relocated")
	}

	return value, nil
}

func (s *store) renameProject(ctx context.Context, params projectRename) (project, error) {
	if !validID(params.ProjectID) || !validAlias(params.Name) {
		return project{}, problem("invalid_request", "rename requires a project ID and valid alias")
	}

	var result project

	err := s.write(ctx, func(tx *sql.Tx) error {
		value, err := mutableProject(ctx, tx, params.ProjectID)
		if err != nil {
			return err
		}

		var occupied int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM projects WHERE alias=? AND id<>?", params.Name, value.ID).Scan(&occupied); err != nil {
			return err
		}

		if occupied != 0 {
			return problem("alias_conflict", "alias belongs to another project")
		}

		if _, err := tx.ExecContext(ctx, "UPDATE projects SET alias=? WHERE id=?", params.Name, value.ID); err != nil {
			return err
		}

		result, err = projectByID(ctx, tx, value.ID)

		return err
	})

	return result, err
}

func (s *store) relocateProject(ctx context.Context, params projectRelocation) (project, error) {
	if !validID(params.ProjectID) {
		return project{}, problem("invalid_request", "relocation requires a project ID")
	}

	path, err := canonicalDirectory(params.Path)
	if err != nil {
		return project{}, err
	}

	var result project

	err = s.write(ctx, func(tx *sql.Tx) error {
		value, err := mutableProject(ctx, tx, params.ProjectID)
		if err != nil {
			return err
		}

		var occupied int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM projects WHERE canonical_path=? AND id<>?", path, value.ID).Scan(&occupied); err != nil {
			return err
		}

		if occupied != 0 {
			return problem("project_conflict", "directory belongs to another project ID")
		}

		if _, err := tx.ExecContext(ctx, "UPDATE projects SET canonical_path=? WHERE id=?", path, value.ID); err != nil {
			return err
		}

		result, err = projectByID(ctx, tx, value.ID)

		return err
	})

	return result, err
}
