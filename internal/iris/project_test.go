package iris

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func projectDirectory(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}

	return path
}

func registeredProject(t *testing.T, s *store, name string) project {
	t.Helper()

	value, err := s.registerProject(t.Context(), projectRegistration{Path: projectDirectory(t, name)})
	if err != nil {
		t.Fatal(err)
	}

	return value
}

// Registration derives or accepts an alias without creating repository-local state.
func TestProjectRegistration(t *testing.T) {
	cases := []struct{ name, directory, override, explicit, want string }{
		{"derived", "Project One", "", "", "project-one"},
		{"override", "source", "# Custom project\nname = \"alternate\"\n", "", "alternate"},
		{"explicit", "source", "name = \"alternate\"\n", "chosen", "chosen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := testStore(t)

			directory := projectDirectory(t, tc.directory)
			writeAliasOverride(t, directory, tc.override)

			value, err := s.registerProject(t.Context(), projectRegistration{Path: directory, Name: tc.explicit})
			if err != nil || value.Alias != tc.want || value.Path == nil || *value.Path != directory || len(value.ID) != 32 {
				t.Fatalf("project=%+v error=%v", value, err)
			}

			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}

			wantFiles := 0
			if tc.override != "" {
				wantFiles = 1
			}

			if len(entries) != wantFiles {
				t.Fatalf("unexpected generated files: %v", entries)
			}
		})
	}
}

// Override parsing accepts only its documented one-field grammar and valid aliases.
func TestProjectOverride(t *testing.T) {
	cases := []struct {
		name, value string
		valid       bool
	}{
		{"name", "name = \"alpha-beta\"", true},
		{"unknown", "other = \"alpha\"", false},
		{"duplicate", "name=\"alpha\"\nname=\"beta\"", false},
		{"empty", "", false},
		{"uppercase", "name=\"Alpha\"", false},
		{"edge hyphen", "name=\"-alpha\"", false},
		{"double hyphen", "name=\"alpha--beta\"", false},
		{"invalid utf8", string([]byte{255}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseProjectAlias([]byte(tc.value)); (err == nil) != tc.valid {
				t.Fatalf("parse error=%v", err)
			}
		})
	}
}

// Discovery chooses the nearest canonical ancestor; explicit selectors ignore cwd.
func TestProjectSelection(t *testing.T) {
	s, _ := testStore(t)
	child, nested, link := selectionFixture(t, s)

	for _, ref := range []projectSelection{{CWD: nested}, {CWD: link}, {Project: child.ID}, {Project: child.Alias}, {Project: *child.Path}} {
		selected, err := selectTestProject(t.Context(), s, ref, false)
		if err != nil || selected.ID != child.ID {
			t.Fatalf("reference=%+v selected=%+v error=%v", ref, selected, err)
		}
	}

	unregistered := projectDirectory(t, "separate")
	if _, err := selectTestProject(t.Context(), s, projectSelection{CWD: unregistered}, false); !hasCode(err, "project_not_found") {
		t.Fatalf("unregistered error=%v", err)
	}

	selected, err := selectTestProject(t.Context(), s, projectSelection{CWD: unregistered}, true)
	if err != nil || selected.Alias != "separate" {
		t.Fatalf("automatic project=%+v error=%v", selected, err)
	}
}

func selectTestProject(ctx context.Context, s *store, ref projectSelection, register bool) (project, error) {
	plan, err := s.planSelection(ctx, ref, register)
	if err != nil {
		return project{}, err
	}

	var selected project

	err = s.write(ctx, func(tx *sql.Tx) error { selected, err = s.selectProjectTx(ctx, tx, plan); return err })

	return selected, err
}

// Project pages preserve stable IDs, expose truncation and never drop later rows.
func TestProjectPages(t *testing.T) {
	s, _ := testStore(t)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		registeredProject(t, s, name)
	}

	first, err := s.listProjects(t.Context(), projectListParams{Limit: 2})
	if err != nil || !first.Truncated || first.NextCursor == nil || len(first.Projects) != 2 {
		t.Fatalf("first=%+v error=%v", first, err)
	}

	second, err := s.listProjects(t.Context(), projectListParams{Limit: 2, Cursor: *first.NextCursor})
	if err != nil || second.Truncated || len(second.Projects) != 1 || second.Projects[0].ID <= *first.NextCursor {
		t.Fatalf("second=%+v error=%v", second, err)
	}
}

func writeAliasOverride(t *testing.T, directory, value string) {
	t.Helper()

	if value == "" {
		return
	}

	if err := os.WriteFile(filepath.Join(directory, ".iris.toml"), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func selectionFixture(t *testing.T, s *store) (project, string, string) {
	t.Helper()

	parent := registeredProject(t, s, "parent")

	childDirectory := filepath.Join(*parent.Path, "child")
	if err := os.Mkdir(childDirectory, 0o700); err != nil {
		t.Fatal(err)
	}

	child, err := s.registerProject(t.Context(), projectRegistration{Path: childDirectory})
	if err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(childDirectory, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(nested, link); err != nil {
		t.Fatal(err)
	}

	return child, nested, link
}
