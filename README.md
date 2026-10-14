# Iris

Iris connects local project directories through short invitations. A per-user
service owns a private Unix socket and SQLite database. A caller stores the
topic and initial request; the invited project joins using only the code and
reads that request immediately. Conversations have stable IDs, independent
pair numbering, and retained history after closure.

Messages persist in sequence order and consumers explicitly acknowledge
processing. See the [delivery reference](docs/reference/delivery.md) for the
CLI, protocol fields, cursor rules and limits.

The [accepted specification](ops/default/spec/iris.md) and
[data model](ops/default/spec/iris-model.md) also define automatic lifecycle maintenance and history export. Those
capabilities remain in the implementation plan.

Run explicit foreground workers with `iris bridge start CONVERSATION_ID` in
each joined project. The [Codex bridge reference](docs/reference/bridge.md)
explains separate owned threads, sandbox/approval settings, completion, limits,
and `bridge status` / `bridge stop`.

## Run the service

Requirements: Linux, Go 1.26.3, and an existing private `XDG_RUNTIME_DIR`.
The pure-Go SQLite driver does not require a C compiler.

Build the executable and make it available on `PATH` in each terminal:

```sh
go build -o _workspace/iris .
export PATH="$(pwd)/_workspace:$PATH"
iris --help
```

Run the foreground service in one terminal:

```sh
iris serve
```

Inspect readiness from another terminal:

```sh
iris stats --all-projects --json
```

The service listens on `$XDG_RUNTIME_DIR/iris/iris.sock`, with mode 0600
inside a user-owned mode-0700 directory. A runtime lock prevents another daemon
from replacing the socket. SIGINT or SIGTERM closes owned connections, waits
for workers, and removes the socket while preserving committed data.

The database is `$XDG_DATA_HOME/iris/iris.db`, defaulting to
`~/.local/share/iris/iris.db`. Only the daemon opens it during normal operation.
Schema migration finishes before the listener starts. An incompatible schema
or checksum fails startup. Do not copy a live database file alone as a backup.

## Connect two projects

From the intended root of project `alpha`, register and invite `beta`:

```sh
iris projects register
iris call --to beta --topic "Storage protocol" --message "Which format is accepted?"
```

The caller prints the three-word invitation, conversation ID, readable label,
and initial request. From the root of project `beta`, run `iris join` with the
printed code. For example, replace this illustrative code with the actual one:

```sh
iris join amber-otter-river
```

Joining registers `beta` if needed, activates that conversation, and prints
the stored topic and initial message. Repeating the same join returns the same
request without another join event or processing acknowledgment. Another
project cannot accept the invitation. Only the code needs to move between
project contexts.

Use `--file request.txt` instead of `--message`, or pipe the initial content:

```sh
printf 'Which format is accepted?\n' | iris call --to beta --topic "Storage protocol" --json
```

Topic text is required and limited to 256 UTF-8 bytes; initial content must be
nonempty UTF-8, at most 256 KiB, and fit the encoded 1 MiB request. Iris preserves
newlines. `--message` and `--file` cannot be combined. `--idempotency-key` lets
a caller safely retry creation: the same key and content return the original
conversation, while changed content conflicts. Without an explicit key, each
call starts an independent conversation.

Either joined project can close using the returned ID:

```sh
iris close CONVERSATION_ID
```

Closure retains the initial request and prevents another join from reactivating
the conversation. Invitations reject acceptance at their two-hour deadline;
active conversations reject operations at 24 hours without significant
activity. Duplicate retries do not renew activity. Lifecycle checks currently
run during call retries, join, send, receive, ack, and close; configurable policy and background
maintenance remain in the implementation plan.

The invitation code identifies a conversation within the same-user local
service. It is not a strong credential or isolation between processes sharing
a user account. The service also validates the selected recipient project.

## Exchange and process messages

After joining, either participant can send an ordered message or status. Use
the conversation ID returned by call/join:

```sh
iris send --message "The accepted format is JSON." --reply-to INITIAL_MESSAGE_ID CONVERSATION_ID
iris send --kind status --message "Checking the updated format." CONVERSATION_ID
iris receive --consumer manual --wait 30 --json CONVERSATION_ID
```

Send also accepts `--file` or stdin and preserves the same content limits as
call. The default recipient is the other participant. `--to` accepts a
comma-separated list of participant project IDs. Reply targets must be in the
same conversation; `--correlation-id` attaches a caller identifier. A send
success means its message and recipients have committed to storage.

