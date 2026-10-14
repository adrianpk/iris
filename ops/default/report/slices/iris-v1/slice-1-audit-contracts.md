# Slice 1: Audit and Contracts

Status: drafting
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `docs/slice-1-iris-audit-contracts`
PR: pending

## Purpose

Establish the contracts and acceptance baseline needed before Iris runtime
implementation. Verify the installed Codex integration with bounded execution.

## Delivered Behavior

The family audit below records observed source behavior and deliberate Iris
decisions. Runtime and bridge implementation belong to subsequent slices.

## Implementation Notes

### Family audit

Inspected Acta at
[`7cf66a0188ae187a75cc239059d30b814f510331`](https://forge.adrianpk.com/adrianpk/acta/src/commit/7cf66a0188ae187a75cc239059d30b814f510331)
and Lore at
[`fd9a511fea397788c3ab9c276a70b520463246e5`](https://forge.adrianpk.com/adrianpk/lore/src/commit/fd9a511fea397788c3ab9c276a70b520463246e5).
Both source worktrees were clean and remained unchanged. Paths in the table
are relative to each inspected repository.

| Concern | Observed evidence | Iris decision |
| --- | --- | --- |
| 1. Organization | Thin `main.go`; cohesive `internal/acta/` and `internal/lore/`; `assets/systemd/user/`; package-local tests and README. | Adopt a thin root executable and `internal/iris/`; split files by CLI, transport, store, lifecycle, and bridge ownership. |
| 2. Language, build, install | Both `go.mod` files use Go 1.26.3 without external requirements. Make targets include `check`, `lint`, `verify`, and `docs-check`; style scripts check function signatures and package READMEs. `docs/tutorials/user-guide/01-install-and-activate.md` uses `GOBIN` with `go install .` and explicit user-unit installation. | Preserve tooling and explicit activation. SQLite needs one independently selected driver; neither sibling becomes a runtime dependency. |
| 3. CLI/service protocol | `internal/<product>/transport.go`: one JSON request per Unix stream, versioned `protocol`/`operation`/`params`, `ok`/`result`/`error`; request limits 64 KiB in Acta and 1 MiB in Lore, responses 8 MiB, default connection timeout 10 seconds. | Adopt the envelope shape with Iris versions and explicit limits. Waiting has its own bounded deadline; do not apply a ten-second timeout to a longer wait. |
| 4. systemd and shutdown | `assets/systemd/user/*.socket`: inherited listener, `Accept=no`, socket 0600, directory 0700, removal on stop. Services use `UMask=0077`, `NoNewPrivileges=yes`, `PrivateTmp=yes`, and ten-second stop timeout. `main.go` uses signal cancellation; activation and transport tests cover ownership. | Adopt private user socket activation and orderly shutdown. Keep bridge workers separate so daemon confinement does not grant or silently restrict worker authority. |
| 5. Paths, config, errors | `config.go`: Acta data under `~/.acta`; Lore honors absolute `XDG_DATA_HOME` and otherwise uses `~/.local/share/lore`. Both require `XDG_RUNTIME_DIR`; configuration roots use `~/.config/<product>`. Entrypoints print errors to stderr and exit 1. | Use XDG config/data/runtime roots for Iris, SQLite under the data root, and no automatic repository files. Validate absolute paths, private ownership, and stable errors. |
| 6. Commands and output | `App.Run` dispatches explicit subcommands with injected output streams; command implementations use standard-library flags and JSON serialization. Root `--help` and `help` currently return unknown-command errors in both installed siblings. | Keep explicit dispatch, flags, human output, and clean JSON stdout. Implement successful discoverable root help rather than copying the observed missing help behavior. |
| 7. Tests, storage, delivery | `acceptance_test.go`, transport, activation, and systemd tests exercise real local boundaries, temporary data, concurrency, and restart. Siblings use append-only JSONL recovery, not SQLite migrations. Make `verify` runs lint and Go tests. No tracked `.github/` or `.forgejo/` workflow tree was present. | Use real SQLite migrations/reopen/transactions, isolated process and user-manager tests, and a separately bounded live Codex gate. Do not infer CI or release automation from local unit checks. |
| 8. Documentation and policy | README, CHANGELOG, Diataxis docs, progressive User Guide, package README, and `.agents/README.md` routing to shared rules. No tracked root `AGENTS.md`, `CONTRIBUTING.md`, `LICENSE`, or `LICENSE.md` was present at either revision. | Preserve documentation layout and shared policy routing. Do not invent a license, contribution policy, release channel, or global agent integration. |

## Contracts Added or Changed

The audit selects compatible structure and operation while preserving SQLite
persistence and on-demand model execution as separate Iris responsibilities.
Detailed runtime and data contracts are pending T1.3.

## Files of Interest

- This report: canonical family audit and Slice 1 evidence.
- [Plan](../../../plan/iris-v1.md): exact slice identities and acceptance owners.
- [Tracker](../../../tracker/iris-v1.md): task state and commit references.

## Validation

T1.1 source inspection completed at the two immutable revisions above. Final
documentation and cross-reference checks will be recorded before PR readiness.

## Risks and Follow-ups

The installed Codex execution probe and detailed SQLite/protocol contracts
remain in the current slice's T1.2 through T1.4 tasks.
