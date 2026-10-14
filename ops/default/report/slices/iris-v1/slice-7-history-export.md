# Slice 7: History and Export

Status: reviewing
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-7-iris-history-export`
PR: pending

## Purpose

Deliver bounded historical discovery and complete explicit transcript exports
without changing active inbox or processing state.

## Delivered Behavior

Conversation listings default to registered project participation. Explicit
all-project scope supports administration outside a project. State filters
include pending, active, closed and expired history. Show reads immutable
participant labels and ordered message pages without creating consumers.
Complete text and JSON exports freeze lifecycle metadata, participant labels
and the highest sequence. Runtime leases block pruning. Private spools enforce
a 128 MiB ceiling and publish only after all bounded pages succeed. Failure,
cancellation, expiry or restart cannot publish a successful partial transcript.
Search combines literal case-sensitive text, project pair, inclusive creation
dates, ordinal, ID and state. Matching message IDs, sequences and bounded
excerpts support explicit inspection. Pair/ordinal selection is unambiguous.
Statistics report scoped state/message/body/retention counts and loaded policy,
with separately labeled service-wide SQLite page allocation.

## Implementation Notes

History uses serialized short transactions, deterministic creation-time/ID
keysets and scope-bound cursors. Message pages bound item count and wire size.
Pair filesystem selectors canonicalize before the transaction. No migration or
new dependency is required. The CLI checks contiguous export sequences and
completeness before publishing its spool.

## Contracts Added or Changed

`dump.open/page/close` use at most 16 runtime leases with a fixed 120-second
lifetime. Restart invalidates leases. `dump --format json` emits one complete
transcript object; `--json` wraps that object in the standard response envelope.
`conversations` and `show` use the existing versioned Unix protocol. Lists and
message pages default to 50 and permit up to 100 items, with explicit next
cursor and truncation fields.

## Files of Interest

- `internal/iris/history.go`: selection and historical metadata.
- `internal/iris/history_query.go`: participant filters and keyset pages.
- `internal/iris/history_messages.go`: independent message inspection.
- `internal/iris/dump.go`: serialized runtime lease and snapshot ownership.
- `internal/iris/dump_spool.go`: complete bounded transcript serialization.
- `internal/iris/statistics.go`: scoped counts and service allocation.
- `internal/iris/process_history_test.go`: actual-binary and jq acceptance.
- `docs/reference/history.md`: commands, filters, exports and statistics.

## Validation

Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestHistory' -count=1`.
Scope, retained-state access, cursors, consumer isolation and real Unix CLI
requests pass.

Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestDump|^TestHistory|^TestPrune' -count=1`. Export snapshots, wire/item pages,
lease bounds, retained data, private cleanup, expiry/restart and no partial
publication pass.

Passed: `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run '^TestProcessHistory|^TestHistoryStatistics|^TestHistorySearch|^TestHistorySelectors' -count=1`. Actual binaries and jq prove complete text/JSON
exports beyond 100 messages, retained restart discovery, default/admin scope,
clean machine output and unchanged activity/consumer state.

Ordered controller validation is pending on the clean candidate before PR
creation and must be regenerated after the PR-reference follow-up.

## Risks and Follow-ups

Export leases are runtime-only and expire after 120 seconds; a daemon restart
requires a fresh export. Literal substring search uses existing indexes to
narrow structured filters and scans candidate bodies rather than a full-text
index. Statistics report logical shared SQLite allocation, not physical WAL size.
