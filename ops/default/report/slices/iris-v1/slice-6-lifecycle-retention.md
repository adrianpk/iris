# Slice 6: Lifecycle and Retention

Status: drafting
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-6-iris-lifecycle-retention`
PR: pending

## Purpose

Deliver daemon-owned expiration, retained terminal history, and deletion only
after an exact retention preview is explicitly confirmed.

## Delivered Behavior

Startup loads validated invitation, inactivity and retention durations with
2-hour, 24-hour and 30-day defaults. Issued invitation deadlines remain fixed.
Current daemon policy controls inactivity and retention. Reads, waits and
acknowledgments do not renew activity.

The daemon reconciles overdue state before readiness and expires pending or
active conversations without an agent. Work is bounded to 128 transitions per
transaction; timers use the nearest deadline with a 30-second maximum recheck.
Terminal state, its system event and bridge shutdown markers commit together;
only committed transitions notify waiters.

Bare prune previews terminal history without changing it. Explicit confirmation
revalidates exact membership, revisions, terminal times, policy, cutoff, token
lifetime and export leases before deleting. New eligibility is not silently
added. The token is consumed atomically with deletion; retained allocation and
code/request tombstones survive. Previews cap at 1,000 candidates and 64 live
120-second tokens. A terminal bridge drains reachable informational events
before stopping, without processing unfinished ordinary work.

## Implementation Notes

The store's injected clock owns durable timestamps, including migration
application. New calls, successful joins and new participant message/status
inserts are significant activity; duplicate operations are not.

## Contracts Added or Changed

Version-1 configuration accepts a partial `lifecycle` object with
`invitation_seconds`, `inactivity_seconds` and `retention_seconds`, each from
1 through 315360000. Policy is immutable during a daemon run.

## Files of Interest

- `internal/iris/prune_confirm.go`: transactional revalidation and deletion.
- `internal/iris/prune_preview.go`: bounded exact membership and tokens.
- `internal/iris/prune_cli.go`: default preview and noninteractive confirmation.
- `docs/reference/lifecycle.md`: policy and operation contracts.
- `internal/iris/maintenance.go`: bounded scheduling, startup and shutdown.
- `internal/iris/maintenance_expire.go`: transactional expiration batches.
- `internal/iris/lifecycle_policy.go`: defaults, bounds and durations.
- `internal/iris/conversation_lifecycle.go`: configured expiration decisions.
- `internal/iris/call_create.go`: persisted invitation deadlines and quotas.

## Validation

- Passed: `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run 'TestLifecycle|TestInvitation|TestConfiguration|TestJoin|TestSend' -count=1`.
- Passed: `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix --allow-serial-runners ./...`.

- Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestMaintenance|TestLifecycle|TestBridgeIdle|TestCallBounds' -count=1`.

- Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestPrune|TestMaintenance|TestLifecycle|TestBridge' -count=1`.

## Risks and Follow-ups

Process-level retention acceptance
remain tasks in this slice.
