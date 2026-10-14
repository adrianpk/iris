//go:build integration

package iris

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

type processHandshake struct {
	fixture  processFixture
	alpha    project
	beta     project
	first    conversation
	second   conversation
	callArgs []string
}

// Two actual project contexts exchange only a code; identity, content and isolation survive a crash.
func TestProcessHandshake(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	process := fixture.start(t)
	flow := establishProcessHandshake(t, fixture)
	flow.verifyHumanAndIsolation(t)
	flow.verifyRenameAndMove(t)
	flow.verifyClosure(t)
	flow.verifyContentInputs(t)
	flow.verifyProjectFiles(t)
	stopProcess(t, process, syscall.SIGKILL)
	flow.verifyStoredHandshake(t)

	process = flow.fixture.start(t)

	retry := processJSON[conversation](t, flow.fixture, flow.fixture.projects[0], flow.callArgs...)
	if retry.ID != flow.first.ID || retry.State != "closed" || retry.Label != flow.first.Label {
		t.Fatalf("restart retry=%+v", retry)
	}

	joined := processJSON[conversation](t, flow.fixture, flow.fixture.projects[1], "join", "--json", flow.second.InvitationCode)
	assertSameInitialMessage(t, flow.second, joined)
	stopProcess(t, process, syscall.SIGTERM)
}

func processJSON[T any](t *testing.T, fixture processFixture, path string, args ...string) T {
	t.Helper()

	stdout, stderr, err := fixture.command(t.Context(), path, args...)
	if err != nil || len(stderr) != 0 {
		t.Fatalf("command=%v error=%v stderr=%s", args, err, stderr)
	}

	return decodeProcessJSON[T](t, stdout)
}

func decodeProcessJSON[T any](t *testing.T, data []byte) T {
	t.Helper()

	jq := exec.CommandContext(t.Context(), "jq", "-e", `.protocol == "iris/v1" and .schema_version == 1 and .ok == true and .result.schema_version == 1`)

	jq.Stdin = bytes.NewReader(data)
	if output, err := jq.CombinedOutput(); err != nil {
		t.Fatalf("jq validation: %v %s", err, output)
	}

	var reply response
	if err := decodeJSON(data, &reply); err != nil {
		t.Fatal(err)
	}

	var result T
	if err := decodeJSON(reply.Result, &result); err != nil {
		t.Fatal(err)
	}

	return result
}

