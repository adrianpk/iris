# Iris v1 Implementation Tracker

Status: Completed

Approved: maintainer, 2026-10-14; execute this delivery set with Looper.

Delivery set: `iris-v1`

Plan: [Iris v1 plan](../plan/iris-v1.md)

Spec: [Iris specification](../spec/iris.md)

Model: [Iris data model](../spec/iris-model.md)

Base branch: `dev`

Active slice: none; all ten slice implementations are integrated.

Active tasks: none; the integrated gate and documentation closure are complete.

Execution gate: authorized by the user's 2026-10-14 instruction to use Looper.
Startup requires committed/published planning artifacts and successful preflight.

Progress: 10 of 10 slices delivered; 44 of 44 tasks completed.

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
| Full/live/systemd acceptance environment | satisfied | The controller's integrated full gate passed real Codex and isolated user-systemd acceptance on `5b951abbf401eb3226ab294cceb2b1089da99f5a`. Owned fixtures cleaned up; no acceptance units remain running. |

Slice 1 was integrated through PR `#1` at
`06041b58bf5dd44c42202c5463ab202da511308a`. The primary checkout remains on
`dev`; the external controller owns subsequent slice integration.

## Slice status

| Slice | Short Name | Status | Branch | Expected PR title | PR | Report |
| --- | --- | --- | --- | --- | --- | --- |
| Slice 1 | audit and contracts | delivered | `docs/slice-1-iris-audit-contracts` | `docs(slice-1): define Iris implementation contracts` | `#1` | `ops/default/report/slices/iris-v1/slice-1-audit-contracts.md` |
| Slice 2 | service and storage | delivered | `feat/slice-2-iris-service-storage` | `feat(slice-2): establish Iris service and SQLite storage` | `#2` | `ops/default/report/slices/iris-v1/slice-2-service-storage.md` |
| Slice 3 | projects and handshake | delivered | `feat/slice-3-iris-projects-handshake` | `feat(slice-3): connect projects through Iris invitations` | `#3` | `ops/default/report/slices/iris-v1/slice-3-projects-handshake.md` |
| Slice 4 | durable delivery | delivered | `feat/slice-4-iris-durable-delivery` | `feat(slice-4): deliver ordered Iris messages` | `#4` | `ops/default/report/slices/iris-v1/slice-4-durable-delivery.md` |
| Slice 5 | Codex exchange | delivered | `feat/slice-5-iris-codex-exchange` | `feat(slice-5): enable autonomous Codex conversations` | `#5` | `ops/default/report/slices/iris-v1/slice-5-codex-exchange.md` |
| Slice 6 | lifecycle and retention | delivered | `feat/slice-6-iris-lifecycle-retention` | `feat(slice-6): manage Iris expiration and retention` | `#6` | `ops/default/report/slices/iris-v1/slice-6-lifecycle-retention.md` |
| Slice 7 | history and export | delivered | `feat/slice-7-iris-history-export` | `feat(slice-7): inspect and export Iris conversations` | `#7` | `ops/default/report/slices/iris-v1/slice-7-history-export.md` |
| Slice 8 | bridge recovery | delivered | `feat/slice-8-iris-bridge-recovery` | `feat(slice-8): recover and bound Iris bridge execution` | `#8` | `ops/default/report/slices/iris-v1/slice-8-bridge-recovery.md` |
| Slice 9 | Linux operation | delivered | `feat/slice-9-iris-linux-operation` | `feat(slice-9): operate Iris through user systemd` | `#9` | `ops/default/report/slices/iris-v1/slice-9-linux-operation.md` |
| Slice 10 | acceptance and documentation | delivered | `test/slice-10-iris-acceptance-documentation` | `test(slice-10): verify Iris v1 workflows and documentation` | `#10` | `ops/default/report/slices/iris-v1/slice-10-acceptance-documentation.md` |

All ten slices have delivered reports and canonical PRs merged into `dev`.
The controller's final integrated gate passed at
`5b951abbf401eb3226ab294cceb2b1089da99f5a`; its evidence is distinct from the
focused PR checks and subsequent documentation closure.

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
| T2.4 | completed | `test(service): verify Iris daemon and database recovery` | `8ceba294286f` | `make test-integration` passed: actual binary, two isolated project roots, persisted state across graceful/crash restart, exclusive ownership, invalid-checksum startup rejection, and owned cleanup; integration-tag lint passed. |

