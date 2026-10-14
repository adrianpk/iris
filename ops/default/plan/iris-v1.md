# Iris v1 Implementation Plan

Status: approved

Approved: maintainer, 2026-10-14; execute this delivery set with Looper.

Full gate: `make verify && make docs-check && make test-integration && make test-systemd && make test-bridge && git diff --check`

Delivery set: `iris-v1`

Slice strategy: behavior-first

Reason: establish the required contracts first, then deliver working local
workflows. Demonstrate real Codex communication in Slice 5, before completing
history management and operational hardening, so agent integration cannot
become an unverified final addition.

Spec: [Iris specification](../spec/iris.md)

Model: [Iris data model](../spec/iris-model.md)

Tracker: [Iris v1 tracker](../tracker/iris-v1.md)

Base branch: `dev`

Active slice: Slice 8, reviewing; bridge recovery. The first seven slices are integrated.

Execution gate: the user's 2026-10-14 instruction to use Looper authorizes
execution of this delivery set and its controller-owned integration into `dev`.
Start after committing/publishing these artifacts and passing preflight.

## Objective

Deliver independent, local project-to-project conversations with SQLite
persistence, a private Unix socket service, a human and machine CLI, and an
on-demand Codex bridge. Two temporary projects must exchange a question,
response, and acknowledgment without a human relaying messages.

Closed and expired histories remain available for explicit inspection and
export during retention. They never become automatic context for a new
conversation.

## Verified planning baseline

The initial workspace is committed on `main` as `ea16e3a` with the exact
subject `Initial commit`. Development continues on `dev` using Conventional
Commits. `origin` is configured. Publish the initial `main` and the approved
planning baseline on `dev` before starting the authorized controller.

Reference inspection used local Acta revision
`7cf66a0188ae187a75cc239059d30b814f510331` and Lore revision
`fd9a511fea397788c3ab9c276a70b520463246e5`. Neither checkout was changed.

| Concern | Observed pattern | Iris planning consequence |
| --- | --- | --- |
| Structure and dependencies | Both have a thin root `main.go`, an owning `internal/<product>` package, Go 1.26.3, and no external module requirements. | Start with the same structure; introduce only the dependency needed for SQLite after selecting and confirming it. |
| CLI | `App.Run` dispatches commands; standard-library flags and injected streams support testing. Errors reach stderr and the entrypoint exits nonzero. | Preserve those conventions while delivering Iris's required root help, project selection, and stable JSON output. |
| Local protocol | Both use versioned JSON request/result/error envelopes over Unix sockets. Acta allows 64 KiB requests; Lore allows 1 MiB; both use bounded responses and a 10-second connection timeout. | Define Iris limits explicitly. A receive/wait operation needs cancellation and a deadline appropriate to waiting rather than a copied short request timeout. |
| Data paths | Acta's inspected configuration uses `~/.acta`; Lore supports `XDG_DATA_HOME` with a `~/.local/share/lore` fallback. Both require `XDG_RUNTIME_DIR` for their socket. | Follow the XDG data pattern for `iris/iris.db`, with separate runtime and configuration paths. Do not inherit Acta's storage path or either JSONL format. |
| systemd | User socket activation, inherited-listener support, socket mode 0600, directory mode 0700, and service `UMask=0077`. | Provide equivalent private Iris service/socket operation. Review daemon restrictions separately from the bridge's worker needs. |
| Tests | Package-local Go tests include actual socket/service and CLI round trips, temporary data, concurrency, and systemd asset checks. | Keep fast deterministic coverage and add separate real process, user-systemd, and live Codex acceptance checks. |
| Documentation and delivery | README, Diataxis documentation, user guides, package READMEs, and shared global agent rules. No root license, CONTRIBUTING file, or workflow tree was present in the inspected checkouts. | Follow the documentation structure. Do not assume a license, CI pipeline, release channel, or verification timer that was not established for Iris. |

The installed CLI reports `codex-cli 0.161.0`. Its locally generated default
app-server schema exposes `thread/start`, `thread/resume`, `thread/read`,
`turn/start`, and `turn/interrupt`, with working-directory, sandbox, and approval
fields. The schema also includes command and file-change approval requests.
This verifies interface availability, not successful agent execution.

