package iris

import (
	"fmt"
	"strings"
	"testing"
)

func historyReference(owner project) historyParams {
	return historyParams{projectSelection: projectSelection{Project: owner.ID}}
}

// Retained history is participant-scoped, explicitly administrative, and independent of delivery progress.
func TestHistoryScope(t *testing.T) {
	f := newDeliveryFixture(t)
	other := registeredProject(t, f.store, "other")

	cases := []struct {
		name   string
		params historyParams
		count  int
		code   string
	}{
		{"participant", historyReference(f.alpha), 1, ""},
		{"unrelated", historyReference(other), 0, ""},
		{"explicit admin", historyParams{AllProjects: true}, 1, ""},
		{"no selection", historyParams{}, 0, "project_not_found"},
		{"mixed selection", historyParams{AllProjects: true, projectSelection: projectSelection{Project: f.alpha.ID}}, 0, "invalid_request"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result, err := f.store.listConversations(t.Context(), test.params)
			if test.code != "" {
				if !hasCode(err, test.code) {
					t.Fatalf("error=%v", err)
				}

				return
			}

			if err != nil || len(result.Conversations) != test.count {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}

	assertRetainedHistory(t, f, other)

	assertHistoryReadOnly(t, f)
}

// Equal timestamps use IDs as a stable tie-breaker and cursors cannot cross scope or state filters.
func TestHistoryPages(t *testing.T) {
	f := newDeliveryFixture(t)
	for i := range 4 {
		mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, fmt.Sprintf("history-%d", i)))
	}

	params := historyReference(f.alpha)
	params.Limit = 2
	seen := map[string]bool{}

	for {
		page, err := f.store.listConversations(t.Context(), params)
		if err != nil {
			t.Fatal(err)
		}

		for _, value := range page.Conversations {
			if seen[value.ID] {
				t.Fatal("duplicate history")
			}

			seen[value.ID] = true
		}

		if !page.Truncated {
			break
		}

		assertHistoryCursorScope(t, f, params, *page.NextCursor)

		params.Cursor = *page.NextCursor
	}

	if len(seen) != 5 {
		t.Fatalf("count=%d", len(seen))
	}

	assertPendingHistory(t, f, params)
}

func assertHistoryReadOnly(t *testing.T, f deliveryFixture) {
	t.Helper()

	var count int
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM consumer_cursors").Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("history created %d consumers", count)
	}
}

// Actual CLI requests infer project scope, allow explicit administration, and keep machine errors on stderr.
func TestHistoryCLI(t *testing.T) {
	f := newDeliveryFixture(t)
	cfg, _ := testService(t, f.store.handle)

	for _, command := range []string{"conversations", "show"} {
		args := []string{command, "--json"}
		if command == "show" {
			args = append(args, f.conversation.ID)
		}

		code, out, stderr := cliDelivery(t, cfg, *f.alpha.Path, "", args...)
		if code != 0 || len(stderr) != 0 || !strings.Contains(string(out), f.conversation.ID) {
			t.Fatalf("%s: %d %s %s", command, code, out, stderr)
		}
	}

	assertHistoryAdministration(t, f, cfg)
}

func assertRetainedHistory(t *testing.T, f deliveryFixture, other project) {
	t.Helper()

	if _, err := f.store.closeConversation(t.Context(), closeReference(f.alpha, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	params := historyReference(f.beta)
	params.ConversationID = f.conversation.ID
	params.Limit = 1

	first, err := f.store.showConversation(t.Context(), params)
	if err != nil || first.Conversation.State != "closed" || len(first.Messages) != 1 || !first.Truncated {
		t.Fatalf("show=%+v error=%v", first, err)
	}

	params.Cursor = *first.NextCursor

	second, err := f.store.showConversation(t.Context(), params)
	if err != nil || second.Messages[0].Sequence != 2 {
		t.Fatalf("second=%+v error=%v", second, err)
	}

	params.projectSelection = projectSelection{Project: other.ID}
	if _, err := f.store.showConversation(t.Context(), params); !hasCode(err, "not_found") {
		t.Fatalf("foreign show=%v", err)
	}
}

func assertHistoryCursorScope(t *testing.T, f deliveryFixture, params historyParams, cursor string) {
	t.Helper()

	foreign := params
	foreign.State = "pending"

	foreign.Cursor = cursor
	if _, err := f.store.listConversations(t.Context(), foreign); !hasCode(err, "invalid_cursor") {
		t.Fatalf("foreign cursor=%v", err)
	}
}

func assertHistoryAdministration(t *testing.T, f deliveryFixture, cfg config) {
	t.Helper()

	code, out, stderr := cliDelivery(t, cfg, t.TempDir(), "", "conversations", "--json")
	if code == 0 || len(out) != 0 || !strings.Contains(string(stderr), "project_not_found") {
		t.Fatalf("outside=%d %s %s", code, out, stderr)
	}

	code, out, stderr = cliDelivery(t, cfg, t.TempDir(), "", "conversations", "--all-projects")
	if code != 0 || len(stderr) != 0 || !strings.Contains(string(out), f.conversation.Label) {
		t.Fatalf("admin=%d %s %s", code, out, stderr)
	}
}

func assertPendingHistory(t *testing.T, f deliveryFixture, params historyParams) {
	t.Helper()

	params.Cursor = ""
	params.State = "pending"

	page, err := f.store.listConversations(t.Context(), params)
	if err != nil || len(page.Conversations) != 2 || !page.Truncated {
		t.Fatalf("pending=%+v error=%v", page, err)
	}
}
