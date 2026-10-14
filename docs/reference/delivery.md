# Message Delivery

The CLI and private Unix protocol expose the same durable delivery operations.
Protocol requests use `iris/v1`; success and error envelopes use schema version
1. Project selection accepts `project` (registered ID, canonical path or alias)
or `cwd` (nearest registered canonical ancestor). An explicit project disables
cwd inference. Delivery never auto-registers a project.

## Send

```sh
iris send --message "Accepted format: JSON." --reply-to MESSAGE_ID --idempotency-key response-one CONVERSATION_ID
```

The `send` operation accepts:

| Parameter | Contract |
| --- | --- |
| `conversation_id` | Required 32-character lowercase hexadecimal ID. |
| `body` | Required nonempty UTF-8, at most 256 KiB and within the encoded request limit. |
| `kind` | `message` or `status`; defaults to `message`. |
| `recipients` | Optional participant project IDs; defaults to the other participant. Duplicates and foreign projects fail. |
| `reply_to` | Optional message ID within this conversation. |
| `correlation_id` | Optional nonempty UTF-8 identifier, at most 128 bytes. |
| `idempotency_key` | Optional nonempty UTF-8 identifier, at most 128 bytes, scoped to conversation and sender. |

The CLI also accepts `--file` or stdin for the body. Content is preserved
verbatim. Message/file flags cannot be combined; with neither flag, stdin is
the input. Options precede the conversation operand.

A successful send commits sequence, message, recipients, counters and activity
before confirmation. Identical key retries return the original public message,
including after closure. Changed payloads with the same key fail with
`idempotency_conflict`. Without a key, repeated sends create distinct messages.
New sends require active state and joined membership. Only the service creates
`system` messages.

Public messages contain `schema_version`, `conversation_id`, `message_id`,
`sequence`, `sender_project_id`, `sender_alias`, `recipients`, `created_at`,
`kind`, `body`, `topic`, `reply_to`, `correlation_id` and `idempotency_key`.
Timestamps are UTC RFC3339Nano. Optional fields are JSON null when absent;
service events have a null sender project and sender alias `iris`.

## Receive

```sh
iris receive --consumer manual --wait 30 --limit 50 --json CONVERSATION_ID
```

The `receive` operation accepts `conversation_id`, optional `consumer_id`
(default `manual`), `limit` (default 50, range 1–100), `cursor`, and
`wait_seconds` (protocol default 0, CLI default 30, range 0–60).

The result contains schema version 1, `conversation_id`, `project_id`,
`consumer_id`, `acked_sequence`, `delivered_sequence`, `messages`,
`next_cursor` and `truncated`. Addressed messages are ordered by sequence.
Default reads start after the acknowledged position, so unread processing
repeats after a crash. Reading advances delivery only; it never acknowledges.
Consumers have independent progress, including across restarts.

A nonempty page returns a cursor after its last message. An empty page returns
null. `truncated` reports more addressed messages beyond this page, including
when byte bounds limit the page before its requested item count. Cursors use
unpadded URL-safe base64 of versioned JSON with conversation, project, consumer
and `after_sequence`. Foreign scope/version, negative/future positions and
jumps beyond that consumer's delivered history fail with `invalid_cursor`.
A cursor grants no participation rights.

Empty waits subscribe before rechecking storage, release SQLite, and block
until notification, cancellation, disconnect or timeout. The receive deadline
is the requested wait plus five seconds on both sides of the socket. Closed
and expired conversations fail with `conversation_inactive`. Reads, waits and
duplicate retries do not renew inactivity.

## Acknowledge

```sh
iris ack --consumer manual --message-id DELIVERED_MESSAGE_ID --json CONVERSATION_ID
```

The `ack` operation accepts `conversation_id`, `message_id` and `consumer_id`
(default `manual`). Its result contains schema version 1, conversation,
project, consumer, acknowledged sequence and delivered sequence, using the
same field names as receive.

Only the next addressed unacknowledged message delivered to that consumer can
advance processing. Unaddressed sequence gaps do not block it. Skipping work
or acknowledging unseen work fails with `ack_out_of_order`. Duplicate
acknowledgments return current progress. Previously delivered work can be
acknowledged after closure without reopening or extending the conversation.

Delivery is at least once until acknowledgment. Consumers must handle repeated
work safely; acknowledgment does not promise exactly-once external execution.

## Bounds and output

There are at most 64 consumers per conversation/project, 64 service waiters,
and 16 waiters per project. Excess waiters return retryable `server_busy`.
Conversations allow 10,000 messages and 64 MiB of body content; one message slot
is reserved for a terminal event. Encoded request and response ceilings are
1 MiB and 8 MiB. Pages leave room for envelope and cursor metadata.

`--json` prints one success envelope to stdout. Failures leave stdout empty
and write an error envelope to stderr. Exit codes are 0 for success, 2 for
invalid CLI usage, 1 for service/operating failure, and 130 for interruption.
