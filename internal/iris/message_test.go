package iris

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

type deliveryFixture struct {
	store        *store
	config       config
	alpha, beta  project
	conversation conversation
}

func newDeliveryFixture(t *testing.T) deliveryFixture {
	t.Helper()

	s, cfg := testStore(t)
	alpha, beta := registeredProject(t, s, "alpha"), registeredProject(t, s, "beta")
	pending := mustCall(t, s, callFixture(t, s, alpha, beta.ID, "initial"))
	active := mustJoin(t, s, joinParams{projectSelection: projectSelection{Project: beta.ID}, Code: pending.InvitationCode})

	return deliveryFixture{store: s, config: cfg, alpha: alpha, beta: beta, conversation: active}
}

func sendReference(from project, id, body string) sendParams {
	return sendParams{conversationParams: closeReference(from, id), Body: body}
}

func mustSend(t *testing.T, s *store, params sendParams) message {
	t.Helper()

	value, err := s.send(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

// Concurrent senders share one sequence and committed messages are visible through a new connection.
func TestSendConcurrent(t *testing.T) {
	fixture := newDeliveryFixture(t)

	other, err := openStore(t.Context(), fixture.config)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := other.close(); err != nil {
			t.Error(err)
		}
	})

	results := make(chan message, 16)

	var workers sync.WaitGroup

	for index := range 16 {
		target, from := fixture.store, fixture.alpha
		if index%2 == 1 {
			target, from = other, fixture.beta
		}

		workers.Go(func() {
			value, err := target.send(t.Context(), sendReference(from, fixture.conversation.ID, fmt.Sprintf("Message %d", index)))
			if err != nil {
				t.Error(err)

				return
			}

			results <- value
		})
	}

	workers.Wait()
	close(results)

	assertCommittedMessages(t, other, results)
}

// Retry keys compare canonical payloads and return immutable fields even after closure or rename.
func TestSendIdempotency(t *testing.T) {
	fixture := newDeliveryFixture(t)
	key, correlation := "request-one", "question-one"
	params := sendReference(fixture.beta, fixture.conversation.ID, "The accepted format is JSON.\n")
	params.IdempotencyKey, params.CorrelationID = &key, &correlation
	initial := fixture.conversation.InitialMessage.ID
	params.ReplyTo = &initial

	first := mustSend(t, fixture.store, params)
	if first.SchemaVersion != 1 || first.Sequence != 3 || first.SenderAlias != "beta" {
		t.Fatalf("message=%+v", first)
	}

	if _, err := fixture.store.renameProject(t.Context(), projectRename{ProjectID: fixture.beta.ID, Name: "new-beta"}); err != nil {
		t.Fatal(err)
	}

	mustClose(t, fixture.store, fixture.alpha, fixture.conversation.ID)

	params.Recipients = []string{fixture.alpha.ID}

	retry := mustSend(t, fixture.store, params)
	if !reflect.DeepEqual(first, retry) {
		t.Fatalf("retry=%+v original=%+v", retry, first)
	}

	params.Body = "Different response"
	if _, err := fixture.store.send(t.Context(), params); !hasCode(err, "idempotency_conflict") {
		t.Fatalf("changed retry=%v", err)
	}

	params.IdempotencyKey = nil
	if _, err := fixture.store.send(t.Context(), params); !hasCode(err, "conversation_inactive") {
		t.Fatalf("late send=%v", err)
	}
}

