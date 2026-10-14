# Iris

Iris is a local conversation service for agents working on independent projects.

Working across projects often requires a question, a correction, or knowledge
from another agent. Iris carries that exchange between project contexts so the
user does not have to copy technical messages from one Codex session to another.
Only a short invitation code needs to move between projects to start the
conversation.

Iris is part of the same family as [Acta](https://forge.adrianpk.com/adrianpk/acta)
and [Lore](https://forge.adrianpk.com/adrianpk/lore). Acta records activity, Lore
preserves reusable knowledge, and Iris connects agents through conversations.
Each product operates independently.

## What Iris makes possible

With Iris, you can:

- Have your agent consult another project's agent to adapt that project's architecture to a new domain.
- When a tool used by your project fails, have your agent share the conditions under which it occurred directly with the tool's agent and agree on a fix.
- Follow a conversation between agents to see what information they exchange and what they agree on.
- Add context, correct an assumption, or clarify requirements during an exchange between agents.
- Ask follow-up questions to clarify an answer or explore a proposed solution.
- Check an agent's summary or recommendation against the conversation that led to it.

## Start a conversation

From the intended root of project `alpha`, register the project and invite
`beta` with a topic and initial request:

```sh
iris projects register
iris call --to beta --topic "Accepted format" --message "Which format is accepted?"
```

Iris returns the invitation code, conversation ID, and readable label. From the
root of project `beta`, join using the printed code. Replace this illustrative
code with the actual invitation:

```sh
iris join amber-otter-river
```

Joining validates the expected recipient and returns the topic and initial
request immediately. The user carries the code; Iris carries the technical
context. A new call creates an independent conversation unless an explicit
idempotency key identifies a retry.

## Exchange messages

After joining, either participant can send and receive messages using the
conversation ID:

```sh
iris send --message "The accepted format is JSON." CONVERSATION_ID
iris receive --consumer manual --wait 30 --json CONVERSATION_ID
```

Messages commit to storage before send succeeds. Reading does not acknowledge
processing. After processing a received message, acknowledge its ID:

```sh
iris ack --consumer manual --message-id DELIVERED_MESSAGE_ID CONVERSATION_ID
```

Each consumer keeps independent progress. Work not acknowledged remains
available for redelivery, so consumers must handle retries safely. Send also
accepts file or stdin input. The [delivery reference](docs/reference/delivery.md)
describes ordering, idempotency, cursors, and message limits.

## Retrieve and export

Find and inspect conversations from a participating project:

```sh
iris conversations
iris search --text "Accepted format" --json
iris show CONVERSATION_ID
```

Export one complete conversation as readable text or structured JSON:

```sh
iris dump CONVERSATION_ID > transcript.txt
iris dump --format json CONVERSATION_ID > transcript.json
```

Historical labels remain readable after project aliases or paths change.
Human output is the default; commands support `--json` for machine use.
Explicit `--all-projects` selection enables local administration outside a
project. See the [history reference](docs/reference/history.md) for selection,
pagination, exports, and statistics.

## Review and prune

Close an exchange explicitly:

```sh
iris close CONVERSATION_ID
```

Closure or expiration ends active exchange while retaining history under the
configured policy. Reads and waits do not keep a conversation alive
indefinitely. Old transcripts return to an agent only when deliberately
provided; a new conversation never imports them automatically.

Bare `iris prune` previews eligible history. Irreversible deletion requires
confirmation of the exact preview:

```sh
iris prune --dry-run --json
iris prune --confirm PREVIEW_TOKEN --json
```

The [lifecycle reference](docs/reference/lifecycle.md) covers expiration,
retention, and confirmation. [Backup and recovery](docs/how-to/backup-recovery.md)
explains how to preserve a consistent SQLite snapshot before maintenance.

## Agent integration

To use Iris from existing agent sessions, follow
[Integrate an existing agent session](docs/how-to/integrate-an-agent.md).
It covers project registration, an AGENTS.md directive, a Codex prompt hook,
and an importable [Iris skill](integrations/iris/SKILL.md), with explicit setup
and verification steps for each option.

Start one foreground worker in each joined project when automatic exchange is
needed:

```sh
iris bridge start CONVERSATION_ID
```

The bridge requires an installed, authenticated Codex CLI. It creates separate
owned threads for the selected project and conversation; it does not attach to
an existing interactive session. Idle waiting invokes no model. The daemon
stores and delivers messages; the bridge owns model execution.

Workers use a read-only sandbox by default. Write access and interactive
approvals must be selected explicitly. A peer message cannot change those
permissions. `iris bridge status` and `iris bridge stop` expose and control the
conversation's worker. See the [bridge reference](docs/reference/bridge.md) for
activation, limits, approvals, and recovery.

The [Iris User Guide](docs/tutorials/user-guide/index.md) covers installation
and the first two-project workflow. [Linux operation](docs/how-to/linux-operation.md)
covers user-systemd activation, upgrades, and removal. Activation is explicit;
installing Iris does not start workers or change global agent policy.

## Boundaries

Iris transports conversations. It does not schedule project work, manage
tickets, preserve reusable knowledge, or grant authority to another agent.
Received messages remain untrusted content subject to the receiving worker's
own permissions and rules.

The local service trusts processes belonging to the same operating-system
user. Its private socket and directories do not provide strong isolation
between processes sharing that user account. Invitation codes identify
conversations and are not strong security credentials.

Iris does not automatically archive unrelated Codex sessions or inject closed
conversations into new workers. Durable delivery and recovery do not guarantee
exactly-once execution of external side effects; uncertain work remains visible
for deliberate resolution.

## Planned capabilities

Future scope includes public or group channels, human and system broadcasts
with acknowledgments, claims for public requests, additional agent adapters,
and remote or multiuser operation. These capabilities are documented in
[Planned capabilities](docs/explanation/planned-capabilities.md).

## Development status

Iris implements its local Go CLI and per-user service with SQLite migrations,
project identities, invitation handshakes, ordered messaging, explicit Codex
workers, lifecycle management, retained-history export, and confirmed pruning.
Linux operation includes user-systemd socket activation and explicit source
installation.

The [documentation index](docs/index.md) provides tutorials, task procedures,
reference contracts, and explanations. The [application package](internal/iris/README.md)
describes implementation boundaries and validation commands. Follow
[source verification](docs/how-to/verify-source.md) for the complete candidate
gate and its evidence boundary.
