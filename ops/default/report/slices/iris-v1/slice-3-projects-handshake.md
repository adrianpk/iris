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
all-project scope.

## Implementation Notes

Filesystem discovery and optional-file validation happen before the write
transaction. The transaction rechecks identity and unique path/alias ownership.
Project JSON has schema version 1 and nullable provisional registration fields.

## Contracts Added or Changed

`projects.register` and `projects.list` operate over the versioned local
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

Final controller-recorded exact-HEAD checks follow the remaining slice tasks.

## Risks and Follow-ups

Alias/move operations, invitation allocation, join/close, and actual-binary
handshake acceptance remain in T3.2 through T3.5.
