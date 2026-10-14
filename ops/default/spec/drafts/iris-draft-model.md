# Iris Data Model

**Status:** Draft companion; entity requirements extracted from the supplied
specification. Physical SQLite design remains open.
**Specification:** [Iris functional and technical specification](iris-draft.md).

## Ownership and persistence

The Iris daemon owns central SQLite persistence in the Iris XDG data directory.
Projects do not store Iris runtime state. The optional `.iris.toml` file is an
explicit user-authored alias override.

The entities below capture the specification's required semantics and
suggested storage concepts. They do not prescribe finalized tables, columns,
SQL types, indexes, or migrations. Verify Acta and Lore patterns and the actual
Codex integration before completing those implementation decisions.

## Entities and durable contracts

| Entity or value | Required information | Identity, relationships, and invariants |
| --- | --- | --- |
| Project | Stable internal ID, canonical absolute path, visible alias. | Identity is independent of alias, Git, and Codex sessions. A move requires explicit reassociation. |
| Project alias | Visible name and associated project ID. | Derive from the canonical path unless explicitly overridden. An ambiguous alias must never silently select a project. The precise collision-registration policy remains open. |
| Project pair sequence | Unordered participant pair and its ordinal allocation state. | A-to-B and B-to-A share one counter. Allocate transactionally; retain sufficient state to prevent reuse after conversation pruning. |
| Conversation | Global internal ID, human pair ordinal and label, topic, state, and lifecycle information. | Allow simultaneous independent conversations between the same pair. A new conversation inherits no messages. |
| Participant | Conversation association, project identity, and readable historical identity information. | Enforce participation on agent-facing operations. Preserve internal references and readable labels after alias changes. Model membership so future groups remain possible. |
| Invitation | Conversation association, three-word code, expected recipient, and expiration information. | Codes are unique among current invitations. A destination may initially be an unresolved alias. Validate the accepting project and resolve it during join. |
| Message | Version, conversation ID, message ID, conversation-local sequence, sender ID and alias, recipients, UTC creation time, kind, text body, topic, optional reply and correlation references, and optional idempotency key. | Persist before send acknowledgment. Order is stable within one conversation. Define the exact duplicate-detection scope and payload mapping during detailed design. |
| Consumer cursor or delivery | Conversation, consumer or adapter identity, stable read position, and explicit processing acknowledgment. | Reading does not imply processing. Recover pending delivery after daemon restart. Keep consumer progress separate. |
| Worker binding | Conversation, project, Codex execution association, processing state, errors, and configured execution limits. | Bind only after explicit activation. The binding is ephemeral and is not project identity; retain enough recovery information for restart without treating it as a permanent session. |

The specification suggests `projects`, `project_aliases`, `pair_sequences`,
`conversations`, `participants`, `invitations`, `messages`, and
`consumer_cursors` or `deliveries`. These names are provisional. Worker-binding
storage and exact recovery fields also remain to be designed.

## Conversation lifecycle

| State | Meaning | Messaging and visibility |
| --- | --- | --- |
| `pending` | The initiator created an invitation and stored an initial message. | The expected recipient has not joined; join returns the topic and initial message. Ordinary exchange is not yet enabled. |
| `active` | The validated recipient joined. | Participants can exchange messages while the conversation remains active. |
| `closed` | A participant explicitly closed the conversation. | Reject new ordinary messages; remove from active inboxes; retain history under retention policy. |
| `expired` | The service expired the invitation or conversation. | Reject late ordinary messages without reopening; remove from active inboxes; retain history under retention policy. |

Default pending lifetime is 2 hours. Default active inactivity lifetime is
24 hours. Default closed or expired history retention is 30 days. These are
configurable policy values. Reads, listings, and long-polls must not keep a
conversation alive indefinitely.

Logical expiration, retained history, and irreversible deletion are separate
operations. Pruning must preserve non-reuse of pair ordinals. Destructive
actions require a preview and explicit confirmation as defined in the
specification.

## Message and delivery guarantees

- Define a versioned public message representation with the fields required by
  specification section 7.2.
- Apply foreign keys and indexes appropriate to project participation,
  conversation ordering, invitation resolution, delivery recovery, and
  structured historical search.
- Allocate pair ordinals and per-conversation message sequences, and apply
  lifecycle changes, transactionally.
- Persist messages before confirming sends and recover pending messages after
  daemon restart.
- Document delivery semantics. Do not promise exactly-once agent execution.
- Process each worker's messages sequentially and acknowledge only after its
  response or result is recorded.
- Keep conversation context isolated. Closed, expired, and unrelated history
  must not be injected into a worker automatically.
- Enforce configurable message-size and bridge-execution limits. Their exact
  defaults are not supplied by the initial specification.

## Queries and external mappings

Project-facing queries infer the project from the working directory or an
explicit `--project` selector. They must validate participation and avoid
accidental access to unrelated conversations. Explicit human administrative
queries must also work outside repositories.

Historical lookup must support project pair, date, ordinal, internal ID, and
structured text filters. Exports include conversation metadata, participating
projects, lifecycle, and ordered semantic messages. Human exports remain
chronological conversations rather than raw internal event logs.

The Unix socket protocol, CLI JSON responses, and transcript exports must have
documented versioned mappings to stored semantics. Codex execution identifiers
belong to worker bindings, not project identity. Received messages remain
untrusted data and grant no additional authority.

## Decisions still requiring verification or design

1. The SQLite driver, physical schema, SQL types, concrete indexes, migration
   mechanism, and operational use of WAL and `busy_timeout`.
2. Canonical-path discovery from nested working directories and the explicit
   project-move procedure.
3. Alias collision handling, expected-recipient resolution for initially
   unregistered destinations, and historical label representation.
4. Invitation word list, generation, collision retries, expiration, and reuse
   policy consistent with conversation isolation.
5. Which events count as significant activity, retention behavior after export,
   and the exact destructive-preview confirmation contract.
6. Stable cursor representation, consumer identities, acknowledgment scope,
   idempotency-key scope, and crash recovery around a completed worker turn.
7. The installed Codex interface, session or separate-worker behavior, binding
   recovery, execution limits, and available cost or usage measurements.
8. Exact message-kind vocabulary, size limits, error contracts, protocol
   envelopes, and JSON schema-version policy.

Future channels, broadcasts, and claims remain documented future scope. Do not
implement their tables or APIs solely to prepare for them; preserve the ability
to extend participant membership.
