package iris

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func receiveReference(owner project, id, consumer string) receiveParams {
	return receiveParams{conversationParams: closeReference(owner, id), ConsumerID: consumer}
}

func mustReceive(t *testing.T, s *store, params receiveParams) receivePage {
	t.Helper()

	value, err := s.receive(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func ackReference(owner project, value message, consumer string) ackParams {
	return ackParams{conversationParams: closeReference(owner, value.ConversationID), ConsumerID: consumer, MessageID: value.ID}
}

func mustAck(t *testing.T, s *store, params ackParams) consumerPosition {
	t.Helper()

	value, err := s.acknowledge(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

// Delivery is replayed until acknowledgment, with independent durable positions per consumer.
func TestReceiveConsumerProgress(t *testing.T) {
	f := newDeliveryFixture(t)
	params := receiveReference(f.beta, f.conversation.ID, "manual")
	params.Limit = 1

	first := mustReceive(t, f.store, params)
	if len(first.Messages) != 1 || first.Messages[0].Sequence != 1 || first.AckedSequence != 0 || !first.Truncated {
		t.Fatalf("first page=%+v", first)
	}

	retry := mustReceive(t, f.store, params)
	if retry.Messages[0].ID != first.Messages[0].ID {
		t.Fatal("unacknowledged delivery was lost")
	}

	manual := mustAck(t, f.store, ackReference(f.beta, first.Messages[0], "manual"))
	if manual.AckedSequence != 1 {
		t.Fatalf("position=%+v", manual)
	}

	other := mustReceive(t, f.store, receiveReference(f.beta, f.conversation.ID, "worker"))
	if other.AckedSequence != 0 || other.Messages[0].Sequence != 1 {
		t.Fatalf("consumer progress leaked: %+v", other)
	}

	assertConsumerRestart(t, f, params, first)
}

// Scope-bound cursors permit pagination but cannot forge delivery of skipped addressed messages.
func TestReceiveCursors(t *testing.T) {
	f := newDeliveryFixture(t)
	params := receiveReference(f.beta, f.conversation.ID, "manual")
	params.Limit = 1
	first := mustReceive(t, f.store, params)
	params.Cursor = *first.NextCursor

	second := mustReceive(t, f.store, params)
	if len(second.Messages) != 1 || second.Messages[0].Sequence != 2 {
		t.Fatalf("cursor page=%+v", second)
	}

	assertForeignCursors(t, f, *first.NextCursor)

	cursor := deliveryCursor{SchemaVersion: 1, ConversationID: f.conversation.ID, ProjectID: f.beta.ID, ConsumerID: "manual", AfterSequence: 100}

	raw, err := json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}

	params.Cursor = base64.RawURLEncoding.EncodeToString(raw)
	if _, err := f.store.receive(t.Context(), params); !hasCode(err, "invalid_cursor") {
		t.Fatalf("future cursor=%v", err)
	}

	params.ConsumerID = "new-consumer"
	cursor.ConsumerID = params.ConsumerID
	cursor.AfterSequence = 1

	raw, err = json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}

	params.Cursor = base64.RawURLEncoding.EncodeToString(raw)
	if _, err := f.store.receive(t.Context(), params); !hasCode(err, "invalid_cursor") {
		t.Fatalf("undelivered cursor=%v", err)
	}
}

func assertForeignCursors(t *testing.T, f deliveryFixture, cursor string) {
	t.Helper()

	other := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, "other"))

	cases := []receiveParams{receiveReference(f.alpha, f.conversation.ID, "manual"), receiveReference(f.beta, f.conversation.ID, "other"), receiveReference(f.alpha, other.ID, "manual")}
	for index, params := range cases {
		t.Run(fmt.Sprintf("scope %d", index), func(t *testing.T) {
			params.Cursor = cursor
			if _, err := f.store.receive(t.Context(), params); !hasCode(err, "invalid_cursor") {
				t.Fatalf("scope error=%v", err)
			}
		})
	}
}

