//go:build integration

package iris

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The actual executable preserves committed SQLite data through clean and crashed restarts.
func TestProcessRecovery(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	assertProcessHelp(t, fixture)

	process := fixture.start(t)
	assertProcessCount(t, fixture, 0)
	assertSecondProcessRejected(t, fixture)
	stopProcess(t, process, syscall.SIGTERM)
	assertSocketRemoved(t, fixture)
	seedProcessStore(t, fixture)

	process = fixture.start(t)
	assertProcessCount(t, fixture, 1)
	stopProcess(t, process, syscall.SIGKILL)

	process = fixture.start(t)
	assertProcessCount(t, fixture, 1)
	stopProcess(t, process, syscall.SIGTERM)
	assertSocketRemoved(t, fixture)
	assertBrokenStartup(t, fixture)
}

func stopProcess(t *testing.T, process *ownedProcess, signal os.Signal) {
	t.Helper()

	if err := process.stop(signal); err != nil {
		t.Fatal(err)
	}

	if signal == syscall.SIGTERM && process.err != nil {
		t.Fatalf("unclean shutdown: %v %s", process.err, process.stderr.String())
	}

	if signal == syscall.SIGKILL && process.err == nil {
		t.Fatal("crash fixture exited successfully")
	}
}

func assertProcessCount(t *testing.T, fixture processFixture, want int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	stdout, stderr, err := fixture.command(ctx, fixture.projects[1], "stats", "--all-projects", "--json")
	if err != nil || len(stderr) != 0 {
		t.Fatalf("stats error=%v stderr=%s", err, stderr)
	}

	var reply response
	if err := decodeJSON(stdout, &reply); err != nil {
		t.Fatal(err)
	}

	var result statistics
	if err := decodeJSON(reply.Result, &result); err != nil {
		t.Fatal(err)
	}

	if !result.Ready || result.StorageSchemaVersion != storageSchemaVersion || result.Projects != want {
		t.Fatalf("statistics=%+v", result)
	}
}

func assertSocketRemoved(t *testing.T, fixture processFixture) {
	t.Helper()

	if _, err := os.Lstat(fixture.config.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains: %v", err)
	}

	if err := privateRegularFile(fixture.config.database); err != nil {
		t.Fatal(err)
	}
}

func seedProcessStore(t *testing.T, fixture processFixture) {
	t.Helper()

	s, err := openStore(t.Context(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := s.close(); err != nil {
			t.Error(err)
		}
	}()

	if err := s.write(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), "INSERT INTO projects VALUES (?,?,?,0,0)", strings.Repeat("a", 32), fixture.projects[0], "alpha")

		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func assertProcessHelp(t *testing.T, fixture processFixture) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	withoutRuntime := fixture
	withoutRuntime.env = append(append([]string(nil), fixture.env...), "XDG_RUNTIME_DIR=")

	stdout, stderr, err := withoutRuntime.command(ctx, fixture.projects[0], "--help")
	if err != nil || len(stderr) != 0 || !strings.Contains(string(stdout), "Usage: iris") {
		t.Fatalf("help error=%v stdout=%s stderr=%s", err, stdout, stderr)
	}

	stdout, stderr, err = fixture.command(ctx, fixture.projects[0], "unknown", "--json")

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || len(stdout) != 0 {
		t.Fatalf("usage error=%v stdout=%s", err, stdout)
	}

	var reply response
	if err := decodeJSON(stderr, &reply); err != nil || reply.Error == nil || reply.Error.Code != "invalid_request" {
		t.Fatalf("usage response=%s error=%v", stderr, err)
	}
}

func assertSecondProcessRejected(t *testing.T, fixture processFixture) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	_, _, err := fixture.command(ctx, fixture.projects[1], "serve")

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("second owner error=%v", err)
	}

	assertProcessCount(t, fixture, 0)
}

func assertBrokenStartup(t *testing.T, fixture processFixture) {
	t.Helper()

	s, err := openStore(t.Context(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}

	_, updateErr := s.db.ExecContext(t.Context(), "UPDATE schema_migrations SET checksum='invalid'")
	if err := errors.Join(updateErr, s.close()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	stdout, stderr, err := fixture.command(ctx, fixture.projects[0], "serve")

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || len(stdout) != 0 || !strings.Contains(string(stderr), "schema_mismatch") {
		t.Fatalf("incompatible startup error=%v stdout=%s", err, stdout)
	}

	assertSocketRemoved(t, fixture)
}
