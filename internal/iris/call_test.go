package iris

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func callFixture(t *testing.T, s *store, from project, to, key string) callParams {
	t.Helper()

	return callParams{projectSelection: projectSelection{Project: from.ID}, To: to, Topic: "Storage protocol", Body: "Which format is accepted?\nKeep this initial request.", RequestKey: key}
}

func mustCall(t *testing.T, s *store, params callParams) conversation {
	t.Helper()

	result, err := s.call(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

// Initial content, reserved recipient identity and idempotent results survive reopening.
func TestCallPersistsInvitation(t *testing.T) {
	s, cfg := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	params := callFixture(t, s, alpha, "beta", "first")
	s.now = func() time.Time { return time.Unix(1700000000, 123000000) }
	original := mustCall(t, s, params)
	assertInitialCall(t, original, alpha)

	reopened, err := openStore(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := reopened.close(); err != nil {
			t.Error(err)
		}
	})

	retry := mustCall(t, reopened, params)
	if retry.ID != original.ID || retry.InitialMessage.ID != original.InitialMessage.ID || !retry.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("retry changed original result: %+v", retry)
	}

	if _, err := s.renameProject(t.Context(), projectRename{ProjectID: alpha.ID, Name: "renamed"}); err != nil {
		t.Fatal(err)
	}

	if renamed := mustCall(t, s, params); renamed.Label != original.Label {
		t.Fatalf("historical label changed: %q", renamed.Label)
	}

	params.Body = "Changed request"
	if _, err := s.call(t.Context(), params); !hasCode(err, "idempotency_conflict") {
		t.Fatalf("changed retry=%v", err)
	}
}

func assertInitialCall(t *testing.T, value conversation, alpha project) {
	t.Helper()

	if !validID(value.ID) || value.Ordinal != 1 || value.State != "pending" || !strings.Contains(value.Label, "#0001") {
		t.Fatalf("invalid invitation metadata: %+v", value)
	}

	assertInitialMessage(t, value.InitialMessage, alpha.ID, value.Topic)

	if !validInvitationCode(value.InvitationCode) || value.InvitationExpiresAt.Sub(value.CreatedAt) != defaultInvitationLifetime {
		t.Fatalf("invitation=%+v", value)
	}
}

func assertInitialMessage(t *testing.T, msg message, sender, topic string) {
	t.Helper()

	if msg.Sequence != 1 || msg.SenderProjectID == nil || *msg.SenderProjectID != sender || msg.Topic != topic || msg.SchemaVersion != 1 {
		t.Fatalf("initial message metadata: %+v", msg)
	}

	if len(msg.Recipients) != 1 || msg.Recipients[0] == sender || msg.Body != "Which format is accepted?\nKeep this initial request." {
		t.Fatalf("initial delivery: %+v", msg)
	}
}

// Concurrent calls in both directions allocate one gap-free sequence for the unordered pair.
func TestCallConcurrentOrdinals(t *testing.T) {
	s, cfg := testStore(t)
	alpha, beta := registeredProject(t, s, "alpha"), registeredProject(t, s, "beta")

	other, err := openStore(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := other.close(); err != nil {
			t.Error(err)
		}
	})

	results := make(chan conversation, 12)
	failures := make(chan error, 12)

	var workers sync.WaitGroup

	for index := range 12 {
		params := callFixture(t, s, alpha, beta.ID, fmt.Sprintf("call-%d", index))
		target := s

		if index%2 == 1 {
			params.Project, params.To = beta.ID, alpha.ID
			target = other
		}

		workers.Go(func() {
			value, err := target.call(t.Context(), params)
			if err != nil {
				failures <- err

				return
			}

			results <- value
		})
	}

	workers.Wait()
	close(results)
	close(failures)

	for err := range failures {
		t.Error(err)
	}

	assertCallOrdinals(t, results, 12)
}

func assertCallOrdinals(t *testing.T, results <-chan conversation, count int) {
	t.Helper()

	ordinals := []int64{}

	ids, codes := map[string]bool{}, map[string]bool{}
	for value := range results {
		if ids[value.ID] || codes[value.InvitationCode] {
			t.Fatalf("reused call identity: %+v", value)
		}

		ids[value.ID], codes[value.InvitationCode] = true, true
		ordinals = append(ordinals, value.Ordinal)
	}

	slices.Sort(ordinals)

	if len(ordinals) != count {
		t.Fatalf("created=%d want %d", len(ordinals), count)
	}

	for index, ordinal := range ordinals {
		if ordinal != int64(index+1) {
			t.Fatalf("ordinals=%v", ordinals)
		}
	}
}