## Slice 3 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T3.1 | completed | `feat(projects): register Iris project identities` | `acec8da3ec95` | Focused canonical/nested/symlink, optional override, explicit selector, bounded project page and no-generated-project-state tests passed; lint passed. |
| T3.2 | completed | `feat(projects): manage Iris aliases and project moves` | `4ab7e92e56c6` | Focused collision, ambiguous selector, stable ID, historical snapshot, and explicit relocation tests passed; lint passed. |
| T3.3 | completed | `feat(conversations): create Iris invitations` | `c66c7fa45eb1` | Concurrent A/B allocation and retries, atomic provisional identity, initial persistence/reopen, bounded input/quota, collision rollback and permanent code/request tombstones passed with the race detector; lint passed. |
| T3.4 | completed | `feat(conversations): join and close Iris conversations` | `3151c6e7f69c` | Correct/wrong/concurrent joins, persisted expiry boundaries, unchanged initial content, duplicate non-renewal, participant-only idempotent closure, close/join races, reserved terminal event and no consumer acknowledgment passed with the race detector; lint passed. |
| T3.5 | completed | `test(conversations): verify Iris project handshakes` | `0857a91d0f42` | Actual-binary handshake and jq passed with code-only join, human/JSON output, explicit selection, rename/move, separate initial requests, text/file/stdin and encoded-frame rejection, terminal rejection, no generated project files, zero consumer state, and graceful/crash recovery; integration-tag lint passed. |

## Slice 4 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T4.1 | completed | `feat(messages): persist ordered Iris messages` | `ea515bdcd08c` | Iris concurrent send, canonical retry, participant/reply isolation and injected rollback checks passed with the race detector; lint passed. |
| T4.2 | completed | `feat(delivery): track Iris consumer processing` | `9de12aa0d1a9` | Iris independent consumer replay, scoped cursor, ordered acknowledgment, reopen, bounded page/consumer and atomic result-plus-ack race checks passed; lint passed. |
| T4.3 | completed | `feat(delivery): wait for Iris messages efficiently` | `ea44d65ac4b9` | Iris long-poll commit/subscription race, terminal wakeup, timeout, cancellation, Unix half-close/full-disconnect and waiter capacity race checks passed; lint passed. |
| T4.4 | completed | `feat(cli): send and receive Iris message content` | `218c736533dd` | Iris CLI text/file/stdin, UTF-8/body bounds, invalid metadata, clean JSON errors, human receive and explicit acknowledgment checks passed through real Unix IPC; lint passed. |
| T4.5 | completed | `test(delivery): verify Iris message recovery and isolation` | `936a275e4a84` | Iris actual clients passed concurrent sequencing, lost-confirmation retry after crash, independent consumer recovery, requested wait deadlines, terminal wakeup and project/conversation isolation; race and integration-tag lint passed. |

## Slice 5 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T5.1 | completed | `feat(bridge): manage conversation-scoped Iris workers` | `842f9dcb8f89` | Lease, concurrent ownership, stop/restart, public status isolation and migration race checks passed; foreground worker cleanup releases only its own binding. |
| T5.2 | completed | `feat(bridge): run Iris exchanges through Codex` | `eb4150888e0c` | Installed app-server protocol adapter passes race checks for handshake, bound thread/resumption, final-only output, usage, failure/interrupt, and owned process cwd/cleanup. Live end-to-end exchange remains T5.5. |
| T5.3 | completed | `feat(bridge): preserve Codex authority boundaries` | `482d7e7dd69a` | Denial, one-request human acceptance, foreign/unknown requests, empty permission grants, durable pending identity, nonterminal refusal and final-success rejection passed under race checks. |
| T5.4 | completed | `feat(bridge): complete bounded Iris exchanges` | `9757ea078bae` | Real SQLite rollback/retry, stale owner, close/completion race, informational no-model processing, turn/time/usage limits, idle ownership and confirmed/uncertain interruption passed with the race detector. |
| T5.5 | completed | `test(bridge): demonstrate autonomous Iris communication` | `df1a3822a8b5` | All eight ordered Looper checks passed on clean candidate `3235973b8edc63ba07efa8b7a7f20d566857f080`; actual Codex question/response/ack, distinct threads, project-local random proof, one turn each and zero idle invocations passed. |

