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
