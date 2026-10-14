package iris

import (
	"testing"
	"time"
)

// Restart cannot reset persisted limits, authority, turn count or lifetime origin.
func TestBridgeRestartLimits(t *testing.T) {
	f := newDeliveryFixture(t)
	claim := mustClaimBridge(t, f, f.beta)

	owner := bridgeOwner{BindingID: claim.Binding.ID, OwnerToken: claim.OwnerToken}
	if _, err := f.store.checkpointBridge(t.Context(), bridgeCheckpoint{bridgeOwner: owner, Action: "stop"}); err != nil {
		t.Fatal(err)
	}

	changed := bridgeReference(f.beta, f.conversation.ID)

	changed.Options.MaxTurns++
	if _, err := f.store.claimBridge(t.Context(), changed); !hasCode(err, "invalid_request") {
		t.Fatalf("changed limits=%v", err)
	}

	recovered, err := f.store.recoverBridgeClaim(t.Context(), closeReference(f.beta, f.conversation.ID))
	if err != nil || recovered.Binding.Options != claim.Binding.Options || !recovered.Binding.StartedAt.Equal(claim.Binding.StartedAt) {
		t.Fatalf("recovered=%+v %v", recovered, err)
	}
}

// Missing telemetry stays null while the last observed cumulative floor still protects the next invocation.
func TestBridgeUsageFloor(t *testing.T) {
	f := newDeliveryFixture(t)
	claim, input := invokingBridge(t, f)
	usage := int64(40)
	params := completionReference(claim, input)

	params.UsageTokens = &usage
	if _, err := f.store.completeBridge(t.Context(), params); err != nil {
		t.Fatal(err)
	}

	info := mustBridgeInput(t, f, claim)
	if _, err := f.store.completeBridge(t.Context(), bridgeCompletion{bridgeOwner: params.bridgeOwner, InputMessageID: info.ID, Answer: bridgeAnswer{Action: "ack"}}); err != nil {
		t.Fatal(err)
	}

	next := mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, "next bounded request"))
	mustBridgeInput(t, f, claim)

	for _, checkpoint := range []bridgeCheckpoint{{Action: "invoke", InputMessageID: next.ID}, {Action: "turn", TurnID: "second-turn"}} {
		checkpoint.bridgeOwner = params.bridgeOwner
		if _, err := f.store.checkpointBridge(t.Context(), checkpoint); err != nil {
			t.Fatal(err)
		}
	}

	params.InputMessageID, params.TurnID, params.UsageTokens = next.ID, "second-turn", nil

	value, err := f.store.completeBridge(t.Context(), params)
	if err != nil || value.UsageTokens != nil || value.UsageLimitation == nil {
		t.Fatalf("usage=%+v %v", value, err)
	}

	assertUsageFloor(t, f, claim, 40)
}

func assertUsageFloor(t *testing.T, f deliveryFixture, claim bridgeClaim, want int64) {
	t.Helper()

	value, err := readBridge(t.Context(), f.store.db, "b.id=?", claim.Binding.ID)
	if err != nil || value.usageFloor != want {
		t.Fatalf("floor=%+v %v", value, err)
	}

	value.Options.MaxTokens = want
	if err := f.store.bridgeBudget(value); !hasCode(err, "limit_exceeded") {
		t.Fatalf("missing telemetry bypassed floor: %v", err)
	}
}

// A saved completed result remains publishable after execution time is exhausted, without starting a model.
func TestBridgeSavedBudgetRecovery(t *testing.T) {
	f := newDeliveryFixture(t)
	clock := useLifecycleClock(f)

	claim, input := invokingBridge(t, f)
	if _, err := f.store.recordBridgeResult(t.Context(), completionReference(claim, input)); err != nil {
		t.Fatal(err)
	}

	if _, err := f.store.db.ExecContext(t.Context(), "UPDATE bridge_bindings SET started_at=? WHERE id=?", time.Now().Add(-time.Hour).UnixMilli(), claim.Binding.ID); err != nil {
		t.Fatal(err)
	}

	clock.advance(bridgeLease)

	recovered := mustClaimBridge(t, f, f.beta)
	cfg, _ := testService(t, f.store.handle)
	t.Setenv("PATH", t.TempDir())

	value, err := NewApp(nil, nil).runBridgeOwner(t.Context(), cfg, recovered)
	if !hasCode(err, "limit_exceeded") || value.State != "stopped" {
		t.Fatalf("budget recovery=%+v %v", value, err)
	}

	assertBridgePosition(t, f, claim, 1, 3)
}

// The global owner bound rejects a fifth bridge before allocating its consumer or invoking anything.
func TestBridgeOwnerCapacity(t *testing.T) {
	f := newDeliveryFixture(t)
	for i, key := range []string{"initial", "second", "third"} {
		active := f.conversation
		if i > 0 {
			pending := mustCall(t, f.store, callFixture(t, f.store, f.alpha, f.beta.ID, key))
			active = mustJoin(t, f.store, joinParams{projectSelection: projectSelection{Project: f.beta.ID}, Code: pending.InvitationCode})
		}

		for _, selected := range []project{f.alpha, f.beta} {
			_, err := f.store.claimBridge(t.Context(), bridgeReference(selected, active.ID))
			if i == 2 {
				if !hasCode(err, "limit_exceeded") {
					t.Fatalf("capacity=%v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		}
	}

	var count int
	if err := f.store.db.QueryRowContext(t.Context(), "SELECT count(*) FROM consumer_cursors").Scan(&count); err != nil || count != 4 {
		t.Fatalf("consumers=%d %v", count, err)
	}
}

// An exhausted budget rejects the next addressed message before starting any provider process.
func TestBridgeBudgetBeforeModel(t *testing.T) {
	f := newDeliveryFixture(t)
	claim, input := invokingBridge(t, f)
	params := completionReference(claim, input)
	usage := int64(100000)
	params.UsageTokens = &usage
	if _, err := f.store.completeBridge(t.Context(), params); err != nil {
		t.Fatal(err)
	}

	info := mustBridgeInput(t, f, claim)
	if _, err := f.store.completeBridge(t.Context(), bridgeCompletion{bridgeOwner: params.bridgeOwner, InputMessageID: info.ID, Answer: bridgeAnswer{Action: "ack"}}); err != nil {
		t.Fatal(err)
	}

	mustSend(t, f.store, sendReference(f.alpha, f.conversation.ID, "must remain pending"))

	cfg, _ := testService(t, f.store.handle)
	worker := bridgeWorker{config: cfg, owner: params.bridgeOwner}

	t.Setenv("PATH", t.TempDir())

	if err := worker.run(t.Context()); !hasCode(err, "limit_exceeded") {
		t.Fatalf("budget=%v", err)
	}

	if worker.client != nil {
		t.Fatal("exhausted budget started provider")
	}

	assertBridgePosition(t, f, claim, 2, 4)
}
