# Iris

Iris provides the local foundation for project-to-project conversations. A
per-user daemon owns a private Unix socket and a migrated SQLite database.
The current CLI runs the service and inspects readiness and storage counts.

The [accepted specification](ops/default/spec/iris.md) and
[data model](ops/default/spec/iris-model.md) define project invitations,
ordered messaging, on-demand Codex workers, and retained conversation history.
Those workflows build on this service foundation.

## Run the service

Requirements: Linux, Go 1.26.3, and an existing private `XDG_RUNTIME_DIR`.
The pure-Go SQLite driver does not require a C compiler.

Inspect available commands:

```sh
go run . --help
```

Run the foreground service in one terminal:

```sh
go run . serve
```

Inspect it from another terminal:

```sh
go run . stats --all-projects --json
```

The service listens on `$XDG_RUNTIME_DIR/iris/iris.sock`, with mode 0600
inside a user-owned mode-0700 directory. A runtime lock prevents another daemon
from replacing the socket. SIGINT or SIGTERM closes owned connections, waits
for workers, and removes the socket while preserving committed data.

The database is `$XDG_DATA_HOME/iris/iris.db`, defaulting to
`~/.local/share/iris/iris.db`. Only the daemon opens it during normal operation.
Schema migration finishes before the listener starts. An incompatible schema
or checksum fails startup. Do not copy a live database file alone as a backup.

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
Iris creates no project-local runtime state.

Human output is the default. `stats --all-projects --json` emits one
versioned success envelope to stdout. Failures leave stdout empty and write
a readable error or JSON error envelope to stderr. Usage exits 2, operating
failures exit 1, and interrupted client calls exit 130. Root and command help
work without configuration or a running service.

## Development

The Makefile selects Go 1.26.3 by default. Go can obtain that toolchain through
its standard toolchain mechanism; an explicit `GOTOOLCHAIN` override remains
available. Make, Git, Python 3, and golangci-lint v2 are required for the checks.

```sh
make check
make lint
make docs-check
make test-integration
python3 scripts/check-contracts.py
```

Ordinary tests use temporary SQLite databases and real Unix sockets. The
integration target builds the actual binary and runs isolated daemon processes
with private XDG roots and two temporary project directories. It checks
readiness, committed-state preservation, graceful/crashed restart, exclusive
ownership, incompatible-schema rejection, and cleanup of owned resources.
It does not install a service or invoke a model.

See the [documentation index](docs/index.md) and
[application package](internal/iris/README.md).
