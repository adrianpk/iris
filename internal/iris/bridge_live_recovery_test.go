//go:build bridge

package iris

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func (flow *liveExchange) startFaultedWorker(t *testing.T) *ownedProcess {
	t.Helper()

	var failed atomic.Bool

	cfg, _ := testService(t, func(ctx context.Context, req request) response {
		if req.Operation == "bridge.record" && failed.CompareAndSwap(false, true) {
			return failure("storage_unavailable", "fixture dropped completed result checkpoint", false)
		}

		reply, err := callService(ctx, flow.fixture.config, req)
		if err != nil {
			return failure("storage_unavailable", "fixture forwarding failed", false)
		}

		return reply
	})
	env := append(append([]string(nil), flow.fixture.env...), "XDG_RUNTIME_DIR="+filepath.Dir(cfg.runtimeRoot))

	return flow.startBridgeWorker(t, 1, flow.workerArguments(), env)
}

func (flow *liveExchange) awaitRecoveryFailure(t *testing.T) {
	t.Helper()

	select {
	case <-flow.workers[1].exited:
		if flow.workers[1].err == nil {
			t.Fatal("lost result checkpoint did not fail the worker")
		}
	case <-time.After(130 * time.Second):
		t.Fatal("faulted real worker did not stop")
	}

	value := flow.status(t, 1)
	assertLiveRecoveryCheckpoint(t, value)

	page := processJSON[receivePage](t, flow.fixture, flow.fixture.projects[1], "receive", "--consumer", value.ConsumerID, "--wait", "0", "--json", flow.conversation.ID)
	if page.AckedSequence != 0 || page.Messages[0].ID != flow.conversation.InitialMessage.ID {
		t.Fatal("unrecorded result acknowledged input")
	}

	t.Log("real recovery checkpoint: completed provider turn, lost result delivery, unacknowledged input, durable thread/turn identity")
}

func (flow *liveExchange) verifyRestartExchange(t *testing.T) {
	t.Helper()

	prior := [2]bridgeBinding{flow.status(t, 0), flow.status(t, 1)}

	proof, err := newID()
	if err != nil {
		t.Fatal(err)
	}

	flow.proof = "format=JSON; proof=" + proof
	if err := os.WriteFile(filepath.Join(flow.fixture.projects[1], "answer.txt"), []byte(flow.proof), 0o600); err != nil {
		t.Fatal(err)
	}

	input := processJSON[message](t, flow.fixture, flow.fixture.projects[0], "send", "--message", "Read answer.txt again: it now contains a new proof. Reply with its exact complete contents and no prefix. This new question requires a fresh file read; the completed answer needs acknowledgment only.", "--json", flow.conversation.ID)
	for index := range flow.workers {
		flow.workers[index] = flow.startWorker(t, index)
	}

	flow.awaitRestartExchange(t, input.ID)

	flow.assertResumedBindings(t, prior)
	flow.verifyRestartTranscript(t, input)
	waitLiveInterval(t, 500*time.Millisecond)

	for index := range flow.workers {
		if flow.status(t, index).TurnsUsed != 2 {
			t.Fatal("restart caused idle model invocation")
		}
	}

	flow.stopWorkers(t)
	t.Log("real restart: both workers resumed their original threads/consumers, processed pending new work once, retained budgets and generated no idle turns")
}

func (flow *liveExchange) awaitRestartExchange(t *testing.T, input string) {
	t.Helper()

	deadline := time.NewTimer(150 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		flow.assertWorkersRunning(t)

		alpha, beta := flow.status(t, 0), flow.status(t, 1)
		if alpha.State == "acked" && beta.State == "acked" && alpha.TurnsUsed == 2 && beta.TurnsUsed == 2 && beta.InputMessageID != nil && *beta.InputMessageID == input {
			return
		}

		select {
		case <-deadline.C:
			t.Fatalf("resumed exchange timed out: alpha=%+v beta=%+v", alpha, beta)
		case <-ticker.C:
		}
	}
}

func (flow *liveExchange) verifyRestartTranscript(t *testing.T, input message) {
	t.Helper()

	value := processJSON[transcript](t, flow.fixture, flow.fixture.projects[0], "dump", "--json", flow.conversation.ID)
	if len(value.Messages) != 5 {
		t.Fatalf("restart transcript count=%d", len(value.Messages))
	}

	reply := value.Messages[4]
	if reply.Body != flow.proof || reply.ReplyTo == nil || *reply.ReplyTo != input.ID {
		t.Fatal("resumed response does not match the fresh project proof")
	}
}

type idleResources struct {
	ticks uint64
	rss   int64
}

func (flow *liveExchange) idleResources(t *testing.T) idleResources {
	t.Helper()

	result := idleResources{}

	for _, process := range []*ownedProcess{flow.service, flow.workers[0], flow.workers[1]} {
		value := readIdleProcess(t, process.cmd.Process.Pid)
		result.ticks += value.ticks
		result.rss += value.rss
	}

	return result
}

func readIdleProcess(t *testing.T, pid int) idleResources {
	t.Helper()

	root := "/proc/" + strconv.Itoa(pid)

	data, err := os.ReadFile(root + "/stat")
	if err != nil {
		t.Fatal(err)
	}

	fields := strings.Fields(string(data)[strings.LastIndex(string(data), ")")+1:])

	user, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	system, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	status, err := os.ReadFile(root + "/status")
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			rss, err := strconv.ParseInt(strings.Fields(line)[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}

			return idleResources{ticks: user + system, rss: rss}
		}
	}

	t.Fatal("owned process RSS is unavailable")

	return idleResources{}
}

func assertLiveRecoveryCheckpoint(t *testing.T, value bridgeBinding) {
	t.Helper()

	if value.State != "uncertain" || value.ThreadID == nil || value.TurnID == nil || value.TurnsUsed != 1 || value.InputMessageID == nil {
		t.Fatalf("fault checkpoint=%+v", value)
	}
}

func (flow *liveExchange) assertResumedBindings(t *testing.T, prior [2]bridgeBinding) {
	t.Helper()

	for index, old := range prior {
		value := flow.status(t, index)
		if value.ID != old.ID || value.ConsumerID != old.ConsumerID || value.ThreadID == nil || *value.ThreadID != *old.ThreadID || value.TurnsUsed != 2 || !value.StartedAt.Equal(old.StartedAt) {
			t.Fatalf("resumed binding=%+v prior=%+v", value, old)
		}
	}
}
