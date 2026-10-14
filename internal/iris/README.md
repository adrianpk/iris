# Iris application

This package owns command dispatch, configuration, local transport, and SQLite
persistence. The root executable assembles signal cancellation and streams.
Constructors perform no IO; setup and service readiness are explicit.

Tests use temporary paths and package-owned fixtures. Run focused checks with
`go test ./internal/iris`. The repository Makefile defines delivery checks.

Configuration resolves the standard XDG data, config, and runtime roots. The
optional `iris/config.json` accepts `schema_version` (1), `max_connections`
(1 through 128, default 128), and `request_timeout_seconds` (1 through 60,
default 10). Unknown fields, malformed JSON and relative XDG paths fail.
Root help performs no configuration or service IO.

The service uses one compact JSON request per Unix stream, protocol `iris/v1`,
and schema version 1. Requests end with a newline and write EOF; clients call
`CloseWrite` while retaining the read side. The daemon validates the entire
bounded frame before dispatch and closes the response stream after one result.
Requests are limited to 1 MiB and responses to 8 MiB. Peer disconnect, service
cancellation, deadlines, and connection saturation are bounded.

The daemon holds a private runtime flock before opening SQLite. A second
owner cannot replace its socket. Only a private, user-owned stale socket is
reclaimed. `iris stats --all-projects --json` confirms readiness after schema
migration and returns current project, conversation and message counts.
