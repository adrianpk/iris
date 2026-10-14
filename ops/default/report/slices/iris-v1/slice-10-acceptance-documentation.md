# Slice 10: Acceptance and Documentation

Status: reviewing
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
candidate procedure records a clean full hash and ordered results. Full-gate
execution and delivery closure remain pending controller integration.

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

All ten ordered PR checks passed through the controller on clean implementation
candidate `47a9163f33c751ae40c01c82134d329276c951fb` before canonical PR #10 opened:

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

The real bridge fixture passed in 64.153 seconds with Codex CLI 0.161.0. It
recovered a completed result without another invocation, resumed both original
threads/consumers for pending work, and completed another conversation with two
new owned thread/consumer IDs and a new random project-local proof. Prior history
remained retained; the new transcript contained only its own exchange. Turn
counts remained one per fresh worker with no idle echo. The one-second sample
observed 0 CPU ticks and 36,472 KiB RSS for the daemon and two Iris workers,
excluding provider subprocesses. These are bounded observations.

Changed guide/lifecycle/reference race checks passed in 3.709 seconds. Published
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
checks, acceptance mapping and installation checks. The integrated full gate has
not run in this phase. T10.4's integrated candidate and full-gate recording remain
pending the controller's gate and closure phases.

The PR-reference follow-up requires regeneration on its exact clean head with
this same ordered command list. The canonical PR head and controller ledger
identify final PR validation evidence.

## Risks and Follow-ups

The integrated immutable candidate and full gate belong to the controller's
later gate/closure phases. Current focused evidence does not close that gate.
