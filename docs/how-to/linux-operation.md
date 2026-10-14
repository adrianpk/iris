# Install and operate Iris on Linux

Requirements: Go 1.26.3, Python 3, and a working systemd user manager. Build and
install as your normal user from the repository root:

```sh
make build
make install
```

`make build` creates `_workspace/bin/iris` with source paths, VCS metadata and
the build ID removed. Repeating the build with the same sources, dependencies,
Go toolchain and target produces the same binary. `make install` rebuilds, then
atomically installs the executable to `$HOME/.local/bin/iris` and private unit
files to `$XDG_CONFIG_HOME/systemd/user` (default `$HOME/.config/systemd/user`).
Put `$HOME/.local/bin` on your shell's `PATH`.

`make install PREFIX=/absolute/prefix` selects a different executable prefix.
For staging, use `python3 scripts/install.py install --prefix /absolute/prefix
--unit-dir /absolute/unit-directory --binary /absolute/built/iris` on one line.
Installation captures the current absolute XDG data, config and runtime roots
in the service and socket. Clients must use those same roots. Reinstall after
changing them. Installation does not start, enable or reload any unit.

## Activate the private socket

```sh
systemctl --user daemon-reload
systemctl --user enable --now iris.socket
iris stats --all-projects --json
```

Only the socket starts at enable time. The first request starts `iris.service`.
The manager owns the 0600 socket inside its 0700 directory; the daemon adopts
it, validates its path and permissions, and keeps its exclusive runtime lock.
Do not run a second manual `iris serve` against that endpoint.

The daemon is restricted to Unix sockets, has no capabilities and cannot gain
new privileges. It does not launch model processes. Start each optional worker
separately, from its participant project, with a deliberate conversation ID:

```sh
iris bridge start CONVERSATION_ID
```

The worker uses your existing Codex installation and authority. Its tools and
approvals follow the [bridge contract](../reference/bridge.md); daemon unit
restrictions neither grant tools nor change the worker's authority. No worker
or model starts merely because the daemon receives a request.

## Upgrade and restart

```sh
make install
systemctl --user daemon-reload
systemctl --user restart iris.service
iris stats --all-projects --json
```

The binary is replaced atomically. Restart deliberately to run the new version;
an already running process continues using the old executable until restarted.
The manager preserves its listening socket across service restarts. SQLite
migrations and interrupted-operation reconciliation finish before requests are
served. Stop active bridge workers before planned maintenance and resume them
using the recorded [recovery policy](../reference/bridge.md).

## Shut down and remove files

Stop each active foreground bridge with Ctrl-C, or request `iris bridge stop`
with the conversation and participant selector documented by `iris bridge --help`.
Stop the socket before the service so queued clients cannot reactivate it:

```sh
systemctl --user disable --now iris.socket
systemctl --user stop iris.service
make uninstall
systemctl --user daemon-reload
```

`make uninstall` removes only the installed binary and the two unit files. Use
the same `PREFIX` and XDG config root as installation. It preserves Iris data,
configuration, backups and project files. Deliberate data deletion is a separate
manual operation after a verified backup and stopped service. Stop units before
uninstalling; the installer does not control running processes.
