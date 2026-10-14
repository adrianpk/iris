package iris

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A real child that stops reading stdin cannot defeat the protocol call deadline or owned cleanup.
func TestCodexWriteDeadline(t *testing.T) {
	bin := t.TempDir()

	program := "#!/bin/sh\nprintf '%s\\n' '{\"id\":1,\"result\":{}}'\nexec sleep 60\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	client, err := startCodex(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := client.close(); err != nil {
			t.Error(err)
		}
	}()

	if err := client.initialize(t.Context()); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		_, err := client.call(ctx, "turn/start", map[string]any{"input": strings.Repeat("x", 1<<20)}, client.rejectRequests)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked stdin write ignored cancellation")
	}
}