func establishProcessHandshake(t *testing.T, fixture processFixture) *processHandshake {
	t.Helper()

	alpha := processJSON[project](t, fixture, fixture.projects[0], "projects", "register", "--json")

	nested := filepath.Join(fixture.projects[0], "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}

	firstArgs := []string{"call", "--to", "beta", "--topic", "Storage protocol", "--message", "Which format is accepted?\nKeep this initial request.", "--idempotency-key", "first", "--json"}
	first := processJSON[conversation](t, fixture, nested, firstArgs...)
	second := processJSON[conversation](t, fixture, fixture.projects[0], "call", "--to", "beta", "--topic", "Independent question", "--message", "A separate initial request.", "--json")
	joined := processJSON[conversation](t, fixture, fixture.projects[1], "join", "--json", first.InvitationCode)
	assertSameInitialMessage(t, first, joined)

	if first.Ordinal != 1 || second.Ordinal != 2 || first.ID == second.ID {
		t.Fatalf("conversation isolation: first=%+v second=%+v", first, second)
	}

	projects := processJSON[projectPage](t, fixture, fixture.projects[0], "projects", "list", "--json")

	var beta project

	for _, value := range projects.Projects {
		if value.Alias == "beta" {
			beta = value
		}
	}

	if beta.Path == nil || *beta.Path != fixture.projects[1] {
		t.Fatalf("join did not register beta: %+v", beta)
	}

	return &processHandshake{fixture: fixture, alpha: alpha, beta: beta, first: first, second: second, callArgs: firstArgs}
}

func assertSameInitialMessage(t *testing.T, original, joined conversation) {
	t.Helper()

	if joined.ID != original.ID || joined.Topic != original.Topic || joined.InitialMessage.ID != original.InitialMessage.ID || joined.InitialMessage.Body != original.InitialMessage.Body {
		t.Fatalf("initial request changed: %+v", joined)
	}

	if joined.State != "active" || joined.InitialMessage.Sequence != 1 || joined.Label != original.Label {
		t.Fatalf("joined metadata=%+v", joined)
	}
}

func (flow *processHandshake) verifyHumanAndIsolation(t *testing.T) {
	t.Helper()

	stdout, stderr, err := flow.fixture.command(t.Context(), flow.fixture.projects[1], "join", flow.first.InvitationCode)
	if err != nil || len(stderr) != 0 {
		t.Fatalf("human join error=%v stderr=%s", err, stderr)
	}

	for _, text := range []string{flow.first.Label, flow.first.ID, flow.first.Topic, flow.first.InitialMessage.Body} {
		if !strings.Contains(string(stdout), text) {
			t.Fatalf("human output missing %q: %s", text, stdout)
		}
	}

	wrong := projectDirectory(t, "outsider")
	assertProcessError(t, flow.fixture, wrong, "recipient_mismatch", "join", "--json", flow.first.InvitationCode)
	assertProcessCount(t, flow.fixture, 2)

	joined := processJSON[conversation](t, flow.fixture, flow.fixture.projects[1], "join", "--json", flow.second.InvitationCode)
	assertSameInitialMessage(t, flow.second, joined)

	if strings.Contains(joined.InitialMessage.Body, flow.first.InitialMessage.Body) {
		t.Fatal("another conversation's content leaked")
	}
}

func assertProcessError(t *testing.T, fixture processFixture, path, code string, args ...string) {
	t.Helper()

	stdout, stderr, err := fixture.command(t.Context(), path, args...)
	assertProcessFailure(t, stdout, stderr, err, code)
}

func assertProcessFailure(t *testing.T, stdout, stderr []byte, err error, code string) {
	t.Helper()

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || len(stdout) != 0 {
		t.Fatalf("failure exit=%v stdout=%s", err, stdout)
	}

	var reply response
	if err := decodeJSON(stderr, &reply); err != nil || reply.Error == nil || reply.Error.Code != code {
		t.Fatalf("error envelope=%s decode=%v", stderr, err)
	}
}

func (flow *processHandshake) verifyRenameAndMove(t *testing.T) {
	t.Helper()

	renamed := processJSON[project](t, flow.fixture, flow.fixture.projects[1], "projects", "rename", "--name", "beta-renamed", "--json", flow.beta.ID)
	if renamed.ID != flow.beta.ID {
		t.Fatal("rename replaced stable identity")
	}

	moved := filepath.Join(filepath.Dir(flow.fixture.projects[1]), "moved-beta")
	if err := os.Rename(flow.fixture.projects[1], moved); err != nil {
		t.Fatal(err)
	}

	relocated := processJSON[project](t, flow.fixture, flow.fixture.projects[0], "projects", "relocate", "--path", moved, "--json", flow.beta.ID)
	if relocated.ID != flow.beta.ID || relocated.Path == nil || *relocated.Path != moved {
		t.Fatalf("relocated=%+v", relocated)
	}

	flow.fixture.projects[1] = moved
	joined := processJSON[conversation](t, flow.fixture, flow.fixture.projects[0], "join", "--project", flow.beta.ID, "--json", flow.first.InvitationCode)
	assertSameInitialMessage(t, flow.first, joined)

	reverse := processJSON[conversation](t, flow.fixture, moved, "call", "--to", flow.alpha.ID, "--topic", "Fresh reverse request", "--message", "No inherited context.", "--json")
	if reverse.Ordinal != 3 || !strings.Contains(reverse.Label, "beta-renamed") || reverse.InitialMessage.Sequence != 1 {
		t.Fatalf("reverse call=%+v", reverse)
	}
}

func (flow *processHandshake) verifyClosure(t *testing.T) {
	t.Helper()

	closed := processJSON[conversation](t, flow.fixture, flow.fixture.projects[0], "close", "--json", flow.first.ID)
	if closed.State != "closed" {
		t.Fatalf("closure=%+v", closed)
	}

	assertProcessError(t, flow.fixture, flow.fixture.projects[1], "conversation_inactive", "join", "--json", flow.first.InvitationCode)

	other := processJSON[conversation](t, flow.fixture, flow.fixture.projects[1], "join", "--json", flow.second.InvitationCode)
	assertSameInitialMessage(t, flow.second, other)
}

func (flow *processHandshake) verifyStoredHandshake(t *testing.T) {
	t.Helper()

	s, err := openStore(t.Context(), flow.fixture.config)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := s.close(); err != nil {
			t.Error(err)
		}
	}()

	assertHandshakeRows(t, s, flow.first.ID, "closed", 3, 0)
	assertHandshakeRows(t, s, flow.second.ID, "active", 2, 0)

	var consumers int
	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM consumer_cursors").Scan(&consumers); err != nil || consumers != 0 {
		t.Fatalf("implicit processing acknowledgment: count=%d error=%v", consumers, err)
	}
}

func (flow *processHandshake) verifyProjectFiles(t *testing.T) {
	t.Helper()

	for index, path := range flow.fixture.projects {
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}

		names := []string{}
		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		expected := []string{}
		if index == 0 {
			expected = append(expected, "nested")
		}

		if !slices.Equal(names, expected) {
			t.Fatalf("unexpected project files: %v want %v", names, expected)
		}
	}
}

func (flow *processHandshake) verifyContentInputs(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "initial.txt")

	body := "File input keeps\nall lines.\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	fromFile := processJSON[conversation](t, flow.fixture, flow.fixture.projects[0], "call", "--to", flow.beta.ID, "--topic", "File input", "--file", path, "--json")
	if fromFile.InitialMessage.Body != body {
		t.Fatalf("file content=%q", fromFile.InitialMessage.Body)
	}

	stdout, stderr, err := flow.fixture.commandInput(t.Context(), flow.fixture.projects[0], strings.NewReader("Stdin input\n"), "call", "--to", flow.beta.ID, "--topic", "Stdin input", "--json")
	if err != nil || len(stderr) != 0 {
		t.Fatalf("stdin error=%v stderr=%s", err, stderr)
	}

	fromStdin := decodeProcessJSON[conversation](t, stdout)
	if fromStdin.InitialMessage.Body != "Stdin input\n" {
		t.Fatalf("stdin content=%q", fromStdin.InitialMessage.Body)
	}

	stdout, stderr, err = flow.fixture.commandInput(t.Context(), flow.fixture.projects[0], strings.NewReader(strings.Repeat("\x00", 200000)), "call", "--to", flow.beta.ID, "--topic", "Encoded overflow", "--json")
	assertProcessFailure(t, stdout, stderr, err, "limit_exceeded")
}

// commandInput runs only the fixture-owned executable with a bounded caller-provided input source.
func (fixture processFixture) commandInput(ctx context.Context, path string, input io.Reader, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, fixture.binary, args...)
	command.Env = fixture.env
	command.Dir = path
	command.Stdin = input

	var stdout, stderr bytes.Buffer

	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()

	return stdout.Bytes(), stderr.Bytes(), err
}
