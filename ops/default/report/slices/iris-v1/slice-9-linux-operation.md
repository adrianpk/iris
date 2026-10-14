# Slice 9: Linux Operation

Status: reviewing
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-9-iris-linux-operation`
PR: [#9](https://forge.adrianpk.com/adrianpk/iris/pulls/9)

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

All nine ordered checks passed through Looper on clean implementation candidate
`95ab6656dfee575f190c4180775e4b10edaba59b` before canonical PR #9 opened:

- `make check`
- `make lint`
- `make docs-check`
- `make test-systemd`
- `python3 scripts/check-contracts.py`
- `python3 scripts/test-install.py`
- `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run '^TestProcess|^TestActivation|^TestBackup|^TestServeDiagnostic$' -count=1`
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --allow-serial-runners --build-tags=integration,bridge,systemd ./...`
- `git diff --check`

The controller's required `make test-systemd` run passed in 5.062 seconds.
Uniquely named runtime units verified inactive-before-request activation,
matching XDG roots with spaces/percent/dollar signs, 0600 socket/database/WAL/SHM,
0700 roots, no implicit restart during upgrade, deliberate restart, crash
restart, durable IDs/messages, long-poll disconnect, cancelled waits, retained
socket inode and orderly shutdown/reopen. Its one-second idle observation
measured 0 CPU ticks and 13,524 KiB daemon RSS with no bridge/provider children.
Cleanup removed all owned units, links and roots; missing manager access fails.

The actual-process and changed-boundary race command passed in 13.981 seconds.
Descriptor tests reject incorrect metadata, network/packet listeners, unsafe
modes and foreign paths while preserving manager ownership. Three installation
cases verify repeated build hashes, executable output, atomic replacement,
real unit syntax, escaped paths, rejected symlinks and data/config preservation.
The executable documentation recipe refuses live WAL, verifies stopped-copy
integrity and restores CLI IDs/messages while discarding later writes. Online
backup/cancellation and daemon stderr regressions also pass. The contract checker
verified ten matching slices, 44 tasks, fourteen tables and eleven invalid cases.

Additional focused check passed:
`GOTOOLCHAIN=go1.26.3 go test -v -race -tags=systemd ./internal/iris -run '^TestSystemdOperation$' -count=1 -timeout=2m`
(6.028 seconds). The initial run failed at `218/CAPABILITIES`: an empty capability
bounding-set directive required privilege unavailable to the user manager.
The corrected unit uses unprivileged-manager restrictions; the running daemon
has zero effective capabilities and no new privileges. The shutdown fixture
accepts an explicit cancellation error envelope as well as EOF.

The PR-reference follow-up requires regeneration on its exact clean head with
this same ordered command list. The canonical PR head and controller ledger
identify final integration evidence.

## Risks and Follow-ups

Service and clients must use matching XDG roots. Installation captures those
roots; reinstall after changing them. Same-UID processes share the local trust
boundary. Idle measurements describe the sampled interval and daemon only.
Backups restore worker/consumer progress to snapshot time; inspect uncertain
external effects before deliberately resuming workers.
