# Slice 2: Service and Storage

Status: reviewing
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-2-iris-service-storage`
PR: `#2`

## Purpose

Establish the executable, migrated SQLite store, private Unix service, and
real-process recovery fixtures needed before project conversations.

## Delivered Behavior

Root help works without configuration or a running daemon. Commands use
separate output/error streams and stable exit codes. Configuration resolves
XDG roots, validates versioned bounded JSON, and rejects unsupported input.
The selected pure-Go SQLite driver creates the accepted schema, verifies
applied migration checksums, serializes immediate transactions, and preserves
committed state across reopening. Online backup includes committed WAL data.
The foreground daemon exposes
`iris stats --all-projects [--json]` through a private bounded Unix socket
after migration, with stable JSON results/errors and explicit human scope.

## Implementation Notes

The root executable owns signal cancellation and umask 0077. Application
constructors perform no IO. The Makefile selects the declared Go 1.26.3
baseline by default: the installed linter's analyzer panics on host Go 1.27
standard-library IR, while the unchanged checks pass with Go 1.26.3.

The store opens one pooled connection with foreign keys, WAL, full sync,
a five-second busy timeout, and immediate write transactions. Versioned SQL
and its checksum ledger commit together; failed upgrades roll back all pending
DDL and ledger changes. Unknown newer schemas and changed checksums fail
closed. Private directories/files and existing sidecars are checked before
opening; symlinks and broad permissions are rejected. Backup is exclusive-create
and uses the driver's online backup API with cancellation and cleanup.

The daemon acquires its runtime lock before opening storage, migrates before
listening, and reclaims only an owned private stale socket. Saturation returns
retryable backpressure. Cancellation closes owned connections and waits for
workers before closing SQLite; peer disconnect does not stop the daemon.

The process integration fixture builds the executable, creates two isolated
project roots and private XDG locations, checks successful JSON readiness, and
rejects a second daemon. Test-only SQL seeds committed state while the daemon
is stopped; the real CLI observes that state after clean and crash restarts.
A deliberately changed checksum rejects startup before any listener. Only
fixture-owned processes and temporary resources are stopped or removed.

## Contracts Added or Changed

The optional version-1 configuration accepts `schema_version`,
`max_connections` (1–128), and `request_timeout_seconds` (1–60).
Unknown fields and trailing JSON values fail. Help does not require an XDG
runtime directory. Exit codes distinguish usage (2), operation failure (1),
and cancellation (130). The versioned protocol uses one compact newline
frame plus write EOF, with 1 MiB requests and 8 MiB responses. The client
half-closes its write side and validates the response envelope before output.
A size error replaces a response before any partial payload is published.

## Files of Interest

- `main.go`: thin executable and signal ownership.
- `internal/iris/app.go`: CLI dispatch and exit/output boundary.
- `internal/iris/config.go`: XDG paths and configuration policy.
- `internal/iris/codec.go`: bounded-contract JSON decoding rules.
- `internal/iris/store.go`, `migration.go`, and `migrations/001_initial.sql`: private SQLite and atomic checksum-verified schema.
- `internal/iris/backup.go`: consistent online backup primitive.
- `internal/iris/runtime.go`, `server.go`, `client.go`, and `protocol.go`: private ownership, bounded IPC and cancellation.
- `internal/iris/service.go` and `cli.go`: readiness counts and public output/errors.
- `internal/iris/process*_test.go` and `make test-integration`: actual binary and isolated process recovery.
- `README.md`: current commands, configuration, output and development checks.

## Validation

Task checks passed for configuration/CLI behavior, real SQLite migrations and
backup, socket/protocol/ownership/cancellation cases, and focused race tests.
`make test-integration` passed with graceful restart, crash/stale-socket
recovery, persisted state, second-owner refusal and schema-checksum failure.
Integration-tag lint passed with the configured checks unchanged.

The following checks passed through Looper for
`8ceba294286f8011a4d882079770a6f51249cbcc`:

- `make check` — passed.
- `make lint` — passed.
- `make docs-check` — passed.
- `make test-integration` — passed.
- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris` — passed.
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --build-tags=integration ./...` — passed.
- `python3 scripts/check-contracts.py` — passed.
- `git diff --check` — passed.

The PR-reference follow-up regenerates the same ordered commands for its
final clean HEAD before handoff; controller evidence owns that exact identity.

This is the slice gate. The full delivery-set gate is controller-owned after
all configured slices are integrated.

## Risks and Follow-ups

Project registration and conversation workflows remain assigned to Slice 3.
User-systemd activation, public backup/restore commands, and live agent exchange
retain their owning slices. The online backup implementation here is a tested
store primitive; it is not presented as an available CLI command.
