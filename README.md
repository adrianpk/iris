# Iris

Iris connects local project directories through short invitations. A private
per-user service stores ordered conversations in SQLite. Independent consumers
acknowledge completed work; explicit Codex workers can exchange a question,
project-local answer and acknowledgment without a human relaying messages.
Closed and expired conversations remain available for inspection and export.

Follow the [User Guide](docs/tutorials/user-guide/index.md) from installation
through a two-project exchange. See the [documentation index](docs/index.md)
for reference, maintenance and recovery procedures.

## Install and start

Use Linux, Go 1.26.3, Make, Python 3 and a working systemd user manager. The
pure-Go SQLite driver does not require a C compiler. From this source checkout:

```sh
make install
export PATH="$HOME/.local/bin:$PATH"
systemctl --user daemon-reload
systemctl --user enable --now iris.socket
iris stats --all-projects --json
```

Installation writes files only; activation is explicit. The first request starts
the daemon. Foreground `iris serve` is also supported against its own endpoint.
Do not start it alongside an active systemd socket at the same runtime root.
See [Linux operation](docs/how-to/linux-operation.md) for overrides and upgrades.

## Connect projects

In project alpha, run `iris projects register`, then:

```sh
iris call --to beta --topic "Accepted format" --message "Which format is accepted?"
```

In project beta, run `iris join CODE` with the printed invitation. It returns the
initial request immediately. `iris send`, `receive` and `ack` provide durable
manual processing. Start an optional foreground worker with
`iris bridge start CONVERSATION_ID` in each joined project. Workers require an
installed, authenticated Codex CLI; the verified adapter uses version 0.161.0.

Workers create separate owned project/conversation threads. They do not attach
to existing interactive sessions. Tool permission and approval policy remain
explicit. Idle waits do not invoke a model. See the [bridge reference](docs/reference/bridge.md).

## Keep and inspect history

`iris conversations`, `search` and `show` inspect scoped retained history.
`iris dump --format text|json ID` exports a complete transcript deliberately;
old history never enters a new worker context automatically. `iris close ID`
retains history. Bare `iris prune` previews only; deletion requires exact
confirmation after retention eligibility. Take a
[consistent backup](docs/how-to/backup-recovery.md) before maintenance.

The service uses a user-owned 0600 Unix socket inside a 0700 runtime directory.
It is a same-user trust boundary. Standard XDG data/config/runtime roots apply;
clients and daemon must match them. Only the daemon normally writes the database.

[Planned capabilities](docs/explanation/planned-capabilities.md) records future
forum/group channels, broadcasts, claims and other adapters.

## Development

The Makefile selects Go 1.26.3. Checks also require Git, Python 3, jq and
golangci-lint v2 built for the selected Go version.

```sh
make check
make lint
make docs-check
make test-integration
make test-systemd
```

Ordinary and process checks use owned temporary roots and do not invoke a model.
Systemd acceptance requires the real user manager and uniquely named runtime
units. `make test-bridge` requires the real authenticated provider and fails
when it is unavailable. See the [application package](internal/iris/README.md)
and [accepted specification](ops/default/spec/iris.md). Follow
[source verification](docs/how-to/verify-source.md) for the complete immutable-candidate
gate and its evidence boundary.
