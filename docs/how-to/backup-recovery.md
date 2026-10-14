# Back up and recover Iris

The daemon uses SQLite WAL mode with full synchronization. A live `iris.db`
copy can omit committed messages still in `iris.db-wal`. Never copy that file
alone while the daemon or another database connection is running.

## Make a stopped, consistent backup

Stop active bridge workers, then stop the socket before the service. This blocks
reactivation while the daemon cancels requests, closes SQLite and releases its
runtime lock:

```sh
systemctl --user stop iris.socket iris.service
systemctl --user is-active iris.socket iris.service
```

Both units must report `inactive`; the second command returns nonzero for that
state. Ensure any manually started `iris serve` and external SQLite tools using
this database are also stopped. A normal shutdown checkpoints committed WAL
content. If shutdown failed or WAL/SHM files remain, restart and stop the daemon
cleanly before following this single-file procedure. Do not delete journals to
force a backup. The internal online-backup primitive is tested separately; this
procedure deliberately requires closed connections.

Choose a new absolute snapshot path in a private directory. These commands
refuse to overwrite an existing snapshot and verify SQLite integrity:

```sh
umask 077
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
backup_dir="$HOME/.local/share/iris-backups"
mkdir -p -m 700 "$backup_dir"
snapshot="$backup_dir/iris-$(date -u +%Y%m%dT%H%M%SZ).db"
python3 - "$data_home/iris/iris.db" "$snapshot" <<'PY'
import os
from pathlib import Path
import shutil
import sqlite3
import sys

source, snapshot = map(Path, sys.argv[1:])
assert not Path(str(source) + "-wal").exists(), "WAL remains; stop all connections cleanly"
assert not Path(str(source) + "-shm").exists(), "SHM remains; stop all connections cleanly"
with source.open("rb") as original, snapshot.open("xb") as output:
    os.chmod(snapshot, 0o600)
    shutil.copyfileobj(original, output)
connection = sqlite3.connect(snapshot.as_uri() + "?mode=ro", uri=True)
try:
    assert connection.execute("PRAGMA integrity_check").fetchone() == ("ok",)
finally:
    connection.close()
PY
```

Keep backups private. Back up configuration separately if you use it. A
transcript dump is useful readable history, but does not preserve identities,
idempotency keys, consumer progress or the complete database.

## Restore and reopen

Keep the socket, daemon, workers and external SQLite connections stopped.
Verify the selected snapshot's integrity with the same read-only check above.
Preserve the current database and any journals together in a separate private
recovery directory before replacing anything. Never mix an old database with
newer WAL/SHM files. After a clean stop with no journals, restore the snapshot:

```sh
install -m 600 "$snapshot" "$data_home/iris/iris.db"
systemctl --user start iris.socket
iris stats --all-projects --json
iris conversations --all-projects --json
```

The daemon verifies migrations and reconciles lifecycle state before serving.
Use a binary that understands the snapshot's schema; a newer-than-supported
schema or changed migration checksum fails startup. Inspect expected project
IDs, conversation IDs and messages after reopening. Restoration rolls consumer
and worker progress back to the snapshot time; inspect uncertain external work
and use the [bridge recovery policy](../reference/bridge.md) before restarting
workers. A restored database never authorizes replay of external tools.

## Diagnose startup and shutdown

```sh
systemctl --user status iris.socket iris.service
journalctl --user -u iris.service -b --no-pager -n 100
systemctl --user show iris.service -p MainPID -p Result -p ExecMainStatus
```

Permission failures require matching XDG roots, a user-owned 0700 runtime
directory and 0600 socket/database files. Do not broaden modes to bypass a
failure. A runtime ownership error means another daemon is using the same
root. A `schema_mismatch` requires the matching executable or a verified backup;
do not edit migration checksums. `storage_unavailable` includes the underlying
SQLite or filesystem cause in daemon stderr. Check free space, ownership and
journal errors there.

For a manual daemon, SIGINT or SIGTERM cancels waits, closes connections and
SQLite, then exits. Under systemd, stopping the socket and service performs the
same orderly shutdown. Stopping only the service leaves the manager socket
available for another request. systemd restarts unexpected service failures;
repeated failures reach its start limit and require diagnosis before
`systemctl --user reset-failed iris.service` and an explicit restart.
