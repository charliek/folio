# Development Setup

## Prerequisites

- [mise](https://mise.jdx.dev/) for tool version management
- [uv](https://docs.astral.sh/uv/) for Python/docs dependencies

## Install Tools

```bash
mise install
```

This installs Go 1.25 and golangci-lint 2.10.1.

## Build

```bash
make build
```

Produces `server/folio`.

## Test

```bash
make test          # Run all unit tests
make coverage      # Tests with coverage report
```

## Lint

```bash
make lint          # Run golangci-lint
make fmt           # Format code with gofmt
make check         # Run lint + test
```

## Documentation

```bash
make docs          # Build mkdocs site (strict mode)
make docs-serve    # Serve at http://127.0.0.1:7070
```

## Docker

```bash
make docker-build  # Build container image
```

Run locally:

```bash
docker run -p 8080:8080 \
  -e GCS_BUCKET=test \
  -e AUTH_MODE=none \
  folio
```

## All Make Targets

| Target | Description |
|--------|-------------|
| `build` | Build server binary |
| `test` | Run unit tests |
| `lint` | Run golangci-lint |
| `fmt` | Format code |
| `tidy` | Run go mod tidy |
| `clean` | Remove build artifacts |
| `check` | Run lint + test |
| `coverage` | Tests with coverage report |
| `docs` | Build mkdocs site |
| `docs-serve` | Serve docs locally |
| `docker-build` | Build container image |