## Slice 6 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T6.1 | completed | `feat(lifecycle): define Iris expiration policy` | `64b475195ae1` | Configured defaults, invalid/null/overflow policy rejection, fixed invitation deadlines, exact inactivity boundaries and receive/ack non-renewal passed; focused lint passed. |
| T6.2 | completed | `feat(lifecycle): expire inactive Iris conversations` | `c3a5d34ec8fb` | Independent timer, receive wakeup, idle bridge shutdown, bounded startup reconciliation, rollback/no false notification, significant activity and exact-deadline join/send races passed with the race detector; lint passed. |
| T6.3 | completed | `feat(retention): prune confirmed Iris history` | `8c5019f0b7aa` | Preview non-mutation, exact/frozen selections, stale revisions/time/policy/membership/leases, deletion rollback, one-use tokens, bounds, scope, noninteractive IPC and retained allocations passed with race checks; terminal event acknowledgment regression fixed and lint passed. |
| T6.4 | completed | `test(lifecycle): verify Iris retention and conversation isolation` | `a11af121a744` | Actual daemon/CLI expiration without an agent, readiness reconciliation, retained history, policy reload/stale token, confirmed pruning and fresh IDs/context passed with the race detector; clock rollback and all terminal routes passed; combined-tag lint passed. |

## Slice 7 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T7.1 | completed | `feat(history): inspect Iris conversations` | `f2430a906c3e` | Participant/default/all-project scope, retained message pages, scoped cursors, unchanged consumers and real Unix CLI checks passed with the race detector. |
| T7.2 | completed | `feat(search): find Iris conversation history` | `dfc2b5479dda` | Combined literal text/pair/date/ordinal/ID filters, bounded matching excerpts, immutable aliases, equal-timestamp keysets and SQLite query-plan indexes passed with race checks. |
| T7.3 | completed | `feat(export): dump readable Iris transcripts` | `3af9bcf380c4` | Frozen metadata/sequence, 112-message text/JSON exports, encoded wire bounds, lease/prune races, expiration/restart, private spool cleanup, cancellation and zero partial stdout passed with race checks. |
| T7.4 | completed | `feat(cli): summarize Iris storage and conversations` | `bb351bbc0245` | Actual binary/jq checks passed for scoped discovery, text/JSON transcripts beyond 100 messages, restart, stderr separation and zero consumer/activity changes. State/body/retention/storage statistics match fixtures; human examples documented. |

## Slice 8 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T8.1 | completed | `feat(bridge): recover Iris worker progress` | `fdf1fa70cddd` | Real SQLite reopen, result checkpoint immutability, exclusive generation ownership, lost completion confirmation and no-model publication passed with race checks; exact stored turn recovery rejects foreign/incomplete work. |
| T8.2 | completed | `feat(bridge): report uncertain Iris worker outcomes` | `32a9b0816257` | Distinct failure/status hints, expired-lease uncertainty, three safe startup retries with 1/2/4-second delays, definitive rejection and durable approval-decline recovery checks passed; no turn invocation retries. |
| T8.3 | completed | `feat(bridge): enforce Iris execution limits` | `c6e7ce18fe48` | Persisted restart policy, pre-provider budgets, nullable telemetry with observed floor, expired-budget saved-result recovery, global owner cap, blocked-stdin deadlines and owned process cleanup passed with race checks. |
| T8.4 | completed | `test(security): verify Iris input and authority boundaries` | `1f1c2b352c1d` | Actual Unix malformed/oversized/UTF-8/scope rejection, literal peer injection, forged owner and private mode checks passed with race detection; bounded request/recovery fuzzing passed with two workers. |
| T8.5 | completed | `test(bridge): verify Iris recovery and idle behavior` | `24099e0d5d52` | All ten ordered Looper checks passed on candidate `e8e23503a43ca443dac802eb1b312894df003c34`: real lost-result recovery without another invocation, both original threads resumed with pending work, unchanged idle counts and measured CPU/RSS. Resume-event regression fixed in `e8e23503a43c`. |

