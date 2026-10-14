# Slice 7: History and Export

Status: drafting
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

## Implementation Notes

History uses serialized short transactions, deterministic creation-time/ID
keysets and scope-bound cursors. Message pages bound item count and wire size.

## Contracts Added or Changed

`conversations` and `show` use the existing versioned Unix protocol. Lists and
message pages default to 50 and permit up to 100 items, with explicit next
cursor and truncation fields.

## Files of Interest

- `internal/iris/history.go`: selection and historical metadata.
- `internal/iris/history_query.go`: participant filters and keyset pages.
- `internal/iris/history_messages.go`: independent message inspection.

## Validation

Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestHistory' -count=1`.
Scope, retained-state access, cursors, consumer isolation and real Unix CLI
requests pass. Slice-level validation remains pending during implementation.

## Risks and Follow-ups

Search, exports and extended statistics remain in this slice's recorded tasks.
