//go:build integration

package iris

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func configureFixtureLifecycle(t *testing.T, fixture processFixture, policy lifecyclePolicy) {
	t.Helper()

	settings := defaultSettings()
	settings.Lifecycle = policy

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(fixture.config.configPath), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(fixture.config.configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The actual daemon expires pending/active state and wakes a waiting client without any agent or bridge.
func TestProcessAutonomousExpiration(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	configureFixtureLifecycle(t, fixture, lifecyclePolicy{InvitationSeconds: 2, InactivitySeconds: 3, RetentionSeconds: 3600})

	process := fixture.start(t)
	flow := establishProcessHandshake(t, fixture)
	page := flow.receive(t, 1, "expiration-wait")
	flow.acknowledgePage(t, 1, "expiration-wait", page)

	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
	defer cancel()

	stdout, stderr, err := fixture.command(ctx, fixture.projects[1], "receive", "--consumer", "expiration-wait", "--wait", "30", "--json", flow.first.ID)
	assertLifecycleFailure(t, stdout, stderr, err, "conversation_inactive")
	stopProcess(t, process, syscall.SIGTERM)
	withLifecycleStore(t, fixture, func(s *store) {
		assertStoredTerminal(t, s, flow.first.ID, 3)
		assertStoredTerminal(t, s, flow.second.ID, 2)
	})
}

func assertLifecycleFailure(t *testing.T, stdout, stderr []byte, err error, code string) {
	t.Helper()

	if err == nil || len(stdout) != 0 {
		t.Fatalf("expected %s failure; stdout=%s, error=%v", code, stdout, err)
	}

	var reply response
	if err := decodeJSON(stderr, &reply); err != nil || reply.Error == nil || reply.Error.Code != code {
		t.Fatalf("error response=%s, error=%v", stderr, err)
	}
}

func withLifecycleStore(t *testing.T, fixture processFixture, inspect func(*store)) {
	t.Helper()

	cfg := fixture.config

	if err := cfg.loadSettings(); err != nil {
		t.Fatal(err)
	}

	s, err := openStore(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.close(); err != nil {
			t.Error(err)
		}
	}()

	inspect(s)
}

func assertStoredTerminal(t *testing.T, s *store, id string, messages int) time.Time {
	t.Helper()

	var terminal time.Time

	err := s.write(t.Context(), func(tx *sql.Tx) error {
		value, err := readConversation(t.Context(), tx, id)
		if err != nil {
			return err
		}

		if value.State != "expired" || value.TerminalAt == nil {
			t.Fatalf("terminal metadata=%+v", value)
		}

		terminal = *value.TerminalAt

		page, err := readDeliveryPage(t.Context(), tx, consumerPosition{ConversationID: id, ProjectID: value.Participants[1].ProjectID}, 0, 100)
		if err != nil {
			return err
		}

		var count int
		if err := tx.QueryRowContext(t.Context(), "SELECT count(*) FROM messages WHERE conversation_id=?", id).Scan(&count); err != nil {
			return err
		}

		if count != messages || len(page.Messages) == 0 || value.InitialMessage.ConversationID != id {
			t.Fatalf("retained history: count=%d page=%+v", count, page)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return terminal
}

type lifecycleSeed struct {
	alpha, beta     project
	active, pending conversation
}

func seedOverdueLifecycle(t *testing.T, fixture processFixture) lifecycleSeed {
	t.Helper()

	var seed lifecycleSeed

	withLifecycleStore(t, fixture, func(s *store) {
		old := time.Now().Add(-time.Hour)
		s.now = func() time.Time { return old }

		var err error

		seed.alpha, err = s.registerProject(t.Context(), projectRegistration{Path: fixture.projects[0], Name: "alpha"})
		if err != nil {
			t.Fatal(err)
		}

		seed.beta, err = s.registerProject(t.Context(), projectRegistration{Path: fixture.projects[1], Name: "beta"})
		if err != nil {
			t.Fatal(err)
		}

		seed.active = mustCall(t, s, callFixture(t, s, seed.alpha, seed.beta.ID, "old active"))
		seed.active = mustJoin(t, s, joinParams{projectSelection: projectSelection{Project: seed.beta.ID}, Code: seed.active.InvitationCode})
		seed.pending = mustCall(t, s, callFixture(t, s, seed.alpha, seed.beta.ID, "old pending"))
	})

	return seed
}

// Startup repairs overdue history before readiness; policy reload, explicit pruning and restart preserve fresh context.
func TestProcessLifecycleRecovery(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	policy := lifecyclePolicy{InvitationSeconds: 60, InactivitySeconds: 60, RetentionSeconds: 3600}
	configureFixtureLifecycle(t, fixture, policy)

	seed := seedOverdueLifecycle(t, fixture)
	started := time.Now().Add(-time.Millisecond)
	process := fixture.start(t)
	assertPruneHumanPreview(t, fixture)

	preview := processJSON[prunePreview](t, fixture, fixture.projects[0], "prune", "--all-projects", "--json")
	if len(preview.Candidates) != 0 {
		t.Fatal("newly expired history did not receive its retention interval")
	}

	stopProcess(t, process, syscall.SIGTERM)

	var terminal time.Time

	withLifecycleStore(t, fixture, func(s *store) {
		terminal = assertStoredTerminal(t, s, seed.active.ID, 3)
		assertStoredTerminal(t, s, seed.pending.ID, 2)

		if terminal.Before(started) {
			t.Fatal("startup did not reconcile before readiness")
		}
	})

	policy.RetentionSeconds = 1
	configureFixtureLifecycle(t, fixture, policy)
	waitLifecycleInterval(t, time.Second+100*time.Millisecond)

	process = fixture.start(t)
	stdout, stderr, err := fixture.command(t.Context(), fixture.projects[0], "prune", "--confirm", preview.Token, "--json")
	assertLifecycleFailure(t, stdout, stderr, err, "preview_stale")

	preview = processJSON[prunePreview](t, fixture, fixture.projects[0], "prune", "--dry-run", "--json")
	if len(preview.Candidates) != 2 {
		t.Fatalf("retention selection=%+v", preview)
	}

	result := processJSON[pruneResult](t, fixture, t.TempDir(), "prune", "--confirm", preview.Token, "--json")
	if len(result.ConversationIDs) != 2 || result.Messages != 5 {
		t.Fatalf("deletion=%+v", result)
	}

	fresh := assertProcessFreshConversation(t, fixture, seed)
	stopProcess(t, process, syscall.SIGTERM)
	withLifecycleStore(t, fixture, func(s *store) { assertFreshStoredContext(t, s, fresh) })
}

func assertProcessFreshConversation(t *testing.T, fixture processFixture, seed lifecycleSeed) conversation {
	t.Helper()

	alpha := processJSON[project](t, fixture, fixture.projects[0], "projects", "register", "--json")
	if alpha.ID != seed.alpha.ID {
		t.Fatal("pruning replaced project identity")
	}

	fresh := processJSON[conversation](t, fixture, fixture.projects[0], "call", "--to", "beta", "--topic", "Fresh after pruning", "--message", "Only this new request belongs to this conversation.", "--json")

	fresh = processJSON[conversation](t, fixture, fixture.projects[1], "join", "--json", fresh.InvitationCode)
	if fresh.ID == seed.active.ID || fresh.ID == seed.pending.ID || fresh.Ordinal != 3 || fresh.InvitationCode == seed.active.InvitationCode || fresh.InvitationCode == seed.pending.InvitationCode {
		t.Fatalf("fresh allocation=%+v", fresh)
	}

	page := processJSON[receivePage](t, fixture, fixture.projects[1], "receive", "--wait", "0", "--json", fresh.ID)
	if len(page.Messages) != 2 || page.Messages[0].Body != fresh.InitialMessage.Body || page.AckedSequence != 0 {
		t.Fatalf("inherited history=%+v", page)
	}

	return fresh
}

func assertFreshStoredContext(t *testing.T, s *store, value conversation) {
	t.Helper()

	var threads, messages int
	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM bridge_bindings WHERE conversation_id=?", value.ID).Scan(&threads); err != nil {
		t.Fatal(err)
	}

	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM messages WHERE conversation_id=?", value.ID).Scan(&messages); err != nil {
		t.Fatal(err)
	}

	if threads != 0 || messages != 2 {
		t.Fatalf("fresh threads=%d messages=%d", threads, messages)
	}

	var ordinal int64
	if err := s.db.QueryRowContext(t.Context(), "SELECT last_ordinal FROM pair_sequences").Scan(&ordinal); err != nil || ordinal != 3 {
		t.Fatalf("ordinal=%d, error=%v", ordinal, err)
	}
}

func waitLifecycleInterval(t *testing.T, duration time.Duration) {
	t.Helper()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	case <-timer.C:
	}
}

func assertPruneHumanPreview(t *testing.T, fixture processFixture) {
	t.Helper()

	stdout, stderr, err := fixture.command(t.Context(), fixture.projects[0], "prune", "--all-projects")
	if err != nil || len(stderr) != 0 || !strings.Contains(string(stdout), "Preview: 0 eligible conversations") || !strings.Contains(string(stdout), "Confirmation token:") {
		t.Fatalf("human prune: stdout=%s stderr=%s error=%v", stdout, stderr, err)
	}
}