// Membership, reply scope, kinds and storage budgets fail without consuming a sequence.
func TestSendRejections(t *testing.T) {
	fixture := newDeliveryFixture(t)
	outsider := registeredProject(t, fixture.store, "outsider")
	other := mustCall(t, fixture.store, callFixture(t, fixture.store, fixture.alpha, fixture.beta.ID, "other"))

	cases := []struct {
		name   string
		params sendParams
		code   string
	}{
		{"outsider", sendReference(outsider, fixture.conversation.ID, "Body"), "not_participant"},
		{"pending", sendReference(fixture.alpha, other.ID, "Body"), "conversation_inactive"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := fixture.store.send(t.Context(), test.params); !hasCode(err, test.code) {
				t.Fatalf("rejection=%v", err)
			}
		})
	}

	params := sendReference(fixture.alpha, fixture.conversation.ID, "Body")

	params.ReplyTo = &other.InitialMessage.ID
	if _, err := fixture.store.send(t.Context(), params); !hasCode(err, "not_found") {
		t.Fatalf("cross-conversation reply=%v", err)
	}

	params.ReplyTo = nil

	params.Recipients = []string{outsider.ID}
	if _, err := fixture.store.send(t.Context(), params); !hasCode(err, "recipient_mismatch") {
		t.Fatalf("foreign recipient=%v", err)
	}

	params.Recipients = nil

	params.Kind = "system"
	if _, err := fixture.store.send(t.Context(), params); !hasCode(err, "invalid_request") {
		t.Fatalf("forged system=%v", err)
	}

	params.Kind = "message"

	params.Body = strings.Repeat("x", maxMessageBodyBytes+1)
	if _, err := fixture.store.send(t.Context(), params); !hasCode(err, "invalid_request") {
		t.Fatalf("body bound=%v", err)
	}

	assertHandshakeRows(t, fixture.store, fixture.conversation.ID, "active", 2, 0)
}

// A failure after the message insert rolls back its sequence, counters, recipients and key.
func TestSendRollback(t *testing.T) {
	fixture := newDeliveryFixture(t)

	s := fixture.store
	if _, err := s.db.ExecContext(t.Context(), `CREATE TRIGGER reject_delivery BEFORE INSERT ON message_recipients BEGIN SELECT RAISE(ABORT,'injected delivery failure'); END`); err != nil {
		t.Fatal(err)
	}

	key := "retry-after-failure"
	params := sendReference(fixture.alpha, fixture.conversation.ID, "Durable content")

	params.IdempotencyKey = &key
	if _, err := s.send(t.Context(), params); err == nil {
		t.Fatal("injected failure succeeded")
	}

	if _, err := s.db.ExecContext(t.Context(), "DROP TRIGGER reject_delivery"); err != nil {
		t.Fatal(err)
	}

	result := mustSend(t, s, params)
	if result.Sequence != 3 {
		t.Fatalf("rollback consumed a sequence: %d", result.Sequence)
	}

	assertHandshakeRows(t, s, fixture.conversation.ID, "active", 3, 0)
}

func assertCommittedMessages(t *testing.T, other *store, results <-chan message) {
	t.Helper()

	sequences := []int64{}
	for value := range results {
		sequences = append(sequences, value.Sequence)

		var persisted string
		if err := other.db.QueryRowContext(t.Context(), "SELECT body FROM messages WHERE id=?", value.ID).Scan(&persisted); err != nil || persisted != value.Body {
			t.Fatalf("send confirmed before persistence: %v", err)
		}
	}

	slices.Sort(sequences)

	if len(sequences) != 16 {
		t.Fatalf("sequences=%v", sequences)
	}

	for index, sequence := range sequences {
		if sequence != int64(index+3) {
			t.Fatalf("sequences=%v", sequences)
		}
	}
}

// Independent SQLite connections racing the same key publish exactly one durable message identity.
func TestSendConcurrentRetry(t *testing.T) {
	f := newDeliveryFixture(t)

	other, err := openStore(t.Context(), f.config)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := other.close(); err != nil {
			t.Error(err)
		}
	}()

	key := "concurrent-retry"
	params := sendReference(f.alpha, f.conversation.ID, "One durable input")
	params.IdempotencyKey = &key
	results := make(chan message, 8)

	var workers sync.WaitGroup

	for index := range 8 {
		target := f.store
		if index%2 == 1 {
			target = other
		}

		workers.Go(func() {
			value, err := target.send(t.Context(), params)
			if err != nil {
				t.Error(err)

				return
			}

			results <- value
		})
	}

	workers.Wait()
	close(results)

	id := ""
	for value := range results {
		if id != "" && id != value.ID {
			t.Fatal("concurrent retry allocated another message")
		}

		id = value.ID
		if value.Sequence != 3 {
			t.Fatalf("retry sequence=%d", value.Sequence)
		}
	}

	assertHandshakeRows(t, f.store, f.conversation.ID, "active", 3, 0)
}
