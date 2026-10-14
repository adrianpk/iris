# Slice 2: Service and Storage

Status: drafting
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-2-iris-service-storage`
PR: pending

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

## Contracts Added or Changed

The optional version-1 configuration accepts `schema_version`,
`max_connections` (1–128), and `request_timeout_seconds` (1–60).
Unknown fields and trailing JSON values fail. Help does not require an XDG
runtime directory. Exit codes distinguish usage (2), operation failure (1),
and cancellation (130).

## Files of Interest

- `main.go`: thin executable and signal ownership.
- `internal/iris/app.go`: CLI dispatch and exit/output boundary.
- `internal/iris/config.go`: XDG paths and configuration policy.
- `internal/iris/codec.go`: bounded-contract JSON decoding rules.
- `internal/iris/store.go`, `migration.go`, and `migrations/001_initial.sql`: private SQLite and atomic checksum-verified schema.
- `internal/iris/backup.go`: consistent online backup primitive.

## Validation

- `go test ./internal/iris` — passed for executable/configuration behavior.
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix ./internal/iris` — passed.

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestStore|TestMigration|TestBackup' -count=1` — passed.

Final controller-owned exact-HEAD validation follows completion of the slice.

## Risks and Follow-ups

Unix service and process recovery remain in T2.3 and T2.4.
