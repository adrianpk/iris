# Conversation History and Exports

`conversations`, `show`, `search`, `dump` and `stats` default to the registered
project containing the current directory. Use `--project <ID|alias|path>` to
select another registered project. Each project sees only conversations in
which it participates, including invitations it has not joined.

Outside a project, use `--all-projects` explicitly for local administration.
It cannot be combined with `--project`. Inspection never activates a bridge,
renews inactivity or retention, creates a consumer, delivers messages to an
inbox, or acknowledges processing.

## Discover and inspect

```sh
iris conversations
iris conversations --active --json
iris conversations --state expired --limit 20 --json
iris conversations --all-projects --all --json
iris show --json CONVERSATION_ID
iris show --limit 20 --cursor CURSOR --json CONVERSATION_ID
iris show --pair alpha,beta --ordinal 2 --json
```

The default listing includes pending, active, closed and expired conversations.
`--active` selects pending and active state; `--state` selects one exact state.
`--all` explicitly includes every state. These state options are mutually
exclusive. Conversation IDs are full 32-character internal IDs. An ordinal
requires an explicit pair of two project selectors separated by a comma.
A bare ordinal or incomplete pair is rejected.

Lists order by creation time and ID, ascending. Show orders messages by
sequence. Both default to 50 items and accept `--limit` from 1 through 100.
Responses include `next_cursor` and `truncated`. Repeat the same selection
with its cursor; a different project or filter rejects that cursor.
Message pages also stop below the 8 MiB response ceiling. Show is paged
inspection; use dump for one complete transcript.

Labels, participant aliases and message senders preserve their historical
snapshots after project rename or relocation. Lists return lifecycle metadata,
participant labels, message counts and stored body bytes without embedding
whole message bodies.

## Search

```sh
iris search --text 'storage format' --json
iris search --pair alpha,beta --ordinal 2 --json
iris search --since 2026-10-01T00:00:00Z --until 2026-10-14T23:59:59Z --json
iris search --conversation CONVERSATION_ID --state closed --json
```

All filters combine with the selected participant scope. Pair selectors resolve
current project identities; results preserve historical labels. Dates are
inclusive conversation creation times in UTC milliseconds. An omitted bound is
open. `since` must not follow `until`. Use RFC3339 timestamps, including a zone.

Text is a case-sensitive literal substring of topics, message bodies or
historical participant aliases, limited to 512 UTF-8 bytes. SQL metacharacters,
percent signs and regular-expression syntax have no special meaning.
Each matching conversation includes at most five matching message IDs,
sequences and excerpts of at most 512 characters. `matches_truncated` indicates
additional matches; show or dump provides the messages. Topic/alias-only matches
can have no matching message excerpt. Search uses the same deterministic
keyset pages and limits as conversation listings.

## Export complete history

```sh
iris dump CONVERSATION_ID > transcript.txt
iris dump --format json CONVERSATION_ID > transcript.json
iris dump --json CONVERSATION_ID | jq '.result.messages'
iris dump --all-projects --pair alpha,beta --ordinal 2 --format json
```

`--format text` is the default. Text includes readable participant labels,
lifecycle times, topic and each complete message in sequence order.
`--format json` emits one transcript object containing `schema_version` (1),
`conversation`, `participants` and ordered `messages`. Messages retain IDs,
sequences, sender and recipient identities, kinds, timestamps, complete bodies,
and reply/correlation/idempotency metadata. `--json` emits that same transcript
inside the standard `iris/v1` success envelope.

The daemon freezes conversation metadata, participants and the highest sequence
under a runtime lease. Later messages and lifecycle changes do not enter that
snapshot. There are at most 16 live leases, each lasting 120 seconds. Pages have
at most 100 messages and fit the wire ceiling. A live lease blocks pruning;
export never extends the retention deadline. Expired leases and daemon restart
produce `export_expired`.

The client writes pages to a mode-0600 temporary spool in the private runtime
directory, with a 128 MiB ceiling. It publishes stdout only after every page and
lease closure succeed, then removes its spool. Failure or cancellation removes
owned temporary output and attempts bounded lease cleanup; an unreachable
service cannot keep the lease alive past its fixed expiry. Failed exports leave
stdout empty before publication. An output-device failure can interrupt the
final copy and returns a nonzero exit code.

## Statistics

```sh
iris stats
iris stats --json | jq '{states: .result.states, retained: .result.retained_conversations}'
iris stats --all-projects --json | jq '.result.storage'
```

Project scope counts its visible conversations, all their messages and stored
UTF-8 body bytes. Administrative scope counts all conversations and registered
projects. `states` reports pending, active, closed and expired counts.
`retained_conversations` counts closed and expired histories. `prune_eligible`
counts terminal histories at or beyond the current retention deadline; an
export lease or changed preview can still prevent confirmation. `lifecycle`
reports the daemon's loaded duration policy in seconds.

`storage.scope` is always `service`: SQLite allocation is shared by every
project. Page size, page count and free pages describe the current database
view; allocated bytes are page size times page count and reusable bytes are
page size times free pages. These are logical database allocation values,
not the sum of physical database, WAL and shared-memory files.

## Protocol

History operations use the existing versioned private Unix protocol:
`conversations`, `show`, `search`, `dump.open`, `dump.page`, `dump.close`, and
`stats`. History parameters use `project` or `cwd`, or explicit `all_projects`;
optional filters are `conversation_id`, `state`, `pair` (two selectors),
`ordinal`, `text`, `since`, `until`, `limit` and `cursor`.

`dump.open` requires one conversation ID or pair/ordinal, without a cursor.
It returns `lease_id`, expiry, frozen conversation/participants and
`highest_sequence`. Page requests contain `lease_id` and optional cursor.
Close accepts only `lease_id` and is idempotent. Lease tokens and cursors are
runtime capabilities within the same private local-user boundary.
JSON command errors go to stderr, with empty stdout and a nonzero exit code.
