# Slice 9: Linux Operation

Status: reviewing
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-9-iris-linux-operation`
PR: pending

## Purpose

Operate the private service through user systemd, with reproducible source
installation, deliberate worker activation and verified backup/recovery.

## Delivered Behavior

The daemon accepts one inherited listening Unix stream at its configured private
socket path. It rejects malformed activation metadata, foreign paths, network
listeners, packet listeners and unsafe permissions. Closing the adopted listener preserves the
manager-owned socket. Manual service startup retains its existing behavior.

Source installation writes the executable and private units atomically, captures
matching XDG roots and leaves activation explicit. Reinstallation upgrades files;
uninstallation preserves data/configuration. Workers remain deliberate.

Stopped-backup guidance refuses live WAL/SHM files, creates a new private
snapshot and checks integrity. Restore reopens original identities and messages;
later writes roll back to the snapshot. Daemon startup stderr includes its
underlying configuration/runtime/storage cause for journal diagnostics.

## Implementation Notes

The user socket owns the listening endpoint. The daemon retains its exclusive
runtime lock and startup migration/reconciliation sequence. Its unit applies
0077 umask, no new privileges, no capabilities and Unix-only socket families.
Bridge workers remain separate foreground processes with deliberate authority.

## Contracts Added or Changed

Activation uses descriptor 3, this process's `LISTEN_PID`, exactly one
`LISTEN_FDS`, and optional `LISTEN_FDNAMES=iris`. Socket and parent permissions
remain 0600 and 0700. Shutdown closes only the inherited descriptor.

## Files of Interest

- `internal/iris/activation.go`: inherited listener validation and ownership.
- `assets/systemd/user/iris.socket`: private start-on-request endpoint.
- `assets/systemd/user/iris.service`: daemon restrictions and orderly shutdown.
- `scripts/install.py`: atomic installation/removal without process mutations.
- `docs/how-to/linux-operation.md`: installation and explicit activation.
- `docs/how-to/backup-recovery.md`: verified stopped-WAL snapshot/restore recipe.
- `internal/iris/systemd_test.go`: actual isolated user-manager acceptance.

## Validation

Passed focused checks:

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestActivation|^TestRuntime|^TestClientSocket' -count=1`
- `python3 scripts/test-install.py`
- `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run '^TestProcessBackupRestore$|^TestBackup|^TestServeDiagnostic$' -count=1`
- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestActivation|^TestServeDiagnostic$' -count=1`
- `GOTOOLCHAIN=go1.26.3 go test -v -race -tags=systemd ./internal/iris -run '^TestSystemdOperation$' -count=1 -timeout=2m`
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix --allow-serial-runners --build-tags=integration,bridge,systemd ./...`

Descriptor tests cover metadata, type, path, permissions and retained manager
ownership. Three installation cases verify repeated build hashes, actual binary
execution, atomic replacement, real unit verification, escaped paths, rejected
symlinks and preserved data/config. The executable documentation recipe refuses
live WAL, then verifies stopped-copy integrity and restored CLI IDs/messages;
later writes disappear after restoration. Online-backup/cancellation and daemon
stderr regressions also pass.

The initial focused systemd run failed at `218/CAPABILITIES`: an empty capability
bounding-set directive required privilege unavailable to the user manager.
The unit now uses restrictions supported by the unprivileged manager; the real
test verifies zero effective capabilities and no new privileges. The shutdown
fixture also accepts an explicit cancellation error envelope as well as EOF.

The corrected real user-manager race check passed in 6.028 seconds. Private,
uniquely named runtime units verified inactive-before-request activation,
matching XDG roots with spaces/percent/dollar signs, 0600 socket/database/WAL/SHM,
0700 roots, no implicit restart during upgrade, deliberate restart, crash
restart, durable IDs/messages, long-poll disconnect, cancelled waits, retained
socket inode and orderly shutdown/reopen. A one-second idle observation measured
0 CPU ticks and 13,656 KiB daemon RSS with no bridge/provider children. Cleanup
removed all owned units, links and roots. Missing manager access fails the test.

The ordered controller validation must bind the clean PR head before integration.

## Risks and Follow-ups

Service and clients must use matching XDG roots. Installation captures those
roots; reinstall after changing them. Same-UID processes share the local trust
boundary. Idle measurements describe the sampled interval and daemon only.
Backups restore worker/consumer progress to snapshot time; inspect uncertain
external effects before deliberately resuming workers.
