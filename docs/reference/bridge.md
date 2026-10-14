# Codex bridge

An explicitly started bridge runs a separate foreground Codex app-server worker
for one registered project and active conversation. It starts Codex only when
an addressed ordinary message needs processing. It creates a fresh thread or
resumes the thread already stored in that binding. It does not attach to an
existing interactive session or import another conversation's history.

Start one worker from each participating project, using the conversation ID
printed by call or join:

```sh
iris bridge start --sandbox read-only --approvals deny CONVERSATION_ID
```

The installed `codex` executable and its existing authentication must work.
The adapter implements the installed 0.161.0 app-server JSON protocol. It runs
Codex in the registered canonical directory; `--project` can select that
project explicitly. The daemon remains the sole normal database writer.

From another terminal in the same project:

```sh
iris bridge status --json CONVERSATION_ID
iris bridge stop CONVERSATION_ID
```

Start remains in the foreground until stopped, terminal conversation state,
execution failure, or budget exhaustion. `--json` on start emits one final
success envelope on normal stop. Failures leave stdout empty and use stderr.
Status includes state, thread/turn/input/result identifiers, limits, usage,
turn count, pending approval identity, and lease/stop timestamps. Ownership
secrets are omitted. At most four workers run concurrently, with one owner
per project/conversation. A fifteen-second lease renews every five seconds.

## Permissions and approvals

`--sandbox read-only` is the default. `--sandbox workspace-write` permits
writes in the selected project, disables network access, and excludes temporary
directories from writable roots. Thread and turn requests carry the selected
sandbox and approval policy. Peer message content cannot change those settings.
Codex's sandbox and tools define the actual provider authority boundary.

`--approvals deny` never asks for local input and declines command/file
approval requests. `--approvals interactive` requires a connected terminal.
It displays each matching request on stderr and accepts only an exact local
`approve` response, for that request once. Other answers decline. Session-wide
or persistent approvals are never granted. Additional permission profiles and
unsupported requests are denied. A disconnect or deadline cannot approve work.

A declined request, failed tool/turn, invalid final output, interruption, or
uncertain outcome leaves the input unacknowledged. Pending approval is recorded
before an answer is sent and is visible through status.

## Completion and bounds

Workers process addressed inputs sequentially. Only a completed turn's final
structured agent result can produce a reply. Replies use a stable input-based
key and commit with processing acknowledgment in one transaction. Intermediate
reasoning, tools and progress are excluded. A completed answer may be
acknowledged without sending another message. System/status inputs never call
the model, preventing acknowledgment and status echo loops.

| Option | Default | Bound |
| --- | --- | --- |
| `--model` | Codex configured model | Explicit identifier, at most 128 bytes |
| `--turn-seconds` | 120 | 1 through 3600 |
| `--total-seconds` | 600 | 1 through 3600 |
| `--max-turns` | 20 | 1 through 100 |
| `--max-tokens` | 100000 | Positive reported-token budget |

Turn count, lifetime origin and available usage persist in the binding. Safe
idle restarts keep those budgets. Restart cannot replace the saved options;
`bridge recover` reuses them. Missing usage is null with a limitation;
it is never reported as zero. The last observed cumulative usage remains a
private lower bound for budget enforcement when later telemetry is missing.
A reported usage limit blocks the next turn before provider startup;
a provider can exceed it during the current turn. Deadline/closure cancellation
interrupts only the owned active turn, with a bounded confirmation and cleanup
interval. No reply or work acknowledgment can publish after closure or loss
of ownership. App-server pipe writes obey cancellation/deadlines even when
the child stops reading. Shutdown closes owned pipes, joins the reader and
terminates only the owned process group, escalating after two seconds.
The direct app-server also receives a parent-death signal. Unconfirmed
external side effects remain uncertain after a crash. Reachable terminal system events can be acknowledged without a
model; earlier unfinished ordinary work remains unacknowledged.

Expired idle ownership can be reclaimed. Invoking, approval-pending, failed,
interrupted, capacity, or uncertain execution is not automatically replayed.
Idle workers wait on bounded service subscriptions and generate no model turns.

## Recovery and failures

```sh
iris bridge status --json CONVERSATION_ID
iris bridge recover --json CONVERSATION_ID
```

Completed results are stored before publication. A restart can publish a saved
result without contacting the model. Reply and acknowledgment still commit
together with a stable response key; losing the success response cannot cause
a duplicate reply or another invocation.

`bridge recover` acquires only a prior binding and retains its options, thread,
turn and consumer. It inspects the exact recorded turn through bounded stored
history. Only confirmed completion, successful tool outcomes and a valid final
answer can be published. A missing turn ID, unfinished/mismatched history,
unknown completion or disconnected approval remains blocked. A recorded
authority decline survives restart and cannot become success through recovery.

Status distinguishes failed, interrupted, capacity, approval-required and
uncertain execution. Expired invoking ownership is reported as uncertain.
`recovery_hint` explains the next action. If completion remains unknown, inspect
external effects, close the old conversation and create a deliberate replacement
request. Iris does not automatically retry a possibly executed tool operation
or claim exactly-once external execution. Failed input stays unacknowledged.

Only setup before the invocation checkpoint can retry. Transient disconnects
and provider-capacity errors have at most three retries, delayed by 1, 2 and
4 seconds; cancellation ends the delay. No turn-start request is automatically
retried. A definite overload rejection reports capacity; a lost invocation
confirmation remains uncertain.

## Live acceptance

`make test-bridge` builds Iris, starts a private service and two temporary
projects, then explicitly starts two real Codex workers using existing provider
configuration. It requires a random fact read from the receiving project's
file, distinct threads, one technical reply and durable processing acknowledgment.
It also verifies zero initial idle turns and stable turn/message counts after
completion. It never skips missing authentication or capacity, relays a reply
manually, or substitutes a fake model. Cleanup stops only fixture-owned workers
and service. The separate unit suite uses protocol fixtures for failures and
approval decisions; those fixtures do not establish live acceptance.
