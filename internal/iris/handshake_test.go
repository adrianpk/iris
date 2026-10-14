package iris

import (
	"database/sql"
	"sync"
	"testing"
	"time"
)

func joinReference(path, code string) joinParams {
	return joinParams{projectSelection: projectSelection{CWD: path}, Code: code}
}

func closeReference(owner project, id string) conversationParams {
	return conversationParams{projectSelection: projectSelection{Project: owner.ID}, ConversationID: id}
}

func mustJoin(t *testing.T, s *store, params joinParams) conversation {
	t.Helper()

	value, err := s.join(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func mustClose(t *testing.T, s *store, owner project, id string) conversation {
	t.Helper()

	value, err := s.closeConversation(t.Context(), closeReference(owner, id))
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func fixedClock(s *store) *time.Time {
	now := time.Unix(1700000000, 0).UTC()
	s.now = func() time.Time { return now }

	return &now
}

// Only the reserved project can join; registration is atomic and visibility never acknowledges work.
func TestJoinReservedRecipient(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	pending := mustCall(t, s, callFixture(t, s, alpha, "beta-custom", "first"))

	wrong := projectDirectory(t, "wrong")
	if _, err := s.join(t.Context(), joinReference(wrong, pending.InvitationCode)); !hasCode(err, "recipient_mismatch") {
		t.Fatalf("wrong join=%v", err)
	}

	assertRegistryCount(t, s, 2)

	path := projectDirectory(t, "beta")
	writeAliasOverride(t, path, "name = \"beta-custom\"\n")

	joined := mustJoin(t, s, joinReference(path, pending.InvitationCode))
	if joined.ID != pending.ID || joined.State != "active" || joined.Topic != pending.Topic || joined.InitialMessage.ID != pending.InitialMessage.ID {
		t.Fatalf("join changed initial request: %+v", joined)
	}

	assertHandshakeRows(t, s, pending.ID, "active", 2, 0)

	registered, err := exactProject(t.Context(), s.db, "beta-custom")
	if err != nil || registered.Path == nil || *registered.Path != path {
		t.Fatalf("recipient=%+v error=%v", registered, err)
	}

	if _, err := s.join(t.Context(), joinParams{projectSelection: projectSelection{Project: alpha.ID}, Code: pending.InvitationCode}); !hasCode(err, "recipient_mismatch") {
		t.Fatalf("initiator joined as recipient: %v", err)
	}
}

func assertHandshakeRows(t *testing.T, s *store, id, state string, messages, consumers int) {
	t.Helper()

	var (
		actual                                     string
		messageCount, consumerCount, sequenceCount int
	)

	err := s.db.QueryRowContext(t.Context(), `SELECT state,
        (SELECT count(*) FROM messages WHERE conversation_id=c.id),
        (SELECT count(*) FROM consumer_cursors WHERE conversation_id=c.id),
        (SELECT count(DISTINCT sequence) FROM messages WHERE conversation_id=c.id)
        FROM conversations c WHERE id=?`, id).Scan(&actual, &messageCount, &consumerCount, &sequenceCount)
	if err != nil {
		t.Fatal(err)
	}

	if actual != state || messageCount != messages || consumerCount != consumers || sequenceCount != messages {
		t.Fatalf("state=%s messages=%d consumers=%d sequences=%d", actual, messageCount, consumerCount, sequenceCount)
	}
}

// Concurrent accepts record one join, and duplicate retries do not renew inactivity.
func TestJoinConcurrent(t *testing.T) {
	s, cfg := testStore(t)
	now := fixedClock(s)
	alpha := registeredProject(t, s, "alpha")
	pending := mustCall(t, s, callFixture(t, s, alpha, "beta", "one"))
	path := projectDirectory(t, "beta")

	other, err := openStore(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	other.now = s.now

	t.Cleanup(func() {
		if err := other.close(); err != nil {
			t.Error(err)
		}
	})

	*now = now.Add(time.Hour)

	runConcurrentJoins(t, []*store{s, other}, joinReference(path, pending.InvitationCode), pending.ID)
	assertHandshakeRows(t, s, pending.ID, "active", 2, 0)

	*now = now.Add(23 * time.Hour)

	retry := mustJoin(t, s, joinReference(path, pending.InvitationCode))
	if !retry.LastActivityAt.Equal(pending.CreatedAt.Add(time.Hour)) {
		t.Fatalf("retry renewed inactivity: %s", retry.LastActivityAt)
	}

	assertHandshakeRows(t, s, pending.ID, "active", 2, 0)
}

func runConcurrentJoins(t *testing.T, stores []*store, params joinParams, id string) {
	t.Helper()

	var workers sync.WaitGroup

	for index := range 8 {
		target := stores[index%len(stores)]

		workers.Go(func() {
			joined, err := target.join(t.Context(), params)
			if err != nil {
				t.Error(err)

				return
			}

			if joined.ID != id || joined.State != "active" {
				t.Errorf("concurrent join=%+v", joined)
			}
		})
	}

	workers.Wait()
}

// Expiry is inclusive at the stored invitation boundary and cannot bind a late directory.
func TestJoinInvitationExpiry(t *testing.T) {
	cases := []struct {
		name    string
		elapsed time.Duration
		code    string
	}{
		{"before boundary", defaultInvitationLifetime - time.Millisecond, ""},
		{"at boundary", defaultInvitationLifetime, "invitation_expired"},
		{"after boundary", defaultInvitationLifetime + time.Millisecond, "invitation_expired"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			s, _ := testStore(t)
			now := fixedClock(s)
			alpha := registeredProject(t, s, "alpha")
			pending := mustCall(t, s, callFixture(t, s, alpha, "beta", "one"))
			*now = now.Add(test.elapsed)
			params := joinReference(projectDirectory(t, "beta"), pending.InvitationCode)

			_, err := s.join(t.Context(), params)
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}

				return
			}

			if !hasCode(err, test.code) {
				t.Fatalf("expiry=%v", err)
			}

			assertExpiredJoin(t, s, params, pending.ID, test.code)
		})
	}
}

func assertExpiredJoin(t *testing.T, s *store, params joinParams, id, code string) {
	t.Helper()

	if _, err := s.join(t.Context(), params); !hasCode(err, code) {
		t.Fatalf("expired retry=%v", err)
	}

	assertHandshakeRows(t, s, id, "expired", 2, 0)

	beta, err := exactProject(t.Context(), s.db, "beta")
	if err != nil || beta.Path != nil {
		t.Fatalf("late join registered recipient: %+v error=%v", beta, err)
	}
}

// Active inactivity is checked before repeated joins and closure, without reopening terminal history.
func TestJoinInactiveConversation(t *testing.T) {
	s, _ := testStore(t)
	now := fixedClock(s)
	alpha := registeredProject(t, s, "alpha")
	pending := mustCall(t, s, callFixture(t, s, alpha, "beta", "one"))
	params := joinReference(projectDirectory(t, "beta"), pending.InvitationCode)
	mustJoin(t, s, params)

	*now = now.Add(defaultInactivityLifetime)

	if _, err := s.closeConversation(t.Context(), closeReference(alpha, pending.ID)); !hasCode(err, "conversation_inactive") {
		t.Fatalf("expired close=%v", err)
	}

	if _, err := s.join(t.Context(), params); !hasCode(err, "conversation_inactive") {
		t.Fatalf("expired join=%v", err)
	}

	assertHandshakeRows(t, s, pending.ID, "expired", 3, 0)
}

// Either joined participant can close, but outsiders and unjoined recipients cannot alter state.
func TestCloseConversation(t *testing.T) {
	cases := []struct {
		name   string
		active bool
	}{{"pending", false}, {"active", true}}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			exerciseClose(t, test.active)
		})
	}
}

