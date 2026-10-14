# Trust and authority

Iris trusts processes local to the same operating-system user. Private Unix
socket and filesystem permissions exclude other UIDs under normal OS rules;
they do not isolate mutually hostile processes sharing a UID. There is no
network listener, remote identity service or multiuser authorization system.

An invitation code selects a local conversation and intended recipient. It is
not a standalone security credential outside this trust boundary. Canonical
project selection and joined membership constrain ordinary operations. Explicit
all-project history/stats/prune scope is local administration, not a second
identity or remote access grant.

Peer message bodies are untrusted data. They cannot select the worker's project,
thread, tools, sandbox, approval policy or ownership token. Prompts contain only
the current conversation's initial request, current addressed input and bound
project/membership. Inspection/export never supplies old history automatically.
A new conversation creates a fresh binding and Codex thread; only that binding's
own persisted thread can resume.

The daemon performs storage, delivery and lifecycle work. An explicitly started
bridge is a separate foreground process and starts Codex lazily. Default workers
use a read-only sandbox and deny approvals. Workspace writes and once-only
interactive authority require deliberate local selection. Unknown approval
requests fail closed; disconnection cannot grant permission. Daemon unit
restrictions neither grant tools nor define the separate worker's tool policy.

Iris provides at-least-once delivery and ordered processing acknowledgment.
Completed reply and acknowledgment publication is atomic, but external tools
are not transactional with SQLite. Uncertain execution cannot be silently
replayed. Inspect external effects and follow the [recovery contract](bridge.md).
Limits bound frames, storage, cursors, waiters, workers and execution lifetime;
reported token usage can overshoot within an active turn, and missing telemetry
remains null. No exactly-once external-effects guarantee is made.

Backups, transcripts and worker logs can contain project content. Keep them
private. A deliberate export can be supplied to another session, but its presence
does not turn retained history into an automatic inbox. Credentials and provider
configuration remain with the existing Codex installation; Iris does not copy or
change them.
