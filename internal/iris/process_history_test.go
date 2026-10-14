//go:build integration

package iris

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

// Actual binaries discover, inspect and export complete retained history without changing active delivery context.
func TestProcessHistoryExport(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	process := fixture.start(t)

	flow := establishProcessHandshake(t, fixture)
	for i := range 101 {
		processJSON[message](t, fixture, fixture.projects[0], "send", "--message", fmt.Sprintf("history item %03d café", i), "--json", flow.first.ID)
	}

	closed := processJSON[conversation](t, fixture, fixture.projects[0], "close", "--json", flow.first.ID)
	assertProcessHistoryPages(t, flow)
	assertProcessHistoryDumps(t, flow)

	stats := processJSON[statistics](t, fixture, fixture.projects[0], "stats", "--json")
	if stats.Conversations != 2 || stats.Messages != 105 || stats.States.Closed != 1 || stats.States.Pending != 1 || stats.RetainedConversations != 1 {
		t.Fatalf("stats=%+v", stats)
	}

	assertProcessHistoryAdmin(t, flow)
	stopProcess(t, process, syscall.SIGTERM)
	assertProcessHistoryStorage(t, flow, closed, stats)

	process = fixture.start(t)

	dumped := processJSON[transcript](t, fixture, fixture.projects[0], "dump", "--json", flow.first.ID)
	if len(dumped.Messages) != 104 || dumped.Conversation.State != "closed" {
		t.Fatalf("restart export=%+v", dumped.Conversation)
	}

	stopProcess(t, process, syscall.SIGTERM)
}

func assertProcessHistoryPages(t *testing.T, flow *processHandshake) {
	t.Helper()

	fixture := flow.fixture

	page := processJSON[historyPage](t, fixture, fixture.projects[0], "conversations", "--limit", "1", "--json")
	if !page.Truncated || page.NextCursor == nil {
		t.Fatalf("list=%+v", page)
	}

	next := processJSON[historyPage](t, fixture, fixture.projects[0], "conversations", "--limit", "1", "--cursor", *page.NextCursor, "--json")
	if next.Truncated || page.Conversations[0].ID == next.Conversations[0].ID {
		t.Fatal("history keyset lost a conversation")
	}

	search := processJSON[historyPage](t, fixture, fixture.projects[1], "search", "--pair", "alpha,beta", "--ordinal", "1", "--text", "history item", "--json")
	if len(search.Conversations) != 1 || len(search.Conversations[0].Matches) != 5 || !search.Conversations[0].MatchesTruncated {
		t.Fatalf("search=%+v", search)
	}

	assertProcessHistoryMessages(t, flow)
}

func assertProcessHistoryDumps(t *testing.T, flow *processHandshake) {
	t.Helper()

	fixture := flow.fixture

	value := processJSON[transcript](t, fixture, fixture.projects[0], "dump", "--json", flow.first.ID)
	if len(value.Messages) != 104 || value.Messages[103].Kind != "system" || len(value.Participants) != 2 {
		t.Fatalf("export messages=%d", len(value.Messages))
	}

	for _, format := range []string{"text", "json"} {
		out, stderr, err := fixture.command(t.Context(), fixture.projects[1], "dump", "--format", format, flow.first.ID)
		if err != nil || len(stderr) != 0 {
			t.Fatalf("format=%s error=%v %s", format, err, stderr)
		}

		assertProcessTranscript(t, format, out)
	}

	assertNoDumpSpools(t, fixture.config)
}

func assertProcessTranscript(t *testing.T, format string, out []byte) {
	t.Helper()

	if format == "text" {
		if !strings.Contains(string(out), "State: closed") || !strings.Contains(string(out), "[104]") || !strings.Contains(string(out), "history item 100 café") {
			t.Fatalf("text transcript=%s", out)
		}

		return
	}

	command := exec.CommandContext(t.Context(), "jq", "-e", `.schema_version == 1 and (.messages|length) == 104 and (.participants|length) == 2 and .conversation.state == "closed"`)

	command.Stdin = bytes.NewReader(out)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("jq=%v %s", err, output)
	}
}

func assertProcessHistoryAdmin(t *testing.T, flow *processHandshake) {
	t.Helper()

	fixture := flow.fixture
	outside := t.TempDir()
	assertProcessError(t, fixture, outside, "project_not_found", "conversations", "--json")

	page := processJSON[historyPage](t, fixture, outside, "conversations", "--all-projects", "--state", "closed", "--json")
	if len(page.Conversations) != 1 {
		t.Fatalf("admin=%+v", page)
	}

	out, stderr, err := fixture.command(t.Context(), outside, "stats", "--all-projects")
	if err != nil || len(stderr) != 0 || !strings.Contains(string(out), "retained=1") || !strings.Contains(string(out), "Service storage:") {
		t.Fatalf("human stats=%s %s %v", out, stderr, err)
	}
}

func assertProcessHistoryStorage(t *testing.T, flow *processHandshake, closed conversation, stats statistics) {
	t.Helper()
	withLifecycleStore(t, flow.fixture, func(s *store) {
		var consumers, messages, bodyBytes, activity int64
		if err := s.db.QueryRowContext(t.Context(), "SELECT (SELECT count(*) FROM consumer_cursors),(SELECT count(*) FROM messages),(SELECT sum(length(CAST(body AS BLOB))) FROM messages),(SELECT last_activity_at FROM conversations WHERE id=?)", flow.first.ID).Scan(&consumers, &messages, &bodyBytes, &activity); err != nil {
			t.Fatal(err)
		}

		if consumers != 0 || messages != stats.Messages || bodyBytes != stats.BodyBytes || activity != closed.LastActivityAt.UnixMilli() {
			t.Fatalf("consumer=%d messages=%d bytes=%d activity=%d", consumers, messages, bodyBytes, activity)
		}
	})
}

func assertProcessHistoryMessages(t *testing.T, flow *processHandshake) {
	t.Helper()

	fixture := flow.fixture

	shown := processJSON[historyShow](t, fixture, fixture.projects[1], "show", "--limit", "1", "--json", flow.first.ID)
	if !shown.Truncated || shown.Messages[0].Sequence != 1 {
		t.Fatalf("show=%+v", shown)
	}

	second := processJSON[historyShow](t, fixture, fixture.projects[1], "show", "--limit", "1", "--cursor", *shown.NextCursor, "--json", flow.first.ID)
	if second.Messages[0].Sequence != 2 {
		t.Fatal("message pagination failed")
	}
}
