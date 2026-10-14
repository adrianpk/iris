# Iris Functional and Technical Specification for Codex

**Status:** Accepted implementation baseline; Slice 1 contracts settled.
**Product:** Iris.
**Family:** Acta, Lore, and Iris.
**Initial platform:** Linux, a local per-user service, CLI, and user systemd.
**Audience:** The Codex agent responsible for designing, implementing, testing,
and documenting Iris.
**Source:** User-supplied `IRIS_SPEC.md`, translated into English.
**Model companion:** [Iris data model](iris-model.md).

## 1. Primary instruction

Implement **Iris**, a lightweight local communication tool for agents working
on independent projects. Iris lets two agents exchange questions, answers, and
results directly, without requiring the user to copy messages between Codex
sessions.

Iris is the third application in the **Acta, Lore, and Iris** family:

- **Acta** records agent activity and events.
- **Lore** preserves and retrieves reusable knowledge.
- **Iris** temporarily connects agents and projects through conversations.

These are independent products with distinct responsibilities. Their
architecture, conventions, structure, CLI experience, documentation, and local
operation must make them recognizable as sibling tools.

**Iris is not a task scheduler, ticket manager, memory system, or general
orchestrator.** Its responsibility is to establish conversations, transport
messages, manage their lifecycle, and support delivery to reactive agents.
Each agent's own tools and permissions govern work execution.

## 2. Mandatory first task: study Acta and Lore

Before deciding the structure of Iris or writing its implementation, inspect
these repositories in the development environment:

- Acta: <https://forge.adrianpk.com/adrianpk/acta>.
- Lore: <https://forge.adrianpk.com/adrianpk/lore>.

Do not assume their details match this specification. **Verify them.** Compare
at least:

1. Directory, package, and component organization.
2. Language, code conventions, dependencies, build, and installation.
3. CLI-to-service contracts, local protocol, and Unix socket.
4. User systemd units, activation, restart, and shutdown.
5. XDG conventions, configuration, logs, errors, and exit codes.
6. Command style, flags, `--help`, and output formats.
7. Tests, fixtures, migrations, releases, and CI.
8. README, additional documentation, examples, CONTRIBUTING or AGENTS.md when
   present, and licensing conventions.

Use those patterns as the primary reference for Iris when they are compatible
with this document. **Do not modify Acta or Lore** as part of this work. Do not
introduce runtime dependencies on either product.

**Iris-specific decision: primary persistence must use SQLite.** If Acta or
Lore use JSONL, follow their compatible operational and structural patterns,
but do not copy their storage format by inertia. The user selected SQLite for
Iris.

If repository access is unavailable, identify that blocker explicitly before
inventing family conventions.

## 3. Objective and governing use case

The user works with several Codex sessions, normally one per project. One
session may discover that it needs knowledge or a correction in another tool
being developed by another agent. The user currently acts as an intermediary,
carrying the problem to the other agent and manually bringing back its answer.

The intended workflow is:

1. In project A, the user asks the agent to consult the agent for project B.
2. Agent A opens an Iris conversation and presents an identifier that is easy
   to communicate.
3. The user tells agent B to join that conversation. The user must not need to
   copy technical context or messages.
4. The agents exchange messages through Iris, may work within their own
   projects, and communicate results.
5. The conversation closes or expires. A new conversation between A and B
   starts **without inherited messages**.
6. The user can later inspect or export the history through the CLI.

Ordinary human intervention should be limited to starting collaboration and,
when required, authorizing sensitive actions. **Subsequent communication must
not require the user to relay messages.**

The design must work with any pair of local projects, rather than only a
specific application and its looping tool.

## 4. Design constraints

- **Local first:** an always-available, low-overhead per-user daemon, without a
  mandatory remote server.
- **Low token cost:** no inbox check automatically added to every ordinary
  agent turn. No model invocation when no messages arrive.
- **Reactive agents:** storing messages is insufficient. An explicit,
  verifiable mechanism must deliver them to a Codex execution that can respond.
- **Ephemeral Codex sessions:** do not depend on an interactive session staying
  open or use its internal identifier as permanent project identity.
- **No mandatory repository configuration:** normal operation must not create
  files in the project.
- **Ephemeral messages with recoverable audit history:** expired content leaves
  active context but remains exportable during retention.
- **First-class CLI:** usable by humans and agents, without a web interface in
  the first version.
- **Simplicity:** avoid a distributed platform, generic bus, or unnecessary
  infrastructure.

## 5. Project identity

Stable identity belongs to the **project**, not to a Codex session.

### 5.1 Discovery and names

- Initially identify each project by its **canonical absolute filesystem
  path**, with consistent normalization and symbolic-link resolution.
- Derive its **visible alias** from the final path segment:
  `/home/user/projects/looper` becomes `looper`.
- Support an **optional override** in a project-local file, preferably
  `.iris.toml`, such as `name = "looper-dev"`. The user may add this file to
  `.gitignore`. Iris **never creates it automatically**.
- Persist project registration and the path, identity, and alias association in
  central Iris storage, outside the repository.
- Allow explicit project registration, alias inspection, and alias changes
  through the CLI.
- Do not depend on Git, remotes, or GitHub or forge repository names.

### 5.2 Collisions and changes

