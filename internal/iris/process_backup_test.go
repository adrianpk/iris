//go:build integration

package iris

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The documented stopped-WAL recipe refuses a live copy and restores real CLI history.
func TestProcessBackupRestore(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	process := fixture.start(t)
	args := []string{"call", "--to", "beta", "--topic", "Recovery snapshot", "--message", "Preserve this request.", "--idempotency-key", "snapshot", "--json"}
	original := processJSON[conversation](t, fixture, fixture.projects[0], args...)
	snapshot := filepath.Join(t.TempDir(), "snapshot.db")
	runBackupRecipe(t, fixture.config.database, snapshot, false)
	stopProcess(t, process, syscall.SIGTERM)
	runBackupRecipe(t, fixture.config.database, snapshot, true)

	if err := privateRegularFile(snapshot); err != nil {
		t.Fatal(err)
	}

	process = fixture.start(t)
	processJSON[conversation](t, fixture, fixture.projects[0], "call", "--to", "beta", "--topic", "After snapshot", "--message", "Not in backup", "--json")
	stopProcess(t, process, syscall.SIGTERM)

	if err := os.Rename(fixture.config.database, fixture.config.database+".preserved"); err != nil {
		t.Fatal(err)
	}

	command := exec.CommandContext(t.Context(), "install", "-m", "600", snapshot, fixture.config.database)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("restore: %v %s", err, output)
	}

	process = fixture.start(t)

	restored := processJSON[conversation](t, fixture, fixture.projects[0], args...)
	if restored.ID != original.ID || restored.InitialMessage.Body != original.InitialMessage.Body || restored.Ordinal != original.Ordinal {
		t.Fatalf("restored=%+v", restored)
	}

	stats := processJSON[statistics](t, fixture, fixture.projects[0], "stats", "--all-projects", "--json")
	if stats.Conversations != 1 {
		t.Fatalf("restore did not roll back later data: %+v", stats)
	}

	stopProcess(t, process, syscall.SIGTERM)
}

func runBackupRecipe(t *testing.T, source, snapshot string, success bool) {
	t.Helper()

	document, err := os.ReadFile("../../docs/how-to/backup-recovery.md")
	if err != nil {
		t.Fatal(err)
	}

	_, body, ok := strings.Cut(string(document), "<<'PY'\n")
	if !ok {
		t.Fatal("backup recipe missing")
	}

	script, _, ok := strings.Cut(body, "\nPY\n")
	if !ok {
		t.Fatal("backup recipe terminator missing")
	}

	command := exec.CommandContext(t.Context(), "python3", "-", source, snapshot)
	command.Stdin = strings.NewReader(script)

	output, err := command.CombinedOutput()
	if (err == nil) != success {
		t.Fatalf("backup recipe success=%v error=%v output=%s", success, err, output)
	}

	if !success {
		if _, err := os.Lstat(snapshot); !os.IsNotExist(err) {
			t.Fatalf("live copy created a snapshot: %v", err)
		}
	}
}
