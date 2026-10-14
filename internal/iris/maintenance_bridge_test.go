package iris

import (
	"bytes"
	"testing"
	"time"
)

// A waiting owned bridge observes committed expiration and stops without starting Codex.
func TestMaintenanceStopsBridge(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)
	f.store.policy.InactivitySeconds = 1
	cfg, _ := testService(t, f.store.handle)
	claim := mustClaimBridge(t, f, f.alpha)
	t.Setenv("PATH", t.TempDir())

	app := NewApp(&bytes.Buffer{}, &bytes.Buffer{})
	done := make(chan error, 1)

	go func() { _, err := app.runBridgeOwner(t.Context(), cfg, claim); done <- err }()

	awaitWaiterCount(t, f.store, 1)
	startMaintenance(t, f.store)
	clock.advance(time.Second)
	f.store.notify(f.conversation.ID)

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("terminal bridge did not stop")
	}

	value, err := f.store.inspectBridge(t.Context(), closeReference(f.alpha, f.conversation.ID))
	if err != nil || value.State != "stopped" || value.StopRequestedAt == nil || value.TurnsUsed != 0 || value.ThreadID != nil {
		t.Fatalf("terminal binding=%+v, error=%v", value, err)
	}
}
