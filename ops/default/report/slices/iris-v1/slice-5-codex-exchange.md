# Slice 5: Codex Exchange

Status: reviewing
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-5-iris-codex-exchange`
PR: [#5](https://forge.adrianpk.com/adrianpk/iris/pulls/5)

## Purpose

Deliver separate, explicitly started Codex workers for one project and Iris
conversation, including a real autonomous question/response/acknowledgment.

## Delivered Behavior

`iris bridge start` runs a foreground worker with exclusive project/conversation
ownership. Status exposes public progress; stop requests owned shutdown. Each
binding keeps a private consumer, selected limits, thread and input/result
progress. Idle workers wait on service notifications without starting Codex.

Ordinary inputs run sequentially in the registered project directory. A fresh
binding creates a fresh Codex thread; safe idle restart resumes only that bound
thread. Completed final agent output becomes a technical reply or acknowledgment
without another message. System/status inputs never invoke the model.

Replies and input acknowledgment commit atomically with a stable response key.
Failed or denied work remains unacknowledged. Duplicate completion cannot
replace or duplicate a result. Closure and loss of ownership prevent publication.
Turn, lifetime and available reported-token bounds block further execution.

## Implementation Notes

The daemon remains the normal SQLite writer. Fifteen-second token-checked leases
renew every five seconds, with at most four concurrent owners. Thread and turn
requests carry the selected sandbox and approval policy. The adapter uses the
installed Codex 0.161.0 app-server protocol and separate owned processes; it never
attaches to a pre-existing interactive session or imports unrelated history.

Command/file approvals default to decline. Interactive mode requires a connected
local terminal and exact once-only operator acceptance. Pending request identity
is durable before answering. Additional permission profiles and unknown/foreign
requests cannot grant authority. Reasoning, tool output and progress cannot
become replies. Active cancellation requires owned terminal interruption proof;
unconfirmed execution is uncertain and cannot be automatically replayed.

## Contracts Added or Changed

Migration 2 adds durable turn count, activation time and pending approval identity
while preserving migration 1. Owner-only typed receive, checkpoint, renew and
completion RPCs share the existing bounded transport and wait hub. Missing
provider usage stays null with a limitation instead of reporting zero. A reported
usage bound blocks the next turn and is not a hard provider token ceiling.

## Files of Interest

- `internal/iris/bridge_complete.go`: owned atomic reply and acknowledgment.
- `internal/iris/bridge_exchange.go`: bounded conversation worker loop.
- `internal/iris/bridge_store.go`: scoped bindings and lease authorization.
- `internal/iris/codex_thread.go` and `codex_turn.go`: thread/turn protocol.
- `internal/iris/codex_approval.go`: pending identity and explicit decisions.
- `internal/iris/bridge_live_test.go`: actual Codex exchange and idle assertions.
- `docs/reference/bridge.md`: operation, authority, budgets and recovery limits.

## Validation

All eight ordered commands passed through Looper on clean implementation
candidate `3235973b8edc63ba07efa8b7a7f20d566857f080`:

- `make check`
- `make lint`
- `make docs-check`
- `make test-bridge`
- `python3 scripts/check-contracts.py`
- `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -count=1`
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --allow-serial-runners --build-tags=integration,bridge ./...`
- `git diff --check`

The real target passed in 29.540 seconds. Two temporary projects used distinct
Codex threads and exactly one turn each. The reply contained a random value
stored only in the receiving project's file. Ordered bridge consumers durably
acknowledged the question and response without human relay. Initial idle time
was 500 milliseconds with no thread or turn; after completion, one second of
idle time preserved both turn counts and exactly three stored messages. Fixture
workers and service stopped through owned cleanup.

Focused race tests also passed injected reply/ack rollback, idempotent completion,
stale ownership, close/completion races, budgets, informational processing,
approval denial/once-only decisions, terminal checks and interruption proof.
Final PR evidence must be regenerated after the PR-reference follow-up and match
the canonical PR head and this exact ordered command list.

## Risks and Follow-ups

More extensive crash reconciliation and explicit recovery remain assigned to the
recorded recovery slice. This implementation fails closed for prior invoking,
failed, pending-approval, interrupted, capacity or uncertain work. Live acceptance
requires existing provider authentication and capacity; the target never skips
an unavailable provider or substitutes a fake exchange.
