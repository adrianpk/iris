# Iris v1 Implementation Tracker

Status: approved

Approved: maintainer, 2026-10-14; execute this delivery set with Looper.

Delivery set: `iris-v1`

Plan: [Iris v1 plan](../plan/iris-v1.md)

Spec: [Iris specification](../spec/iris.md)

Model: [Iris data model](../spec/iris-model.md)

Base branch: `dev`

Active slice: Slice 2, reviewing; service and storage.

Active tasks: none; Slice 2 awaits required PR validation and controller integration.

Execution gate: authorized by the user's 2026-10-14 instruction to use Looper.
Startup requires committed/published planning artifacts and successful preflight.

Progress: 1 of 10 slices delivered; 8 of 44 tasks completed.

## Entry conditions

| Condition | State | Evidence or next action |
| --- | --- | --- |
| Workspace and supplied spec/model available | satisfied | Initial scaffold and draft pair are present. |
| Exact initial commit on `main` | satisfied | `ea16e3a` has subject `Initial commit`. |
| Development branch and commit policy | satisfied | Current branch is `dev`; subsequent commits use Conventional Commits. |
| Canonical remote configured | satisfied | `origin` is configured; the remote was empty when inspected. |
| Plan and tracker drafted | satisfied | This tracker and its linked plan define ten slices and 44 tasks. |
| Plan approved for implementation | satisfied | User authorized Looper execution on 2026-10-14, including the controller's configured slice integrations into `dev`. |
| Approved plan/tracker committed on `dev` | satisfied | Canonical `dev` is `5b548d2851eb60e2a06140b17793157efe893fab`. |
| Published canonical `dev` base | satisfied | Live origin refs match the planning baseline and initial `main`; Forgejo was inspected before branching. |
| Detailed spec/model baseline | satisfied | Accepted `spec/iris.md` and `spec/iris-model.md` settle section 17 behavior and physical storage together. |
| SQLite dependency selected and confirmed | satisfied | T2.2 added modernc.org/sqlite v1.60.1 and verified migration, transaction, concurrency, reopen and online-backup behavior with the real driver. |
| Real Codex integration feasibility | satisfied | T1.2 verified separate owned workers using real turns, own-thread resumption, denial, and interruption. The Iris exchange remains T5.5 acceptance. |
| Full/live/systemd acceptance environment | pending | Establish isolated fixtures and provider/user-manager availability in their owning slices. |

Slice 1 was integrated through PR `#1` at
`06041b58bf5dd44c42202c5463ab202da511308a`. The primary checkout remains on
`dev`; the external controller owns subsequent slice integration.

## Slice status

| Slice | Short Name | Status | Branch | Expected PR title | PR | Report |
| --- | --- | --- | --- | --- | --- | --- |
| Slice 1 | audit and contracts | delivered | `docs/slice-1-iris-audit-contracts` | `docs(slice-1): define Iris implementation contracts` | `#1` | `ops/default/report/slices/iris-v1/slice-1-audit-contracts.md` |
| Slice 2 | service and storage | reviewing | `feat/slice-2-iris-service-storage` | `feat(slice-2): establish Iris service and SQLite storage` | pending | `ops/default/report/slices/iris-v1/slice-2-service-storage.md` |
| Slice 3 | projects and handshake | pending | `feat/slice-3-iris-projects-handshake` | `feat(slice-3): connect projects through Iris invitations` | pending | `ops/default/report/slices/iris-v1/slice-3-projects-handshake.md` |
| Slice 4 | durable delivery | pending | `feat/slice-4-iris-durable-delivery` | `feat(slice-4): deliver ordered Iris messages` | pending | `ops/default/report/slices/iris-v1/slice-4-durable-delivery.md` |
| Slice 5 | Codex exchange | pending | `feat/slice-5-iris-codex-exchange` | `feat(slice-5): enable autonomous Codex conversations` | pending | `ops/default/report/slices/iris-v1/slice-5-codex-exchange.md` |
| Slice 6 | lifecycle and retention | pending | `feat/slice-6-iris-lifecycle-retention` | `feat(slice-6): manage Iris expiration and retention` | pending | `ops/default/report/slices/iris-v1/slice-6-lifecycle-retention.md` |
| Slice 7 | history and export | pending | `feat/slice-7-iris-history-export` | `feat(slice-7): inspect and export Iris conversations` | pending | `ops/default/report/slices/iris-v1/slice-7-history-export.md` |
| Slice 8 | bridge recovery | pending | `feat/slice-8-iris-bridge-recovery` | `feat(slice-8): recover and bound Iris bridge execution` | pending | `ops/default/report/slices/iris-v1/slice-8-bridge-recovery.md` |
| Slice 9 | Linux operation | pending | `feat/slice-9-iris-linux-operation` | `feat(slice-9): operate Iris through user systemd` | pending | `ops/default/report/slices/iris-v1/slice-9-linux-operation.md` |
| Slice 10 | acceptance and documentation | pending | `test/slice-10-iris-acceptance-documentation` | `test(slice-10): verify Iris v1 workflows and documentation` | pending | `ops/default/report/slices/iris-v1/slice-10-acceptance-documentation.md` |

