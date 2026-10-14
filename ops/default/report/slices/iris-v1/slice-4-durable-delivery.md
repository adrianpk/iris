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

## Implementation Notes

The existing SQLite schema owns delivery identities and constraints. One write
transaction validates membership and lifecycle, normalizes recipients, compares
the payload digest, allocates a sequence and persists all message rows before
returning success. Expiry transitions commit before returning lifecycle errors.

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

Controller-recorded exact-HEAD validation follows the remaining tasks.

## Risks and Follow-ups

Efficient waits, CLI content and process recovery remain in T4.3 through T4.5.