func verifyCloseAccess(t *testing.T, s *store, id string) {
	t.Helper()

	outsider := registeredProject(t, s, "outsider")
	if _, err := s.closeConversation(t.Context(), closeReference(outsider, id)); !hasCode(err, "not_participant") {
		t.Fatalf("outsider close=%v", err)
	}
}

// The reserved last sequence and empty-body fallback allow closure at both storage budgets.
func TestCloseReservedEvent(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	pending := mustCall(t, s, callFixture(t, s, alpha, "beta", "one"))

	_, err := s.db.ExecContext(t.Context(), "UPDATE conversations SET message_count=?,next_sequence=?,body_bytes=? WHERE id=?", maxConversationMessages-1, maxConversationMessages, maxConversationBodyBytes, pending.ID)
	if err != nil {
		t.Fatal(err)
	}

	params := joinReference(projectDirectory(t, "beta"), pending.InvitationCode)
	if _, err := s.join(t.Context(), params); !hasCode(err, "limit_exceeded") {
		t.Fatalf("join consumed reserved slot: %v", err)
	}

	beta, err := exactProject(t.Context(), s.db, "beta")
	if err != nil || beta.Path != nil {
		t.Fatalf("failed join registered recipient: %+v error=%v", beta, err)
	}

	mustClose(t, s, alpha, pending.ID)

	var (
		body  string
		count int
	)

	err = s.db.QueryRowContext(t.Context(), `SELECT body,(SELECT message_count FROM conversations WHERE id=?) FROM messages WHERE conversation_id=? AND sequence=?`, pending.ID, pending.ID, maxConversationMessages).Scan(&body, &count)
	if err != nil || body != "" || count != maxConversationMessages {
		t.Fatalf("terminal body=%q count=%d error=%v", body, count, err)
	}
}

