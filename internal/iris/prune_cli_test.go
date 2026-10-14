package iris

import (
	"strings"
	"testing"
)

// Bare prune previews through real IPC; explicit confirmation works without a terminal or project inference.
func TestPruneCLIConfirmation(t *testing.T) {
	f, _ := retainedFixture(t)
	cfg, _ := testService(t, f.store.handle)

	code, stdout, stderr := cliDelivery(t, cfg, *f.alpha.Path, "", "prune", "--json")
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("preview exit=%d stderr=%s", code, stderr)
	}

	var reply response
	if err := decodeJSON(stdout, &reply); err != nil {
		t.Fatal(err)
	}

	var preview prunePreview
	if err := decodeJSON(reply.Result, &preview); err != nil {
		t.Fatal(err)
	}

	assertRetainedConversation(t, f.store, f.conversation.ID)

	code, stdout, stderr = cliDelivery(t, cfg, t.TempDir(), "", "prune", "--confirm", preview.Token, "--json")
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("confirm exit=%d stderr=%s", code, stderr)
	}

	if err := decodeJSON(stdout, &reply); err != nil || !reply.OK {
		t.Fatalf("confirmation JSON=%s, %v", stdout, err)
	}

	assertPruneAllocations(t, f)
}

// Invalid or stale confirmations produce clean failures and cannot delete history.
func TestPruneCLIErrors(t *testing.T) {
	f, _ := retainedFixture(t)
	cfg, _ := testService(t, f.store.handle)

	cases := []struct {
		name string
		args []string
	}{
		{"unknown token", []string{"--confirm", strings.Repeat("a", 32)}},
		{"malformed token", []string{"--confirm", "wrong"}},
		{"ambiguous confirmation", []string{"--confirm", strings.Repeat("a", 32), "--dry-run"}},
		{"scoped confirmation", []string{"--confirm", strings.Repeat("a", 32), "--all-projects"}},
		{"ambiguous scope", []string{"--project", f.alpha.ID, "--all-projects"}},
		{"invalid before", []string{"--before", "yesterday"}},
		{"active filter", []string{"--state", "active"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"prune", "--json"}, tc.args...)

			code, stdout, stderr := cliDelivery(t, cfg, *f.alpha.Path, "", args...)
			if code == 0 || len(stdout) != 0 {
				t.Fatalf("invalid prune exit=%d stdout=%s", code, stdout)
			}

			var reply response
			if err := decodeJSON(stderr, &reply); err != nil || reply.Error == nil {
				t.Fatalf("error JSON=%s, %v", stderr, err)
			}
		})
	}

	assertRetainedConversation(t, f.store, f.conversation.ID)
}