A duplicate alias **must not** silently resolve to the wrong project. On a
collision, require explicit identification or an alternative alias and report
the problem clearly. Use a stable internal key distinct from the visible
alias.

Historical conversations must retain internal references and readable labels
when an alias changes. Provide an explicit reassociation procedure when a
project moves. Do not assume the old path still identifies it after the move.

The CLI must infer the current project from the working directory or accept an
explicit `--project` option.

## 6. Conversations and identifiers

### 6.1 Bilateral relationships and independent conversations

A relationship between A and B is not a single permanent conversation. Two
projects can have many separate conversations over time.

- Treat the project pair as **unordered**. A-to-B and B-to-A use the same pair
  for numbering.
- Assign a **sequential number per pair**, incremented transactionally:
  `hatmax ↔ looper #0001`, `#0002`, and so on.
- Each conversation must also have a **globally unique, stable, unambiguous
  internal ID**. The ordinal and visible label are for humans, not the only
  primary key.
- Never reuse ordinals after conversations close, expire, or are pruned.
- Allow concurrent conversations between the same pair and keep them isolated.

### 6.2 Invitation code

Creating a conversation generates a short, pronounceable **three-word** code,
such as `amber-otter-river`. This is an illustrative example, not a prescribed
algorithm. The code:

- Is easy to read, dictate, copy, and paste between sessions.
- Identifies **an invitation to a conversation**, not a project or the entire
  relationship between projects.
- Is unique among current invitations; regenerate it after a collision.
- Expires and must not associate a new conversation with old messages through
  reuse.
- Must not be presented as a strong security credential. Local security rests
  on system permissions and participant-project validation.

The daemon must verify that the project accepting an invitation matches the
expected recipient or clearly report a mismatch. The destination may be given
as an alias before its path is registered; resolve and validate it during join.

### 6.3 States and expiration

Model at least these states:

- `pending`: the invitation exists and awaits the second participant.
- `active`: the second participant has joined and exchange is enabled.
- `closed`: explicit closure.
- `expired`: automatic expiration.

Initial **configurable defaults**, not hardcoded invariants:

- Pending invitation: **2 hours**.
- Active conversation: **24 hours without messages or significant activity**.
- Closed or expired history retention: **30 days**, subject to user-configured
  policy or export.

The service may expire invitations and conversations without an agent. Reading,
listing, or long-polling a conversation must not renew it indefinitely. Closure
or expiration **prevents new ordinary messages** and **removes the conversation
from active inboxes**, without immediately deleting its history.

A new conversation always has an empty history and a distinct ID, even for the
same participants.

## 7. Handshake and message protocol

### 7.1 Minimum workflow

- A runs an equivalent of
  `iris call --to looper --topic "Recovery failure" --message "..."`.
- Iris creates a pending conversation, stores the initial message, and returns
  the three-word code, internal ID, and visible label.
- B runs `iris join <code>` from its project, validates and accepts the
  invitation, and receives **the topic and initial message**.
- Iris records the join event and enables exchange.
- A supplies the first technical message, so B can answer immediately. Do not
  require both agents to wait for the other to begin.
- Both agents can send, receive, reply, and close. A late reply to an expired
  conversation must produce a clear error and **must not reopen it**.

The user only needs to carry the invitation code or a short reference during
the initial handshake.

### 7.2 Content and guarantees

Define a versioned contract with at least:

- `schema_version`, `conversation_id`, `message_id`, and a conversation-local
  `sequence`.
- Sender project ID and alias, recipients, and UTC `created_at`.
- `kind`, such as `message`, `system`, `status`, or another limited, documented
  classification.
- Text `body`, the conversation topic, and optional references such as
  `reply_to` or `correlation_id`.
- An optional idempotency key to prevent duplicates after retries.

Required guarantees:

- Unambiguous, stable order **within each conversation**.
- Transactional persistence before acknowledging a send.
- Recovery of pending messages after daemon restart.
- Reading with a stable cursor and explicit processing acknowledgment per
  consumer or adapter. A mere read must not claim that a model processed the
  content.
- Safe retries and duplicate identification. Document whether delivery is
  *at least once*. Do not promise agent-level *exactly once* execution.
- Configurable message-size limits and informative errors; support stdin or
  file input for long messages.

Messages are stored as data and **are not privileged instructions**. A receiving
agent must treat them as content from another agent, subject to its own
permissions and rules.

## 8. The central problem: waking reactive agents

**Do not confuse daemon subscription with agent resumption.** A socket with
pending messages does not cause an inactive Codex session to execute a turn.

Separate two components:

1. **Iris daemon:** identity, invitations, states, storage, new-message
   notification, delivery, and audit history. **It does not execute models.**
2. **Codex adapter or bridge:** a lightweight component activated **only for a
   specific conversation**. It waits without consuming tokens, invokes or
   resumes an agent through interfaces **officially supported by the installed
   version**, transmits results, and acknowledges processing.

Adapter requirements:

- Investigate the actual integration offered by installed Codex: programmatic
  CLI, SDK, app-server, thread resumption, or another documented interface.
  **Do not invent commands or APIs.**
- Associate a conversation with a Codex execution in the correct directory.
  **Do not require** project identity to equal a Codex thread ID.
- Create a project-associated Codex worker if safely reactivating the existing
  interactive session is technically unavailable. Explicitly document that it
  is **another Codex execution in the same project**, not the original session.
