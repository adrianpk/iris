# Slice 6: Lifecycle and Retention

Status: delivered
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-6-iris-lifecycle-retention`
PR: [#6](https://forge.adrianpk.com/adrianpk/iris/pulls/6)

## Purpose

Deliver daemon-owned expiration, retained terminal history, and deletion only
after an exact retention preview is explicitly confirmed.

## Delivered Behavior

Startup loads bounded invitation, inactivity and retention durations with
2-hour, 24-hour and 30-day defaults. Issued invitation deadlines remain fixed.
Current daemon policy controls inactivity and retention. New calls, first joins
and new participant message/status inserts renew activity; reads, waits,
acknowledgments, heartbeats, duplicate retries and maintenance do not.

The daemon reconciles overdue state before readiness and expires conversations
without an agent. Terminal state, its system event and bridge shutdown markers
commit together. Receivers wake after commit. Bridges acknowledge reachable
informational events without a model before stopping, preserving unfinished
ordinary work. Terminal histories remain stored through retention.

Bare prune previews without changing history or consumer progress. Explicit
confirmation revalidates exact membership, revisions, terminal times, current
policy/eligibility, token lifetime and export leases before deleting. Changed,
missing, newly ineligible or leased candidates reject the whole selection.
Newly eligible history is never silently added. Deletion and one-use token
consumption commit together while permanent project/pair/code/request
allocations survive.

## Implementation Notes

The store's injected clock owns durable UTC millisecond timestamps, including
migrations. Maintenance uses at most 128 transitions per transaction and the
nearest deadline with a maximum 30-second recheck. Cancellation joins its owned
loop; maintenance failure stops the service. No model performs lifecycle work.

Prune defaults to project scope; explicit all-project scope supports human
administration. Previews contain a complete selection of at most 1,000 candidates,
sizes, cutoff and a random 120-second token. At most 64 live previews exist.
Confirmation is noninteractive and cannot accept a changed selection. SQLite
rollback preserves all history and the token if any deletion fails.

## Contracts Added or Changed

Version-1 configuration accepts a partial `lifecycle` object containing
`invitation_seconds`, `inactivity_seconds` and `retention_seconds`, each from
1 through 315360000. Policy loads at daemon startup. `prune.preview` and
`prune.confirm` use the existing versioned private protocol and bounded
preview/candidate tables. Export-lease guards share serialized store
transactions; lease acquisition and public exports belong to the history slice.

## Files of Interest

- `internal/iris/lifecycle_policy.go`: defaults, bounds and duration conversion.
- `internal/iris/maintenance.go`: scheduling, reconciliation and shutdown.
- `internal/iris/maintenance_expire.go`: atomic bounded expiration batches.
- `internal/iris/prune_confirm.go`: revalidation and transactional deletion.
- `internal/iris/prune_preview.go`: exact membership and confirmation metadata.
- `internal/iris/process_lifecycle_test.go`: real daemon/CLI retention workflow.
- `docs/reference/lifecycle.md`: policy, scope, preview and confirmation.

## Validation

Passed focused commands:

- `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run 'TestLifecycle|TestInvitation|TestConfiguration|TestJoin|TestSend' -count=1`
- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestPrune|TestMaintenance|TestLifecycle|TestBridge' -count=1`
- `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run 'TestProcess.*Expiration|TestProcessLifecycle|TestLifecycleFresh' -count=1`
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix --allow-serial-runners --build-tags=integration,bridge ./...`

Evidence covers exact boundaries, significant-activity rules, independent timer
expiry, multi-batch reconciliation, receive/bridge notification, failed-event
rollback, exact-deadline races, complete preview caps, stale revision/time/policy/
membership/export-lease rejection, deletion rollback, one-use tokens, project
scope, noninteractive IPC, human/JSON output and preserved allocations.
A deterministic test reproduced the terminal-stop race before the fix and now
requires the terminal event's processing acknowledgment without a model turn.

Actual-binary fixtures also prove readiness reconciliation before stateful
requests, retained metadata/messages while the daemon is stopped, policy reload,
prune/restart, and fresh conversation context. Clock rollback cannot reopen
terminal state or cause ordinal reuse. All seven ordered PR checks passed through Looper on clean candidate
`f802b42de7d94253aa3545ae382f9d6c74dc312b`:

- `make check`
- `make lint`
- `make docs-check`
- `python3 scripts/check-contracts.py`
- `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -count=1`
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --allow-serial-runners --build-tags=integration,bridge ./...`
- `git diff --check`

The complete integration-tag race check passed in 42.316 seconds. The controller
integrated canonical PR #6 into `dev` at that same validated head using Rebase
+ Fast-forward. Its exact ordered evidence is retained in the controller ledger.

## Risks and Follow-ups

Public history discovery and exports remain assigned to the next recorded slice.
Current evidence confirms retained data through store/offline inspection.
Active bridge shutdown is observed on its five-second ownership renewal;
full recovery and execution cleanup acceptance remain in the recovery slice.
