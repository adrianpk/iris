# SQLite storage

The daemon normally owns all database writes and initially uses one connection.
Each connection enables foreign keys, WAL journal mode, `synchronous=FULL`, a
five-second busy timeout and immediate write transactions. No storage transaction
is held while waiting for a socket, model or approval. See
[backup and recovery](../how-to/backup-recovery.md) before copying data.

## Schema and migrations

Storage schema version is 4; wire/result schema version is independently 1.
Ordered embedded migrations and their checksums are applied atomically before
readiness. A newer schema, modified applied checksum or failed migration stops
startup. Failed upgrades roll back both schema and migration ledger writes.
Applied migrations are immutable.

| Migration | Change |
| --- | --- |
| 1 | Initial fourteen-table conversation/storage model. |
| 2 | Persist turn count, lifetime origin and pending approval identity. |
| 3 | Store versioned completed result checkpoints before publication. |
| 4 | Preserve observed usage enforcement independently of nullable telemetry. |

The [accepted physical model](../../ops/default/spec/iris-model.md) contains the
canonical SQL and operation-to-query mapping. Embedded migrations under
`internal/iris/migrations/` implement it. No separate per-project databases,
project-local ledgers or automatically generated repository files are used.

| Tables | Durable responsibility |
| --- | --- |
| `schema_migrations` | Applied version/checksum ledger. |
| `projects` | Stable IDs, unique canonical paths/aliases, provisional recipients. |
| `pair_sequences` | Monotonic unordered-pair ordinals, retained after pruning. |
| `conversations`, `participants` | Lifecycle, counters, activity and frozen membership labels. |
| `call_requests` | Scoped creation retries and permanent key tombstones. |
| `invitation_code_tombstones`, `invitations` | Non-reused code digests and fixed recipient/deadline. |
| `messages`, `message_recipients` | Ordered content, stable retries, replies and addressed delivery. |
| `consumer_cursors` | Independent delivered and acknowledged progress. |
| `bridge_bindings` | Ownership generation, thread/turn/input, result, limits and authority state. |
| `prune_previews`, `prune_candidates` | Bounded exact confirmation metadata. |

Allocation and message insertions commit together. Reply publication and bridge
processing acknowledgment share one transaction. SQL constraints enforce
membership, uniqueness, valid state and ordering alongside typed validation.
An uncertain reply does not justify a new write key or an assumed acknowledgment.

Terminal retention preserves history and processing records. Confirmed pruning
cascades conversation payload, recipients, membership, consumers and bindings,
while keeping project IDs, pair counters, code digests and request-key tombstones.
Export leases are runtime state: they block prune while valid, freeze metadata
and maximum sequence, and do not survive daemon restart.

## Recovery and observability

A clean close checkpoints SQLite; a crash reopens its journal normally. Never
combine a snapshot with unrelated newer WAL/SHM files. Use the tested stopped-copy
procedure or SQLite's online backup interface, never a live database-only copy.
Restoration rolls processing state back to snapshot time and cannot reverse or
authorize repeating external effects.

`iris stats --all-projects --json` reports readiness, storage schema, counts,
retained/eligible history and page allocation. Page size/count/freelist metrics
cover the service database, not a per-project file-size attribution or an exact
filesystem/WAL byte count. Logs and raw SQLite inspection are maintenance tools;
stop the service and all owned connections before offline writes or restoration.