Slice 1 has its canonical report. Remaining report paths identify future
artifacts; create each during its owning slice.

## Slice 1 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T1.1 | completed | `docs(architecture): record Iris family patterns` | `9dc6a7a7c8c0` | Eight-point source audit recorded in the canonical report; `make docs-check` and the contract checker passed through Looper. |
| T1.2 | completed | `docs(bridge): verify Codex integration contract` | `27ac95322321` | Six live probe assertions passed on Codex 0.161.0: cwd/turn, fresh thread, own-thread recovery, interrupt, denied approval, and absent write. Existing-session attachment unverified. |
| T1.3 | completed | `docs(spec): finalize Iris runtime and data contracts` | `2bd054774713` | Selected and confirmed modernc.org/sqlite v1.60.1 metadata; settled schema/query mapping, identity, delivery, lifecycle, bounds, and recovery contracts; fourteen-table schema and eleven negative constraints passed through the controller contract check. |
| T1.4 | completed | `docs(plan): establish Iris acceptance baseline` | `dcefd22ca18a` | Specification/model promoted together; ten acceptance fixtures and command ownership defined; `make docs-check`, `python3 scripts/check-contracts.py`, and `git diff --check` passed via Looper; PR-reference follow-up requires exact-HEAD regeneration. |

## Slice 2 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T2.1 | completed | `feat(core): establish Iris executable and configuration` | `d37086957a3f` | Focused path/config/help/error/cancellation tests passed; Go 1.26.3 lint passed. |
| T2.2 | completed | `feat(store): establish Iris SQLite persistence` | `8f9d64b71c9c` | Driver tests passed for fresh/reopened data, atomic upgrade rollback/retry, checksum/newer-schema rejection, foreign keys/pragmas/private files, concurrent transactions, and online backup; race checks passed. |
| T2.3 | completed | `feat(service): expose Iris local protocol` | `231c20bc091c` | Real Unix round trips passed for version/JSON/size/scope errors, private socket and owner lock, stale recovery, retryable connection limits, cancellation/disconnect/shutdown, and CLI JSON/errors; focused race checks passed. |
| T2.4 | completed | `test(service): verify Iris daemon and database recovery` | pending | `make test-integration` passed: actual binary, two isolated project roots, persisted state across graceful/crash restart, exclusive ownership, invalid-checksum startup rejection, and owned cleanup; integration-tag lint passed. |

## Slice 3 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T3.1 | pending | `feat(projects): register Iris project identities` | pending | Canonical/nested/symlink paths, optional override, explicit project, and no generated repository state. |
| T3.2 | pending | `feat(projects): manage Iris aliases and project moves` | pending | Collision rejection, rename history, and explicit reassociation. |
| T3.3 | pending | `feat(conversations): create Iris invitations` | pending | Concurrent unordered-pair ordinals, unique live codes, expected recipient, and initial persistence. |
| T3.4 | pending | `feat(conversations): join and close Iris conversations` | pending | Correct/wrong/expired/concurrent join, first message, terminal-state rejection, and no reopen. |
| T3.5 | pending | `test(conversations): verify Iris project handshakes` | pending | Real two-project handshake; human output and valid JSON/`jq`; no implicit acknowledgment. |

