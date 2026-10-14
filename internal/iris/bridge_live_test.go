//go:build bridge

package iris

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type liveExchange struct {
	fixture      processFixture
	conversation conversation
	workers      [2]*ownedProcess
	proof        string
}

// Two real Codex workers exchange a project-local fact through Iris and acknowledge it without a relay or echo.
func TestBridgeLiveExchange(t *testing.T) {
	flow := prepareLiveExchange(t)
	flow.workers[0] = flow.startWorker(t, 0)
	flow.assertInitialIdle(t)

	flow.workers[1] = flow.startWorker(t, 1)
	flow.awaitExchange(t)
	flow.verifyExchange(t)
	flow.assertFinalIdle(t)
	flow.stopWorkers(t)
}

func prepareLiveExchange(t *testing.T) *liveExchange {
	t.Helper()

	fixture := newProcessFixture(t, buildProcessBinary(t))

	proof, err := newID()
	if err != nil {
		t.Fatal(err)
	}

	flow := &liveExchange{fixture: fixture, proof: "format=JSON; proof=" + proof}

	instructions := "This directory is an isolated Iris conversation fixture. Use only this project's task-relevant files. For the incoming question read answer.txt and reply with its exact contents. When receiving a completed answer to our initial question, acknowledge it without sending a reply. Do not create or edit files, contact external services, or run background work.\n"
	for _, path := range fixture.projects {
		if err := os.WriteFile(filepath.Join(path, "AGENTS.md"), []byte(instructions), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(fixture.projects[1], "answer.txt"), []byte(flow.proof), 0o600); err != nil {
		t.Fatal(err)
	}

	fixture.start(t)
	processJSON[project](t, fixture, fixture.projects[0], "projects", "register", "--json")
	processJSON[project](t, fixture, fixture.projects[1], "projects", "register", "--json")

	flow.conversation = processJSON[conversation](t, fixture, fixture.projects[0], "call", "--to", "beta", "--topic", "Accepted format", "--message", "Read answer.txt in your own project directory. What exact format and proof does it contain? Reply with the exact complete file contents, with no prefix. This is one question; the completed answer needs acknowledgment only.", "--json")
	processJSON[conversation](t, fixture, fixture.projects[1], "join", "--json", flow.conversation.InvitationCode)

	return flow
}

func (flow *liveExchange) startWorker(t *testing.T, index int) *ownedProcess {
	t.Helper()

	worker := &ownedProcess{exited: make(chan struct{})}
	worker.cmd = exec.Command(flow.fixture.binary, "bridge", "start", "--sandbox", "read-only", "--approvals", "deny", "--turn-seconds", "90", "--total-seconds", "210", "--max-turns", "2", "--max-tokens", "150000", "--json", flow.conversation.ID)
	worker.cmd.Dir, worker.cmd.Env = flow.fixture.projects[index], flow.fixture.env

	worker.cmd.Stdout, worker.cmd.Stderr = &worker.stdout, &worker.stderr

	if err := worker.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { worker.err = worker.cmd.Wait(); close(worker.exited) }()

	t.Cleanup(func() { cleanupLiveWorker(t, worker) })

	return worker
}

func cleanupLiveWorker(t *testing.T, worker *ownedProcess) {
	t.Helper()

	select {
	case <-worker.exited:
		return
	default:
	}

	if err := worker.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Error(err)

		return
	}

	select {
	case <-worker.exited:
	case <-time.After(15 * time.Second):
		if err := worker.forceStop(); err != nil {
			t.Error(err)
		}
	}
}

func (flow *liveExchange) status(t *testing.T, index int) bridgeBinding {
	t.Helper()

	return processJSON[bridgeBinding](t, flow.fixture, flow.fixture.projects[index], "bridge", "status", "--json", flow.conversation.ID)
}

func (flow *liveExchange) awaitBinding(t *testing.T, index int) bridgeBinding {
	t.Helper()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		stdout, _, err := flow.fixture.command(t.Context(), flow.fixture.projects[index], "bridge", "status", "--json", flow.conversation.ID)
		if err == nil {
			return decodeProcessJSON[bridgeBinding](t, stdout)
		}

		select {
		case <-flow.workers[index].exited:
			t.Fatalf("bridge startup failed: %v %s", flow.workers[index].err, flow.workers[index].stderr.String())
		case <-deadline.C:
			t.Fatal("bridge binding did not appear")
		case <-ticker.C:
		}
	}
}

func (flow *liveExchange) assertInitialIdle(t *testing.T) {
	t.Helper()
	flow.awaitBinding(t, 0)
	waitLiveInterval(t, 500*time.Millisecond)

	value := flow.status(t, 0)
	if value.TurnsUsed != 0 || value.ThreadID != nil {
		t.Fatalf("idle worker invoked Codex: %+v", value)
	}
}

