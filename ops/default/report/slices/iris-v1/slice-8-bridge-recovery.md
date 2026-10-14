# Slice 8: Bridge Recovery

Status: drafting
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-8-iris-bridge-recovery`
PR: pending

## Purpose

Recover durable worker progress and bound execution while keeping ambiguous
external work visible and preventing automatic unsafe replay.

## Delivered Behavior

Completed final results are checkpointed before reply/ack publication. A new
owner can publish a saved result without another model invocation. Explicit
bridge recovery inspects only the recorded owned thread/turn; unconfirmed,
foreign or unfinished execution cannot become successful processing.

## Implementation Notes

Migration 3 preserves earlier migration checksums and adds versioned result
JSON. Claim and mutation tokens serialize through SQLite. Reply, acknowledgment
and result-checkpoint clearance remain one transaction. Lost completion
confirmation preserves the committed acknowledgment.

## Contracts Added or Changed

`bridge.record` stores an immutable result checkpoint. `bridge.recover` acquires
a prior binding with its persisted options. Public status does not expose owner
tokens or result bodies. Recovery validates stored thread ID, cwd, exact latest
turn, completion status, tool outcomes and final answer schema.

## Files of Interest

- `internal/iris/bridge_result.go`: durable result and recovery orchestration.
- `internal/iris/codex_recovery.go`: bounded read-only provider inspection.
- `internal/iris/migrations/003_bridge_result.sql`: result checkpoint storage.

## Validation

Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestBridgeRecorded|^TestBridgeInvocationRecovery|^TestBridgeLost|^TestBridgeUnconfirmed|^TestCodex.*Recover|^TestMigration' -count=1`.
Real SQLite reopen, scoped owner replacement, immutable checkpoint retries,
publication without a model, lost confirmation and failed/foreign/incomplete
provider history checks passed. Slice validation remains pending.

## Risks and Follow-ups

Failure states/retry policy, execution limits, security and real adapter
recovery/idle measurements remain in this slice's recorded tasks.
The installed 0.161.0 schema supports `thread/read` with `includeTurns`;
recovery fails explicitly if bounded complete history is unavailable.
[Official app-server reference](https://learn.chatgpt.com/docs/app-server).
