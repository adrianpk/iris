# Runtime and protocol

Iris is a local per-user Linux daemon and CLI. It opens no public network
listener. Normal database writes belong to the daemon; bridge workers use the
same private Unix protocol as other clients. See [trust](trust.md).

## Paths and configuration

| Item | Path or fallback |
| --- | --- |
| Database | `$XDG_DATA_HOME/iris/iris.db`, otherwise `$HOME/.local/share/iris/iris.db` |
| Configuration | `$XDG_CONFIG_HOME/iris/config.json`, otherwise `$HOME/.config/iris/config.json` |
| Socket | `$XDG_RUNTIME_DIR/iris/iris.sock`; runtime root is required |
| Runtime lock | Socket path plus `.lock`; inode persists after shutdown |

XDG overrides must be absolute. Runtime roots must be user-owned and private;
Iris-owned directories use 0700 and database/socket/journal/export files use
0600. The executable sets umask 0077. It refuses unsafe existing paths instead
of broadening access. Installed units capture the installation-time XDG roots.

The optional configuration is a version-1 JSON object:

```json
{
  "schema_version": 1,
  "max_connections": 128,
  "request_timeout_seconds": 10,
  "lifecycle": {
    "invitation_seconds": 7200,
    "inactivity_seconds": 86400,
    "retention_seconds": 2592000
  }
}
```

Connections range from 1 through 128, ordinary timeout from 1 through 60 seconds,
and lifecycle durations from 1 through 315360000 seconds. Omitted fields retain
defaults. Unknown fields, null policy values, unsupported versions and malformed
JSON fail startup. Policy reload requires daemon restart. Issued invitation
deadlines stay fixed; inactivity/retention use the current loaded policy.

## Wire framing

Each Unix connection carries one UTF-8 JSON object, a newline, then write EOF.
Clients must half-close their write side while keeping their read side open.
The server validates the complete bounded frame before dispatch. Extra raw
newlines, trailing data, non-object JSON, unknown typed fields and invalid
UTF-8 fail. Escaped newlines inside message strings are supported.

```json
{"protocol":"iris/v1","operation":"stats","params":{"all_projects":true}}
```

Success and failure envelopes have schema version 1:

```json
{"protocol":"iris/v1","schema_version":1,"ok":true,"result":{"schema_version":1,"ready":true}}
```

```json
{"protocol":"iris/v1","schema_version":1,"ok":false,"error":{"code":"invalid_request","message":"Invalid request","retryable":false}}
```

The success example shows an envelope shape, not the complete stats result.
Public timestamps use UTC RFC3339Nano; stored timestamps use UTC milliseconds.
Optional semantic fields use JSON null where documented. Errors have stable
codes, a human message and `retryable`; message text is not a machine contract.

| Limit | Bound |
| --- | --- |
| Request / response frame | 1 MiB / 8 MiB including the final newline |
| Message body / topic | 256 KiB / 256 UTF-8 bytes, also subject to encoded frame size |
| Pending/active conversations | 64 per project |
| Conversation storage | 10,000 messages / 64 MiB body content; reserve final event slot |
| Consumer count | 64 per project/conversation |
| Receive page / wait | 1–100 messages / 0–60 seconds; wait plus 5-second wire deadline |
| Wait subscriptions | 64 service-wide, 16 per project |
| History page | 1–100 items and encoded response bound |
| Export leases / lifetime / spool | 16 service-wide / 120 seconds / 128 MiB private CLI spool |
| Prune previews / candidates / lifetime | 64 live / 1,000 exact candidates / 120 seconds |
| Bridge owners / lease / heartbeat | 4 service-wide / 15 seconds / 5 seconds |

See the owning [reference](index.md) for narrower content, cursor and budget
contracts. Unknown operations return `unknown_operation`; protocol mismatches
return `unsupported_protocol`. Invalid scope/input returns `invalid_request`,
`project_not_found`, `not_participant` or `recipient_mismatch` as applicable.
`invalid_cursor`, `idempotency_conflict`, `ack_out_of_order`, `preview_stale`,
`export_expired` and `limit_exceeded` identify specific corrective actions.
`server_busy`/`storage_busy` indicate bounded contention. Do not retry an
uncertain write with a new idempotency key. Bridge failures distinguish capacity,
authority, interruption and uncertainty; see [bridge recovery](bridge.md).

## Activation and shutdown

User systemd passes descriptor 3, `LISTEN_PID` for this process, exactly one
`LISTEN_FDS` and optional `LISTEN_FDNAMES=iris`. Iris validates a listening Unix
stream at the configured private path. Incorrect metadata, socket families,
packet listeners, paths or modes fail closed. The manager retains its socket
when the daemon closes its adopted descriptor. Without activation metadata,
manual startup owns and removes its socket; only an owned private stale socket
can be reclaimed. A runtime flock prevents competing daemon owners.

Migrations and lifecycle reconciliation finish before readiness. SIGINT/SIGTERM
cancel connections and maintenance, wait for owned handlers, close SQLite and
release the lock. Stop the socket before the service to prevent reactivation.
See [Linux operation](../how-to/linux-operation.md) and
[backup/recovery](../how-to/backup-recovery.md) for exact procedures.
