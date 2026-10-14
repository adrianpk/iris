package iris

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const projectColumns = "id,canonical_path,alias,created_at,registered_at"

type projectReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readAliasFile(file *os.File) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(file, 4097))

	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(data) > 4096 {
		return nil, problem("invalid_request", ".iris.toml must be readable and at most 4 KiB")
	}

	return data, nil
}

func exactProject(ctx context.Context, reader projectReader, selector string) (project, error) {
	var matches int

	row := reader.QueryRowContext(ctx, "SELECT "+projectColumns+",count(*) OVER () FROM projects WHERE id=? OR canonical_path=? OR alias=? LIMIT 1", selector, selector, selector)

	result, err := scanProject(row, &matches)
	if errors.Is(err, sql.ErrNoRows) {
		return project{}, problem("project_not_found", "project is not registered")
	}

	if err != nil {
		return project{}, err
	}

	if matches != 1 {
		return project{}, problem("project_conflict", "project selector is ambiguous; use its canonical path")
	}

	return result, nil
}

func nearestProject(ctx context.Context, reader projectReader, path string) (project, error) {
	for {
		result, err := scanProject(reader.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE canonical_path=?", path))
		if err == nil {
			return result, nil
		}

		if !errors.Is(err, sql.ErrNoRows) {
			return project{}, err
		}

		parent := filepath.Dir(path)
		if parent == path {
			return project{}, problem("project_not_found", "no registered project contains the current directory")
		}

		path = parent
	}
}

func (s *store) registerProject(ctx context.Context, params projectRegistration) (project, error) {
	registration, err := prepareRegistration(params)
	if err != nil {
		return project{}, err
	}

	var result project

	err = s.write(ctx, func(tx *sql.Tx) error { result, err = s.registerProjectTx(ctx, tx, registration); return err })

	return result, err
}

func (s *store) registerProjectTx(ctx context.Context, tx *sql.Tx, params projectRegistration) (project, error) {
	existing, err := scanProject(tx.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE canonical_path=?", params.Path))
	if err == nil {
		if existing.Alias != params.Name {
			return project{}, problem("project_conflict", "project is already registered; use projects rename")
		}

		return existing, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return project{}, err
	}

	reserved, err := exactProject(ctx, tx, params.Name)
	if err == nil {
		if reserved.Path != nil {
			return project{}, problem("alias_conflict", "alias belongs to another project; choose --name")
		}

		if _, err := tx.ExecContext(ctx, "UPDATE projects SET canonical_path=?,registered_at=? WHERE id=? AND canonical_path IS NULL", params.Path, s.now().UnixMilli(), reserved.ID); err != nil {
			return project{}, err
		}

		return exactProject(ctx, tx, reserved.ID)
	}

	if !hasCode(err, "project_not_found") {
		return project{}, err
	}

	id, err := newID()
	if err != nil {
		return project{}, err
	}

	now := s.now().UnixMilli()
	if _, err := tx.ExecContext(ctx, "INSERT INTO projects VALUES (?,?,?,?,?)", id, params.Path, params.Name, now, now); err != nil {
		return project{}, err
	}

	return exactProject(ctx, tx, id)
}

func (s *store) planSelection(ctx context.Context, ref projectSelection, register bool) (selectionPlan, error) {
	if ref.Project != "" {
		return explicitSelection(ref.Project)
	}

	if ref.CWD == "" {
		return selectionPlan{}, problem("project_not_found", "no project was selected")
	}

	path, err := canonicalDirectory(ref.CWD)
	if err != nil {
		return selectionPlan{}, err
	}

	plan := selectionPlan{directory: path}
	if _, err := nearestProject(ctx, s.db, path); err == nil || !hasCode(err, "project_not_found") {
		return plan, err
	}

	if register {
		registration, err := prepareRegistration(projectRegistration{Path: path})
		if err != nil {
			return selectionPlan{}, err
		}

		plan.registration = &registration
	}

	return plan, nil
}

func (s *store) selectProjectTx(ctx context.Context, tx *sql.Tx, plan selectionPlan) (project, error) {
	if plan.selector != "" {
		result, err := exactProject(ctx, tx, plan.selector)
		if err == nil && result.Path == nil {
			return project{}, problem("project_not_found", "project recipient has not registered a directory")
		}

		return result, err
	}

	result, err := nearestProject(ctx, tx, plan.directory)
	if err == nil || !hasCode(err, "project_not_found") {
		return result, err
	}

	if plan.registration != nil {
		return s.registerProjectTx(ctx, tx, *plan.registration)
	}

	return project{}, err
}

func explicitSelection(selector string) (selectionPlan, error) {
	if len(selector) > 4096 {
		return selectionPlan{}, problem("invalid_request", "project selector is too long")
	}

	if filepath.IsAbs(selector) {
		canonical, err := canonicalDirectory(selector)
		if err != nil {
			return selectionPlan{}, err
		}

		selector = canonical
	}

	return selectionPlan{selector: selector}, nil
}
