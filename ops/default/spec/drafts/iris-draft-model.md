# Iris Data Model

Status: settled implementation contract; promoted with its specification in T1.4.
Specification: [Iris specification](iris-draft.md).

## Ownership and representation

The daemon owns SQLite and applies the specification's section 17 contracts.
Project identity is independent of Codex sessions and optional `.iris.toml`.
Use `modernc.org/sqlite v1.60.1` through `database/sql` in T2.2. Its module
requires Go 1.26.0; selection is confirmed before addition to the Go module.

IDs are 32-character lowercase hexadecimal random 128-bit values. Storage
times are UTC Unix milliseconds in INTEGER columns, rendered as RFC3339Nano
UTC strings in public JSON. Nullable timestamps and references render as
`null`; do not invent zero times. Store canonical absolute paths or NULL for
unregistered recipient placeholders. Alias snapshots never follow renames.

## Comparable field and query mapping

| Concept and public fields | Stored fields, in corresponding order | Constraint and principal query |
| --- | --- | --- |
| Project: `project_id`, `path`, `alias`, `created_at`, `registered_at` | `projects.id`, `canonical_path`, `alias`, `created_at`, `registered_at` | Unique non-NULL path and unique alias; exact selector or nearest registered canonical ancestor. NULL path means reserved recipient placeholder. |
| Pair: sorted project IDs, next human ordinal | `pair_sequences.project_low`, `project_high`, `last_ordinal` | Composite primary key, low < high; increment only in the call transaction; never prune. |
| Conversation: `conversation_id`, pair, `ordinal`, `topic`, `state`, lifecycle | `conversations.id`, pair columns, `pair_ordinal`, `topic`, `state`, `created_at`, `last_activity_at`, `terminal_at` | Unique ordinal within pair; terminal time iff closed/expired; active lists and expiry indexes. |
| Conversation allocation and limits | `next_sequence`, `revision`, `message_count`, `body_bytes` | Allocate sequence and increment counters/revision with each event; reserve a terminal slot and reject over-budget ordinary sends. |
| Participant: `conversation_id`, `project_id`, `alias`, `joined_at` | `participants.conversation_id`, `project_id`, `alias_snapshot`, `joined_at` | Composite key; sender joined on call, expected recipient joined on acceptance; project visibility index. |
| Invitation: conversation, code, expected project, expiry, acceptance | `invitations.conversation_id`, `code`, `expected_project_id`, `expires_at`, `accepted_at`, `code_digest` | One invitation per bilateral conversation, unique code; code digest retained separately forever to prevent reuse. |
| Code reservation | `invitation_code_tombstones.code_digest`, `created_at` | SHA-256 digest primary key; retained across conversation pruning. |
| Call request: initiator, key, payload digest, original conversation | `call_requests.initiator_project_id`, `request_key`, `payload_digest`, `conversation_id`, `created_at` | Composite key; same payload returns original result; retain the key with NULL conversation after pruning. |
| Message: version, conversation, ID, sequence, sender, alias, time, kind, body, topic, reply, correlation, key | Constant version 1; `messages.conversation_id`, `id`, `sequence`, `sender_project_id`, `sender_alias`, `created_at`, `kind`, `body`; conversation topic; `reply_to`, `correlation_id`, `idempotency_key` | Unique conversation sequence and scoped sender/key. Reply FK stays within conversation. Only system events have NULL sender. |
| Message recipients and retry identity | `message_recipients.conversation_id`, `message_id`, `project_id`; `messages.payload_digest` | Recipient must be a participant. Canonical payload hash detects a changed retry; immutable message and recipient rows form one insert transaction. |
| Consumer: conversation, project, stable consumer ID, acknowledgment, delivered high-water mark | `consumer_cursors.conversation_id`, `project_id`, `consumer_id`, `acked_sequence`, `delivered_sequence` | Per-consumer progress; ack <= delivered; next addressed message only. Reads never count as processing. |
| Worker binding: binding, conversation/project/consumer, thread, turn, input, result, state | `bridge_bindings.id`, `conversation_id`, `project_id`, `consumer_id`, `thread_id`, `turn_id`, `input_message_id`, `result_message_id`, `state` | One durable binding per conversation/project. Resume only its own thread; immutable conversation identity. |
| Worker limits, usage, ownership and errors | `limits_json`, `usage_tokens`, `owner_token`, `lease_until`, `updated_at`, `error_code`, `stop_requested_at` | Limits are the section 17 policy fields. Token-guarded updates; NULL usage means unavailable; lease expiry is not proof of a safe replay. |
| Prune preview: token, selected digest/policy/cutoff, expiry | `prune_previews.token_digest`, `selection_digest`, `policy_digest`, `eligibility_cutoff`, `expires_at` | One-use bounded preview; compare candidate identities/revisions/terminal times, policy and live export leases before deleting anything. |
| Prune candidates | `prune_candidates.token_digest`, `conversation_id`, `revision`, `terminal_at` | Exact membership of one complete preview; no new candidate added during confirmation. |
| Migration: version, checksum, applied time | `schema_migrations.version`, `checksum`, `applied_at` | Ordered, immutable checksums; ledger and DDL in the same transaction. |