// Code tombstones and request tombstones prevent reuse after retained history is removed.
func TestCallTombstones(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	params := callFixture(t, s, alpha, "beta", "first")
	first := mustCall(t, s, params)

	s.makeCode = func() (string, error) { return first.InvitationCode, nil }
	if _, err := s.call(t.Context(), callFixture(t, s, alpha, "gamma", "collision")); !hasCode(err, "limit_exceeded") {
		t.Fatalf("collision exhaustion=%v", err)
	}

	assertRegistryCount(t, s, 2)

	if _, err := s.db.ExecContext(t.Context(), "DELETE FROM conversations WHERE id=?", first.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := s.call(t.Context(), params); !hasCode(err, "not_found") {
		t.Fatalf("pruned retry=%v", err)
	}

	params.RequestKey = "second"
	if _, err := s.call(t.Context(), params); !hasCode(err, "limit_exceeded") {
		t.Fatalf("retired code=%v", err)
	}

	attempts := 0
	s.makeCode = func() (string, error) {
		attempts++
		if attempts == 1 {
			return first.InvitationCode, nil
		}

		return "maple-river-cloud", nil
	}

	second := mustCall(t, s, params)
	if second.Ordinal != 2 || attempts != 2 {
		t.Fatalf("ordinal=%d attempts=%d", second.Ordinal, attempts)
	}
}

func assertRegistryCount(t *testing.T, s *store, want int) {
	t.Helper()

	var count int
	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM projects").Scan(&count); err != nil || count != want {
		t.Fatalf("project count=%d error=%v", count, err)
	}
}

// Concurrent retries consume only one ordinal and one request/message/invitation identity.
func TestCallConcurrentRetry(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	params := callFixture(t, s, alpha, "beta", "same-request")
	results := make(chan conversation, 8)
	failures := make(chan error, 8)

	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			value, err := s.call(t.Context(), params)
			if err != nil {
				failures <- err

				return
			}

			results <- value
		})
	}

	workers.Wait()
	close(results)
	close(failures)

	for err := range failures {
		t.Error(err)
	}

	id := ""
	for value := range results {
		if id != "" && value.ID != id {
			t.Fatalf("retry allocated new conversation: %+v", value)
		}

		id = value.ID
		if value.Ordinal != 1 {
			t.Fatalf("retry ordinal=%d", value.Ordinal)
		}
	}
}

// Validation and quota failures leave auto-registration, pair allocations and codes untouched.
func TestCallBounds(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	beta := registeredProject(t, s, "beta")
	base := callFixture(t, s, alpha, beta.ID, "bounds")
	cases := map[string]callParams{"self": base, "empty body": base, "large body": base, "topic": base, "key": base, "recipient": base}
	value := cases["self"]
	value.To = alpha.ID
	cases["self"] = value
	value = cases["empty body"]
	value.Body = ""
	cases["empty body"] = value
	value = cases["large body"]
	value.Body = strings.Repeat("x", maxMessageBodyBytes+1)
	cases["large body"] = value
	value = cases["topic"]
	value.Topic = strings.Repeat("é", 129)
	cases["topic"] = value
	value = cases["key"]
	value.RequestKey = ""
	cases["key"] = value
	value = cases["recipient"]
	value.To = "invalid_alias"

	cases["recipient"] = value
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.call(t.Context(), params); err == nil {
				t.Fatal("invalid call accepted")
			}
		})
	}

	seedConversationQuota(t, s, alpha, beta)

	if _, err := s.call(t.Context(), base); !hasCode(err, "limit_exceeded") {
		t.Fatalf("quota error=%v", err)
	}

	var tombstones int
	if err := s.db.QueryRowContext(t.Context(), "SELECT count(*) FROM invitation_code_tombstones").Scan(&tombstones); err != nil || tombstones != 0 {
		t.Fatalf("tombstones=%d error=%v", tombstones, err)
	}
}

func seedConversationQuota(t *testing.T, s *store, alpha, beta project) {
	t.Helper()

	low, high := alpha.ID, beta.ID
	if low > high {
		low, high = high, low
	}

	err := s.write(t.Context(), func(tx *sql.Tx) error {
		ctx := t.Context()
		if _, err := tx.ExecContext(ctx, "INSERT INTO pair_sequences VALUES (?,?,64)", low, high); err != nil {
			return err
		}

		for index := range maxActiveConversations {
			id := fmt.Sprintf("%032x", index+1)
			if _, err := tx.ExecContext(ctx, `INSERT INTO conversations (id,project_low,project_high,pair_ordinal,topic,state,created_at,last_activity_at) VALUES (?,?,?,?,'Quota','pending',?,?)`, id, low, high, index+1, s.now().UnixMilli(), s.now().UnixMilli()); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, "INSERT INTO participants VALUES (?,?,?,0)", id, alpha.ID, alpha.Alias); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The protocol routes call parameters through the same durable store operation.
func TestCallProtocol(t *testing.T) {
	s, _ := testStore(t)
	alpha := registeredProject(t, s, "alpha")
	params := callFixture(t, s, alpha, "beta", "protocol")

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	reply := s.handle(t.Context(), request{Protocol: protocolVersion, Operation: "call", Params: encoded})
	if !reply.OK {
		t.Fatalf("call response=%+v", reply)
	}
}
