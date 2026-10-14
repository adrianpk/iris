package iris

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Process ownership is exercised without a model: cwd is explicit and shutdown reaps the spawned worker.
func TestCodexOwnedProcess(t *testing.T) {
	bin, project := t.TempDir(), t.TempDir()

	program := "#!/bin/sh\npwd > worker-cwd\nwhile IFS= read -r frame; do\n case \"$frame\" in\n *initialize*) printf '%s\\n' '{\"id\":1,\"result\":{}}' ;;\n esac\ndone\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	client, err := startCodex(t.Context(), project)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.initialize(t.Context()); err != nil {
		t.Fatal(err)
	}

	if err := client.close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(project, "worker-cwd"))
	if err != nil || strings.TrimSpace(string(data)) != project {
		t.Fatalf("cwd=%q, error=%v", data, err)
	}

	if err := syscall.Kill(client.cmd.Process.Pid, 0); err != syscall.ESRCH {
		t.Fatalf("owned process remains: %v", err)
	}
}
