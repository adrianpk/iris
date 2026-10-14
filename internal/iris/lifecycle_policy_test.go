package iris

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Policy configuration keeps defaults for omitted fields and rejects unsafe duration values.
func TestLifecycleConfiguration(t *testing.T) {
	cases := []struct {
		name, data string
		valid      bool
	}{
		{"partial", `{"lifecycle":{"invitation_seconds":3}}`, true},
		{"null policy", `{"lifecycle":null}`, false},
		{"null duration", `{"lifecycle":{"invitation_seconds":null}}`, false},
		{"zero", `{"lifecycle":{"inactivity_seconds":0}}`, false},
		{"negative", `{"lifecycle":{"retention_seconds":-1}}`, false},
		{"overflow", `{"lifecycle":{"retention_seconds":9223372036854775807}}`, false},
		{"unknown", `{"lifecycle":{"other":5}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config{configPath: filepath.Join(t.TempDir(), "config.json"), settings: defaultSettings()}
			if err := os.WriteFile(cfg.configPath, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}

			if err := cfg.loadSettings(); (err == nil) != tc.valid {
				t.Fatalf("configuration=%v", err)
			}

			if tc.valid && cfg.settings.Lifecycle.InactivitySeconds != 86400 {
				t.Fatal("partial policy lost defaults")
			}
		})
	}
}

// Persisted invitation deadlines survive policy changes, while active inactivity uses the configured duration.
func TestLifecyclePolicyBoundaries(t *testing.T) {
	f := newDeliveryFixture(t)
	now := fixedClock(f.store)
	f.store.policy = lifecyclePolicy{InvitationSeconds: 3, InactivitySeconds: 5, RetentionSeconds: 7}

	pending := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "short invitation"))
	if pending.InvitationExpiresAt.Sub(pending.CreatedAt) != 3*time.Second {
		t.Fatal("invitation policy was ignored")
	}

	f.store.policy.InvitationSeconds = 10
	*now = now.Add(3*time.Second - time.Millisecond)
	joined := mustJoin(t, f.store, joinParams{projectSelection: projectSelection{Project: f.beta.ID}, Code: pending.InvitationCode})
	activity := joined.LastActivityAt
	*now = activity.Add(5*time.Second - time.Millisecond)

	page := mustReceive(t, f.store, receiveReference(f.beta, joined.ID, "policy"))
	for _, input := range page.Messages {
		mustAck(t, f.store, ackReference(f.beta, input, "policy"))
	}

	assertConversationActivity(t, f.store, joined.ID, activity)

	*now = activity.Add(5 * time.Second)

	if _, err := f.store.receive(t.Context(), receiveReference(f.beta, joined.ID, "policy")); !hasCode(err, "conversation_inactive") {
		t.Fatalf("inactivity boundary=%v", err)
	}

	assertConversationActivity(t, f.store, joined.ID, activity)
}

func assertConversationActivity(t *testing.T, s *store, id string, want time.Time) {
	t.Helper()

	var actual int64
	if err := s.db.QueryRowContext(t.Context(), "SELECT last_activity_at FROM conversations WHERE id=?", id).Scan(&actual); err != nil {
		t.Fatal(err)
	}

	if actual != want.UnixMilli() {
		t.Fatalf("activity=%d, want %d", actual, want.UnixMilli())
	}
}

// A later policy cannot extend the already-issued code's acceptance deadline.
func TestInvitationPolicySnapshot(t *testing.T) {
	f := newDeliveryFixture(t)
	now := fixedClock(f.store)
	f.store.policy.InvitationSeconds = 1
	pending := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "expires"))
	f.store.policy.InvitationSeconds = 100
	*now = now.Add(time.Second)

	if _, err := f.store.join(t.Context(), joinParams{projectSelection: projectSelection{Project: f.beta.ID}, Code: pending.InvitationCode}); !hasCode(err, "invitation_expired") {
		t.Fatalf("issued deadline changed: %v", err)
	}
}