// Acknowledgment cannot skip earlier addressed work, acknowledge unseen work or cross scope.
func TestAckOrdering(t *testing.T) {
	f := newDeliveryFixture(t)
	next := mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, "Another input"))
	params := receiveReference(f.beta, f.conversation.ID, "manual")
	params.Limit = 1

	first := mustReceive(t, f.store, params)
	if _, err := f.store.acknowledge(t.Context(), ackReference(f.beta, next, "manual")); !hasCode(err, "ack_out_of_order") {
		t.Fatalf("skipped ack=%v", err)
	}

	if _, err := f.store.acknowledge(t.Context(), ackReference(f.beta, first.Messages[0], "other")); !hasCode(err, "ack_out_of_order") {
		t.Fatalf("undelivered ack=%v", err)
	}

	if _, err := f.store.acknowledge(t.Context(), ackReference(f.alpha, first.Messages[0], "manual")); !hasCode(err, "not_found") {
		t.Fatalf("foreign ack=%v", err)
	}

	mustAck(t, f.store, ackReference(f.beta, first.Messages[0], "manual"))

	page := mustReceive(t, f.store, params)
	mustAck(t, f.store, ackReference(f.beta, page.Messages[0], "manual"))

	page = mustReceive(t, f.store, params)

	position := mustAck(t, f.store, ackReference(f.beta, page.Messages[0], "manual"))
	if position.AckedSequence != next.Sequence {
		t.Fatalf("contiguous position=%+v", position)
	}
	// Alpha's own initial request is not addressed to alpha; its first ack legitimately skips it.
	alphaPage := mustReceive(t, f.store, receiveReference(f.alpha, f.conversation.ID, "manual"))

	position = mustAck(t, f.store, ackReference(f.alpha, alphaPage.Messages[0], "manual"))
	if position.AckedSequence != 2 {
		t.Fatalf("unaddressed gap=%+v", position)
	}
}

// Result and processing acknowledgment either both commit or both roll back, including retries.
func TestDeliveryCompletion(t *testing.T) {
	f := newDeliveryFixture(t)
	initial := f.conversation.InitialMessage
	key := "result-one"
	params := sendReference(f.beta, f.conversation.ID, "The format is JSON.")
	params.IdempotencyKey, params.ReplyTo = &key, &initial.ID

	ack := ackReference(f.beta, initial, "worker")
	if _, err := f.store.completeDelivery(t.Context(), params, ack); !hasCode(err, "ack_out_of_order") {
		t.Fatalf("undelivered completion=%v", err)
	}

	assertHandshakeRows(t, f.store, f.conversation.ID, "active", 2, 0)
	mustReceive(t, f.store, receiveReference(f.beta, f.conversation.ID, "worker"))

	response, err := f.store.completeDelivery(t.Context(), params, ack)
	if err != nil {
		t.Fatal(err)
	}

	retry, err := f.store.completeDelivery(t.Context(), params, ack)
	if err != nil || !reflect.DeepEqual(response, retry) {
		t.Fatalf("completion retry=%+v error=%v", retry, err)
	}

	page := mustReceive(t, f.store, receiveReference(f.beta, f.conversation.ID, "worker"))
	if page.AckedSequence != 1 {
		t.Fatalf("result without ack: %+v", page)
	}

	assertHandshakeRows(t, f.store, f.conversation.ID, "active", 3, 1)
}

// Consumer allocation and encoded page bytes are bounded independently of item count.
func TestReceiveBounds(t *testing.T) {
	t.Run("consumer limit", func(t *testing.T) {
		f := newDeliveryFixture(t)
		for index := range 64 {
			mustReceive(t, f.store, receiveReference(f.beta, f.conversation.ID, fmt.Sprintf("consumer-%d", index)))
		}

		if _, err := f.store.receive(t.Context(), receiveReference(f.beta, f.conversation.ID, "excess")); !hasCode(err, "limit_exceeded") {
			t.Fatalf("consumer limit=%v", err)
		}
	})
	t.Run("encoded page", func(t *testing.T) {
		f := newDeliveryFixture(t)

		body := strings.Repeat("x", maxMessageBodyBytes)
		for range 34 {
			mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, body))
		}

		params := receiveReference(f.beta, f.conversation.ID, "manual")
		params.Limit = 100
		page := mustReceive(t, f.store, params)

		encoded, err := json.Marshal(success(page))
		if err != nil {
			t.Fatal(err)
		}

		if !page.Truncated || len(encoded) >= maxResponseBytes || page.NextCursor == nil {
			t.Fatalf("bounded page: messages=%d bytes=%d truncated=%v", len(page.Messages), len(encoded), page.Truncated)
		}
	})
}

func assertConsumerRestart(t *testing.T, f deliveryFixture, params receiveParams, first receivePage) {
	t.Helper()

	reopened, err := openStore(t.Context(), f.config)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := reopened.close(); err != nil {
			t.Error(err)
		}
	}()

	second := mustReceive(t, reopened, params)
	if second.Messages[0].Sequence != 2 || second.AckedSequence != 1 {
		t.Fatalf("restart position=%+v", second)
	}

	duplicate := mustAck(t, reopened, ackReference(f.beta, first.Messages[0], "manual"))
	if duplicate.AckedSequence != 1 {
		t.Fatalf("duplicate ack=%+v", duplicate)
	}
}
