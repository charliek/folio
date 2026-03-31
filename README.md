# folio

A self-hosted, private GitHub Pages for your personal repositories.

Build documentation from any private repo with GitHub Actions, publish to Google Cloud Storage, and serve it through an authenticated Go server on Cloud Run that scales to zero.

## Features

- **Password-protected** documentation server with signed cookies
- **GCS-backed** static site serving with in-memory caching
- **Multi-repo** support under one domain with automatic repo discovery
- **Reusable GitHub Actions workflow** for building and publishing mkdocs sites
- **Near-zero cost** — Cloud Run scales to zero, GCS stores a few megabytes
- **Pre-built container image** at `ghcr.io/charliek/folio`

## Quick Start

```bash
# Pull and run with Docker
docker run -p 8080:8080 \
  -e GCS_BUCKET=your-bucket \
  -e LOGIN_PASSWORD=your-password \
  -e COOKIE_HMAC_KEY=$(openssl rand -hex 32) \
  -e COOKIE_SECURE=false \
  ghcr.io/charliek/folio:latest
```

Or deploy directly to Cloud Run:

```bash
gcloud run deploy folio-server \
  --image ghcr.io/charliek/folio:latest \
  --set-env-vars GCS_BUCKET=my-bucket \
  --set-secrets LOGIN_PASSWORD=folio-password:latest,COOKIE_HMAC_KEY=folio-hmac:latest \
  --min-instances 0 --max-instances 2 \
  --allow-unauthenticated
```

## Architecture

```
Private repos (GitHub Actions)
        │
        ▼ build & upload
┌──────────────────┐
│  GCS Bucket       │
│  /_root/...       │
│  /repos/repo-a/   │
│  /repos/repo-b/   │
└────────┬─────────┘
         │
         ▼ proxy
┌──────────────────┐     ┌──────────┐
│  folio server     │────▶│  Browser  │
│  (Cloud Run)      │◀────│  (user)   │
│  auth + cache     │     └──────────┘
└──────────────────┘
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `8080` |
| `GCS_BUCKET` | GCS bucket name | (required) |
| `AUTH_MODE` | `password` or `none` | `password` |
| `LOGIN_PASSWORD` | Login password | — |
| `LOGIN_PASSWORD_SECRET` | Secret Manager resource name | — |
| `COOKIE_HMAC_KEY` | HMAC signing key | — |
| `COOKIE_HMAC_SECRET` | Secret Manager resource name | — |
| `COOKIE_MAX_AGE` | Cookie expiry | `2160h` |
| `COOKIE_SECURE` | Set Secure flag on cookie | `true` |
| `CACHE_TTL` | Cache entry TTL | `5m` |
| `CACHE_MAX_MB` | Max cache size | `128` |
| `CACHE_MAX_OBJECT_MB` | Max cacheable object size | `10` |
| `ROOT_PREFIX` | GCS prefix for root site | `_root` |
| `REPOS_PREFIX` | GCS prefix for repo sites | `repos` |

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | No | Health check |
| GET | `/_login` | No | Login page |
| POST | `/_login` | No | Login submit |
| GET | `/_api/repos` | Yes | List published repos |
| POST | `/_admin/cache/purge` | Yes | Clear cache |
| GET | `/*` | Yes | Serve from GCS |

## Publishing Docs

Add a workflow to your repo that calls folio's reusable workflow:

```yaml
name: Publish Docs
on:
  push:
    branches: [main]

jobs:
  publish:
    uses: charliek/folio/.github/workflows/publish-docs.yml@main
    with:
      gcs-bucket: my-docs-bucket
    secrets:
      GCP_WORKLOAD_IDENTITY_PROVIDER: ${{ secrets.GCP_WIF_PROVIDER }}
      GCP_SERVICE_ACCOUNT: ${{ secrets.GCP_SA_EMAIL }}
```

See `examples/` for starter configs.

## Development

Prerequisites: [mise](https://mise.jdx.dev/) for Go and linter versions, [uv](https://docs.astral.sh/uv/) for docs.

```bash
mise install           # Install Go 1.24 + golangci-lint
make build             # Build server binary
make test              # Run unit tests
make lint              # Run linter
make check             # Run lint + test
make docs-serve        # Serve docs at http://127.0.0.1:7070
make docker-build      # Build container image
```

## Documentation

Full documentation: [charliek.github.io/folio](https://charliek.github.io/folio/)

## License

MIT
