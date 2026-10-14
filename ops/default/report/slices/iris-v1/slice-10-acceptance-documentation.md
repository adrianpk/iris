# Slice 10: Acceptance and Documentation

Status: reviewing
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `test/slice-10-iris-acceptance-documentation`
PR: pending

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

Passed: `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run '^TestProcessUserGuide$' -count=1`.
The published commands create/join, inspect, export both formats, close and
preview without deletion. IDs, ordinal, terminal state and ordered messages
are checked through the actual binary. An initial guide assertion expected an
envelope for plain `--format json`; the real execution caught it and the
published jq assertion now checks the semantic transcript object.

Passed: `python3 scripts/check-contracts.py`: forty documents, matching ten-slice
and 44-task identities, eighteen typed operation names, storage schema 4,
fourteen SQL tables and eleven rejected invalid cases.

Passed: `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run '^TestReferenceConfiguration$|^TestMigrationMatchesContract$' -count=1`.
The full published configuration equals actual defaults; the embedded migrations
match the accepted SQL model. Combined-tag lint also passed.

Passed: `python3 scripts/check-acceptance.py`: ten consecutive groups and 28
existing cases assigned to their actual gates and selectors.

Passed: `GOTOOLCHAIN=go1.26.3 go test -race -tags=integration ./internal/iris -run '^TestProcessUserGuide$|^TestProcessLifecycleRecovery$|^TestReferenceConfiguration$' -count=1`.
Real guide commands, expired history exports, confirmed prune, deleted lookup,
fresh transcript context and documented configuration passed. The changed live
bridge fixture requires ordered clean-head execution before PR readiness.

Passed: `make -n verify` confirms that strict lint, ordinary tests, contract
checks, acceptance-map checks and installation tests are all wired. This is
command-wiring evidence, not execution of the complete delivery gate.

## Risks and Follow-ups

The integrated immutable candidate and full gate belong to the controller's
later gate/closure phases. Current focused evidence does not close that gate.