## Slice 4 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T4.1 | pending | `feat(messages): persist ordered Iris messages` | pending | Stable order, idempotent retry, conflict handling, and durable send confirmation. |
| T4.2 | pending | `feat(delivery): track Iris consumer processing` | pending | Independent cursors, explicit ack, restart, and durable result/completion. |
| T4.3 | pending | `feat(delivery): wait for Iris messages efficiently` | pending | Wait/commit race, notification, timeout, disconnect, and terminal-state wakeup. |
| T4.4 | pending | `feat(cli): send and receive Iris message content` | pending | Text/file/stdin and limits across CLI/socket/store; clean JSON stdout. |
| T4.5 | pending | `test(delivery): verify Iris message recovery and isolation` | pending | Real processes, concurrency, restart/retry, and project/conversation isolation. |

## Slice 5 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T5.1 | pending | `feat(bridge): manage conversation-scoped Iris workers` | pending | Explicit start/status/stop, exclusive binding, correct cwd, and owned-process cleanup. |
| T5.2 | pending | `feat(bridge): run Iris exchanges through Codex` | pending | Verified adapter, sequential turns, final-result handling, and conversation-only context. |
| T5.3 | pending | `feat(bridge): preserve Codex authority boundaries` | pending | Sandbox/approval decisions preserved; no silent auto-approval or false success ack. |
| T5.4 | pending | `feat(bridge): complete bounded Iris exchanges` | pending | Response-before-ack, idempotent completion, time/turn bounds, and no acknowledgment echo loop. |
| T5.5 | pending | `test(bridge): demonstrate autonomous Iris communication` | pending | `make test-bridge` added; real question/response/ack without human relay; zero idle model invocations. |

## Slice 6 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T6.1 | pending | `feat(lifecycle): define Iris expiration policy` | pending | Controlled clock, configured defaults, significant activity, and read/wait non-renewal. |
| T6.2 | pending | `feat(lifecycle): expire inactive Iris conversations` | pending | Autonomous expiration, restart reconciliation, inbox exclusion, bridge notification, and races. |
| T6.3 | pending | `feat(retention): prune confirmed Iris history` | pending | Read-only preview, exact/stale confirmation, eligibility, and preserved pair ordinals. |
| T6.4 | pending | `test(lifecycle): verify Iris retention and conversation isolation` | pending | Retained closed/expired history; prune/restart and fresh conversation ID/context. |

## Slice 7 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T7.1 | pending | `feat(history): inspect Iris conversations` | pending | Project-visible listings/show; explicit user administration outside repositories. |
| T7.2 | pending | `feat(search): find Iris conversation history` | pending | Text/project/pair/date/ordinal/ID filters; deterministic bounded results and indexes. |
| T7.3 | pending | `feat(export): dump readable Iris transcripts` | pending | Complete ordered text/JSON metadata/messages beyond a response page, without inbox effects. |
| T7.4 | pending | `feat(cli): summarize Iris storage and conversations` | pending | Stats agree with fixtures; JSON/`jq`, stderr separation, and documented human examples. |

## Slice 8 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T8.1 | pending | `feat(bridge): recover Iris worker progress` | pending | Crashes around invocation/result/ack, durable checkpoints, and exclusive resumed ownership. |
| T8.2 | pending | `feat(bridge): report uncertain Iris worker outcomes` | pending | Explicit failed/interrupted/capacity/uncertain states; bounded retry and no unsafe side-effect replay. |
| T8.3 | pending | `feat(bridge): enforce Iris execution limits` | pending | Time/turn/available-usage limits, terminal cancellation, and owned-resource cleanup. |
| T8.4 | pending | `test(security): verify Iris input and authority boundaries` | pending | Malformed/oversized input, injection/isolation/permissions, focused fuzz/race checks. |
| T8.5 | pending | `test(bridge): verify Iris recovery and idle behavior` | pending | Real adapter restart/resumption, zero idle model calls, resource measurements, and honest recovery limitations. |

## Slice 9 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T9.1 | pending | `feat(systemd): activate the Iris user service` | pending | Units, private socket, inherited descriptors, and appropriate daemon/bridge restrictions. |
| T9.2 | pending | `build(linux): install and remove Iris user components` | pending | Temporary install/upgrade/removal; explicit activation and data preservation. |
| T9.3 | pending | `docs(operation): document Iris backup and recovery` | pending | SQLite-consistent backup/restore/reopen and documented diagnostics/shutdown. |
| T9.4 | pending | `test(systemd): verify Iris user service operation` | pending | `make test-systemd` added; real isolated user-manager activation/restart/permissions/shutdown. |

