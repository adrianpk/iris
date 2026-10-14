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

## Implementation Notes

The store's injected clock owns durable timestamps, including migration
application. New calls, successful joins and new participant message/status
inserts are significant activity; duplicate operations are not.

## Contracts Added or Changed

Version-1 configuration accepts a partial `lifecycle` object with
`invitation_seconds`, `inactivity_seconds` and `retention_seconds`, each from
1 through 315360000. Policy is immutable during a daemon run.

## Files of Interest

- `internal/iris/lifecycle_policy.go`: defaults, bounds and durations.
- `internal/iris/conversation_lifecycle.go`: configured expiration decisions.
- `internal/iris/call_create.go`: persisted invitation deadlines and quotas.

## Validation

- Passed: `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run 'TestLifecycle|TestInvitation|TestConfiguration|TestJoin|TestSend' -count=1`.
- Passed: `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix --allow-serial-runners ./...`.

## Risks and Follow-ups

Autonomous maintenance, confirmed pruning and process-level retention acceptance
remain tasks in this slice.
