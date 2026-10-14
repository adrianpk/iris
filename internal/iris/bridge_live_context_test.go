//go:build bridge

package iris

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func (flow *liveExchange) verifyFreshConversation(t *testing.T) {
	t.Helper()

	prior := [2]bridgeBinding{flow.status(t, 0), flow.status(t, 1)}
	oldID, oldProof := flow.conversation.ID, flow.proof
	processJSON[conversation](t, flow.fixture, flow.fixture.projects[0], "close", "--json", oldID)

	retained := processJSON[transcript](t, flow.fixture, flow.fixture.projects[0], "dump", "--json", oldID)
	if len(retained.Messages) != 6 {
		t.Fatal("prior live history was not retained")
	}

	proof, err := newID()
	if err != nil {
		t.Fatal(err)
	}

	flow.proof = "format=JSON; proof=" + proof
	if err := os.WriteFile(filepath.Join(flow.fixture.projects[1], "answer.txt"), []byte(flow.proof), 0o600); err != nil {
		t.Fatal(err)
	}

	flow.conversation = processJSON[conversation](t, flow.fixture, flow.fixture.projects[0], "call", "--to", "beta", "--topic", "Fresh conversation", "--message", "Read answer.txt in your own project directory. Reply with its exact complete contents and no prefix. This new conversation needs only its own request and file; a completed answer needs acknowledgment only.", "--json")
	if flow.conversation.ID == oldID || flow.conversation.Ordinal != 2 {
		t.Fatal("fresh conversation reused an identity or ordinal")
	}

	processJSON[conversation](t, flow.fixture, flow.fixture.projects[1], "join", "--json", flow.conversation.InvitationCode)

	flow.workers[0] = flow.startWorker(t, 0)
	flow.assertInitialIdle(t)

	flow.workers[1] = flow.startWorker(t, 1)
	flow.awaitExchange(t)
	flow.verifyExchange(t)
	flow.assertFreshBindings(t, prior)

	flow.assertFreshTranscripts(t, oldID, oldProof, retained)

	waitLiveInterval(t, time.Second)
	flow.assertFreshBindings(t, prior)
	flow.stopWorkers(t)
	t.Log("fresh-context acceptance: new conversation/ordinal, two new owned threads and consumers, new project-local proof, retained prior history, one turn per worker and no idle echo")
}

func (flow *liveExchange) assertFreshBindings(t *testing.T, prior [2]bridgeBinding) {
	t.Helper()

	for index := range flow.workers {
		current := flow.status(t, index)
		if current.ThreadID == nil || current.TurnsUsed != 1 {
			t.Fatal("fresh worker did not complete exactly one turn")
		}

		for _, old := range prior {
			if old.ThreadID == nil || *old.ThreadID == *current.ThreadID || old.ConsumerID == current.ConsumerID {
				t.Fatal("fresh worker inherited an earlier thread or consumer")
			}
		}
	}
}

func (flow *liveExchange) assertFreshTranscripts(t *testing.T, oldID, oldProof string, retained transcript) {
	t.Helper()

	current := processJSON[transcript](t, flow.fixture, flow.fixture.projects[0], "dump", "--json", flow.conversation.ID)
	if len(current.Messages) != 3 {
		t.Fatal("fresh history contains unexpected messages")
	}

	for _, message := range current.Messages {
		if message.ConversationID != flow.conversation.ID || strings.Contains(message.Body, oldProof) {
			t.Fatal("fresh transcript inherited prior conversation content")
		}
	}

	unchanged := processJSON[transcript](t, flow.fixture, flow.fixture.projects[0], "dump", "--json", oldID)
	if len(unchanged.Messages) != len(retained.Messages) || unchanged.Messages[4].Body != oldProof {
		t.Fatal("new context changed retained history")
	}
}
