# Project identities

A project has a stable 32-character lowercase hexadecimal ID, a unique readable
alias and an optional canonical directory. `projects register [directory]`
resolves symlinks and registers an existing directory. `--name` supplies an
explicit alias; otherwise a user-authored `.iris.toml` with one `name = "alias"`
entry overrides the directory basename. Iris never creates project-local files.

Aliases contain 1–64 lowercase ASCII letters/digits and single hyphens, with
letters/digits at both ends. Derived names lowercase the basename, collapse
other runs to hyphens and trim edges. Empty/overlong results and collisions
fail. A reserved invitation recipient can exist before it has a path.

Project-facing commands infer the nearest registered canonical ancestor of cwd.
Explicit `--project` accepts an ID, canonical path or unambiguous alias and
disables cwd inference. Only `call`, `join` and explicit registration may register
an unbound cwd. An unregistered nested call registers that exact directory;
register the intended root first. Administrative history/stats/prune commands
can use explicit `--all-projects` without cwd.

```sh
iris projects register --name alpha /path/to/alpha
iris projects list --limit 50 --json
iris projects rename --name alpha-next PROJECT_ID
iris projects relocate --path /path/to/moved-alpha PROJECT_ID
```

Register/rename/relocate accept `--json`. List has bounded `--limit` (1–100) and
`--cursor`, returning `next_cursor`/`truncated`. Rename and deliberate relocation
preserve identity and history labels; they never merge identities or silently
associate a moved repository. Concurrent aliases and paths are unique in SQLite.

`call --to SELECTOR --topic TEXT` accepts registered IDs/paths/aliases or reserves
a valid unknown alias. Content comes from `--message`, `--file` or stdin. Topic
is nonempty UTF-8 up to 256 bytes; body is nonempty UTF-8 up to 256 KiB and must
fit the encoded wire limit. An optional idempotency key is scoped to the caller.
Identical retries preserve the conversation; changed content conflicts.

`join CODE` resolves a three-word invitation and returns its original initial
message. It binds only the intended recipient before the fixed deadline and
activates exactly once. Either joined participant can `close ID` idempotently;
closure retains history and cannot reopen the invitation. Conversation IDs and
unordered-pair ordinals never reuse deleted history. Readable labels freeze
participant aliases at creation. See [delivery](delivery.md) and
[lifecycle](lifecycle.md) for subsequent processing and retention.
