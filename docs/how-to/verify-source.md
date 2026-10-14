# Verify a source candidate

Use a clean source checkout on the candidate commit, Go 1.26.3, Make, Git,
Python 3, jq and golangci-lint v2 built for the selected Go version. Real
user-systemd acceptance requires a reachable user manager and private runtime
root. Live bridge acceptance requires the verified authenticated Codex CLI and
available provider capacity. Tests create private projects/data and unique
runtime units; ordinary stores and units are not fixtures.

Capture the full candidate identity and refuse dirty inputs:

```sh
candidate="$(git rev-parse HEAD)"
test -z "$(git status --porcelain)"
```

For a full delivery candidate, run the required gate once without changing HEAD:

```sh
make verify && make docs-check && make test-integration && make test-systemd && make test-bridge && git diff --check
```

Check that the tested source remained unchanged:

```sh
test "$candidate" = "$(git rev-parse HEAD)"
test -z "$(git status --porcelain)"
```

Retain the full hash, exact ordered command list, exit status and check output
in the validation evidence. Evidence belongs only to that clean commit. If code
or documentation changes afterward, validate the new candidate rather than
attributing old evidence to it. Do not describe the coverage-map checker or a
selected test run as the complete gate.

`make verify` retains strict lint and ordinary Go tests and also checks document
links, delivery identities, public operation/schema contracts, the ten-group
acceptance map and source installation/removal. The other targets cover actual
processes, user-systemd operation and real Codex execution separately. See the
[acceptance reference](../reference/acceptance.md) for decisive assertions and
[the live example](../reference/acceptance.md#reproduce-the-live-exchange).

Only tests' owned workers, process groups, sockets, temporary roots and units
are stopped after their checks. Missing provider or user-manager prerequisites
fail explicitly. Preserve the failing command and cause; do not substitute fake
results or weaken the gate. Keep credentials, prompts and transcript contents
out of reports. A green source gate is validation evidence, not a version tag,
release or publication.
