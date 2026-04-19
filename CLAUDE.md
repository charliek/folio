# CLAUDE.md

Project context for AI assistants working on this codebase.

## Project Overview

Folio is a self-hosted private documentation platform. It provides a Go server that authenticates users and serves static documentation sites from Google Cloud Storage, plus reusable GitHub Actions workflows that build and publish documentation from any repository.

## Build & Test

```bash
make build          # Build server binary (server/folio)
make test           # Run all unit tests
make lint           # Run golangci-lint
make fmt            # Format code with gofmt
make tidy           # Run go mod tidy
make check          # Run lint + test
make coverage       # Tests with coverage report
make docs           # Build mkdocs site (strict mode)
make docs-serve     # Serve docs at http://127.0.0.1:7070
make docker-build   # Build container image
```

Tools are managed via [mise](https://mise.jdx.dev/) — run `mise install` to set up Go and golangci-lint.

## Project Structure

- `server/` — Go server (all Go code lives here)
  - `main.go` — Entry point, config parsing, route registration
  - `auth.go` — Password login, cookie validation, auth middleware
  - `proxy.go` — GCS proxy, path rewriting, repo discovery
  - `cache.go` — In-memory LRU/TTL cache
- `deploy/cloud-run-service.yaml` — Cloud Run service manifest (source of truth for prod env vars, secrets, SA, resources)
- `docs/` — MkDocs documentation site
- `examples/` — Starter configs for caller repos
- `infra/` — GCP provisioning scripts
- `.github/workflows/` — CI/CD pipelines

## Key Conventions

- **Go version**: 1.25+ (see `.mise.toml`)
- **Formatting**: `gofmt` — run `make fmt` before committing
- **Linting**: `golangci-lint` v2 — run `make lint`
- **Tests**: Table-driven tests with `t.Run()`. Place `_test.go` files alongside source. Use `testify/assert` and `testify/require`.
- **HTTP routing**: `net/http.ServeMux` with Go 1.22+ method routing. No web framework.
- **Logging**: `log/slog` (stdlib structured logging)
- **Configuration**: All via environment variables. No config files.
- **GCS abstraction**: `BucketReader` interface in `proxy.go` for testability. Tests use a mock implementation.

## Server Architecture

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/_health` | No | Health check |
| GET | `/_login` | No | Login page |
| POST | `/_login` | No | Login submit |
| GET | `/_api/repos` | Yes | List published repos |
| POST | `/_admin/cache/purge` | Yes | Clear cache |
| GET | `/*` | Yes | GCS proxy (catch-all) |

### Path Rewriting

- `/repos/*` passes through to GCS directly
- All other paths get `_root/` prepended
- Paths ending in `/` get `index.html` appended
- `path.Clean` normalizes before rewriting

### Auth Flow

1. Check `_session` cookie (HMAC-signed expiry timestamp)
2. Invalid/missing → redirect to `/_login?next={path}`
3. POST `/_login` with correct password → set cookie, redirect to `next`
4. Password compared with `subtle.ConstantTimeCompare`

## Documentation

Docs use MkDocs Material. See `mkdocs.yml` for style guidelines (top comment block). Key rules:
- Professional, direct tone
- Tables for config fields and API endpoints
- Code blocks with language hints
- Examples should be copy-pasteable
- One topic per page

```bash
make docs-serve     # Serve docs at http://127.0.0.1:7070
```
