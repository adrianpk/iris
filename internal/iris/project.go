package iris

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	aliasPattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	aliasSeparators = regexp.MustCompile(`[^a-z0-9]+`)
	aliasAssignment = regexp.MustCompile(`^name[\t ]*=[\t ]*"([^"]*)"[\t ]*$`)
)

type project struct {
	SchemaVersion int        `json:"schema_version"`
	ID            string     `json:"project_id"`
	Path          *string    `json:"path"`
	Alias         string     `json:"alias"`
	CreatedAt     time.Time  `json:"created_at"`
	RegisteredAt  *time.Time `json:"registered_at"`
}

type projectRegistration struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

type projectSelection struct {
	Project string `json:"project,omitempty"`
	CWD     string `json:"cwd,omitempty"`
}

type selectionPlan struct {
	selector     string
	directory    string
	registration *projectRegistration
}

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", bytes[:]), nil
}

func validAlias(alias string) bool {
	return len(alias) >= 1 && len(alias) <= 64 && aliasPattern.MatchString(alias)
}

func canonicalDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) || len(path) > 4096 {
		return "", problem("invalid_request", "project path must be an absolute directory")
	}

	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", problem("invalid_request", "project directory is unavailable")
	}

	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", problem("invalid_request", "project path must identify a directory")
	}

	return resolved, nil
}

func prepareRegistration(params projectRegistration) (projectRegistration, error) {
	path, err := canonicalDirectory(params.Path)
	if err != nil {
		return projectRegistration{}, err
	}

	name := params.Name
	if name == "" {
		name, err = readProjectAlias(path)
		if err != nil {
			return projectRegistration{}, err
		}
	}

	if name == "" {
		name = strings.Trim(aliasSeparators.ReplaceAllString(strings.ToLower(filepath.Base(path)), "-"), "-")
	}

	if !validAlias(name) {
		return projectRegistration{}, problem("invalid_request", "alias must contain 1-64 lowercase letters, digits and single hyphens")
	}

	return projectRegistration{Path: path, Name: name}, nil
}

func readProjectAlias(directory string) (string, error) {
	path := filepath.Join(directory, ".iris.toml")

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", nil
	}

	if err != nil || !info.Mode().IsRegular() {
		return "", problem("invalid_request", ".iris.toml must be a regular file")
	}

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", nil
	}

	if err != nil {
		return "", problem("invalid_request", "cannot read optional .iris.toml")
	}

	data, readErr := readAliasFile(file)
	if readErr != nil {
		return "", readErr
	}

	return parseProjectAlias(data)
}

func parseProjectAlias(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", problem("invalid_request", ".iris.toml must contain valid UTF-8")
	}

	name := ""

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		match := aliasAssignment.FindStringSubmatch(line)
		if match == nil || name != "" {
			return "", problem("invalid_request", ".iris.toml requires exactly one name assignment")
		}

		name = match[1]
		if !validAlias(name) {
			return "", problem("invalid_request", ".iris.toml name is invalid")
		}
	}

	if name == "" {
		return "", problem("invalid_request", ".iris.toml requires a name assignment")
	}

	return name, nil
}

type projectScanner interface{ Scan(...any) error }

func scanProject(row projectScanner, extra ...any) (project, error) {
	result := project{SchemaVersion: 1}

	var (
		path       sql.NullString
		created    int64
		registered sql.NullInt64
	)

	fields := []any{&result.ID, &path, &result.Alias, &created, &registered}
	if err := row.Scan(append(fields, extra...)...); err != nil {
		return project{}, err
	}

	result.CreatedAt = time.UnixMilli(created).UTC()
	if path.Valid {
		result.Path = &path.String
	}

	if registered.Valid {
		value := time.UnixMilli(registered.Int64).UTC()
		result.RegisteredAt = &value
	}

	return result, nil
}
