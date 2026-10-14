package iris

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type transcript struct {
	SchemaVersion int                 `json:"schema_version"`
	Conversation  historyConversation `json:"conversation"`
	Participants  []participant       `json:"participants"`
	Messages      []message           `json:"messages"`
}

// Full text and semantic JSON survive multiple pages with no consumer or activity side effects.
func TestDumpCLIComplete(t *testing.T) {
	f := newDeliveryFixture(t)
	fillDumpHistory(t, f)

	cfg, _ := testService(t, f.store.handle)

	code, out, stderr := cliDelivery(t, cfg, *f.alpha.Path, "", "dump", "--format", "json", f.conversation.ID)
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("json=%d %s", code, stderr)
	}

	var value transcript
	if err := decodeJSON(out, &value); err != nil {
		t.Fatal(err)
	}

	if len(value.Messages) != 112 || value.SchemaVersion != 1 || len(value.Participants) != 2 {
		t.Fatalf("count=%d", len(value.Messages))
	}

	for i, message := range value.Messages {
		if message.Sequence != int64(i+1) {
			t.Fatal("unordered export")
		}
	}

	assertDumpText(t, f, cfg)
	assertDumpEnvelope(t, f, cfg)
	assertHistoryReadOnly(t, f)
	assertNoDumpSpools(t, cfg)
}

func assertDumpText(t *testing.T, f deliveryFixture, cfg config) {
	t.Helper()

	code, out, stderr := cliDelivery(t, cfg, t.TempDir(), "", "dump", "--all-projects", f.conversation.ID)
	if code != 0 || len(stderr) != 0 || !strings.Contains(string(out), "Participant:") || !strings.Contains(string(out), "[112]") {
		t.Fatalf("text=%d %s %s", code, out, stderr)
	}

	if strings.Index(string(out), "history line 000") >= strings.Index(string(out), "history line 109") {
		t.Fatal("text order changed")
	}
}

func assertDumpEnvelope(t *testing.T, f deliveryFixture, cfg config) {
	t.Helper()

	code, out, stderr := cliDelivery(t, cfg, *f.alpha.Path, "", "dump", "--json", f.conversation.ID)
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("envelope=%d %s", code, stderr)
	}

	var reply response
	if err := decodeJSON(out, &reply); err != nil || !reply.OK {
		t.Fatalf("envelope=%s %v", out, err)
	}

	var value transcript
	if err := decodeJSON(reply.Result, &value); err != nil || len(value.Messages) != 112 {
		t.Fatalf("transcript=%v", err)
	}
}

func assertNoDumpSpools(t *testing.T, cfg config) {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(cfg.runtimeRoot, "dump-*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("spools=%v error=%v", files, err)
	}
}

// A failure after the first successful page publishes no bytes and removes its private spool and lease.
func TestDumpCLIFailure(t *testing.T) {
	f := newDeliveryFixture(t)
	fillDumpHistory(t, f)

	var pages atomic.Int32

	cfg, _ := testService(t, func(ctx context.Context, req request) response {
		if req.Operation == "dump.page" && pages.Add(1) == 2 {
			return failure("export_expired", "service restarted", false)
		}

		return f.store.handle(ctx, req)
	})

	code, out, stderr := cliDelivery(t, cfg, *f.alpha.Path, "", "dump", "--json", f.conversation.ID)
	if code == 0 || len(out) != 0 || !strings.Contains(string(stderr), "export_expired") {
		t.Fatalf("failure=%d %s %s", code, out, stderr)
	}

	assertNoDumpSpools(t, cfg)

	if len(f.store.exports) != 0 {
		t.Fatal("failed export leaked lease")
	}
}

// Spooling enforces the byte ceiling before writing and propagates cancellation before publication.
func TestDumpSpoolBounds(t *testing.T) {
	var out bytes.Buffer

	spool := exportSpool{writer: &out, remaining: 3}
	if _, err := io.WriteString(&spool, "abc"); err != nil {
		t.Fatal(err)
	}

	if _, err := io.WriteString(&spool, "d"); !hasCode(err, "limit_exceeded") || out.String() != "abc" {
		t.Fatalf("bound=%q %v", out.String(), err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	reader := exportReader{ctx: ctx, reader: strings.NewReader("secret")}
	if n, err := reader.Read(make([]byte, 10)); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled=%d %v", n, err)
	}
}

// The spool is private during paging; cancellation releases only this export and keeps stdout empty.
func TestDumpCancellation(t *testing.T) {
	f := newDeliveryFixture(t)
	ctx, cancel := context.WithCancel(t.Context())

	var cfg config

	privacy := make(chan bool, 1)
	cfg, _ = testService(t, func(requestCtx context.Context, req request) response {
		if req.Operation == "dump.page" {
			privacy <- privateDumpSpool(cfg)

			cancel()
		}

		return f.store.handle(requestCtx, req)
	})
	snapshot := mustOpenDump(t, f)

	var out bytes.Buffer

	app := NewApp(&out, io.Discard)

	err := app.exportDump(ctx, cfg, snapshot, dumpCLIOptions{format: "json"})
	if err == nil || out.Len() != 0 {
		t.Fatalf("cancelled export=%v %d", err, out.Len())
	}

	if !<-privacy {
		t.Fatal("spool is not private")
	}

	assertNoDumpSpools(t, cfg)
}

func privateDumpSpool(cfg config) bool {
	files, err := filepath.Glob(filepath.Join(cfg.runtimeRoot, "dump-*.tmp"))
	if err != nil || len(files) != 1 {
		return false
	}

	info, err := os.Stat(files[0])

	return err == nil && info.Mode().Perm() == 0o600
}