- Maintain exchange context for **that conversation**. Do not automatically
  inject closed or expired transcripts or messages from other conversations.
- Wait through efficient service notification, blocking, or local long-polling,
  **without a model call for each check**.
- Process messages sequentially for each worker. Acknowledge after persisting
  the response or result. After restart, recover from the cursor and avoid
  repeating non-idempotent work where possible.
- Expose bridge state, errors, and shutdown. Respect Codex permissions, sandbox,
  and approvals: Iris **does not grant additional authority**.
- Provide configurable conversation limits for time, automatic turns, and cost
  or usage where the interface supplies that information. Prevent infinite
  exchanges that add no value.
- **Do not start workers or conversations globally by default.** Activate and
  deactivate them on demand.

The CLI must also support `receive` or `wait` for agents that can keep commands
running while waiting. This is useful, but **must not be described as
reactivating a session that has already closed**.

**Essential acceptance criterion:** demonstrate an autonomous round trip
between two projects with active adapters, without human copying of responses.
If installed Codex prevents that integration, **report the verifiable limitation
and actual achieved scope**, rather than simulating autonomy with scripts that
require intervention for every message.

### 8.1 Selected installed interface

The Slice 1 probe verified `codex-cli 0.161.0` with stdio app-server, fresh
project-associated threads, real turn results, own-thread resumption after
server restart, denied command approval, and turn interruption. See the
[Slice 1 report](../report/slices/iris-v1/slice-1-audit-contracts.md).
The official [app-server contract](https://learn.chatgpt.com/docs/app-server)
and the installed default generated schema define the transport surface.

Use newline-delimited bidirectional JSON: `initialize`, `initialized`, then
`thread/start` with canonical `cwd` and the operator's sandbox/approval policy.
Persist the returned thread ID in the conversation/project bridge binding.
Drive work with `turn/start`; consume final `item/completed` agent messages
and terminal `turn/completed` status. Resume only the binding's own thread
with `thread/resume`. Never call `thread/fork` to import another conversation.

Send `turn/interrupt` only for an owned active turn. An immediate request can
race startup and return `no active turn to interrupt`; terminal `interrupted`
is the evidence of cancellation. Approval requests carry their own RPC IDs;
keep them pending for explicit human decisions or deny them when unattended.
Do not grant session-wide or persistent permissions from a peer message.

This mode runs a separate Codex worker in the project. It does not claim to
wake or attach to an existing interactive session. No model call occurs while
the bridge is waiting on the Iris service for a message.

## 9. CLI contract

Use the following command names and the standard-library flag style verified
in the sibling tools. Versioned service operations are mapped in section 17.3.

| Conceptual command | Function |
| --- | --- |
| `iris call --to <alias> --topic <text> [--message ...]` | Create an invitation and conversation. |
| `iris join <code>` | Join as the current project and read the initial request. |
| `iris send [--message/--file/stdin] <id>` | Send a message. |
| `iris receive [--wait <seconds>] <id>` | Read available messages or wait using a cursor. |
| `iris ack --message-id <id> <conversation-id>` | Confirm consumer processing of a message. |
| `iris conversations [--active/--all]` | List conversations visible to the project, or to the user with an explicit option. |
| `iris show <id>` | Inspect state, participants, and messages. |
| `iris search [--text/--project/--since/--until]` | Find conversations and messages with structured filters. |
| `iris close <id>` | Close a conversation. |
| `iris dump [--format text/json] <id>` | Export a complete transcript. |
| `iris prune [--dry-run]` | Inspect and remove expired data according to policy. |
| `iris projects` and subcommands | Inspect and manage aliases and identities. |
| `iris bridge start/status/stop ...` | Manage the Codex adapter on demand. |
| `iris stats` | Summarize local conversations, retention, and storage. |
| `iris --help` | Discover capabilities with clear text and examples. |

Cross-cutting CLI requirements:

- **Human output** by default: concise lists and readable, flowing transcripts.
- `--json` where appropriate: **valid, stable, complete** output usable by `jq`.
  Specify `schema_version`. Do not mix decorative text into JSON stdout.
- Errors go to stderr with nonzero exit codes and actionable messages.
- Support explicit project selection. Human administrative actions must not
  depend on running the CLI inside a repository.
- Present internal IDs, invitation codes, and visible names without ambiguity.
- Destructive operations require `--dry-run` or a preview and explicit
  confirmation, with an unambiguous noninteractive option for automation.

### 9.1 Transcripts and human recovery

The `text` dump must read as a complete conversation: a header with date, topic,
participants, and state, followed by chronological messages with author and
timestamp. It must not be raw JSON logs or unintelligible internal events.

The `json` dump must contain conversation metadata, participant projects,
lifecycle, and an ordered array of messages with semantic fields. Support
historical selection by pair, date, ordinal, or ID. Finding, for example,
`hatmax ↔ looper #0002` and exporting its exchange for explicit input to a new
Codex session must be straightforward.

**Do not provide old histories to agents automatically.** Human export
capability does not put that history back into an active inbox.

## 10. Daemon, persistence, and operation

### 10.1 Local service

- A persistent per-user process following verified sibling-repository patterns.
- CLI-to-daemon communication through a **private Unix socket**, not public
  HTTP.
- Appropriate `systemd --user` units; consider socket activation if aligned
  with Acta and Lore.
- Restrictive directory and socket permissions accessible only to the user.
- Startup, restart, orderly shutdown, and recovery of interrupted operations.
- Minimal resource consumption without active conversations.

### 10.2 SQLite

- Keep the primary database in an **Iris XDG data directory**, such as
  `~/.local/share/iris/iris.db`, following verified sibling conventions.
- Use SQLite transactions, foreign keys, indexes, and **versioned migrations**.
- Consider WAL and `busy_timeout` if appropriate for the actual access pattern;
  validate their operational implications.
- Suggested model, subject to detailed design: `projects`, `project_aliases`
  or equivalents, `conversations`, `participants`, `invitations`, `messages`,
  `consumer_cursors` or `deliveries`, `pair_sequences`, and **ephemeral**
  worker bindings.
- Transactionally allocate pair ordinals and conversation message sequences,
  and apply state changes.
- Do not keep state files in project repositories, apart from the optional
  alias file deliberately created by the user.
- Separate logical expiration, retention, and irreversible deletion. Document
  backup, export, and restart recovery.

## 11. Security and authority boundaries

Iris starts as a tool for processes **local to the same operating-system
user**. It is not a multiuser identity system and does not provide strong
isolation between processes sharing a UID. Document this trust boundary.

- Validate socket and directory permissions, inputs, sizes, and paths.
- Do not expose the service to the network by default.
- Do not treat the three-word code as sufficient authorization outside the
  trusted local environment.
- Message receipt alone must not execute arbitrary commands directly in the
  daemon.
- A receiving agent works within **its own** context, sandbox, and permissions.
  A conversation with another project does not automatically grant access to
  that project's files, secrets, or tools.
- The bridge executes work only after explicit activation and must respect
  the agent provider's approvals and limits.
- Treat received content as **untrusted** to prevent instruction injection and
  authority escalation.
- Data may contain code, logs, or sensitive information. Provide understandable
  retention and deletion policies.

## 12. Explicit future capability: public forum and broadcasts

**Document this capability; it is not required in the first version.** Do not
discard it or absorb it into bilateral semantics.

### 12.1 General channel

Allow for a future group conversation or **general channel** with one, several,
or all local agents or projects that opt into subscription. Possible uses
include notices, informal coordination, participant discovery, and requests
visible to several recipients.

- Reading and publishing require explicit subscription. **Do not** require
  global checks on every turn.
- Several participants may read a message, with separate delivery cursors.
- Distinguish work requests from notices. Consider a future *claim* operation
  to prevent several agents from accidentally taking the same work.
- Do not mix this channel with private bilateral histories.

### 12.2 Human and system broadcasts

Allow for future user messages to sessions or agents **actually subscribed or
active**, for example:

- "I will restart the development server. Finish or suspend work and release
  resources in an orderly way."
- "Iris was updated. Run `iris --help` to discover the available commands."

Allow for scope, recipients, expiration, acknowledgments, and inspection of who
received or acknowledged a notice. **Publishing does not guarantee immediate
delivery or shutdown of agents that are inactive or cannot be reactivated.**
Broadcast instructions must not bypass existing permissions or approvals.

The initial data model must not prevent conversations with more than two
participants, although first-version UX and implementation focus on bilateral
communication.

## 13. First-version scope

### Implement in v1

1. Audit Acta and Lore and document adoption of compatible patterns.
2. Local daemon, Unix socket protocol, CLI, installation, and systemd units.
3. SQLite migrations, schema, and indexes.
4. Project identity, path-derived aliases, optional overrides, and collisions.
5. Three-word invitations, handshake, and bilateral conversations numbered by
   pair.
6. Durable messaging, cursors, acknowledgments, retries, closure, expiration,
   and retention.
7. Human and JSON CLI output, conversation search and listing, transcripts,
   export, stats, and pruning.
8. **Functional Codex integration on demand**, with a verified automatic round
   trip and explicit limitations when the original interactive session cannot
   be resumed.
9. Tests, documentation, examples, and consistency with Acta and Lore.

### Document for later without unnecessary v1 implementation

- Public or group channels and multiple subscriptions.
- Broadcasts to active agents with acknowledgments.
- *Claim* operations for public requests.
- Adapters for agents other than Codex.
- Remote or multiuser integration.

Do not introduce a substantially larger generic architecture solely for these
possibilities. Avoid decisions that prevent extending the participant model.

## 14. Mandatory validation and tests

Alongside unit tests, implement real daemon and CLI integration tests:

1. **Identity:** new directory, derived alias, optional alias, name conflict,
   moved project, and explicit resolution.
2. **Handshake:** creation, correct acceptance, wrong-recipient rejection,
   expired invitation, and concurrent join.
3. **Numbering:** several conversations between the same pair in both
   directions; monotonic ordinals after pruning; isolated concurrent
   conversations.
4. **Messages:** order, persistence, cursors, idempotent resend, retry after a
   crash, and no message loss after daemon restart.
5. **Isolation:** new conversations receive no messages from earlier ones;
   agent-facing project commands do not accidentally expose unrelated
   conversations.
6. **Expiration and retention:** closure, inactivity expiration, historical
   search and dump, deletion only when eligible; controlled clocks without
   long sleeps.
7. **Output:** JSON parseable and filterable with `jq`; consistent chronological
   human transcripts.
8. **Operation:** socket permissions, user-systemd startup, restart,
   disconnections, and shutdown.
9. **Bridge:** two projects and two Codex executions exchange at least a
   question, response, and acknowledgment **without human intervention between
   messages**. Verify that idle waiting generates no model calls. Simulate
   adapter crashes and recovery.
10. **Security:** malformed content, maximum sizes, untrusted input, permission
    errors, and absence of automatic privileged execution.

Provide a reproducible end-to-end example using **two temporary test projects**.
The test must not require modifying Acta or Lore or using the user's real
projects.

## 15. Documentation and deliverables

The finished repository must be recognizable as a sibling of Acta and Lore.
After inspection, align:

- Repository structure and naming conventions.
- README: introduction, installation, requirements, quickstart, examples,
  architecture overview, and troubleshooting.
- CLI, protocol, schema, lifecycle, expiration, retention, and trust-model
  documentation.
- Service units, build, install, uninstall, tests, and release.
- Agent instructions and project conventions, such as `AGENTS.md`, **only if**
  present or consistent with Acta and Lore.
- Human and JSON output examples, including `jq` filtering.
- An explicit **Planned capabilities** section preserving public-forum and
  broadcast design.

Describe the three products consistently without implying functional
dependencies between them. Use appropriate cross-links and a documentation
tone and structure consistent with existing repositories.

### Expected Codex delivery

1. A brief report of **observed**, not assumed, Acta and Lore patterns and Iris
   decisions.
2. Functional implementation in verifiable stages, including inter-agent
   integration.
3. Test evidence and a real A-to-B round trip without human message relaying.
4. README, technical documentation, and preserved future scope.
5. Concrete limitations, particularly where installed Codex cannot wake or
   resume another interactive session.

## 16. Final product criterion

**Iris succeeds when the user can tell one agent to communicate with the agent
for another project, give the second agent an easy-to-remember identifier, and
let the technical conversation proceed through Iris without personally
transporting each message.**

The service must stay lightweight when unused. Projects must retain
recognizable identities across Codex session creation and closure. Old
conversations must not contaminate new ones. The user must retain the ability
to inspect, export, and prune history through a CLI as consistent as possible
with Acta and Lore.

## 17. Settled v1 implementation contracts

This section resolves the initial design alternatives for implementation. The
[model companion](iris-model.md) defines the matching schema, indexes,
transactions, and query mappings. These contracts are selected for v1; later
slices must provide their runtime evidence before claiming the behavior works.

### 17.1 Storage and dependency

Select `modernc.org/sqlite v1.60.1`, the pure-Go `database/sql` driver, for T2.2.
Its downloaded module metadata requires Go 1.26.0 and is compatible with this
repository's Go 1.26.3 baseline. This confirms the dependency selection before
addition; Slice 1 does not change `go.mod`. Pin the driver and its required
transitive versions when adding it. See its
[versioned documentation](https://pkg.go.dev/modernc.org/sqlite@v1.60.1).

Only the daemon opens the database for normal operation. Configure every
connection with foreign keys, a five-second busy timeout, WAL journal mode,
and `synchronous=FULL`. Use one database connection initially; never hold a
transaction while waiting on a socket, approval, or model. Use immediate write
transactions for allocation and state changes; apply a bounded retry only
before a transaction has committed. An uncertain transport reply is retried
with the original idempotency key. See SQLite's
[transaction](https://www.sqlite.org/lang_transaction.html) and
[WAL](https://www.sqlite.org/wal.html) contracts.

Use ordered embedded SQL migrations with a version/checksum ledger. Apply
pending migrations and their ledger writes atomically before listening; fail
on a newer schema, a changed applied checksum, or migration failure. A failed
upgrade rolls back and does not serve partially migrated state. Backups use
SQLite's online backup interface or a verified consistent snapshot after all
owned connections are closed. Copying a live `iris.db` alone is not a backup.

Use `$XDG_DATA_HOME/iris/iris.db`, defaulting to
`~/.local/share/iris/iris.db`; `$XDG_CONFIG_HOME/iris/config.json`, defaulting
to `~/.config/iris/config.json`; and
`$XDG_RUNTIME_DIR/iris/iris.sock`. Require absolute XDG overrides and an owned
runtime directory. Directories are 0700; database, WAL, backup, and temporary
export files are private to the user, with umask 0077. A version-1 JSON config
contains only documented policy fields; reject unknown fields and invalid
ranges. CLI `--help` works without a socket or XDG runtime setup.

### 17.2 Identity, aliases, and invitation resolution

Canonicalize existing directories with absolute normalization and symlink
resolution. For discovery, choose the nearest registered canonical ancestor
of cwd, without reading Git metadata. When none exists, `call`, `join`, or
`projects register` may register the exact cwd; other project-facing commands
return `project_not_found`. Register from the intended root before using an
unregistered nested directory. Explicit `--project` accepts a stable ID,
canonical path, or unambiguous alias and disables cwd inference.

Aliases use lowercase ASCII letters, digits, and single hyphens, 1 through
64 bytes, with letters or digits at both ends. For a derived alias, lowercase
the basename, replace runs of other characters with a hyphen, and trim edge
hyphens; reject an empty or overlong result. Explicit names must already
satisfy this grammar. The optional `.iris.toml` supports one `name = "alias"`
assignment with blank lines and full-line comments; reject duplicate or other
keys rather than pretending to parse arbitrary TOML. Read it during
registration, including a first call/join registration. Later edits require
an explicit rename. Iris never creates or rewrites it.

Persist a random 128-bit project ID independently of path or alias. An alias
is unique in central storage. Registration with an occupied alias fails
unless it is the unregistered destination placeholder being resolved by the
same canonical registration. Renaming requires `projects rename
--name <alias> <id>`; moving requires `projects relocate --path <directory> <id>`.
Neither operation merges project IDs or changes historical participant labels.
A path already assigned to another ID produces `project_conflict`.

For an unknown `call --to <alias>`, create a provisional project with that
reserved alias and no path. Allocate its stable ID immediately, so pair
numbering does not change when it joins. Registration/join binds the matching
alias to its first canonical path transactionally. A different alias or a
project already registered under another identity cannot claim the invitation.
Placeholders cannot be renamed or relocated before registration.

Sort the two distinct project IDs to identify an unordered pair. Allocate its
next ordinal, conversation, two participant snapshots, invitation, and initial
message in one transaction. Allocate an ordinal only on successful creation;
never reset or delete the pair allocation row or its referenced project IDs
when pruning. Display ordinals with at least four digits and no upper-width
truncation. Topics are required, at most 256 UTF-8 bytes.

Generate three lowercase words independently with `crypto/rand` from a fixed,
versioned 256-word ASCII list; repeated words are allowed. Try at most 16
candidates. Keep SHA-256 code tombstones after history pruning and never
reissue a code. This prevents a saved invitation from joining a later
conversation. The code is a convenience identifier within the same-user trust
boundary. Concurrent join is a compare-and-set transaction: exactly one
project binds the expected participant, records the join event, and activates
the conversation. A repeated join by that same project returns the same
conversation and initial message without another event; another project gets
`recipient_mismatch`. Expired or closed invitations cannot activate anything.

### 17.3 Message, consumer, and protocol contracts

Each public message has `schema_version: 1`, `conversation_id`, `message_id`,
positive `sequence`, nullable `sender_project_id` for daemon events,
`sender_alias`, `recipients` containing project IDs, UTC RFC3339Nano
`created_at`, `kind`, `body`, `topic`, and nullable `reply_to`,
`correlation_id`, and `idempotency_key`. IDs are opaque 128-bit random values
encoded as 32 lowercase hexadecimal characters. `message` and `status` come
from joined participants; `system` is daemon-only and uses sender alias
`iris`. Messages are immutable. Reply targets must be in the same
conversation. Preserve sender/participant alias snapshots on historical reads.

Initial technical content is sequence 1 and is addressed to the expected
recipient. Join and terminal lifecycle events receive ordinary next sequence
numbers. Allocate sequences and insert the message and recipient rows in the
same transaction. Default human sends address the other participant; reject
recipients outside the conversation. Ordinary sends require an active
conversation and joined sender. Lifecycle events do not invoke models.

Creation requires a caller request key scoped to `(initiator_project_id,
request_key)`. The CLI generates one per call unless `--idempotency-key` is
supplied and reuses it for transport retries. Store the digest of normalized
recipient selector, topic and initial content with the resulting conversation.
An identical call retry returns that original invitation and conversation;
changed content conflicts. After pruning, retain the request tombstone and
return `not_found` rather than creating another conversation for that key.

Scope message idempotency to `(conversation_id, sender_project_id, idempotency_key)`.
Store a canonical payload digest covering kind, body, sorted recipients,
reply target, and correlation ID. An identical retry returns the original
message, sequence, and timestamp, including after closure; different content
with the same key returns `idempotency_conflict`. Do not compare an invented
new timestamp or message ID. Without a key, repeated sends are separate
messages. Keys and correlation IDs are at most 128 UTF-8 bytes.

A consumer is identified by `(conversation_id, project_id, consumer_id)`;
`consumer_id` is a nonempty string of at most 128 bytes. Manual receive defaults
to `manual`; each bridge binding uses its own stable consumer ID. Default
receive starts after the durable acknowledged sequence and returns addressed
messages in sequence order. Reading can advance `delivered_sequence` but
never `acked_sequence`. An explicit cursor encodes version, conversation,
project, consumer, and after-sequence; validate all scope fields and reject
future/out-of-range positions. Cursors grant no authority.

`ack --message-id <id> --consumer <id> <conversation>` acknowledges only the
next unacknowledged addressed message that has been delivered to that
consumer. It cannot skip an earlier addressed message or another consumer's
work; duplicate acknowledgments are idempotent. Ignore gaps consisting only
of messages not addressed to the consumer. Persist acknowledgments
transactionally. Delivery is at least once until acknowledgment; no guarantee
of exactly-once agent execution is made. Closed/expired histories are
available through explicit inspection/export, not ordinary receive.

Use one bounded newline-terminated JSON request and response per Unix stream.
The client half-closes its write side after the request while keeping its read
side open; the daemon requires request EOF before dispatch so trailing frames
are rejected. The response ends with connection EOF:
`{"protocol":"iris/v1","operation":"send","params":{}}`.
The success shape is
`{"protocol":"iris/v1","schema_version":1,"ok":true,"result":{}}`;
a failure replaces `result` with
`"error":{"code":"invalid_request","message":"...","retryable":false}`
and sets `ok:false`. Result objects and exported messages use version 1.
Reject malformed UTF-8/JSON, trailing values, unknown fields, unsupported
versions, and unknown operations. CLI JSON success prints the same semantic
result envelope, without logging or decoration. On failure stdout is empty,
stderr contains a readable error or the JSON error envelope when `--json` is
set, and the process exits nonzero. Exit 0 means success, 2 means invalid CLI
usage, 1 means an operating/service failure, and 130 means interruption.

Stable codes are `invalid_request`, `unsupported_protocol`,
`unknown_operation`, `project_not_found`, `project_conflict`, `alias_conflict`,
`not_found`, `recipient_mismatch`, `invitation_expired`,
`conversation_inactive`, `not_participant`, `idempotency_conflict`,
`invalid_cursor`, `ack_out_of_order`, `limit_exceeded`, `server_busy`,
`storage_busy`, `storage_unavailable`, `schema_mismatch`, `export_expired`,
`preview_stale`, `approval_required`, `provider_capacity`, `worker_failed`,
`worker_uncertain`, `cancelled`, and `internal_error`. Mark retryability only
when the operation can safely be attempted again with the same identity.
Never include private paths, credentials, or transcripts in generic errors.

The exact operation names and required parameter concepts are:

| CLI | Service operation and parameters |
| --- | --- |
| `projects register/list/rename/relocate` | `projects.register` (path, optional name), `projects.list` (page), `projects.rename` (project ID, name), `projects.relocate` (project ID, path) |
| `call` | `call` (initiator, recipient selector, topic, initial body, request key) |
| `join <code>` | `join` (project, code) |
| `send <id>` | `send` (project, conversation, body, kind, recipients, reply/correlation/key) |
| `receive --consumer <id> --wait <seconds> <conversation>` | `receive` (project, conversation, consumer, optional cursor, limit, wait seconds) |
| `ack --message-id <id> --consumer <id> <conversation>` | `ack` (project, conversation, consumer, message ID) |
| `conversations`, `show`, `search` | `conversations`, `show`, `search` (explicit project or human all-project scope; ID/pair/ordinal/date/text filters and page where applicable) |
| `close <id>` | `close` (project, conversation) |
| `dump --format text/json <id>` | `dump.open` (scope, conversation), `dump.page` (lease, cursor), `dump.close` (lease); complete client spool |
| `prune --dry-run`, `prune --confirm <token>` | `prune.preview` (scope, eligibility filters), `prune.confirm` (token) |
| `stats` | `stats` (project or explicit all-project scope) |
| `bridge start/status/stop` | `bridge.claim`, `bridge.status`, `bridge.stop`; owner-only `bridge.renew`, `bridge.checkpoint`, `bridge.complete` for the running adapter |

All service field names use snake_case, including `project_id`,
`conversation_id`, `consumer_id`, `message_id`, `request_key`, `wait_seconds`,
`owner_token`, and `lease_id`. CLI flags use kebab-case. `--message`, `--file`,
and stdin are mutually exclusive inputs for call/send; no input or empty
content fails. Options follow the subcommand and precede positional operands,
as required by Go's flag parser. `--project` and `--json` are command options;
project listing and help work without an inferred project.

`bridge start` runs a foreground adapter process for one project/conversation,
with explicit sandbox, approval mode (`deny` by default or interactive with a
terminal), and limits. The daemon handles only its typed persistence and
ownership requests. Interactive approvals are displayed locally and answered
by the operator; no peer response can approve them. `bridge stop` requests
cancellation through the binding; owner lease renewal reports that request,
and the adapter interrupts its owned turn and releases its lease.

### 17.4 Explicit resource bounds

All size units below count encoded bytes. Validate positive policy values on
startup; cap externally supplied limits to the configured maximum and reject
invalid input rather than silently truncating content.

| Policy | Default | Allowed range or invariant |
| --- | --- | --- |
| Message body | 256 KiB | 1 byte through 512 KiB; valid nonempty UTF-8 |
| Encoded IPC request / response | 1 MiB / 8 MiB | Hard version-1 wire ceilings; body must fit the request |
| Topic / query text / identifiers | 256 / 512 / 128 bytes | Topic, search, consumer, correlation, and key limits enforced before storage |
| Result page | 50 items | 1 through 100; explicit next cursor and `truncated` |
| Receive wait | 30 seconds | 0 through 60 seconds; empty result on timeout |
| Ordinary request deadline | 10 seconds | Wait uses requested wait plus five seconds, not the ordinary timeout |
| Connections / waiters | 128 / 64 | At most 16 waiters per project; reject excess with `server_busy` |
| Active conversations | 64 per project | Count pending and active; retries do not consume another slot |
| Messages / stored body bytes | 10,000 / 64 MiB per conversation | Reserve one message slot for a terminal daemon event; reject ordinary sends before exhaustion |
| Consumers | 64 per conversation/project | Reject additional identities before allocation |
| Export leases / lifetime | 16 / 120 seconds | Fixed snapshot and complete output, bounded temporary spool at 128 MiB |
| Prune preview | 1,000 candidates / 120 seconds | Require a narrower filter when the complete selection exceeds the cap |
| Concurrent bridges | 4 | At most one owner per conversation/project |
| Worker time / turns / total time | 120 seconds / 20 / 600 seconds | Time limits 1 through 3,600 seconds; turns 1 through 100 |
| Worker usage | 100,000 reported tokens | Stop before another turn once observed cumulative usage reaches the configured budget; a running turn can overshoot |
| Safe startup retries | 3 | Exponential delays of 1, 2, and 4 seconds; no retry after an uncertain invocation |
| Bridge ownership lease | 15 seconds | Renew every five seconds; loss of ownership prevents further turns or publication |

A conversation close/expire event uses its reserved final message slot and
empty body when the body budget is exhausted. Usage measures only what Codex
reports; unavailable cost or usage fields are `null` with an explicit
limitation, never zero estimates. Token limits do not claim a monetary cap.
A timeout or exhausted budget stops the owned bridge and surfaces the reason.

### 17.5 Bridge completion and recovery

A bridge requires explicit activation with conversation, selected project,
worker permissions, limits, and approval handling. Recheck joined
participation and active lifecycle before each turn and before publishing a
reply. Only Iris-owned thread IDs recorded in that binding may be resumed.
Conversation closure cancels an owned active turn and stops further sends.

Persist an input checkpoint and `invoking` state before `turn/start`. Capture
the turn ID when available. Only a completed turn's final agent message is a
response; intermediate reasoning, tool output, and progress are not replies.
Use `bridge:<binding-id>:<input-message-id>` as the stable response key.
Record the final response and advance processing acknowledgment together in
one SQLite transaction after rechecking ownership. A terminal system event
is acknowledged without a model call. A `status` message is informational
and is also acknowledged without a model turn, preventing status echo loops.

Migration 2 adds durable turn count, activation time and connected approval
request identity while preserving migration 1.

The durable binding stores its thread/turn, input/result checkpoint, limits,
usage, status, and error. The running process and its live handles are
replaceable, never project identity. Claim ownership with an unguessable
generation token and a renewable lease; compare the token on every update.
After loss of ownership, no process may publish or acknowledge. Lease expiry
alone never proves that a model execution did not happen.

After a crash, inspect only the owned thread's recorded turn. A durably
completed result can be published with the same response key and acknowledged
without another invocation. A recorded reply/ack transaction needs no replay.
If invocation may have begun but completion cannot be established, expose
`worker_uncertain` and require an explicit operator recovery decision. Do not
rerun potentially non-idempotent tools automatically. Expose failed,
interrupted, capacity, approval-required, and uncertain outcomes distinctly.
No provider-capacity retry is safe after invocation becomes uncertain.

A disconnected human cannot silently approve an outstanding request. Preserve
the request identity while connected, expose `approval_required`, and deny or
interrupt when the configured deadline expires. Do not widen permissions or
use `thread/shellCommand` as an escape from the worker sandbox. Idle bridges
block on bounded service waits and generate no `turn/start` requests.

### 17.6 Lifecycle, history, and destructive operations

The injected service clock is authoritative for all UTC millisecond storage
times. `call`, successful `join`, and newly persisted participant messages
or status updates are significant activity. Reads, waits, acknowledgments,
exports, bridge heartbeats, duplicate retries, and maintenance do not extend
inactivity. Evaluate due expiration in the same transaction as each stateful
operation and through a bounded service timer; no model is needed.

Pending expires at creation plus the configured two hours. Active expires at
last significant activity plus the configured 24 hours. `close` and expiration
set `terminal_at` once, append one system event, reject new ordinary messages,
and cancel active waits/bridges. Repeated closure does not extend retention.
Retained history becomes eligible at `terminal_at` plus the configured 30 days.
Export neither deletes history nor extends its retention period.

Project-facing `conversations`, `show`, `search`, and `dump` require
participation and default to the inferred project. Human inspection outside a
project requires explicit `--all-projects`; it does not change bridge context.
Lists/search are keyset-paged by `(created_at, id)` and expose truncation.
Message pages order by sequence. Text search is bounded parameterized SQL,
with literal substring semantics rather than user SQL or regular expressions.

`dump` freezes lifecycle metadata, participant labels, and the highest message
sequence in a short-lived daemon-owned export lease. Pages are bounded by the
wire ceiling. Prune cannot remove a leased conversation. The CLI spools pages
to a private bounded temporary file, publishes the complete chronological text
or one complete JSON object only after success, then deletes the spool. An
expired lease or daemon restart fails explicitly; it never reports a partial
transcript as successful. Exported JSON contains `schema_version`,
`conversation`, `participants`, and ordered `messages`.

`prune --dry-run` returns the complete bounded eligible selection, its sizes,
and a random confirmation token. Bind the token digest to sorted candidate
IDs and revisions, policy, eligibility cutoff, and a 120-second lifetime.
`prune --confirm <token>` revalidates the exact selection and export leases in
one write transaction. A changed, missing, newly ineligible, or leased
candidate produces `preview_stale` and deletes nothing. Consume the token on
successful deletion. Preserve project identities, pair counters, and code
tombstones. Data eligible after the preview is not silently added to its
selection. Bare `prune` is a preview and never deletes data.
