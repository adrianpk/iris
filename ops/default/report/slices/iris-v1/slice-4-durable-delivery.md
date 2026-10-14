# Slice 4: Durable Delivery

Status: drafting
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-4-iris-durable-delivery`
PR: pending

## Purpose

Deliver ordered durable messages, independent consumer progress, bounded
waiting, and explicit processing acknowledgment through the service and CLI.

## Delivered Behavior

Joined participants send message/status content to conversation participants.
Sequence, immutable sender alias, reply/correlation metadata, recipients,
counters, and optional retry key commit together. Identical retries return the
original message even after closure; changed payloads conflict. Failed writes
consume no sequence. Replies cannot reference another conversation.

Receive returns addressed messages in order, with independent durable delivered
and acknowledged positions per consumer. Reads replay unacknowledged work.
Scoped cursors paginate only through delivered history and reject cross-scope
or future positions. Acknowledgment requires the next addressed, delivered
message; it cannot skip work or another consumer. A result-plus-ack persistence
primitive rolls both changes back on failure.

Receivers subscribe before rechecking storage and wake only after committed
changes. Empty waits hold no SQLite transaction. Cancellation, full peer
disconnect, timeout and terminal state release subscriptions. There are at
most 64 waiters, with 16 per project. Wire deadlines allow requested wait plus
five seconds rather than using the ordinary request timeout.

The CLI exposes `send`, `receive` and `ack`. Shared text/file/stdin handling
preserves multiline UTF-8 and rejects empty, oversized or conflicting input.
Send supports reply targets, correlation IDs, explicit recipients, status
messages and retry keys. Receive defaults to consumer `manual` and a 30-second
wait; `--wait 0` returns immediately. JSON success is one versioned envelope,
with failures only on stderr.

## Implementation Notes

The existing SQLite schema owns delivery identities and constraints. One write
transaction validates membership and lifecycle, normalizes recipients, compares
the payload digest, allocates a sequence and persists all message rows before
returning success. Expiry transitions commit before returning lifecycle errors.

Linux disconnect detection uses a blocking kernel hangup wait with a private
cancellation pipe. It distinguishes full disconnect from request write EOF;
no repeated SQLite query or timed socket probe runs while idle. The existing
`golang.org/x/sys` module is now a direct dependency for that boundary.

## Contracts Added or Changed

`send` returns the version-1 public message with UTC timestamps and nullable
reply, correlation and idempotency fields. Ordinary messages reserve the final
message slot for a terminal event and respect body/storage bounds.

## Files of Interest

- `internal/iris/message_*.go`: public message reads and transactional sends.
- `internal/iris/message_test.go`: concurrent writers, retry and rollback checks.

## Validation

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestSend' -count=1` — passed.
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix ./...` — passed.

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestReceive|TestAck|TestDelivery' -count=1` — passed, including reopen, byte/page and 64-consumer limits.

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestWait' -count=1 -timeout=30s` — passed after correcting a test fixture's page limit and replacing a racy connection-count assertion with a bounded concurrent writer check.

- `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run '^TestDeliveryCLI|TestCallInitial' -count=1` — passed for human/JSON, boundary-size content and input failures across the real Unix transport.

Controller-recorded exact-HEAD validation follows the remaining tasks.

## Risks and Follow-ups

Actual-process delivery recovery and isolation acceptance remains in T4.5.