Use a stable `--idempotency-key` when retrying an uncertain send. The same key
and payload return the original ID, sequence and timestamp, including after
closure. Changed content with the same key fails. Without a key, repeated
sends are distinct messages.

Receive returns addressed messages in sequence order, starting after that
consumer's durable acknowledged position. Reading may record delivery but
never acknowledges processing. Use a different `--consumer` for independent
progress. When processing succeeds, acknowledge each addressed message in
order using the ID returned by receive:

```sh
iris ack --consumer manual --message-id DELIVERED_MESSAGE_ID CONVERSATION_ID
```

Acknowledgment cannot skip an earlier addressed message, acknowledge a message
not delivered to that consumer, or advance another consumer. A crash before
acknowledgment leaves the work available for redelivery. This provides delivery
at least once; a consumer must handle repeated work safely.

`receive --wait 0` returns immediately. Empty waits block until a relevant
commit, closure, cancellation or the requested timeout. Waits range from 0 to
60 seconds and have an additional five-second wire deadline. They hold no
storage transaction and do not renew inactivity. Closed or expired history is
excluded from receive.

Pages accept `--limit` from 1 through 100 and expose `next_cursor` and
`truncated`. Large content can fill a page before its item limit. Pass the
returned cursor to continue without implying acknowledgment. A cursor is bound
to its project, conversation and consumer and cannot skip undelivered work.
Each project/conversation supports at most 64 consumers. Conversations allow
10,000 messages and 64 MiB of stored body content, with the final message slot
reserved for a terminal service event.

## Project identities

Project IDs are independent of paths and aliases. Registration derives the
alias from the canonical directory basename or reads a user-authored
`.iris.toml` containing `name = "chosen-alias"`. Iris creates no project-local
files. Alias collisions fail with an explicit error.

Within a registered project, commands discover its nearest registered ancestor
from cwd, including through symlinks. `--project` selects a registered ID,
canonical directory, or unambiguous alias instead. A first call or join can
register the exact cwd; register the intended root before working in a nested
unregistered directory.

Inspect, rename, or explicitly reassociate an existing project after a move:

```sh
iris projects list --json
iris projects rename --name new-alias PROJECT_ID
iris projects relocate --path /path/to/moved-project PROJECT_ID
```

Renaming and moving preserve IDs and historical conversation labels. They do
not merge identities. Registry lists use bounded pages with `--limit` and
`--cursor`; JSON results expose `next_cursor` and `truncated`.

## Configuration and output

The optional configuration file is `$XDG_CONFIG_HOME/iris/config.json`,
defaulting to `~/.config/iris/config.json`:

```json
{
  "schema_version": 1,
  "max_connections": 128,
  "request_timeout_seconds": 10
}
```

Connection limits range from 1 through 128; ordinary request timeouts range
from 1 through 60 seconds. Omitted settings retain their defaults. Unknown
fields, unsupported versions, malformed JSON, and relative XDG paths fail.

Human output is the default. `--json` emits one versioned success envelope to
stdout. Failures leave stdout empty and write a readable error or JSON error
envelope to stderr. Usage exits 2, operating failures exit 1, and interrupted
client calls exit 130. Options precede positional operands. Root and command
help work without configuration or a running service.

`iris stats` reports the selected project's counts. Explicit human
`iris stats --all-projects` reports service-wide counts without cwd inference.

## Development

The Makefile selects Go 1.26.3 by default. Go can obtain that toolchain through
its standard toolchain mechanism; an explicit `GOTOOLCHAIN` override remains
available. Make, Git, Python 3, jq, and golangci-lint v2 are required for checks.

```sh
make check
make lint
make docs-check
make test-integration
python3 scripts/check-contracts.py
```

Ordinary tests use temporary SQLite databases, real Unix sockets, and a
controlled clock for invitation and inactivity boundaries. Integration tests
build the actual binary, run private daemon fixtures, and check the two-project
handshake, JSON through `jq`, human output, identity moves, conversation
isolation, concurrent sends, independent consumers, lost-response retry, waits and
graceful/crashed restart. Only owned processes are signaled.
Those tests do not install a service or invoke a model. The separate
`make test-bridge` target requires the installed, authenticated Codex CLI and
runs a real two-project question/response/acknowledgment exchange. It fails
instead of skipping an unavailable provider.

See the [documentation index](docs/index.md) and
[application package](internal/iris/README.md).
