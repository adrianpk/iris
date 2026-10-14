package iris

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func drainConsumer(t *testing.T, f deliveryFixture, consumer string) {
	t.Helper()

	page := mustReceive(t, f.store, receiveReference(f.beta, f.conversation.ID, consumer))
	for _, value := range page.Messages {
		mustAck(t, f.store, ackReference(f.beta, value, consumer))
	}
}

func waiterCount(s *store) int {
	s.events.mu.Lock()
	defer s.events.mu.Unlock()

	return len(s.events.waiters)
}

func awaitWaiterCount(t *testing.T, s *store, want int) {
	t.Helper()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for waiterCount(s) != want {
		select {
		case <-timer.C:
			t.Fatalf("waiters=%d want %d", waiterCount(s), want)
		case <-ticker.C:
		}
	}
}

// Rechecking after subscription closes the check/subscribe gap, and commit notifications wake blocked reads.
func TestWaitCommit(t *testing.T) {
	f := newDeliveryFixture(t)
	drainConsumer(t, f, "manual")

	params := receiveReference(f.beta, f.conversation.ID, "manual")
	page := mustReceive(t, f.store, params)
	mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, "Before subscription"))

	plan, err := f.store.planSelection(t.Context(), params.projectSelection, false)
	if err != nil {
		t.Fatal(err)
	}

	params.WaitSeconds = 60
	params.Limit = 50

	found, err := f.store.waitForDelivery(t.Context(), params, plan, page)
	if err != nil || len(found.Messages) != 1 {
		t.Fatalf("lost gap commit: %+v error=%v", found, err)
	}

	mustAck(t, f.store, ackReference(f.beta, found.Messages[0], "manual"))

	done := make(chan error, 1)

	go func() {
		page, err := f.store.receive(t.Context(), params)
		if err == nil && len(page.Messages) != 1 {
			err = fmt.Errorf("missing notification result")
		}

		done <- err
	}()

	awaitWaiterCount(t, f.store, 1)

	writeCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if _, err := f.store.send(writeCtx, sendReference(f.alpha, f.conversation.ID, "After subscription")); err != nil {
		t.Fatalf("wait blocked writer: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("commit did not wake receiver")
	}

	awaitWaiterCount(t, f.store, 0)
}

// Closure and cancellation release waiters immediately; empty waits stop at their bounded timeout.
func TestWaitTermination(t *testing.T) {
	for _, mode := range []string{"close", "cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) { exerciseWaitTermination(t, mode) })
	}
}

func exerciseWaitTermination(t *testing.T, mode string) {
	t.Helper()

	f := newDeliveryFixture(t)
	drainConsumer(t, f, "manual")

	params := receiveReference(f.beta, f.conversation.ID, "manual")
	params.WaitSeconds = 1

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		page, err := f.store.receive(ctx, params)
		if err == nil && len(page.Messages) != 0 {
			err = fmt.Errorf("unexpected messages")
		}

		done <- err
	}()

	awaitWaiterCount(t, f.store, 1)

	switch mode {
	case "close":
		mustClose(t, f.store, f.alpha, f.conversation.ID)
	case "cancel":
		cancel()
	}

	select {
	case err := <-done:
		assertWaitEnd(t, mode, err)
	case <-time.After(3 * time.Second):
		t.Fatal("wait did not terminate")
	}

	awaitWaiterCount(t, f.store, 0)
}

func assertWaitEnd(t *testing.T, mode string, err error) {
	t.Helper()

	switch mode {
	case "close":
		if !hasCode(err, "conversation_inactive") {
			t.Fatalf("terminal wait=%v", err)
		}
	case "cancel":
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel=%v", err)
		}
	case "timeout":
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Full Unix disconnect cancels a long wait, while the protocol's write half-close keeps it alive.
func TestWaitDisconnect(t *testing.T) {
	f := newDeliveryFixture(t)
	drainConsumer(t, f, "manual")

	cfg, _ := testService(t, f.store.handle)

	connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: cfg.socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	}()

	params := receiveReference(f.beta, f.conversation.ID, "manual")
	params.WaitSeconds = 60

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeFrame(connection, request{Protocol: protocolVersion, Operation: "receive", Params: encoded}, maxRequestBytes); err != nil {
		t.Fatal(err)
	}

	if err := connection.CloseWrite(); err != nil {
		t.Fatal(err)
	}

	awaitWaiterCount(t, f.store, 1)

	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	awaitWaiterCount(t, f.store, 0)

	if _, err := callService(t.Context(), cfg, statsRequest()); err != nil {
		t.Fatalf("service after disconnect=%v", err)
	}
}

// Per-project and global waiter capacity is reclaimed after removal, with no idle goroutines.
func TestWaitCapacity(t *testing.T) {
	var hub waitHub

	subscriptions := []*waitSubscription{}

	for index := range 64 {
		value, err := hub.subscribe("conversation", fmt.Sprintf("project-%d", index/16))
		if err != nil {
			t.Fatal(err)
		}

		subscriptions = append(subscriptions, value)
	}

	for _, project := range []string{"project-0", "project-4"} {
		if _, err := hub.subscribe("conversation", project); !hasCode(err, "server_busy") {
			t.Fatalf("waiter capacity=%v", err)
		}
	}

	hub.remove(subscriptions[63])

	if _, err := hub.subscribe("other", "project-0"); !hasCode(err, "server_busy") {
		t.Fatalf("per-project capacity=%v", err)
	}

	for _, subscription := range subscriptions {
		hub.remove(subscription)
	}

	if len(hub.waiters) != 0 {
		t.Fatalf("retained waiters=%d", len(hub.waiters))
	}
}
