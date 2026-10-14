# Slice 5: Codex Exchange

Status: drafting
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-5-iris-codex-exchange`
PR: pending

## Purpose

Deliver separate, explicitly started Codex workers for one project and Iris
conversation, including a real autonomous question/response/acknowledgment.

## Delivered Behavior

The foreground `iris bridge start` claims an exclusive project/conversation
binding. `bridge status` reports public progress and `bridge stop` requests
owned shutdown. A renewable lease and private token prevent concurrent or
stale owners from updating progress. Each binding has its own durable consumer.

## Implementation Notes

The daemon owns SQLite writes. Workers use typed local RPCs and the registered
canonical project directory. Safe idle restarts retain the binding and replace
the owner token. Expired invoking or failed work requires explicit recovery.
The bridge never attaches to an existing interactive Codex session.

The installed Codex app-server protocol adapter initializes a separate owned
process, creates or resumes only the bound thread, and reads completed final
agent output. Reasoning, progress, tools and unrelated threads cannot become
Iris responses. Unknown usage stays unavailable; confirmed usage is retained.

## Contracts Added or Changed

Migration 2 adds turn count, lifetime origin, and pending approval identity to
the existing binding table. Unknown usage remains null with an explanation.
Operator-selected model, sandbox, approvals, and finite budgets are validated
before ownership is acquired.

## Files of Interest

- `internal/iris/bridge_claim.go`: exclusive claim and stop/status persistence.
- `internal/iris/bridge_store.go`: scoped bindings and lease authorization.
- `internal/iris/codex_thread.go` and `codex_turn.go`: explicit thread/turn protocol.
- `internal/iris/bridge_cli.go`: explicit foreground command boundary.
- `internal/iris/migrations/002_bridge_progress.sql`: incremental progress schema.

## Validation

- Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestBridge|TestMigration' -count=1`.
- Passed: `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix --allow-serial-runners ./...`.
- Passed: `python3 scripts/check-contracts.py`.

- Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestCodex|TestBridge' -count=1`.

## Risks and Follow-ups

Interactive authority handling, atomic completion, and the real exchange are
remaining tasks in this slice. Initial ownership management does not establish
live model acceptance.
