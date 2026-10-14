# Slice 3: Projects and Handshake

Status: drafting
Delivery set: iris-v1
Plan: [Iris v1 plan](../../../plan/iris-v1.md)
Tracker: [Iris v1 tracker](../../../tracker/iris-v1.md)
Branch: `feat/slice-3-iris-projects-handshake`
PR: pending

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

## Validation

- `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run 'TestProject|TestStats' -count=1` — passed.
- `GOTOOLCHAIN=go1.26.3 golangci-lint run --fix ./...` — passed.

- `GOTOOLCHAIN=go1.26.3 go test ./internal/iris -run 'TestProject' -count=1` — passed for collision, rename and relocation behavior.

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestCall|TestInvitation' -count=1` — passed.

- `GOTOOLCHAIN=go1.26.3 go test -race ./internal/iris -run 'TestCall|TestJoin|TestClose' -count=1` — passed.

Final controller-recorded exact-HEAD checks follow the remaining slice tasks.

## Risks and Follow-ups

Actual-binary handshake acceptance remains in T3.5. Configurable lifecycle
policy and autonomous timer reconciliation remain assigned to Slice 6.
