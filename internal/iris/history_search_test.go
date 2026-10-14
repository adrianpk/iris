package iris

import (
	"strings"
	"testing"
	"time"
)

// Literal text and combined pair/date/ordinal/ID filters preserve participant isolation and historical aliases.
func TestHistorySearch(t *testing.T) {
	f := newDeliveryFixture(t)

	sent := mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, "literal %_.*' OR 1=1 -- café"))
	if _, err := f.store.renameProject(t.Context(), projectRename{ProjectID: f.alpha.ID, Name: "renamed"}); err != nil {
		t.Fatal(err)
	}

	since, until := f.conversation.CreatedAt, f.conversation.CreatedAt
	base := historyReference(f.beta)
	base.Pair = []string{"renamed", f.beta.ID}
	base.Ordinal = f.conversation.Ordinal
	base.Since = &since
	base.Until = &until
	base.ConversationID = f.conversation.ID

	cases := []struct {
		name, text string
		count      int
	}{
		{"literal syntax", "%_.*' OR 1=1 --", 1},
		{"not wildcard", "%missing%", 0},
		{"old alias", "alpha", 1},
		{"topic", "Storage protocol", 1},
		{"new alias excluded", "renamed", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			params := base
			params.Text = test.text

			result, err := f.store.listConversations(t.Context(), params)
			if err != nil || len(result.Conversations) != test.count {
				t.Fatalf("result=%+v error=%v", result, err)
			}

			if test.name == "literal syntax" {
				assertHistoryMatch(t, result.Conversations[0], sent)
			}
		})
	}

	base.Since = timePointer(since.Add(time.Millisecond))
	base.Until = nil

	page, err := f.store.listConversations(t.Context(), base)
	if err != nil || len(page.Conversations) != 0 {
		t.Fatalf("date filter=%+v %v", page, err)
	}

	assertHistoryReadOnly(t, f)
}

func timePointer(value time.Time) *time.Time { return &value }

func assertHistoryMatch(t *testing.T, value historyConversation, sent message) {
	t.Helper()

	if len(value.Matches) != 1 || value.Matches[0].MessageID != sent.ID || !strings.Contains(value.Matches[0].Excerpt, "café") {
		t.Fatalf("matches=%+v", value.Matches)
	}

	if !strings.Contains(value.Label, "alpha") || strings.Contains(value.Label, "renamed") {
		t.Fatalf("label=%s", value.Label)
	}
}

// Invalid selectors and bounds cannot broaden a query; pair ordinals resolve one conversation without an ID.
func TestHistorySelectors(t *testing.T) {
	f := newDeliveryFixture(t)
	params := historyReference(f.alpha)
	params.Pair = []string{f.beta.ID, f.alpha.ID}
	params.Ordinal = f.conversation.Ordinal

	value, err := f.store.showConversation(t.Context(), params)
	if err != nil || value.Conversation.ID != f.conversation.ID {
		t.Fatalf("show=%+v %v", value, err)
	}

	cases := []struct {
		name   string
		change func(*historyParams)
	}{
		{"unscoped ordinal", func(p *historyParams) { p.Pair = nil }},
		{"self pair", func(p *historyParams) { p.Pair = []string{f.alpha.ID, f.alpha.ID} }},
		{"overlong text", func(p *historyParams) { p.Text = strings.Repeat("é", 257) }},
		{"inverted dates", func(p *historyParams) { p.Since = timePointer(time.Now()); p.Until = timePointer(time.Unix(0, 0)) }},
		{"invalid limit", func(p *historyParams) { p.Limit = 101 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			p := params
			test.change(&p)

			if _, err := f.store.listConversations(t.Context(), p); !hasCode(err, "invalid_request") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

// Existing indexes serve creation-time pages, pair ordinals and ordered message lookups without schema churn.
func TestHistoryQueryIndexes(t *testing.T) {
	f := newDeliveryFixture(t)

	queries := []struct {
		name, query, index string
		args               []any
	}{
		{"created page", "SELECT id FROM conversations WHERE (created_at,id)>(?,?) ORDER BY created_at,id LIMIT 51", "conversation_created", []any{0, ""}},
		{"pair ordinal", "SELECT id FROM conversations WHERE project_low=? AND project_high=? AND pair_ordinal=?", "sqlite_autoindex_conversations", []any{f.alpha.ID, f.beta.ID, 1}},
		{"message page", "SELECT id FROM messages WHERE conversation_id=? AND sequence>? ORDER BY sequence LIMIT 51", "sqlite_autoindex_messages", []any{f.conversation.ID, 0}},
	}
	for _, test := range queries {
		t.Run(test.name, func(t *testing.T) {
			details := historyQueryPlan(t, f.store, test.query, test.args)
			if !strings.Contains(details, test.index) {
				t.Fatalf("plan=%s", details)
			}
		})
	}
}

func historyQueryPlan(t *testing.T, s *store, query string, args []any) string {
	t.Helper()

	rows, err := s.db.QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()

	var details strings.Builder

	for rows.Next() {
		var (
			id, parent, unused int
			detail             string
		)
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}

		details.WriteString(detail)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return details.String()
}

// Shared timestamps still produce complete deterministic pages, and search reads never renew significant activity.
func TestHistoryStableSearchPages(t *testing.T) {
	f := newDeliveryFixture(t)

	f.store.now = func() time.Time { return f.conversation.CreatedAt }
	for _, key := range []string{"same-a", "same-b", "same-c"} {
		mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, key))
	}

	params := historyReference(f.alpha)
	params.Text = "Storage"
	params.Limit = 1
	previous := ""
	count := 0

	for {
		page, err := f.store.listConversations(t.Context(), params)
		if err != nil {
			t.Fatal(err)
		}

		id := page.Conversations[0].ID
		if id <= previous {
			t.Fatal("keyset ordering changed")
		}

		previous = id
		count++

		if !page.Truncated {
			break
		}

		params.Cursor = *page.NextCursor
	}

	if count != 4 {
		t.Fatalf("count=%d", count)
	}

	var activity int64
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT last_activity_at FROM conversations WHERE id=?", f.conversation.ID).Scan(&activity); err != nil {
		t.Fatal(err)
	}

	if activity != f.conversation.LastActivityAt.UnixMilli() {
		t.Fatal("search renewed activity")
	}
}
