# Iris Functional and Technical Specification for Codex

**Status:** Draft; initial specification for implementation.
**Product:** Iris.
**Family:** Acta, Lore, and Iris.
**Initial platform:** Linux, a local per-user service, CLI, and user systemd.
**Audience:** The Codex agent responsible for designing, implementing, testing,
and documenting Iris.
**Source:** User-supplied `IRIS_SPEC.md`, translated into English.
**Model companion:** [Iris data model](iris-draft-model.md).

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

## 9. Proposed CLI

Adapt names and flags to verified Acta and Lore conventions while preserving
these capabilities. Commands below define **required functions**; final
spelling may change.

| Conceptual command | Function |
| --- | --- |
| `iris call --to <alias> --topic <text> [--message ...]` | Create an invitation and conversation. |
| `iris join <code>` | Join as the current project and read the initial request. |
| `iris send <id> [--message/--file/stdin]` | Send a message. |
| `iris receive <id> [--wait]` | Read available messages or wait using a cursor. |
| `iris ack <id> --message-id <id>` | Confirm consumer processing of a message. |
| `iris conversations [--active/--all]` | List conversations visible to the project, or to the user with an explicit option. |
| `iris show <id>` | Inspect state, participants, and messages. |
| `iris search [--text/--project/--since/--until]` | Find conversations and messages with structured filters. |
| `iris close <id>` | Close a conversation. |
| `iris dump <id> [--format text/json]` | Export a complete transcript. |
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