func (flow *liveExchange) awaitExchange(t *testing.T) {
	t.Helper()
	flow.awaitBinding(t, 1)

	ctx, cancel := context.WithTimeout(t.Context(), 190*time.Second)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		flow.assertWorkersRunning(t)

		alpha, beta := flow.status(t, 0), flow.status(t, 1)
		if liveExchangeCompleted(alpha, beta, flow.conversation.InitialMessage.ID) {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("live exchange timed out; alpha=%+v beta=%+v", alpha, beta)
		case <-ticker.C:
		}
	}
}

func (flow *liveExchange) assertWorkersRunning(t *testing.T) {
	t.Helper()

	for _, worker := range flow.workers {
		select {
		case <-worker.exited:
			t.Fatalf("live Codex worker failed: %v %s", worker.err, worker.stderr.String())
		default:
		}
	}
}

func (flow *liveExchange) verifyExchange(t *testing.T) {
	t.Helper()

	page := processJSON[receivePage](t, flow.fixture, flow.fixture.projects[0], "receive", "--consumer", "live-observer", "--wait", "0", "--json", flow.conversation.ID)

	var replies []message

	for _, value := range page.Messages {
		if value.Kind == "message" {
			replies = append(replies, value)
		}
	}

	if len(replies) != 1 || strings.TrimSpace(replies[0].Body) != flow.proof {
		t.Fatalf("live reply=%+v", replies)
	}

	if replies[0].ReplyTo == nil || *replies[0].ReplyTo != flow.conversation.InitialMessage.ID {
		t.Fatal("response lost its input reference")
	}

	flow.verifyWorkerProgress(t, replies[0])
}

func (flow *liveExchange) verifyWorkerProgress(t *testing.T, reply message) {
	t.Helper()

	alpha, beta := flow.status(t, 0), flow.status(t, 1)
	if alpha.ThreadID == nil || beta.ThreadID == nil || *alpha.ThreadID == *beta.ThreadID || alpha.InputMessageID == nil || *alpha.InputMessageID != reply.ID {
		t.Fatalf("thread/progress isolation: alpha=%+v beta=%+v", alpha, beta)
	}

	for index, binding := range []bridgeBinding{alpha, beta} {
		flow.verifyAcknowledgment(t, index, binding)
	}

	t.Log("real Codex question/response/ack passed: distinct project threads, one turn per worker, project-local random proof, durable ordered acknowledgment")
}

func (flow *liveExchange) verifyAcknowledgment(t *testing.T, index int, binding bridgeBinding) {
	t.Helper()

	page := processJSON[receivePage](t, flow.fixture, flow.fixture.projects[index], "receive", "--consumer", binding.ConsumerID, "--wait", "0", "--json", flow.conversation.ID)

	want := int64(2)
	if index == 0 {
		want = 3
	}

	if page.AckedSequence != want || len(page.Messages) != 0 {
		t.Fatalf("durable bridge acknowledgment=%+v", page)
	}
}

func (flow *liveExchange) assertFinalIdle(t *testing.T) {
	t.Helper()
	waitLiveInterval(t, time.Second)
	flow.assertWorkersRunning(t)

	for index := range flow.workers {
		if value := flow.status(t, index); value.TurnsUsed != 1 {
			t.Fatalf("idle echo invocation: %+v", value)
		}
	}

	stats := processJSON[statistics](t, flow.fixture, flow.fixture.projects[0], "stats", "--all-projects", "--json")
	if stats.Messages != 3 {
		t.Fatalf("message echo: %+v", stats)
	}

	t.Log("idle evidence: zero initial turns; turn counts remain one per worker and three stored messages after completion")
}

func (flow *liveExchange) stopWorkers(t *testing.T) {
	t.Helper()

	for index, worker := range flow.workers {
		processJSON[bridgeBinding](t, flow.fixture, flow.fixture.projects[index], "bridge", "stop", "--json", flow.conversation.ID)

		select {
		case <-worker.exited:
			if worker.err != nil {
				t.Fatalf("bridge stop failed: %v %s", worker.err, worker.stderr.String())
			}

			value := decodeProcessJSON[bridgeBinding](t, worker.stdout.Bytes())
			if value.State != "stopped" {
				t.Fatalf("stopped=%+v", value)
			}
		case <-time.After(8 * time.Second):
			t.Fatal("owned bridge shutdown timed out")
		}
	}
}

func waitLiveInterval(t *testing.T, duration time.Duration) {
	t.Helper()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	case <-timer.C:
	}
}

func liveExchangeCompleted(alpha, beta bridgeBinding, initialID string) bool {
	return alpha.State == "acked" && alpha.TurnsUsed == 1 && beta.State == "acked" && beta.TurnsUsed == 1 && beta.InputMessageID != nil && *beta.InputMessageID != initialID
}
