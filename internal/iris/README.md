# Iris application

This package owns command dispatch, configuration, local transport, and SQLite
persistence. The root executable assembles signal cancellation and streams.
Constructors perform no IO; setup and service readiness are explicit.

Tests use temporary paths and package-owned fixtures. Run focused checks with
`go test ./internal/iris`. The repository Makefile defines delivery checks.

Configuration resolves the standard XDG data, config, and runtime roots. The
optional `iris/config.json` accepts `schema_version` (1), `max_connections`
(1 through 128, default 128), and `request_timeout_seconds` (1 through 60,
default 10). Unknown fields, malformed JSON and relative XDG paths fail.
Root help performs no configuration or service IO.

The service uses one compact JSON request per Unix stream, protocol `iris/v1`,
and schema version 1. Requests end with a newline and write EOF; clients call
`CloseWrite` while retaining the read side. The daemon validates the entire
bounded frame before dispatch and closes the response stream after one result.
Requests are limited to 1 MiB and responses to 8 MiB. Peer disconnect, service
cancellation, deadlines, and connection saturation are bounded.

The daemon holds a private runtime flock before opening SQLite. A second
owner cannot replace its socket. Only a private, user-owned stale socket is
reclaimed. `iris stats --all-projects --json` confirms readiness after schema
migration and returns current project, conversation and message counts.

`make test-integration` enables the integration build tag and runs the actual
binary through graceful and crash recovery using isolated fixture roots.
Only test-owned processes are signaled. A changed migration checksum is
reported as `schema_mismatch` before the listener starts.

Project operations canonicalize filesystem inputs before a write transaction,
then recheck unique path/alias ownership in SQLite. IDs remain stable through
explicit rename/relocation; participants and messages retain alias snapshots.
Only call, join and explicit registration can bind an unregistered cwd.

Call creation atomically stores the unordered-pair ordinal, two participants,
invitation, initial message and request-key digest. Code digests are permanent
tombstones; up to sixteen cryptographically generated candidates are attempted.
The fixed version-1 dictionary contains 256 unique ASCII words. An unknown
recipient alias reserves a project ID before its directory is registered.

Join validates recipient identity before binding its directory. The first join
adds one system event; identical retries return the initial request without
creating consumer state or renewing inactivity. Close requires joined
participation. Terminal transitions and their sequenced system events commit
together, including due-expiry transitions that return an operation error.
The final event has a reserved sequence slot and can use an empty body.

`make test-integration` also runs a two-project actual-binary handshake with
`jq`, text/file/stdin input, scoped errors, rename/move, independent conversations
and crash/restart. Fixture database inspection happens while its daemon is
stopped. Public history paging and autonomous lifecycle maintenance have their
own tasks in the implementation plan.


Delivery uses the existing message/recipient and consumer tables. Send compares
normalized payload digests before rejecting a committed retry on terminal
history. New sends validate joined membership, lifecycle, reply scope and
budgets, then commit sequence, recipients, counters and significant activity
before success. The internal result-plus-ack primitive shares that transaction;
the bridge rechecks ownership in its own completion transaction.

Receive pages bound both item count and encoded bytes. Cursors carry version,
conversation, project, consumer and sequence, and cannot jump beyond the
consumer's delivered high-water mark. Reads update delivered state only; ack
advances the next addressed message and ignores unaddressed sequence gaps.

The service permits 64 waiters overall and 16 per project. Subscription precedes
a storage recheck to prevent lost wakeups. Notifications follow successful
commits. Idle waits hold neither SQLite nor a repeating database timer. Linux
socket hangup detection blocks in the kernel and uses a cancellation pipe for
cleanup, preserving the protocol's required write half-close. Framing remains
bounded; receive operation/client deadlines use wait plus five seconds.


The bridge is a separate foreground process with an exclusive project/conversation
lease. Typed owner RPCs receive, checkpoint, renew and atomically complete work.
`bridge.receive` uses the same bounded wait hub and may expose reachable terminal
system events. The Codex adapter starts lazily, uses one bound thread, extracts
only completed final output, and fails closed for denied or uncertain execution.
Local interactive authority requires a terminal and a once-only decision.
`make test-bridge` enables a separate live build tag and exercises real Codex
question/response/acknowledgment with two isolated projects. Ordinary unit and
integration targets do not invoke a model.