## Slice 9 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T9.1 | completed | `feat(systemd): activate the Iris user service` | `de3bf4c2ca19` | Descriptor/runtime race checks and actual isolated user-manager activation passed. Unit restrictions verified on the running daemon; manager retains its socket across restart. |
| T9.2 | completed | `build(linux): install and remove Iris user components` | `f6a48af24743` | Three isolated installation cases passed: reproducible binary, atomic upgrade, actual unit validation, explicit activation, private units, rejected symlinks and preserved data/config. |
| T9.3 | completed | `docs(operation): document Iris backup and recovery` | `cabd4ac3b5a4` | Actual documented recipe rejects a live WAL copy; stopped snapshot integrity, restored original CLI IDs/messages and rollback of later writes passed with race checks. Daemon startup causes reach stderr; diagnostics/shutdown documented. |
| T9.4 | completed | `test(systemd): verify Iris user service operation` | `95ab6656dfee` | Added real isolated user-manager target. Race acceptance passed activation, upgrade/restart, crash restart, private modes, disconnect/cancellation, shutdown/reopen and idle observation; owned fixtures cleaned. |

## Slice 10 Tasks

| Task | Status | Expected Commit | Commit | Validation |
| --- | --- | --- | --- | --- |
| T10.1 | completed | `docs(guide): explain Iris project conversations` | `790b6f6007ca` | Published invitation/history snippets execute unchanged against the real binary with race checks; source installation and bounded worker forms match owning fixtures. Live fresh-context acceptance follows in T10.3. |
| T10.2 | completed | `docs(reference): document Iris contracts and future capabilities` | `c668448ca53c` | Contract checker passed forty documents, eighteen typed operation names, storage schema 4 and SQL constraints. Published configuration equals executable defaults; migration/model and reference tests passed. Planned capabilities and separate-worker limitations documented. |
| T10.3 | completed | `test(acceptance): verify the complete Iris v1 workflow` | `9b9d85c479f5` | Ten-group map validates 28 actual cases/selectors. Changed guide/lifecycle/reference race checks passed. Real Codex lost-result recovery, own-thread resumption and a new conversation with fresh threads/consumers/proof passed in 58.854 seconds; no idle echo. |
| T10.4 | completed | `chore(delivery): finalize Iris v1 validation` | `47a9163f33c7` | Exact-candidate automation retains strict lint/tests, contracts, acceptance mapping and installation checks. The controller's unchanged full gate passed on `5b951abbf401eb3226ab294cceb2b1089da99f5a`; plan/tracker/report closure records that evidence with only configured documentation changes afterward. |

## Validation evidence

