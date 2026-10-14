//go:build integration

package iris

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// The published invitation and history chapters run unchanged against the actual binary.
func TestProcessUserGuide(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	fixture.start(t)

	script := guideShell(t, "02-connect-projects.md") + guideShell(t, "04-inspect-history.md")
	script += "\niris show --project \"$demo/alpha\" --json \"$conversation\" > \"$TMPDIR/guide-state.json\"\n"
	command := exec.CommandContext(t.Context(), "sh", "-eu", "-c", script)

	command.Env = append(fixture.env, "PATH="+filepath.Dir(fixture.binary)+":"+os.Getenv("PATH"), "TMPDIR="+filepath.Dir(fixture.projects[0]))

	var stdout, stderr bytes.Buffer

	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("guide commands: %v stderr=%s stdout=%s", err, stderr.String(), stdout.String())
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(fixture.projects[0]), "guide-state.json"))
	if err != nil {
		t.Fatal(err)
	}

	result := decodeProcessJSON[historyShow](t, data)
	if result.Conversation.State != "closed" || result.Conversation.Ordinal != 1 || len(result.Messages) != 3 {
		t.Fatalf("guide result=%+v", result)
	}
}

func guideShell(t *testing.T, name string) string {
	t.Helper()

	document, err := os.ReadFile(filepath.Join("../../docs/tutorials/user-guide", name))
	if err != nil {
		t.Fatal(err)
	}

	blocks := regexp.MustCompile("(?s)```sh\n(.*?)```").FindAllSubmatch(document, -1)
	if len(blocks) == 0 {
		t.Fatal("guide chapter contains no executable shell")
	}

	var script bytes.Buffer
	for _, block := range blocks {
		script.Write(block[1])
		script.WriteByte('\n')
	}

	return script.String()
}
