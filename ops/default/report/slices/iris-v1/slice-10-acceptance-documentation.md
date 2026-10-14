# Slice 10: Acceptance and Documentation

Status: delivered
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `test/slice-10-iris-acceptance-documentation`
PR: [#10](https://forge.adrianpk.com/adrianpk/iris/pulls/10)

## Purpose

Complete the user learning path, public contracts and acceptance automation for
the implemented v1 behavior. Preserve exact evidence for controller integration
and the subsequent full delivery gate.

## Delivered Behavior

The User Guide leads from source installation through two-project invitation,
explicit bounded Codex workers, retained history, export and closure. README
and quadrant navigation lead to that path and supporting operational references.

CLI, runtime, storage and trust references expose current forms, defaults,
framing, migrations, bounds and recovery. Explanation separates persistence,
processing and explicit worker context. Planned capabilities retain forum/group,
broadcast, claims, other adapters and remote/multiuser work as future behavior.

The acceptance map binds ten groups to 28 existing cases and their actual gate
selectors. Lifecycle process acceptance exports expired retained history before
confirmed pruning, then rejects deleted lookup and exports only fresh content.
Live Codex acceptance now creates a second conversation after recovery/resumption
and checks two new owned threads/consumers, a new random local proof, preserved
old history and idle counts.

Verification retains strict lint and ordinary tests and adds document/public
contracts, acceptance-map checks and source installation/removal. The exact
candidate procedure records a clean full hash and ordered results. All ten slice
PRs are integrated, and the controller's unchanged full gate passed at
`5b951abbf401eb3226ab294cceb2b1089da99f5a`. T10.4 and delivery documentation
closure are complete.

## Implementation Notes

An actual-binary test executes the published invitation/history shell blocks
unchanged in owned temporary projects. It catches command and JSON-format drift.
The guide distinguishes separate owned workers from existing interactive
sessions and keeps tools and approval decisions explicit.

## Contracts Added or Changed

Documentation exposes existing behavior; runtime and storage formats are unchanged.
Plain JSON transcripts use the semantic transcript object; `dump --json` uses
the versioned success envelope.

## Files of Interest

- `docs/tutorials/user-guide/index.md`: progressive user learning path.
- `docs/reference/projects.md`: identity, selection and invitation contracts.
- `internal/iris/process_guide_test.go`: executable guide commands.

## Validation

All ten ordered PR checks passed through the controller on final canonical PR
head `879bde689b196fe4c8ee74d2afdc1424354075a3`, which merged into `dev` unchanged:

- `make check`
- `make lint`
- `make docs-check`
- `python3 scripts/check-contracts.py`
- `python3 scripts/check-acceptance.py`
- `python3 scripts/test-install.py`
- `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run '^TestProcessUserGuide$|^TestProcessLifecycleRecovery$|^TestReferenceConfiguration$' -count=1`
- `make test-bridge`
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --allow-serial-runners --build-tags=integration,bridge,systemd ./...`
- `git diff --check`

The real bridge fixture passed in 58.854 seconds with Codex CLI 0.161.0. It
recovered a completed result without another invocation, resumed both original
threads/consumers for pending work, and completed another conversation with two
new owned thread/consumer IDs and a new random project-local proof. Prior history
remained retained; the new transcript contained only its own exchange. Turn
counts remained one per fresh worker with no idle echo. The one-second sample
observed 0 CPU ticks and 36,412 KiB RSS for the daemon and two Iris workers,
excluding provider subprocesses. These are bounded observations.

Changed guide/lifecycle/reference race checks passed in 3.721 seconds. Published
invitation/history shell blocks execute unchanged against the actual binary.
Expired history exports before pruning; deleted lookup fails; fresh export has
only its new context. The guide's initial plain-JSON envelope assertion was
corrected after executable validation exposed the format mismatch.

Contract checks passed for 42 documents, ten matching slices, 44 tasks, eighteen
typed operation names, storage schema 4, fourteen SQL tables and eleven rejected
invalid cases. The acceptance checker mapped ten groups to 28 existing cases and
their actual gate selectors. Three isolated installation cases and combined-tag
lint passed. Published configuration equals executable defaults and embedded
migrations match the accepted SQL model.

`make -n verify` confirmed the wiring of strict lint, ordinary tests, contract
checks, acceptance mapping and installation checks before integration.

The PR validation ledger binds the final PR head and ordered command list above.
Acta merge event `01M4XNPYKED01SCTBARZZRB34S` confirms Rebase + Fast-forward
integration. Required tools, private writable runtime/Unix socket and unit-link
paths, a reachable user manager and authenticated Codex 0.161.0 supported the
subsequent integrated gate.

The controller's separate full-gate ledger records successful validation of
immutable candidate `5b951abbf401eb3226ab294cceb2b1089da99f5a` with:

- `make verify && make docs-check && make test-integration && make test-systemd && make test-bridge && git diff --check`

Strict lint reported zero issues; ordinary tests, 42-document contract checks,
the ten-group acceptance map and three installation cases passed. Actual-process
integration passed in 12.267 seconds, isolated user-systemd operation in 4.864
seconds and the real Codex recovery/fresh-context exchange in 70.576 seconds.
The bridge recovered a completed result without another invocation, resumed
both original threads/consumers and completed a second conversation with two
fresh threads/consumers and a new project-local proof while preserving old
history. Idle counts showed no extra turns. One-second samples observed zero
CPU ticks with 13,596 KiB RSS for the idle systemd daemon and 42,748 KiB RSS for
the daemon plus two Iris bridge processes, excluding provider subprocesses.

Owned fixtures cleaned up after their checks; no acceptance units remain
running. The plan and tracker are completed, all ten reports are delivered and
all 44 tasks are complete. Changes after the tested candidate are confined to
configured closure documentation; they do not change the candidate bound to
the full-gate evidence.

## Risks and Follow-ups

Bridge acceptance covers separate conversation-bound workers. Attachment to an
existing interactive session remains unverified. CPU/RSS and idle measurements
are bounded observations from the recorded fixtures, not general resource
guarantees.