## Slice 10 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T10.1 | pending | `docs(guide): explain Iris project conversations` | pending | Reproduce installation/handshake/bridge workflow using the User Guide and temporary projects. |
| T10.2 | pending | `docs(reference): document Iris contracts and future capabilities` | pending | Runtime/schema/CLI consistency, links/package docs, and preserved forum/broadcast future scope. |
| T10.3 | pending | `test(acceptance): verify the complete Iris v1 workflow` | pending | All ten required test groups; real processes/systemd/Codex, retention/export/prune, and fresh-context isolation. |
| T10.4 | pending | `chore(delivery): finalize Iris v1 validation` | pending | Immutable candidate SHA, full gate, canonical reports, and concrete limits; release publication remains separate. |

## Validation evidence

| Evidence | State | Meaning |
| --- | --- | --- |
| Acta/Lore planning inspection | recorded | Source structure, config, protocol, systemd assets, test patterns, and documentation inspected at revisions recorded in the plan. T1.1 completed the eight-point comparison in the canonical report. |
| Planning Codex CLI/schema inspection | passed | CLI reports 0.161.0; default generated schema contains required thread/turn and approval contracts. No model was invoked. |
| Plan/tracker documentation and structural checks | passed | `make docs-check`; agreement of ten slice identities and 44 task/commit identities; all ten acceptance groups; relative links, branch syntax, whitespace, and context privacy checks. |
| Live Codex probe for Slice 1 | passed | Six bounded assertions verified separate owned workers, cwd, real results, resumption, denied approval, and interruption. This is feasibility evidence, not the future Iris exchange. |
| Contract gate for Slice 1 | passed | Looper validated the final PR head `06041b58bf5dd44c42202c5463ab202da511308a` with `make docs-check`, `python3 scripts/check-contracts.py`, then `git diff --check`: fourteen tables, eleven rejected invalid cases, rollback and allocation retention. The canonical PR merged at that same head. |
| Service/storage focused checks | passed | Executable/configuration, real driver/SQLite, private socket/protocol, cancellation, ownership, and real-binary graceful/crash recovery pass. Final PR checks are recorded by the controller for its exact clean HEAD. |
| User-systemd/live Iris exchange | not run | These targets and acceptance workflows remain assigned to their owning slices. |

## Completion gates

- [x] Delivery set explicitly approved for execution through Looper.
- [x] Approved artifacts committed and canonical startup prerequisites verified.
- [x] Canonical `dev` base published for slice PR delivery.
- [x] Slice 1 delivered; detailed spec/model settled and SQLite dependency confirmed.
- [ ] Slice 2 delivered; real CLI/daemon/SQLite integration passes.
- [ ] Slice 3 delivered; identity, numbering, invitation, join, and close pass.
- [ ] Slice 4 delivered; ordered durable messaging, wait, cursor, ack, and isolation pass.
- [ ] Slice 5 delivered; real Codex exchange and zero-idle-call evidence recorded.
- [ ] Slice 6 delivered; service expiration, retention, and confirmed pruning pass.
- [ ] Slice 7 delivered; bounded discovery, complete exports, and stats pass.
- [ ] Slice 8 delivered; bridge crash recovery, uncertainty, budgets, and security pass.
- [ ] Slice 9 delivered; installation and real isolated user-systemd operation pass.
- [ ] Slice 10 delivered; User Guide, references, future capabilities, and complete acceptance pass.
- [ ] All ten canonical reports delivered and slice PRs merged into `dev`.
- [ ] Full gate green for the recorded immutable integrated candidate.
- [ ] Achieved Codex session mode and concrete product limitations documented.

## Update rules

Set only the authorized next slice active. Before starting it, verify the prior
merge, exact branch, recorded prerequisites, and plan/tracker agreement. Record
task commit IDs and exact successful or failed checks as work occurs.

Use pending, in-progress, reviewing, delivered, or blocked slice states as
appropriate. A task is complete only with its actual implementation and
required evidence; planning inspection does not complete future runtime work.

Reserve a table cell beginning with `Slice ` for the canonical slice map.
Use descriptive evidence labels such as `Contract gate for Slice 1` elsewhere,
because the installed Looper parser also examines rows outside the slice map.

Do not close Slice 5 with a fake or manually relayed exchange. Do not close the
delivery set with missing live Codex or user-systemd acceptance. Record the
limitation and revise the authorized plan when an external prerequisite is
unavailable. Bind final evidence to the actual tested SHA, without implying a
later bookkeeping commit was tested.
