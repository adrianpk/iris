# Slice 9: Linux Operation

Status: drafting
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
listeners and unsafe permissions. Closing the adopted listener preserves the
manager-owned socket. Manual service startup retains its existing behavior.

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

## Validation

Passed: `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run '^TestActivation|^TestRuntime|^TestClientSocket' -count=1`.
Metadata rejection, real descriptor adoption, preserved manager ownership,
unsafe path/mode/network rejection and existing manual runtime checks passed.
Go lint passed after formatting.
Actual user-manager acceptance, installation and backup checks remain in progress.

## Risks and Follow-ups

The service and clients must use matching XDG roots. Same-UID processes remain
inside the local trust boundary. Real isolated user-manager acceptance is
required before the slice is ready.