## Version 1 schema contract

This SQL is the initial physical schema contract. T2.2 embeds it as a migration
and tests the selected Go driver against it. Application validation additionally
enforces UTF-8 byte limits, alias/ID grammar, joined membership, clock policy,
resource ceilings, transition rules, and ownership. Foreign keys are enabled
on every connection. Use bound SQL parameters throughout.

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY CHECK (version > 0),
    checksum TEXT NOT NULL,
    applied_at INTEGER NOT NULL
);
CREATE TABLE projects (
    id TEXT PRIMARY KEY NOT NULL,
    canonical_path TEXT UNIQUE,
    alias TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    registered_at INTEGER,
    CHECK ((canonical_path IS NULL) = (registered_at IS NULL))
);
CREATE TABLE pair_sequences (
    project_low TEXT NOT NULL REFERENCES projects(id),
    project_high TEXT NOT NULL REFERENCES projects(id),
    last_ordinal INTEGER NOT NULL CHECK (last_ordinal >= 0),
    PRIMARY KEY (project_low, project_high),
    CHECK (project_low < project_high)
);
CREATE TABLE conversations (
    id TEXT PRIMARY KEY NOT NULL,
    project_low TEXT NOT NULL,
    project_high TEXT NOT NULL,
    pair_ordinal INTEGER NOT NULL CHECK (pair_ordinal > 0),
    topic TEXT NOT NULL CHECK (length(CAST(topic AS BLOB)) BETWEEN 1 AND 256),
    state TEXT NOT NULL CHECK (state IN ('pending','active','closed','expired')),
    created_at INTEGER NOT NULL,
    last_activity_at INTEGER NOT NULL,
    terminal_at INTEGER,
    next_sequence INTEGER NOT NULL DEFAULT 1 CHECK (next_sequence > 0),
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    message_count INTEGER NOT NULL DEFAULT 0 CHECK (message_count >= 0),
    body_bytes INTEGER NOT NULL DEFAULT 0 CHECK (body_bytes >= 0),
    FOREIGN KEY (project_low, project_high)
        REFERENCES pair_sequences(project_low, project_high),
    UNIQUE (project_low, project_high, pair_ordinal),
    CHECK ((state IN ('closed','expired')) = (terminal_at IS NOT NULL))
);
CREATE TABLE call_requests (
    initiator_project_id TEXT NOT NULL REFERENCES projects(id),
    request_key TEXT NOT NULL,
    payload_digest TEXT NOT NULL,
    conversation_id TEXT REFERENCES conversations(id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (initiator_project_id, request_key)
);
CREATE TABLE participants (
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id),
    alias_snapshot TEXT NOT NULL,
    joined_at INTEGER,
    PRIMARY KEY (conversation_id, project_id)
);
CREATE TABLE invitation_code_tombstones (
    code_digest TEXT PRIMARY KEY NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE invitations (
    conversation_id TEXT PRIMARY KEY NOT NULL
        REFERENCES conversations(id) ON DELETE CASCADE,
    code TEXT NOT NULL UNIQUE,
    code_digest TEXT NOT NULL UNIQUE REFERENCES invitation_code_tombstones(code_digest),
    expected_project_id TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    accepted_at INTEGER,
    FOREIGN KEY (conversation_id, expected_project_id)
        REFERENCES participants(conversation_id, project_id)
);
CREATE TABLE messages (
    id TEXT PRIMARY KEY NOT NULL,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    sender_project_id TEXT,
    sender_alias TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('message','status','system')),
    body TEXT NOT NULL,
    reply_to TEXT,
    correlation_id TEXT,
    idempotency_key TEXT,
    payload_digest TEXT NOT NULL,
    UNIQUE (conversation_id, sequence),
    UNIQUE (conversation_id, id),
    UNIQUE (conversation_id, sender_project_id, idempotency_key),
    FOREIGN KEY (conversation_id, sender_project_id)
        REFERENCES participants(conversation_id, project_id),
    FOREIGN KEY (conversation_id, reply_to)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    CHECK ((kind = 'system') = (sender_project_id IS NULL))
);
CREATE TABLE message_recipients (
    conversation_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    PRIMARY KEY (message_id, project_id),
    FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, project_id)
        REFERENCES participants(conversation_id, project_id) ON DELETE CASCADE
);
CREATE TABLE consumer_cursors (
    conversation_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    consumer_id TEXT NOT NULL,
    acked_sequence INTEGER NOT NULL DEFAULT 0 CHECK (acked_sequence >= 0),
    delivered_sequence INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (conversation_id, project_id, consumer_id),
    FOREIGN KEY (conversation_id, project_id)
        REFERENCES participants(conversation_id, project_id) ON DELETE CASCADE,
    CHECK (delivered_sequence >= acked_sequence)
);
CREATE TABLE bridge_bindings (
    id TEXT PRIMARY KEY NOT NULL,
    conversation_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    consumer_id TEXT NOT NULL,
    thread_id TEXT UNIQUE,
    turn_id TEXT,
    input_message_id TEXT,
    result_message_id TEXT,
    state TEXT NOT NULL CHECK (state IN
        ('waiting','invoking','approval_required','result_recorded','acked',
         'stopped','interrupted','failed','capacity','uncertain')),
    limits_json TEXT NOT NULL CHECK (json_valid(limits_json)),
    usage_tokens INTEGER CHECK (usage_tokens >= 0),
    owner_token TEXT,
    lease_until INTEGER,
    updated_at INTEGER NOT NULL,
    error_code TEXT,
    stop_requested_at INTEGER,
    UNIQUE (conversation_id, project_id),
    FOREIGN KEY (conversation_id, project_id, consumer_id)
        REFERENCES consumer_cursors(conversation_id, project_id, consumer_id)
        ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, input_message_id)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id, result_message_id)
        REFERENCES messages(conversation_id, id) ON DELETE CASCADE,
    CHECK ((owner_token IS NULL) = (lease_until IS NULL))
);
CREATE TABLE prune_previews (
    token_digest TEXT PRIMARY KEY NOT NULL,
    selection_digest TEXT NOT NULL,
    policy_digest TEXT NOT NULL,
    eligibility_cutoff INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE TABLE prune_candidates (
    token_digest TEXT NOT NULL REFERENCES prune_previews(token_digest) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    terminal_at INTEGER NOT NULL,
    PRIMARY KEY (token_digest, conversation_id)
);
CREATE INDEX participant_project ON participants(project_id, conversation_id);
CREATE INDEX conversation_state_activity ON conversations(state, last_activity_at, id);
CREATE INDEX conversation_terminal ON conversations(terminal_at, id);
CREATE INDEX conversation_created ON conversations(created_at, id);
CREATE INDEX invitation_expiry ON invitations(expires_at, conversation_id);
CREATE INDEX recipient_inbox ON message_recipients(project_id, conversation_id, message_id);
CREATE INDEX bridge_recovery ON bridge_bindings(state, lease_until);
CREATE INDEX preview_expiry ON prune_previews(expires_at);
```

## Transaction and query ownership

| Operation | Atomic work and invariant |
| --- | --- |
| Register / rename / relocate | Canonicalize before entering the transaction, enforce unique path/alias, bind a placeholder at most once; no ID merge. Nearest-ancestor discovery checks path components, never string-prefix matching. |
| Call | Resolve initiator and check its request key/payload first; return original or conflict without another allocation. Otherwise resolve or reserve destination; reject self-pair; reserve next pair ordinal; insert pending conversation, participant snapshots, tombstoned code, invitation, sequence-1 message and recipients; update counters and persist the call request/result. Roll back the whole allocation on failure. |
| Join | Check expiry and expected recipient; bind provisional path if needed; compare pending state, set recipient joined time, accepted time and active state; append one system event. Duplicate same-project join adds nothing. |
| Send / close / expire | Recheck lifecycle and membership; look up retry key before rejecting an already committed retry; compare payload digest; allocate next sequence and insert all message data with counters/revision. Set terminal time once. |
| Receive / wait | Query participant and state, then addressed messages with sequence > ack or validated explicit cursor, ORDER BY sequence LIMIT page; update delivered high-water mark only. Release SQLite before waiting and recheck after notification to avoid lost wakeups. |
| Ack | Select the next addressed unacked message; require delivery to that consumer; advance only its contiguous processing position. Duplicates return its existing position. |
| Bridge claim / renew | Compare binding ID, prior owner/lease, state and token in the transaction. Claim only under safe recovery policy; failed compare prevents further turns and writes. |
| Bridge completion | Verify current token, active conversation and input; persist final reply using its stable idempotency key and advance the bound consumer's ack in one transaction. If the conversation terminated, retain terminal recovery state without a late send. |
| List / search | Participant-filtered query by default; explicit human all-project scope. Parameterized literal text filter and `(created_at,id)` keyset, limit+1 truncation detection. Pair/ordinal/date/ID filters never change membership. |
| Show / dump | Preserve immutable alias/message fields. Export lease captures metadata, participants and highest sequence under a short lock; bounded pages filter sequence <= that snapshot. Live leases block prune; server restart invalidates them. CLI publishes a complete private spool only after success. |
| Prune preview / confirm | Save bounded exact membership/revisions and policy/cutoff digest. Confirm expiry, hash, current eligibility and absence of export leases; delete only those conversations and cascade their payload/history/cursors/bindings, then consume preview. Preserve projects, pair rows, code digests and call request tombstones. |
| Migrate / recover | Open with FK/WAL/full-sync/busy-timeout settings; verify schema ledger; execute each ordered migration and ledger insert atomically before readiness. Fail closed on incompatible or corrupt storage. |

Export leases, connected approvals, listener handles, notification channels,
and process handles are memory-owned runtime resources. They are not project
identity or durable approval grants. Lease loss during `invoking` can produce
`uncertain` even when the process has exited; do not equate a missing process
with proof that its tools did nothing.

## Lifecycle and retained allocations

Pending invitation expiry is creation plus two hours; active inactivity expiry
is last significant activity plus 24 hours; retained history is terminal time
plus 30 days. These are configurable defaults. Calls, successful joins, and
new participant message/status inserts change significant activity; reads,
acks, waits, duplicates, export and worker heartbeat do not. Export does not
extend retention. Evaluate expiry using the injected daemon clock.

New conversations never copy prior messages or worker threads. Pruning deletes
payload, recipients, membership, consumer state and bindings, while retaining
project IDs, unordered-pair allocation state, invitation code digests and
call request tombstones.
There is no v1 project-ID deletion that could reset pair numbering. Public
forums, broadcasts, claims and other adapters remain future scope; participant
rows permit later membership extension without implementing it now.

## Verification ownership

The Slice 1 document/schema checker exercises DDL creation, uniqueness,
foreign keys, rollback, and preservation of allocation rows in an in-memory
SQLite database. This is contract validation, not Go-driver or runtime
acceptance. T2.2 owns the actual driver, migration failure/reopen and storage
checks; T3 owns identity and joins; T4 owns delivery/ack concurrency; T6/T7 own
retention and complete exports; T8 owns worker recovery and uncertainty.