Official documentation describes a stdio app-server client, thread creation
and resumption, explicit turn execution, and server-initiated approval requests.
Use that interface as the primary bridge candidate and verify the installed
behavior in Slice 1. [Codex App Server](https://learn.chatgpt.com/docs/app-server).

`codex exec` is a documented noninteractive alternative. Keep it as a candidate
only if it preserves the required permissions and interaction model; do not
implement two adapters by default. [Non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode).

No model, worker, service, or provider-backed acceptance test was started during
planning. Resuming somebody else's existing interactive session remains
unverified. A separate worker in the same project is an allowed product mode
and must be described accurately.

## Entry conditions and delivery rules

1. The user approved execution through Looper on 2026-10-14. Commit these
   approved artifacts on `dev` before creating an implementation branch.
2. Establish the canonical forge's `dev` base before branch-and-PR delivery.
   The configured remote was empty when inspected. Publish the bootstrap and
   planning baseline, then let Looper manage the configured slice PRs.
3. Use the exact branch, PR title, and report path in the slice map. Each slice
   depends on the previous slice being merged into `dev`.
4. Resolve and record the detailed contracts in Slice 1 before runtime code in
   Slice 2. Promote the specification and model together when their decisions
   are sufficiently settled; update these document links at that point.
5. Select and confirm the SQLite dependency before adding it. Preserve runtime
   independence from Acta and Lore.
6. Keep each slice buildable and its required checks passing. Prefer one
   Conventional Commit per task and one PR per slice. Looper owns the delegated
   Rebase + Fast-forward merges into `dev`; workers must not submit merges.
7. Update the user-facing changelog when a slice delivers usable behavior, and
   update relevant documentation and the canonical slice report. Do not
   describe preparatory layers as a completed product.

This plan covers v1 product implementation, not permission to publish a tagged
release or install services into the user's ordinary environment. Operational
acceptance uses explicitly isolated fixtures. Looper execution is authorized
for the ten configured slices. Do not add slices, promote `main`, or extend
final-gate repair authority implicitly.

Before the first Looper checkpoint, pin the exact full gate from this plan and
the per-slice validation prefix. Slice 1 requires `make docs-check`. Slices
2 through 10 require `make check`, `make lint`, and `make docs-check`; Slice 5
and Slice 8 also require `make test-bridge`, and Slice 9 additionally requires
`make test-systemd`. Focused checks follow these required commands in PR
validation evidence.

Allow closure changes only to the plan, tracker, ten configured reports, and
the specification/model pair. Include both current draft paths and their
accepted destinations, `ops/default/spec/iris.md` and
`ops/default/spec/iris-model.md`, before freezing the controller configuration.

## Architecture and decisions to settle first

- Keep daemon responsibilities limited to project identity, conversations,
  persistence, notification, delivery, and audit history. Model execution
  belongs to a separate bridge process explicitly bound to a conversation and
  project.
- Keep CLI, service, persistence, lifecycle, and Codex adapter responsibilities
  visible within the familiar Go layout. Add packages only for demonstrated
  ownership boundaries.
- Use SQLite transactions for pair ordinals, message sequences, joins, lifecycle
  changes, duplicate handling, and durable processing completion. Retain pair
  allocation state after history pruning.
- Preserve historical participant labels independently of current aliases.
  Define nested-directory discovery, path reassociation, alias collisions, and
  unresolved invitation-recipient matching.
- Define versioned protocol/output envelopes, stable errors, message kinds,
  idempotency scope, consumer identity, cursor/ack semantics, and bounded wait
  behavior before their implementations diverge.
- Keep durable delivery checkpoints separate from ephemeral worker bindings.
  After an uncertain worker outcome, surface recovery state rather than
  silently replaying non-idempotent work or claiming exactly-once execution.
- Set configurable defaults for message/request sizes, result pagination,
  waits, retries, worker time, automatic turns, and usage limits. Preserve the
  supplied 2-hour invitation, 24-hour inactivity, and 30-day retention defaults.
- Define significant activity, export-versus-retention behavior, and the
  pruning preview/confirmation contract.
- Use a fresh worker context per conversation unless an explicit, verified
  binding provides equivalent isolation. Do not fork or import an unrelated
  interactive transcript automatically.
- Expose approval requirements to the human and preserve Codex's sandbox and
  policy. No received message can grant authority or trigger execution in the
  daemon.

## Slice map

| Slice | Short Name | Branch | Expected PR title | Report | Prerequisites |
| --- | --- | --- | --- | --- | --- |
| Slice 1 | audit and contracts | `docs/slice-1-iris-audit-contracts` | `docs(slice-1): define Iris implementation contracts` | `ops/default/report/slices/iris-v1/slice-1-audit-contracts.md` | Execution approved; plan/tracker committed; forge base established |
| Slice 2 | service and storage | `feat/slice-2-iris-service-storage` | `feat(slice-2): establish Iris service and SQLite storage` | `ops/default/report/slices/iris-v1/slice-2-service-storage.md` | Slice 1 merged; SQLite dependency confirmed |
| Slice 3 | projects and handshake | `feat/slice-3-iris-projects-handshake` | `feat(slice-3): connect projects through Iris invitations` | `ops/default/report/slices/iris-v1/slice-3-projects-handshake.md` | Slice 2 merged |
| Slice 4 | durable delivery | `feat/slice-4-iris-durable-delivery` | `feat(slice-4): deliver ordered Iris messages` | `ops/default/report/slices/iris-v1/slice-4-durable-delivery.md` | Slice 3 merged |
| Slice 5 | Codex exchange | `feat/slice-5-iris-codex-exchange` | `feat(slice-5): enable autonomous Codex conversations` | `ops/default/report/slices/iris-v1/slice-5-codex-exchange.md` | Slice 4 merged; live Codex validation available |
| Slice 6 | lifecycle and retention | `feat/slice-6-iris-lifecycle-retention` | `feat(slice-6): manage Iris expiration and retention` | `ops/default/report/slices/iris-v1/slice-6-lifecycle-retention.md` | Slice 5 merged with real exchange evidence |
| Slice 7 | history and export | `feat/slice-7-iris-history-export` | `feat(slice-7): inspect and export Iris conversations` | `ops/default/report/slices/iris-v1/slice-7-history-export.md` | Slice 6 merged |
| Slice 8 | bridge recovery | `feat/slice-8-iris-bridge-recovery` | `feat(slice-8): recover and bound Iris bridge execution` | `ops/default/report/slices/iris-v1/slice-8-bridge-recovery.md` | Slice 7 merged |
| Slice 9 | Linux operation | `feat/slice-9-iris-linux-operation` | `feat(slice-9): operate Iris through user systemd` | `ops/default/report/slices/iris-v1/slice-9-linux-operation.md` | Slice 8 merged; isolated user-systemd validation available |
| Slice 10 | acceptance and documentation | `test/slice-10-iris-acceptance-documentation` | `test(slice-10): verify Iris v1 workflows and documentation` | `ops/default/report/slices/iris-v1/slice-10-acceptance-documentation.md` | Slice 9 merged; all product acceptance environments available |

## Slice 1: Audit and Contracts

Deliver a reviewed implementation baseline and a bounded feasibility result for
the installed Codex interface. No Iris runtime capability is claimed here.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T1.1 | Complete the eight-point Acta/Lore comparison from spec section 2 and record adopted patterns and deliberate differences. | `docs(architecture): record Iris family patterns` | Evidence references actual source, tests, install instructions, and missing policies rather than assumptions. |
| T1.2 | Probe the selected Codex interface in temporary projects: cwd, fresh thread, explicit turn, own-thread resumption, final result, approval behavior, and cancellation. Record separate-worker versus existing-session support. | `docs(bridge): verify Codex integration contract` | Bounded real execution and installed-schema checks; concrete failures or unsupported session modes recorded. |
| T1.3 | Finalize SQLite/schema/migration choices, project discovery, alias/invitation rules, lifecycle, cursors, idempotency, bridge completion, limits, and error contracts in the spec/model pair. | `docs(spec): finalize Iris runtime and data contracts` | Field-to-storage/query mapping and concurrency invariants agree; SQLite dependency confirmed before addition. |
| T1.4 | Define acceptance fixtures and validation commands, settle the baseline, and promote spec/model together when ready. | `docs(plan): establish Iris acceptance baseline` | Documentation/link checks; every mandatory spec test has an owning slice and a clear pass condition. |

Done when: the chosen Codex mode has evidence, material design questions have
owners and decisions, and runtime implementation can proceed without inventing
an API or incompatible storage contract. If agent invocation cannot work, record
the specific blocker and revise scope with the user before dependent execution.

## Slice 2: Service and Storage

Deliver an executable Iris daemon and CLI foundation using a migrated SQLite
database through the private local service boundary.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T2.1 | Add thin executable assembly, command dispatch, root help, XDG config/runtime/data resolution, and orderly shutdown. | `feat(core): establish Iris executable and configuration` | Config/path validation, stderr/exit behavior, help, and cancellation tests. |
| T2.2 | Add the selected SQLite driver, versioned migrations, foreign keys, indexes, transaction ownership, and backup/reopen primitives. | `feat(store): establish Iris SQLite persistence` | Fresh database, upgrade, interrupted migration rollback, constraints, reopen, and concurrent access tests. |
| T2.3 | Implement versioned bounded Unix requests/responses, private permissions, client errors, and service readiness. | `feat(service): expose Iris local protocol` | Real socket round trips; malformed/version/size errors and disconnect/shutdown cases. |
| T2.4 | Establish temporary-process CLI/daemon integration fixtures and `make test-integration`. | `test(service): verify Iris daemon and database recovery` | Build actual binary, start temporary daemon, reconnect after restart, inspect persisted state, and clean up only owned fixtures. |

Done when: the CLI reaches the daemon, SQLite is the sole canonical store, and
process restart preserves committed data without exposing network listeners.

## Slice 3: Projects and Handshake

Deliver `projects`, `call`, `join`, and explicit `close` through the actual CLI
and daemon, including the initial message and conversation identity.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T3.1 | Register canonical project identities, derive aliases, read optional user-authored `.iris.toml`, and implement cwd/`--project` selection. | `feat(projects): register Iris project identities` | New/nested/symlink directories, explicit selector, absent override, and no automatic repository files. |
| T3.2 | Add alias changes, collision diagnostics, explicit path reassociation, and historical label preservation. | `feat(projects): manage Iris aliases and project moves` | Ambiguity rejects wrong targets; moves and renames preserve internal history references. |
| T3.3 | Transactionally create conversations, unordered-pair ordinals, invitations, expected recipients, and stored initial messages. | `feat(conversations): create Iris invitations` | Concurrent A-to-B/B-to-A calls, unique live codes, collision retry, and isolated simultaneous conversations. |
| T3.4 | Validate and accept joins, return topic/initial message, record join events, enforce terminal states, and support explicit close. | `feat(conversations): join and close Iris conversations` | Correct/wrong/unregistered recipient, expired invite, concurrent join, late send rejection, and no reopen. |
| T3.5 | Cover complete handshake and identity workflows through human and JSON CLI output. | `test(conversations): verify Iris project handshakes` | Real two-project CLI flow, `jq` parsing, stable IDs/labels, and initial-message visibility without implicit processing acknowledgment. |

Done when: a user can carry only the invitation code between projects and both
participants see the correct isolated conversation. Time-sensitive operations
already reject elapsed lifetimes; background maintenance arrives in Slice 6.

## Slice 4: Durable Delivery

Deliver durable `send`, cursor-based `receive`, efficient waiting, and explicit
consumer acknowledgment through the daemon and CLI.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T4.1 | Persist ordered messages with participant validation, versioned fields, reply/correlation references, and transactional idempotency handling. | `feat(messages): persist ordered Iris messages` | Sequence concurrency, duplicate retries, distinct payload conflict, and commit-before-confirmation. |
| T4.2 | Implement consumer identities, stable cursors, delivery state, `receive`, and `ack`; support durable result-plus-processing completion. | `feat(delivery): track Iris consumer processing` | Reads do not acknowledge; cursors recover after restart; result persistence and completion cannot lose a response. |
| T4.3 | Add bounded wait/long-poll notification with cancellation, reconnect, and no work when idle. | `feat(delivery): wait for Iris messages efficiently` | No lost wakeups around commit/subscribe, timeout/disconnect handling, and terminal-state wakeup. |
| T4.4 | Add text/file/stdin input and enforce limits consistently across CLI, protocol, and storage. | `feat(cli): send and receive Iris message content` | Long text, empty/invalid input, boundary sizes, and clean machine stdout. |
| T4.5 | Exercise delivery concurrency, restart, retry, and conversation/project isolation through real processes. | `test(delivery): verify Iris message recovery and isolation` | No lost committed messages, no cross-conversation context, independent consumer progress, and explicit delivery-semantics evidence. |

Done when: messages survive restart and consumers can wait and acknowledge
processing without invoking a model or polling on every ordinary agent turn.

## Slice 5: Codex Exchange

Deliver the first real autonomous conversation using explicitly activated,
conversation-scoped bridges. This is a mandatory product checkpoint.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T5.1 | Implement a separate bridge process and `bridge start/status/stop`, with project/conversation binding and exclusive worker ownership. | `feat(bridge): manage conversation-scoped Iris workers` | No implicit/global startup; correct cwd; visible state/errors; stopping an owned bridge does not disturb unrelated sessions. |
| T5.2 | Implement the verified Codex adapter, thread binding, sequential turns, final-result extraction, and isolated conversation context. | `feat(bridge): run Iris exchanges through Codex` | Installed protocol fixtures and real bounded turns; no unrelated transcript injection or same-session claims. |
| T5.3 | Preserve sandbox/approval policy and expose pending approval requests for explicit human decisions. | `feat(bridge): preserve Codex authority boundaries` | No automatic approval or policy escalation; denied/pending/failed work does not get acknowledged as successfully processed. |
| T5.4 | Persist responses before acknowledgment, enforce initial time/turn limits, and stop acknowledgment-only feedback loops. | `feat(bridge): complete bounded Iris exchanges` | Deterministic completion/idempotency cases; control events do not trigger endless reply turns. |
| T5.5 | Add `make test-bridge` and run the real two-temporary-project question/response/acknowledgment workflow. | `test(bridge): demonstrate autonomous Iris communication` | No human relaying between messages; actual Codex turns; idle interval has zero model invocations; transcript and execution evidence agree. |

Done when: a reproducible real A-to-B round trip passes. A stub or a manually
relayed exchange does not satisfy this slice. Authentication/capacity failures
remain explicit blockers; they must not be reported as a passing adapter.

## Slice 6: Lifecycle and Retention

Deliver service-owned expiration, retained history, and confirmed pruning.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T6.1 | Centralize the clock and configurable invitation/inactivity/retention policy and define significant activity. | `feat(lifecycle): define Iris expiration policy` | Controlled-clock boundary cases; reads, listings, waits, and maintenance do not renew inactivity. |
| T6.2 | Add daemon maintenance, restart reconciliation, active-inbox exclusion, and bridge notification on closure/expiration. | `feat(lifecycle): expire inactive Iris conversations` | Expiration without an agent, restart overdue-state repair, and send/join/expiry races. |
| T6.3 | Implement pruning preview, exact confirmation, noninteractive confirmation, and retention-eligible deletion. | `feat(retention): prune confirmed Iris history` | Preview is read-only; stale/mismatched confirmations reject; ineligible data survives; pair ordinals remain non-reusable. |
| T6.4 | Verify terminal histories and new conversations under clock changes, pruning, and restart. | `test(lifecycle): verify Iris retention and conversation isolation` | Closed/expired data remains inspectable until eligible deletion; new IDs and empty inherited context after every lifecycle route. |

Done when: the daemon manages expiration independently and deletion never
silently destroys active or retained conversations.

## Slice 7: History and Export

Deliver human recovery and bounded discovery without returning historical
messages to active agent context.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T7.1 | Add project-visible conversations/show and explicit user-wide administrative selection. | `feat(history): inspect Iris conversations` | Default project isolation, outside-repository administration, state filters, and unambiguous selectors. |
| T7.2 | Add indexed bounded search by text, project pair, date, ordinal, and internal ID. | `feat(search): find Iris conversation history` | Combined filters, deterministic order/pagination, alias-change history, and index/query checks. |
| T7.3 | Add complete text/JSON transcript exports with lifecycle and participant metadata. | `feat(export): dump readable Iris transcripts` | Chronological readable text, semantic ordered JSON, complete history beyond one response page, and no active-context side effects. |
| T7.4 | Add stats and verify human/JSON command consistency with documented examples. | `feat(cli): summarize Iris storage and conversations` | `jq` parses all machine outputs; counts/retention/storage values match fixtures; stderr remains separate. |

Done when: a user can find a historical pair ordinal, read the exchange, and
export it for deliberate reuse without changing inbox or processing state.

## Slice 8: Bridge Recovery

Deliver bounded recovery and failure handling around the already functional
bridge, with explicit limits on what can be replayed safely.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T8.1 | Add durable turn/result checkpoints, binding recovery, and single-consumer ownership after adapter restart. | `feat(bridge): recover Iris worker progress` | Crash before/after invocation, result storage, and acknowledgment; restart never loses a committed result. |
| T8.2 | Expose uncertain outcomes and bounded provider failure/retry states; require deliberate resolution where replay could repeat side effects. | `feat(bridge): report uncertain Iris worker outcomes` | Failed/interrupted/capacity errors are not success; no automatic unsafe replay or exactly-once claim. |
| T8.3 | Enforce configured time/turn/usage limits, terminal-state cancellation, and resource cleanup. | `feat(bridge): enforce Iris execution limits` | Limit boundaries, absent usage telemetry, orderly owned-worker stop, and no orphaned bridge resources. |
| T8.4 | Harden untrusted message boundaries, protocol parsing, permissions, and project isolation. | `test(security): verify Iris input and authority boundaries` | Malformed/oversized input, injection attempts, unauthorized conversation access, and no daemon command execution; focused parser fuzz/race checks. |
| T8.5 | Extend real bridge acceptance for restart and idle behavior; measure idle resource use and document recovery limits. | `test(bridge): verify Iris recovery and idle behavior` | Two real workers reconnect; pending work is recoverable; idle model calls remain zero; measured evidence states its limits. |

Done when: recovery preserves messages/results, ambiguous external work is
visible, and configured budgets prevent uncontrolled exchanges.

## Slice 9: Linux Operation

Deliver source installation, user socket activation, reliable shutdown, and
documented backup and removal procedures.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T9.1 | Add user service/socket assets and inherited-descriptor activation with correct permissions and daemon hardening. | `feat(systemd): activate the Iris user service` | Unit validation and real socket activation; inherited socket validation; daemon restrictions do not accidentally grant or obstruct bridge authority. |
| T9.2 | Add reproducible build/install/uninstall procedures and separate optional bridge activation. | `build(linux): install and remove Iris user components` | Temporary installation paths, upgrade/restart, deliberate activation, and removal that preserves data unless explicitly selected. |
| T9.3 | Add SQLite backup/restore guidance and verification, diagnostics, and orderly operation. | `docs(operation): document Iris backup and recovery` | Consistent backup including the selected SQLite journal mode; restore/reopen; actionable logs and shutdown behavior. |
| T9.4 | Add `make test-systemd` and verify actual user-manager operation in isolated units and data/runtime roots. | `test(systemd): verify Iris user service operation` | Start-on-request, restart, disconnect, permissions, shutdown, and no mutation of Acta/Lore units or ordinary user stores. |

Done when: real systemd operation is verified, rather than inferred only from
unit-file text, and the daemon remains inexpensive without active bridges.

## Slice 10: Acceptance and Documentation

Complete the product guide and verify the integrated v1 scope against one
immutable candidate.

| Task | Work | Expected Commit | Validation expectation |
| --- | --- | --- | --- |
| T10.1 | Complete README and `docs/tutorials/user-guide/` with installation, handshake, bridge activation, examples, and troubleshooting. | `docs(guide): explain Iris project conversations` | Follow the guide with two temporary projects; all documented commands reflect implemented behavior. |
| T10.2 | Complete CLI/protocol/schema/lifecycle/trust/recovery reference and package documentation; preserve Planned capabilities for forum, broadcasts, claims, and other adapters. | `docs(reference): document Iris contracts and future capabilities` | Cross-reference checks, schema-to-runtime consistency, no implied Acta/Lore dependency, and accurate session limitations. |
| T10.3 | Complete the acceptance matrix for all ten mandatory test groups and the reproducible live example. | `test(acceptance): verify the complete Iris v1 workflow` | Real CLI/daemon/SQLite/systemd/Codex evidence, controlled clocks, transcript export, prune, and fresh-conversation isolation. |
| T10.4 | Finalize delivered-behavior metadata, validation automation, and reports; freeze the integrated candidate and record its full gate. | `chore(delivery): finalize Iris v1 validation` | Exact tested SHA, all required commands green, limitations recorded, and no release/tag/publication claim. |

Done when: the final gate passes for the recorded candidate and the user can
reproduce the governing workflow without acting as a technical message relay.

## Validation policy

Planning validation is documentation-only: `make docs-check`, Markdown-link
and plan/tracker consistency checks, and `git diff --check`. The installed
Codex schema check is capability evidence, not a live acceptance pass.

During implementation, run focused tests for each task and the smallest
regression set covering its changed boundaries. Socket and database tests must
exercise the real boundaries. Use controlled clocks and synchronization rather
than long sleeps. Do not add placeholder or deliberately failing tests merely
to fill a slice.

| Gate | Commands or evidence | Availability and use |
| --- | --- | --- |
| Documentation | `make docs-check`; relative-link checks; `git diff --check` | Available now; required for each documentation change. |
| Changed Go behavior | Focused `go test` on implemented packages/cases; appropriate formatting/static checks | Starts in Slice 2. Record exact commands and test results; an empty test selection is not evidence. |
| Process integration | `make test-integration` | Implemented in T2.4; actual CLI/daemon, private socket, SQLite, two temporary projects, clean/crashed restart, and no provider calls. |
| Concurrency | Targeted `go test -race`; bounded parser fuzzing where relevant | Apply to transaction/wait/bridge ownership changes, then include necessary regressions. |
| Live bridge | `make test-bridge` | Added in T5.5; real Codex workers, bounded turns, no per-message human relay, and zero idle model calls. Fakes do not satisfy this gate. |
| User systemd | `make test-systemd` | Added in T9.4; real isolated user units. Unavailable user-manager access is a blocker, not a pass. |
| Final delivery | `make verify`; `make docs-check`; `make test-integration`; `make test-systemd`; `make test-bridge`; `git diff --check` | Run once for the immutable integrated candidate. Bind every result to its actual tested SHA. |

The process-integration target is implemented in Slice 2. The live bridge
and user-systemd targets remain planned interfaces until their owning slices. The current `make verify` covers lint and ordinary
Go tests; it is insufficient by itself to prove autonomous communication or
real systemd operation.

Use scheduled exact-commit validation if configured during delivery; otherwise
run the required gate in the authorized acceptance environment. Do not repeat
the full live gate for every task or slice. A failed candidate that needs code
changes uses `fix/iris-v1-validation` and a PR targeting `dev`, followed by
validation of the corrected candidate. Keep evidence for the tested commit
distinct from later bookkeeping commits.

## Acceptance traceability

| Spec requirement | Owning slices | Required final evidence |
| --- | --- | --- |
| Section 2: family audit | 1 | Eight-point observed-pattern report with adopted/different decisions. |
| Section 14.1: identity | 3 | Canonical paths, optional alias, collision, move, and explicit project selection. |
| Section 14.2: handshake | 3 | Correct/wrong/expired/concurrent joins and initial-message delivery. |
| Section 14.3: numbering | 3, 6 | Unordered-pair allocation under concurrency and non-reuse after pruning. |
| Section 14.4: messages | 4, 8 | Ordered durable delivery, idempotency, restart, processing acknowledgment, and recovery. |
| Section 14.5: isolation | 3, 4, 5, 8 | Project filtering and fresh conversation/worker context with no historical injection. |
| Section 14.6: expiration/retention | 6, 7 | Controlled-clock expiration, retained retrieval, and confirmed eligible deletion. |
| Section 14.7: output | 3, 4, 7, 10 | Valid JSON/`jq`, readable complete chronological transcripts, and stable contracts. |
| Section 14.8: operation | 2, 9 | Real socket permissions, process restart/shutdown, and user-systemd activation. |
| Section 14.9: bridge | 1, 5, 8, 10 | Installed-interface proof, real autonomous round trip, idle checks, and crash recovery. |
| Section 14.10: security | 2, 4, 5, 8, 9 | Invalid/oversized input, authority/approval boundaries, and local permission checks. |
| Sections 12 and 15: future scope/docs | 10 | User Guide and explicit Planned capabilities; extensible participants without speculative runtime features. |

## Acceptance fixtures and pass conditions

Every fixture uses two newly created test project roots, a fresh data/config
root, and a short private runtime/socket directory. Set child-process
`XDG_DATA_HOME`, `XDG_CONFIG_HOME`, and `XDG_RUNTIME_DIR` to fixture-owned
locations. Never point acceptance at the user's ordinary Iris data, source
projects, Acta/Lore storage, or another controller's processes. Track all
owned process IDs, listeners, app-server threads, temporary exports, and unit
names; stop only those resources after their checks finish. Record failures
and cleanup outcomes. No credential or transcript is written into a report.

| Group | Fixture and decisive pass condition | Owning validation |
| --- | --- | --- |
| 1. Identity | Temporary directories, symlink and nested cwd, duplicate basename, optional alias, and explicit move; stable ID survives rename/relocation and ambiguity never selects another project. | Slice 3 focused Go and process integration |
| 2. Handshake | Registered and provisional destinations; correct, wrong, repeated, concurrent, and clock-expired joins; exactly one activation returns the original initial message. | Slice 3 focused Go and process integration |
| 3. Numbering | Concurrent calls in both directions, two isolated conversations and a pruned predecessor; one monotonic unordered-pair sequence with rollback and no ordinal reuse. | Slices 3/6 focused race and process checks |
| 4. Messages | Real socket/database, keyed creation/send retries, ordered pages, two consumers, process restart before/after acknowledgment, and injected completion failure; no lost durable message or skipped acknowledgment. | Slices 4/8; `make test-integration` |
| 5. Isolation | Concurrent conversations and unrelated project; project queries reject cross-membership and each new conversation uses a fresh owned Codex thread with no earlier transcript. | Slices 3/4/5/8 focused and live bridge checks |
| 6. Lifecycle | Injected service clock across invitation/inactivity/retention boundaries; reads and export do not renew lifetime; preview mutation or export lease prevents deletion, and confirmed eligible prune retains allocation tombstones. | Slices 6/7 deterministic storage/process checks |
| 7. Output | Paged history with Unicode, long content and control characters; parse successful JSON with `jq`, preserve order/labels, export all snapshot messages, and fail without a successful partial dump. | Slices 4/7/10; document and process checks |
| 8. Operation | Real daemon/socket and an isolated `iris-acceptance-<random>` user-unit pair; verify descriptor activation, permissions, restart and orderly stop without changing ordinary installed units. | Slices 2/9; `make test-systemd` |
| 9. Bridge | Two real project workers exchange question, response and acknowledgment with no manual relay; count `turn/start` calls during idle wait; interrupt/restart only owned adapters at recovery checkpoints. | Slices 1/5/8/10; `make test-bridge` |
| 10. Security | Malformed/oversized frames, wrong project, denied approval, untrusted peer text, private file/socket modes and stale owner tokens; failures cannot grant authority or publish under another worker's ownership. | Slices 2/4/5/8/9 focused parser/race/process checks |

T2.4 implements `make test-integration`. T5.5 adds `make test-bridge` and
T9.4 adds `make test-systemd`; those two remain required future interfaces. Live provider tests require authenticated access and bounded turns.
The systemd fixture requires a working user manager and private, uniquely
named test units; unavailable access is an explicit blocker. The installed
Codex probe in Slice 1 is feasibility evidence only. It does not replace the
actual two-party exchange or zero-idle-call measurement owned by Slice 5.

Slice 1 final validation uses `make docs-check`, then
`python3 scripts/check-contracts.py`, then `git diff --check`, recorded through
Looper for the exact clean PR HEAD. The contract checker validates relative
links, ten slice identities, 44 task identities, the accepted spec/model
pair, and meaningful SQLite constraint/rollback/prune invariants. It is not
a substitute for implementation tests with the selected Go driver.

## Completion boundary

All 44 tasks and 10 slices must be delivered and merged into `dev`, with their
canonical reports and required acceptance evidence. A messaging-only daemon
does not complete this delivery set. If the installed environment cannot
support the bridge, report the achieved scope and verified limitation and
revisit the plan rather than silently removing the central requirement.

Forum/group channels, broadcasts, claims, other-agent adapters, and remote or
multiuser operation remain documented future capabilities. Release versions,
tags, publication, and global agent-policy activation require their own
authorization.