// A lifecycle event uses the daemon sender and both immutable participant IDs as recipients.
func TestJoinSystemEvent(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	pending := mustCall(t, s, callFixture(t, s, alpha, "beta", "one"))
	mustJoin(t, s, joinReference(projectDirectory(t, "beta"), pending.InvitationCode))

	var (
		sender      sql.NullString
		alias, kind string
		recipients  int
	)

	err := s.db.QueryRowContext(t.Context(), `SELECT sender_project_id,sender_alias,kind,
        (SELECT count(*) FROM message_recipients WHERE message_id=m.id) FROM messages m WHERE conversation_id=? AND sequence=2`, pending.ID).
		Scan(&sender, &alias, &kind, &recipients)
	if err != nil || sender.Valid || alias != "iris" || kind != "system" || recipients != 2 {
		t.Fatalf("system event sender=%+v alias=%q kind=%q recipients=%d error=%v", sender, alias, kind, recipients, err)
	}
}

func exerciseClose(t *testing.T, active bool) {
	t.Helper()

	s, _ := testStore(t)
	now := fixedClock(s)
	alpha, beta := registeredProject(t, s, "alpha"), registeredProject(t, s, "beta")
	pending := mustCall(t, s, callFixture(t, s, alpha, beta.ID, "one"))

	params := joinParams{projectSelection: projectSelection{Project: beta.ID}, Code: pending.InvitationCode}
	if _, err := s.closeConversation(t.Context(), closeReference(beta, pending.ID)); !hasCode(err, "not_participant") {
		t.Fatalf("unjoined close=%v", err)
	}

	closer := alpha

	if active {
		mustJoin(t, s, params)

		closer = beta
	}

	verifyCloseAccess(t, s, pending.ID)

	closed := mustClose(t, s, closer, pending.ID)
	*now = now.Add(48 * time.Hour)

	again := mustClose(t, s, closer, pending.ID)
	if closed.State != "closed" || !closed.TerminalAt.Equal(*again.TerminalAt) {
		t.Fatalf("close changed retention: %+v", again)
	}

	if _, err := s.join(t.Context(), params); !hasCode(err, "conversation_inactive") {
		t.Fatalf("closed join=%v", err)
	}

	count := 2
	if active {
		count++
	}

	assertHandshakeRows(t, s, pending.ID, "closed", count, 0)
}

// Invitation acceptance uses its persisted deadline rather than recomputing a new lifetime.
func TestJoinStoredDeadline(t *testing.T) {
	s, _ := testStore(t)
	now := fixedClock(s)
	alpha := registeredProject(t, s, "alpha")

	pending := mustCall(t, s, callFixture(t, s, alpha, "beta", "one"))
	if _, err := s.db.ExecContext(t.Context(), "UPDATE invitations SET expires_at=? WHERE conversation_id=?", now.Add(time.Minute).UnixMilli(), pending.ID); err != nil {
		t.Fatal(err)
	}

	*now = now.Add(time.Minute)

	if _, err := s.join(t.Context(), joinReference(projectDirectory(t, "beta"), pending.InvitationCode)); !hasCode(err, "invitation_expired") {
		t.Fatalf("stored deadline=%v", err)
	}

	assertHandshakeRows(t, s, pending.ID, "expired", 2, 0)
}

// Racing acceptance with closure leaves one terminal event and cannot restore active state.
func TestJoinCloseRace(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	pending := mustCall(t, s, callFixture(t, s, alpha, "beta", "one"))
	params := joinReference(projectDirectory(t, "beta"), pending.InvitationCode)

	var workers sync.WaitGroup
	workers.Go(func() {
		if _, err := s.join(t.Context(), params); err != nil && !hasCode(err, "conversation_inactive") {
			t.Error(err)
		}
	})
	workers.Go(func() {
		if _, err := s.closeConversation(t.Context(), closeReference(alpha, pending.ID)); err != nil {
			t.Error(err)
		}
	})
	workers.Wait()

	if _, err := s.join(t.Context(), params); !hasCode(err, "conversation_inactive") {
		t.Fatalf("late join=%v", err)
	}

	var (
		state  string
		events int
	)

	err := s.db.QueryRowContext(t.Context(), `SELECT state,(SELECT count(*) FROM messages WHERE conversation_id=c.id AND body='Conversation closed.') FROM conversations c WHERE id=?`, pending.ID).Scan(&state, &events)
	if err != nil || state != "closed" || events != 1 {
		t.Fatalf("state=%s terminal events=%d error=%v", state, events, err)
	}
}
