//go:build integration

package iris

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Actual clients retain committed sends and independent consumers across lost responses and a daemon crash.
func TestProcessDelivery(t *testing.T) {
	fixture := newProcessFixture(t, buildProcessBinary(t))
	configureShortRequests(t, fixture)

	process := fixture.start(t)
	flow := establishProcessHandshake(t, fixture)
	flow.prepareConsumers(t)
	flow.exchangeMessages(t)
	flow.sendConcurrently(t)
	flow.verifyDeliveryIsolation(t)

	original := flow.loseSendResponse(t)
	stopProcess(t, process, syscall.SIGKILL)

	process = fixture.start(t)
	flow.verifyDeliveryRestart(t, original)
	flow.verifyProcessWait(t)
	stopProcess(t, process, syscall.SIGTERM)
	flow.verifyStoredDelivery(t)
}

func configureShortRequests(t *testing.T, fixture processFixture) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(fixture.config.configPath), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(fixture.config.configPath, []byte(`{"schema_version":1,"request_timeout_seconds":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (flow *processHandshake) receive(t *testing.T, project int, consumer string) receivePage {
	t.Helper()

	return processJSON[receivePage](t, flow.fixture, flow.fixture.projects[project], "receive", "--consumer", consumer, "--wait", "0", "--json", flow.first.ID)
}

func (flow *processHandshake) acknowledgePage(t *testing.T, project int, consumer string, page receivePage) {
	t.Helper()

	for _, value := range page.Messages {
		processJSON[consumerPosition](t, flow.fixture, flow.fixture.projects[project], "ack", "--consumer", consumer, "--message-id", value.ID, "--json", flow.first.ID)
	}
}

func (flow *processHandshake) prepareConsumers(t *testing.T) {
	t.Helper()

	manual := flow.receive(t, 1, "manual")

	worker := flow.receive(t, 1, "worker")
	if manual.AckedSequence != 0 || worker.AckedSequence != 0 || len(manual.Messages) != 2 {
		t.Fatalf("initial delivery=%+v worker=%+v", manual, worker)
	}

	flow.acknowledgePage(t, 1, "manual", manual)

	worker = flow.receive(t, 1, "worker")
	if worker.AckedSequence != 0 || worker.Messages[0].Sequence != 1 {
		t.Fatal("manual acknowledgment advanced another consumer")
	}
}

func (flow *processHandshake) exchangeMessages(t *testing.T) {
	t.Helper()

	result := processJSON[message](t, flow.fixture, flow.fixture.projects[1], "send", "--message", "The accepted format is JSON.", "--reply-to", flow.first.InitialMessage.ID, "--correlation-id", "question-one", "--idempotency-key", "answer-one", "--json", flow.first.ID)
	if result.Sequence != 3 || result.ReplyTo == nil || *result.ReplyTo != flow.first.InitialMessage.ID {
		t.Fatalf("reply=%+v", result)
	}

	stdout, stderr, err := flow.fixture.command(t.Context(), flow.fixture.projects[0], "receive", "--wait", "0", flow.first.ID)
	if err != nil || len(stderr) != 0 || !strings.Contains(string(stdout), result.Body) {
		t.Fatalf("human response error=%v stderr=%s", err, stderr)
	}
}

func (flow *processHandshake) sendConcurrently(t *testing.T) {
	t.Helper()

	results := make(chan message, 8)

	var workers sync.WaitGroup
	for index := range 8 {
		workers.Go(func() {
			value, err := flow.concurrentMessage(t.Context(), index)
			if err != nil {
				t.Error(err)

				return
			}

			results <- value
		})
	}

	workers.Wait()
	close(results)

	sequences := []int64{}
	for value := range results {
		sequences = append(sequences, value.Sequence)
	}

	slices.Sort(sequences)

	if len(sequences) != 8 {
		t.Fatalf("committed sequences=%v", sequences)
	}

	for index, sequence := range sequences {
		if sequence != int64(index+4) {
			t.Fatalf("noncontiguous sequence=%v", sequences)
		}
	}
}

func (flow *processHandshake) verifyDeliveryIsolation(t *testing.T) {
	t.Helper()
	processJSON[conversation](t, flow.fixture, flow.fixture.projects[1], "join", "--json", flow.second.InvitationCode)

	other := processJSON[receivePage](t, flow.fixture, flow.fixture.projects[1], "receive", "--wait", "0", "--json", flow.second.ID)
	if len(other.Messages) != 2 || other.Messages[0].Body != flow.second.InitialMessage.Body {
		t.Fatalf("conversation leaked: %+v", other)
	}

	page := flow.receive(t, 1, "worker")
	assertProcessError(t, flow.fixture, flow.fixture.projects[1], "invalid_cursor", "receive", "--wait", "0", "--cursor", *page.NextCursor, "--json", flow.second.ID)

	outsider := projectDirectory(t, "outsider")
	processJSON[project](t, flow.fixture, outsider, "projects", "register", "--json")
	assertProcessError(t, flow.fixture, outsider, "not_participant", "receive", "--wait", "0", "--json", flow.first.ID)
}

func (flow *processHandshake) loseSendResponse(t *testing.T) message {
	t.Helper()

	key := "lost-response"
	params := sendReference(flow.alpha, flow.first.ID, "Committed without a received confirmation.")
	params.IdempotencyKey = &key

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: flow.fixture.config.socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}

	if err := writeFrame(connection, request{Protocol: protocolVersion, Operation: "send", Params: encoded}, maxRequestBytes); err != nil {
		t.Fatal(err)
	}

	if err := connection.CloseWrite(); err != nil {
		t.Fatal(err)
	}

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		page := flow.receive(t, 1, "observer")
		for _, value := range page.Messages {
			if value.IdempotencyKey != nil && *value.IdempotencyKey == key {
				return value
			}
		}
	}

	t.Fatal("send did not commit after its client disconnected")

	return message{}
}

func (flow *processHandshake) verifyDeliveryRestart(t *testing.T, original message) {
	t.Helper()

	retry := processJSON[message](t, flow.fixture, flow.fixture.projects[0], "send", "--message", original.Body, "--idempotency-key", "lost-response", "--json", flow.first.ID)
	if !reflect.DeepEqual(original, retry) {
		t.Fatalf("restart retry=%+v original=%+v", retry, original)
	}

	assertProcessError(t, flow.fixture, flow.fixture.projects[0], "idempotency_conflict", "send", "--message", "Changed content", "--idempotency-key", "lost-response", "--json", flow.first.ID)

	manual := flow.receive(t, 1, "manual")

	worker := flow.receive(t, 1, "worker")
	if manual.AckedSequence != 2 || worker.AckedSequence != 0 || worker.Messages[0].Sequence != 1 {
		t.Fatalf("lost consumer progress: manual=%+v worker=%+v", manual, worker)
	}

	flow.acknowledgePage(t, 1, "manual", manual)

	start := time.Now()

	empty := processJSON[receivePage](t, flow.fixture, flow.fixture.projects[1], "receive", "--wait", "2", "--json", flow.first.ID)
	if len(empty.Messages) != 0 || time.Since(start) < 1800*time.Millisecond {
		t.Fatalf("wait ignored requested duration: %+v", empty)
	}
}

func (flow *processHandshake) verifyProcessWait(t *testing.T) {
	t.Helper()

	done := make(chan error, 1)

	go func() { done <- flow.terminalWait(t.Context()) }()

	select {
	case err := <-done:
		t.Fatalf("idle waiter returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	processJSON[conversation](t, flow.fixture, flow.fixture.projects[0], "close", "--json", flow.first.ID)

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closure did not release client wait")
	}

	assertProcessError(t, flow.fixture, flow.fixture.projects[0], "conversation_inactive", "send", "--message", "Late ordinary input", "--json", flow.first.ID)
}

func (flow *processHandshake) verifyStoredDelivery(t *testing.T) {
	t.Helper()

	s, err := openStore(t.Context(), flow.fixture.config)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := s.close(); err != nil {
			t.Error(err)
		}
	}()

	var count, distinct, maxSequence, retries int

	err = s.db.QueryRowContext(t.Context(), `SELECT count(*),count(DISTINCT sequence),max(sequence),sum(idempotency_key='lost-response') FROM messages WHERE conversation_id=?`, flow.first.ID).Scan(&count, &distinct, &maxSequence, &retries)
	if err != nil || count != 13 || distinct != 13 || maxSequence != 13 || retries != 1 {
		t.Fatalf("stored delivery count=%d distinct=%d maximum=%d retries=%d error=%v", count, distinct, maxSequence, retries, err)
	}
}

func (flow *processHandshake) concurrentMessage(ctx context.Context, index int) (message, error) {
	stdout, stderr, err := flow.fixture.command(ctx, flow.fixture.projects[index%2], "send", "--message", fmt.Sprintf("Concurrent input %d", index), "--idempotency-key", fmt.Sprintf("concurrent-%d", index), "--json", flow.first.ID)
	if err != nil || len(stderr) != 0 {
		return message{}, fmt.Errorf("concurrent send=%w stderr=%s", err, stderr)
	}

	var reply response
	if err := decodeJSON(stdout, &reply); err != nil {
		return message{}, err
	}

	var value message
	if err := decodeJSON(reply.Result, &value); err != nil {
		return message{}, err
	}

	return value, nil
}

func (flow *processHandshake) terminalWait(ctx context.Context) error {
	stdout, stderr, err := flow.fixture.command(ctx, flow.fixture.projects[1], "receive", "--wait", "60", "--json", flow.first.ID)
	if len(stdout) != 0 {
		return fmt.Errorf("terminal receive emitted stdout")
	}

	var reply response
	if decodeErr := decodeJSON(stderr, &reply); decodeErr != nil {
		return decodeErr
	}

	if err == nil || reply.Error == nil || reply.Error.Code != "conversation_inactive" {
		return fmt.Errorf("terminal receive error=%w stderr=%s", err, stderr)
	}

	return nil
}
