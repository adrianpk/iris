# Iris

Iris is the third project alongside Acta and Lore. Product behavior will be
defined by the project specification.

This workspace contains the initial Go module and development tooling. It
does not contain application packages yet.

## Workspace

- `go.mod`: independent Go module, using the same Go version as Acta and Lore.
- `Makefile`, `.golangci.yml`, and `scripts/`: shared development checks adapted
  for Iris.
- `internal/iris/`: application packages when implementation starts.
- `assets/`: product assets when needed.
- `docs/`: tutorials, how-to guides, reference, and explanation.
- `ops/default/`: specifications and development operations.

Place the initial specification under `ops/default/spec/drafts/`. Accepted
specifications belong directly under `ops/default/spec/`.

## Development

Requirements: Go 1.26.3 or later, Make, Git, and golangci-lint v2 built with a
compatible Go version.

Check workspace documentation:

```sh
make docs-check
```

After Go packages are added, use the same quality commands as Acta and Lore:

```sh
make check
make lint
make verify
```

See the [documentation index](docs/index.md).
