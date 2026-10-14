# Command reference

Run `iris --help` or `iris COMMAND --help` for supported syntax. Flags precede
positional operands. Root and command help work without configuration, runtime
roots or a service. Human output is default. Commands that accept `--json`
write one versioned success envelope to stdout; failures leave stdout empty and
write an error to stderr. Usage exits 2, operating failures 1 and interrupted
client calls 130.

`dump --format json` emits the semantic transcript object directly;
`dump --json` emits its success envelope. Text dumps are complete readable
transcripts. See [history](history.md) for both forms.

| Command | Behavior and principal options |
| --- | --- |
| `serve` | Foreground private daemon; no positional arguments. |
| `projects register [directory]` | Optional `--name`; derive alias or read the optional project name file. |
| `projects list` | Registry page with `--limit` and `--cursor`. |
| `projects rename ID` | Required `--name`; keep stable identity and historical labels. |
| `projects relocate ID` | Required `--path`; deliberately bind a moved canonical directory. |
| `call` | Required `--to`, `--topic`; `--message`, `--file` or stdin body; optional `--project`, `--idempotency-key`. |
| `join CODE` | Accept the intended recipient's invitation; optional `--project`. |
| `close ID` | Idempotent terminal transition; optional `--project`. |
| `send ID` | Body input as for call; optional `--project`, `--to`, `--kind`, `--reply-to`, `--correlation-id`, `--idempotency-key`. |
| `receive ID` | `--project`, `--consumer`, `--wait`, `--limit`, `--cursor`; reading never acknowledges processing. |
| `ack ID` | Required `--message-id`, optional `--project` and `--consumer`; ordered processing acknowledgment. |
| `conversations` | Scoped history with common filters below; all lifecycle states by default; `--active` selects pending/active. |
| `search` | Scoped retained history with literal `--text` and common filters. |
| `show [ID]` | Metadata and bounded ordered messages; requires ID or `--pair` plus `--ordinal`. |
| `dump [ID]` | Complete frozen `--format text|json`; ID or `--pair` plus `--ordinal`; project/all-project scope. |
| `stats` | Project counts or explicit `--all-projects`; logical SQLite page allocation is service-wide. |
| `prune` | Bare or `--dry-run` previews; scope, `--conversation`, `--state` and `--before` select eligible history. |
| `prune --confirm TOKEN` | Confirm the exact preview; cannot add new selection flags. |
| `bridge start ID` | Foreground worker; `--project`, `--sandbox`, `--approvals`, `--model`, `--turn-seconds`, `--total-seconds`, `--max-turns`, `--max-tokens`. |
| `bridge status ID` | Inspect binding, limits, progress, approval and recovery hint for the selected project. |
| `bridge stop ID` | Request only that selected binding's stop. |
| `bridge recover ID` | Deliberately inspect only recorded owned work; persisted options remain fixed. |

Project-facing commands accept `--project` where listed. Registry commands
operate on explicit directory/ID operands or the global registry. `conversations`,
`search` and `show` support `--project` or explicit `--all-projects`, `--state`,
`--active`, `--all`, `--conversation`, `--pair first,second`, `--ordinal`,
`--text`, `--since`, `--until`, `--limit` and `--cursor`, subject to each command's
selection rules. Times use RFC3339. Cursor filters/scope must match the page
that issued them. `show` and `dump` cannot guess an ordinal without a pair.

The private operation groups are `projects.register`, `projects.list`,
`projects.rename`, `projects.relocate`, `call`, `join`, `close`, `send`, `receive`,
`ack`, `conversations`, `search`, `show`, `stats`, `dump.open`, `dump.page`,
`dump.close`, `prune.preview` and `prune.confirm`. Bridge owner RPCs are an
internal authority boundary: `bridge.claim`, `bridge.status`, `bridge.stop`,
`bridge.renew`, `bridge.checkpoint`, `bridge.receive`, `bridge.record`,
`bridge.complete` and `bridge.recover`. Use the public CLI for worker lifecycle;
owner tokens and pending result bodies are not public status fields.

See [projects](projects.md), [delivery](delivery.md), [history](history.md),
[lifecycle](lifecycle.md), [bridge](bridge.md), and [runtime](runtime.md) for typed
parameters, defaults, bounds, durability and error behavior.
