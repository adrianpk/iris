package iris

import (
	"bytes"
	"testing"
	"time"
)

// An idle foreground worker acknowledges system events without even starting the model executable.
func TestBridgeIdleOwner(t *testing.T) {
	f := newDeliveryFixture(t)
	cfg, _ := testService(t, f.store.handle)
	claim := mustClaimBridge(t, f, f.alpha)
	t.Setenv("PATH", t.TempDir())

	app := NewApp(&bytes.Buffer{}, &bytes.Buffer{})
	done := make(chan error, 1)

	go func() { _, err := app.runBridgeOwner(t.Context(), cfg, claim); done <- err }()

	awaitWaiterCount(t, f.store, 1)

	value, err := f.store.inspectBridge(t.Context(), closeReference(f.alpha, f.conversation.ID))
	if err != nil || value.ThreadID != nil || value.TurnsUsed != 0 || value.State != "acked" {
		t.Fatalf("idle=%+v, error=%v", value, err)
	}

	if _, err := f.store.stopBridge(t.Context(), closeReference(f.alpha, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("owned worker did not stop")
	}

	awaitWaiterCount(t, f.store, 0)
}
