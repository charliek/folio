# folio

A self-hosted, private GitHub Pages for your personal repositories.

Folio lets you build documentation from any private repo with GitHub Actions, publish to Google Cloud Storage, and serve it through an authenticated Go server on Cloud Run that scales to zero.

## Key Features

- **Password-protected** — Simple login gate with signed cookies
- **GCS-backed** — Static sites stored in Google Cloud Storage
- **Multi-repo** — Serve docs from many repos under one domain
- **Near-zero cost** — Cloud Run scales to zero, GCS stores a few megabytes
- **Container image** — Pre-built at `ghcr.io/charliek/folio`, deploy anywhere
- **Reusable workflows** — GitHub Actions workflow builds and publishes mkdocs sites

## How It Works

```text
Private repos                        folio
┌──────────┐  ┌──────────┐    ┌──────────────────────┐
│ repo-a    │  │ repo-b    │    │ GCS Bucket            │
│ (mkdocs)  │  │ (mkdocs)  │    │  /_root/...           │
│           │  │           │    │  /repos/repo-a/...    │
│ publish ──┼──┼── publish ─┼──►│  /repos/repo-b/...    │
└───────────┘  └───────────┘    └──────────┬───────────┘
                                           │
                                           ▼
                                ┌──────────────────────┐
                                │ folio server           │
                                │ (Cloud Run)            │
                                │ docs.example.com       │
                                └──────────────────────┘
```

1. Each repo's GitHub Actions workflow builds mkdocs and uploads to GCS
2. The folio server authenticates users and proxies content from GCS
3. Path rewriting maps URLs to the correct GCS objects

## Quick Links

- [Quick Start](getting-started/quick-start.md) — Get running in minutes
- [Configuration](reference/configuration.md) — All environment variables
- [API Reference](reference/api.md) — Server endpoints
- [Caller Setup](guides/caller-setup.md) — Add docs publishing to your repos
