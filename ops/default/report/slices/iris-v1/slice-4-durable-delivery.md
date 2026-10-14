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

The actual-process fixture sends concurrently, loses a confirmation, crashes
and restarts the daemon, verifies stable retry fields and independent consumer
positions, and confirms terminal rejection and conversation/project isolation.
A two-second receive succeeds with the ordinary timeout configured to one
second. Product usage and the delivery reference document the public behavior.

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
message slot for a terminal event and respect body/storage bounds. Receive
returns consumer progress, addressed messages, a scoped cursor and truncation;
ack returns only that consumer's processing/delivery position. Page construction
bounds encoded bytes as well as item count. Public details are in the
[delivery reference](../../../../../docs/reference/delivery.md).

## Files of Interest

- `internal/iris/message_*.go`: public message reads and transactional sends.
- `internal/iris/delivery*.go`, `receive.go`, `ack.go`: cursor and consumer state.
- `internal/iris/wait.go`, `peer_disconnect_linux.go`: bounded notification and cleanup.
- `internal/iris/process_delivery_test.go`: actual-binary recovery and isolation.

## Validation

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestSend' -count=1` — passed.
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix ./...` — passed.

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestReceive|TestAck|TestDelivery' -count=1` — passed, including reopen, byte/page and 64-consumer limits.

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestWait' -count=1 -timeout=30s` — passed after correcting a test fixture's page limit and replacing a racy connection-count assertion with a bounded concurrent writer check.

- `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run '^TestDeliveryCLI|TestCallInitial' -count=1` — passed for human/JSON, boundary-size content and input failures across the real Unix transport.

- `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run '^TestProcessDelivery$|^TestSend|^TestDeliveryActivity$' -count=1 -timeout=30s` — passed.
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix --build-tags=integration ./...` — passed.

Controller-recorded exact-HEAD validation follows PR preparation.

## Risks and Follow-ups

Delivery is at least once, not exactly-once external execution. The internal
result-plus-ack primitive is not an unowned public operation; bridge identity,
lease and invocation checks belong to Slice 5. Configurable lifecycle policy
and autonomous reconciliation remain in Slice 6. No model or installed service
is used by these checks.
