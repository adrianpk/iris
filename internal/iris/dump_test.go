package iris

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustOpenDump(t *testing.T, f deliveryFixture) dumpSnapshot {
	t.Helper()

	params := historyReference(f.alpha)
	params.ConversationID = f.conversation.ID

	value, err := f.store.openDump(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

// Snapshot metadata and the highest sequence stay fixed while live messages and lifecycle state change.
func TestDumpSnapshot(t *testing.T) {
	f := newDeliveryFixture(t)
	snapshot := mustOpenDump(t, f)
	mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, "after snapshot"))

	if _, err := f.store.closeConversation(t.Context(), closeReference(f.alpha, f.conversation.ID)); err != nil {
		t.Fatal(err)
	}

	page, err := f.store.pageDump(t.Context(), dumpLeaseParams{LeaseID: snapshot.LeaseID})
	if err != nil || page.Truncated || len(page.Messages) != 2 {
		t.Fatalf("page=%+v error=%v", page, err)
	}

	if snapshot.Conversation.State != "active" || snapshot.Conversation.TerminalAt != nil || snapshot.HighestSequence != 2 {
		t.Fatalf("snapshot=%+v", snapshot)
	}

	assertHistoryReadOnly(t, f)
	assertDumpRestart(t, f, snapshot)
}

func assertDumpRestart(t *testing.T, f deliveryFixture, snapshot dumpSnapshot) {
	t.Helper()

	reopened, err := openStore(t.Context(), f.config)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := reopened.close(); err != nil {
			t.Error(err)
		}
	})

	if _, err := reopened.pageDump(t.Context(), dumpLeaseParams{LeaseID: snapshot.LeaseID}); !hasCode(err, "export_expired") {
		t.Fatalf("restart=%v", err)
	}
}

// A live export blocks exact prune confirmation without changing retention or processing positions.
func TestDumpPruneLease(t *testing.T) {
	f, clock := retainedFixture(t)
	before := retainedState(t, f)
	preview := mustPreviewPrune(t, f.store, projectPruneParams(f))

	snapshot := mustOpenDump(t, f)
	if _, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token}); !hasCode(err, "preview_stale") {
		t.Fatalf("leased prune=%v", err)
	}

	if !reflect.DeepEqual(before, retainedState(t, f)) {
		t.Fatal("export renewed retention or changed processing")
	}

	clock.advance(exportLifetime)

	if _, err := f.store.pageDump(t.Context(), dumpLeaseParams{LeaseID: snapshot.LeaseID}); !hasCode(err, "export_expired") {
		t.Fatalf("expiry=%v", err)
	}

	fresh := mustOpenDump(t, f)
	if _, err := f.store.closeDump(t.Context(), dumpLeaseParams{LeaseID: fresh.LeaseID}); err != nil {
		t.Fatal(err)
	}

	next := mustPreviewPrune(t, f.store, projectPruneParams(f))
	if _, err := f.store.confirmPrune(t.Context(), pruneConfirmation{Token: next.Token}); err != nil {
		t.Fatal(err)
	}
}

// Lease acquisition is bounded, foreign cursors are rejected, and expired leases release capacity.
func TestDumpLeaseBounds(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)

	first := mustOpenDump(t, f)
	for range maxExportLeases - 1 {
		mustOpenDump(t, f)
	}

	params := historyReference(f.alpha)

	params.ConversationID = f.conversation.ID
	if _, err := f.store.openDump(t.Context(), params); !hasCode(err, "limit_exceeded") {
		t.Fatalf("lease cap=%v", err)
	}

	cursor, err := encodeHistorySequence("another-lease", 1)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.store.pageDump(t.Context(), dumpLeaseParams{LeaseID: first.LeaseID, Cursor: *cursor}); !hasCode(err, "invalid_cursor") {
		t.Fatalf("foreign cursor=%v", err)
	}

	clock.advance(120 * time.Second)
	mustOpenDump(t, f)
}

// Encoded response bytes, including escaped bodies, bound export pages independently of item limits.
func TestDumpWirePages(t *testing.T) {
	f := newDeliveryFixture(t)
	for range 6 {
		mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, strings.Repeat("\x01", maxMessageBodyBytes)))
	}

	snapshot := mustOpenDump(t, f)

	first, err := f.store.pageDump(t.Context(), dumpLeaseParams{LeaseID: snapshot.LeaseID})
	if err != nil || !first.Truncated || first.NextCursor == nil {
		t.Fatalf("truncated=%v error=%v", first.Truncated, err)
	}

	encoded, err := json.Marshal(operationResponse(t.Context(), first, nil))
	if err != nil || len(encoded) >= maxResponseBytes {
		t.Fatalf("wire bytes=%d error=%v", len(encoded), err)
	}

	second, err := f.store.pageDump(t.Context(), dumpLeaseParams{LeaseID: snapshot.LeaseID, Cursor: *first.NextCursor})
	if err != nil || second.Truncated || len(first.Messages)+len(second.Messages) != 8 {
		t.Fatalf("remaining=%d error=%v", len(second.Messages), err)
	}
}

func fillDumpHistory(t *testing.T, f deliveryFixture) {
	t.Helper()

	for i := range 110 {
		mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, fmt.Sprintf("history line %03d\nUnicode café", i)))
	}
}

// Export acquisition and exact prune confirmation serialize: either deletion wins or the lease protects every message.
func TestDumpPruneRace(t *testing.T) {
	for range 8 {
		f, _ := retainedFixture(t)
		preview := mustPreviewPrune(t, f.store, projectPruneParams(f))
		params := historyReference(f.alpha)
		params.ConversationID = f.conversation.ID
		start := make(chan struct{})

		var (
			workers           sync.WaitGroup
			snapshot          dumpSnapshot
			openErr, pruneErr error
		)

		workers.Go(func() { <-start; snapshot, openErr = f.store.openDump(t.Context(), params) })
		workers.Go(func() {
			<-start

			_, pruneErr = f.store.confirmPrune(t.Context(), pruneConfirmation{Token: preview.Token})
		})
		close(start)
		workers.Wait()

		assertDumpPruneOutcome(t, f, snapshot, openErr, pruneErr)
	}
}

func assertDumpPruneOutcome(t *testing.T, f deliveryFixture, snapshot dumpSnapshot, openErr, pruneErr error) {
	t.Helper()

	if openErr != nil {
		if !hasCode(openErr, "not_found") || pruneErr != nil {
			t.Fatalf("open=%v prune=%v", openErr, pruneErr)
		}

		return
	}

	if !hasCode(pruneErr, "preview_stale") {
		t.Fatalf("leased prune=%v", pruneErr)
	}

	if _, err := f.store.pageDump(t.Context(), dumpLeaseParams{LeaseID: snapshot.LeaseID}); err != nil {
		t.Fatal(err)
	}
}
