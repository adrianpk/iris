# Install and start

Goal: get a private, ready Iris service for your operating-system user.

Use Linux with a working systemd user manager, Go 1.26.3, Make, Python 3 and Git.
The examples also use jq to select fields from JSON. To run workers later,
install and authenticate Codex separately; the verified adapter uses Codex CLI
0.161.0. The pure-Go SQLite driver does not require a C compiler.

From an Iris source checkout:

```sh
make install
export PATH="$HOME/.local/bin:$PATH"
systemctl --user daemon-reload
systemctl --user enable --now iris.socket
iris stats --all-projects --json
```

Expect one JSON success envelope with `result.ready` true. Enabling starts the
socket; the first request starts the daemon. Installation itself does not start
services or workers. See [Linux operation](../../how-to/linux-operation.md) for
custom prefixes/XDG roots, upgrades, foreground operation and removal.

The socket is private to your user. Data normally lives in
`$HOME/.local/share/iris/iris.db`; runtime and configuration use their XDG roots.
The service is a same-user trust boundary, not isolation from other processes
sharing your UID. Keep the service and CLI on the same XDG roots.

If readiness fails, inspect `systemctl --user status iris.socket iris.service`
and `journalctl --user -u iris.service -b -n 100 --no-pager`. Follow
[backup and recovery](../../how-to/backup-recovery.md) for startup causes;
do not broaden file permissions or edit migration checksums.

Next: [Invite another project](02-connect-projects.md).
