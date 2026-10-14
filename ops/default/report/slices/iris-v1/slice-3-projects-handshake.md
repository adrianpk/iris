# Slice 3: Projects and Handshake

Status: reviewing
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-3-iris-projects-handshake`
PR: [#3](https://forge.adrianpk.com/adrianpk/iris/pulls/3)

## Purpose

Connect two project identities through a short invitation, with durable initial
content, independent conversation numbering, validated joins, and closure.

## Delivered Behavior

Projects register canonical directories and stable IDs, derive readable aliases
or read a user-authored `.iris.toml`, and expose bounded registry pages. Cwd
selection follows the nearest registered canonical ancestor. An explicit ID,
path, or unambiguous alias selects a project independently of cwd. Iris creates
no project-local runtime files. Statistics support project and explicit human
all-project scope. Explicit rename and relocation preserve project IDs, registration
times, and historical participant aliases. Alias/path collisions and ambiguous
selectors fail rather than selecting another project.

Call creation atomically allocates a globally unique conversation ID, an
unordered-pair ordinal, immutable participant aliases, the initial message,
and a three-word invitation. Unknown recipient aliases reserve stable IDs
until registration. Identical request-key retries return the original result;
changed content conflicts. Failed creation rolls back every allocation.

The CLI accepts initial text, a file, or stdin without altering the content.

Join validates the expected project before binding a provisional directory.
The first acceptance activates the conversation and appends one sequenced
system event; duplicate accepts return the same topic and initial message
without renewing activity or acknowledging consumer work. Either joined
participant can close. Terminal state and its one system event commit together.
Expired or closed invitations cannot activate a conversation.

The actual-binary fixture connects two temporary project contexts by carrying
only the invitation code. It verifies clean human/JSON output, independent
initial requests, explicit selection, alias/move identity preservation, late
join rejection, crash/restart persistence, and absence of generated project
files or consumer acknowledgment state. README examples and Unreleased notes
describe the usable invitation workflow.

## Implementation Notes

Filesystem discovery and optional-file validation happen before the write
transaction. The transaction rechecks identity and unique path/alias ownership.
Project JSON has schema version 1 and nullable provisional registration fields.

The fixed version-1 dictionary contains 256 distinct words. Three independent
cryptographic byte indexes select a code. Allocation tries at most 16 codes;
SHA-256 code tombstones and request tombstones remain after history removal.

The injected clock checks stored invitation expiry and active inactivity in
call retries, join, and close transactions. A due transition commits even
when the operation returns a lifecycle error. The final event has a reserved
message slot and uses an empty body if the stored-body budget is exhausted.

## Contracts Added or Changed

`projects.register`, `projects.list`, `projects.rename`, and `projects.relocate` operate over the versioned local
protocol. The optional alias file accepts exactly one lowercase `name`
assignment and comments, at most 4 KiB. Explicit selection disables cwd
inference. Registry pages expose a next cursor and truncation.

## Files of Interest

- `internal/iris/project*.go`: identity, discovery, aliases, bounded listing and CLI.
- `internal/iris/operations.go`: typed operation failures and project dispatch.
- `internal/iris/project_test.go`: canonical, nested, symlink, override and page checks.

## Task Commits

| Task | Commit |
| --- | --- |
| T3.1 | `acec8da3ec95` |
| T3.2 | `4ab7e92e56c6` |
| T3.3 | `c66c7fa45eb1` |
| T3.4 | `3151c6e7f69c` |
| T3.5 | `0857a91d0f42` |

## Validation

The controller recorded these successful commands, in this order, for clean
implementation head `0857a91d0f4232fa58fd9a19d4516f4f88058f28`:

- `make check`
- `make lint`
- `make docs-check`
- `make test-integration`
- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris`
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --build-tags=integration ./...`
- `python3 scripts/check-contracts.py`
- `git diff --check`

The process suite uses the actual binary, private XDG roots, two project
contexts, and `jq`. It covers code-only join, text/file/stdin input, encoded
request overflow, identity moves, separate conversations, graceful/crashed
restart, and zero consumer state. Unit/race checks cover concurrent allocation,
retry/closure/join races, code exhaustion rollback, provisional registration,
request tombstones, persisted expiry boundaries and terminal event budgets.
The contract checker confirms ten consecutive slices, 44 tasks, fourteen
tables, eleven negative constraints, rollback and allocation retention.
All fixture processes finish under their owned cleanup.

Readiness requires regenerated controller evidence for the clean PR head
after this PR-reference update. That evidence binds its exact SHA and this
ordered command list. The PR links the report's full introduction commit,
`acec8da3ec9521ef5f73b083b92d3bc0d4407b5b`.

## Risks and Follow-ups

Configurable lifecycle policy and autonomous timer reconciliation remain
assigned to Slice 6. Durable send/receive/ack and Codex execution remain in
their recorded slices; joining exposes the initial request without processing
it on behalf of a consumer.