| Evidence | State | Meaning |
| --- | --- | --- |
| Acta/Lore planning inspection | recorded | Source structure, config, protocol, systemd assets, test patterns, and documentation inspected at revisions recorded in the plan. T1.1 completed the eight-point comparison in the canonical report. |
| Planning Codex CLI/schema inspection | passed | CLI reports 0.161.0; default generated schema contains required thread/turn and approval contracts. No model was invoked. |
| Plan/tracker documentation and structural checks | passed | `make docs-check`; agreement of ten slice identities and 44 task/commit identities; all ten acceptance groups; relative links, branch syntax, whitespace, and context privacy checks. |
| Live Codex probe for Slice 1 | passed | Six bounded assertions verified separate owned workers, cwd, real results, resumption, denied approval, and interruption. This is feasibility evidence, not the future Iris exchange. |
| Contract gate for Slice 1 | passed | Looper validated the final PR head `06041b58bf5dd44c42202c5463ab202da511308a` with `make docs-check`, `python3 scripts/check-contracts.py`, then `git diff --check`: fourteen tables, eleven rejected invalid cases, rollback and allocation retention. The canonical PR merged at that same head. |
| Service/storage focused checks | passed | Executable/configuration, real driver/SQLite, private socket/protocol, cancellation, ownership, and real-binary graceful/crash recovery pass. All eight PR checks passed through Looper for final PR head `a42a4152540a53de5811802d0d67b7602663fc58`; the canonical PR merged at that same head. |
| Project handshake gate | passed | All eight ordered PR checks passed through Looper on final PR head `87e469db951c618513335c3d98644a58a81c54fc`. Canonical PR `#3` merged into `dev` at that same head; its report and tracker entries are closed. |
| Durable delivery gate | passed | Eight ordered controller checks passed on final PR head `2d3a02e33e450563ee7758df44ea765cb3f8200e`, including all integration-tag race checks and actual-process recovery. Canonical PR `#4` merged at the same head; report/tracker entries are closed. |
| Codex exchange gate | passed | Eight ordered Looper checks passed on final PR head `c0bb1ff0c0221a3c3d12e962529fbf42f333c563`, including the real two-project exchange, durable acknowledgment, idle counts and integration race checks. Canonical PR `#5` merged at that head; report/tracker entries are closed. |
| Lifecycle and retention gate | passed | All seven ordered Looper checks passed on final PR head `f802b42de7d94253aa3545ae382f9d6c74dc312b`, including actual daemon/CLI expiration, retention/pruning, fresh context and all integration-tag race checks. Canonical PR `#6` merged at that head; report/tracker entries are closed. |
| History and export gate | passed | All seven ordered controller checks passed on final PR head `cb35ff0ce59feb94db6d453626d328eeb875b7e3`, including actual binaries, complete exports, jq and integration race checks. Canonical PR `#7` merged at that head; report/tracker entries are closed. |
| Bridge recovery gate | passed | Ten ordered controller checks passed on final PR head `de6aec26826b250356a584f413c249aacf3de52e`, including real recovery/restart, two bounded fuzz campaigns, integration race checks and combined-tag lint. Canonical PR `#8` merged at that head; report/tracker entries are closed. |
| User-systemd acceptance | passed | All nine ordered controller checks passed on final PR head `562059bc7f3605f784d5e038a24f0425ec1675b2`, including actual isolated units, installation, process race checks and combined-tag lint. Canonical PR `#9` merged at that head; report/tracker entries are closed. |
| Acceptance/documentation PR checks | passed | Ten ordered controller checks passed on final PR head `879bde689b196fe4c8ee74d2afdc1424354075a3`, including changed process race checks, real fresh-context Codex and combined-tag lint. Canonical PR `#10` merged at that head; report/tracker entries are closed. |
| Full-gate prerequisites | satisfied | Required tools, private writable runtime/Unix socket and runtime unit-link paths, reachable user manager and authenticated Codex 0.161.0 supported the successful gate. Process/systemd/live fixtures cleaned their own isolated roots and units. No acceptance unit remains running. |
| Integrated delivery gate | passed | The controller recorded `make verify && make docs-check && make test-integration && make test-systemd && make test-bridge && git diff --check` on immutable candidate `5b951abbf401eb3226ab294cceb2b1089da99f5a`. Process integration passed in 12.267 seconds, isolated user-systemd in 4.864 seconds and real Codex recovery/fresh-context exchange in 70.576 seconds. Later changes are confined to configured closure documentation. |

## Completion gates

- [x] Delivery set explicitly approved for execution through Looper.
- [x] Approved artifacts committed and canonical startup prerequisites verified.
- [x] Canonical `dev` base published for slice PR delivery.
- [x] Slice 1 delivered; detailed spec/model settled and SQLite dependency confirmed.
- [x] Slice 2 delivered; real CLI/daemon/SQLite integration passes.
- [x] Slice 3 delivered; identity, numbering, invitation, join, and close pass.
- [x] Slice 4 delivered; ordered durable messaging, wait, cursor, ack, and isolation pass.
- [x] Slice 5 delivered; real Codex exchange and zero-idle-call evidence recorded.
- [x] Slice 6 delivered; service expiration, retention, and confirmed pruning pass.
- [x] Slice 7 delivered; bounded discovery, complete exports, and stats pass.
- [x] Slice 8 delivered; bridge crash recovery, uncertainty, budgets, and security pass.
- [x] Slice 9 delivered; installation and real isolated user-systemd operation pass.
- [x] Slice 10 delivered; User Guide, references, future capabilities, and focused acceptance pass.
- [x] All ten canonical reports delivered and slice PRs merged into `dev`.
- [x] Full gate green for the recorded immutable integrated candidate.
- [x] Achieved Codex session mode and concrete product limitations documented.

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
