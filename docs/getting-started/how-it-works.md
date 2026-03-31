# How It Works

## Architecture

Folio has three components:

1. **Caller workflows** — GitHub Actions in your repos that build mkdocs sites and upload to GCS
2. **GCS bucket** — Stores all static site content, organized by repo
3. **Folio server** — Authenticates users and proxies content from GCS

## Request Flow

1. User visits `https://docs.example.com/repos/my-project/`
2. Server checks for a valid `_session` cookie
3. No valid cookie → redirect to `/_login`
4. User enters password → signed cookie set for 90 days
5. Server rewrites the URL path to a GCS object key
6. Server fetches the object from GCS (or cache) and serves it

## GCS Bucket Layout

```
your-bucket/
├── _root/                    # Root site content
│   ├── index.html
│   └── css/
└── repos/
    ├── my-project/
    │   ├── _meta.json        # Repo metadata
    │   ├── index.html
    │   └── ...
    └── another-repo/
        ├── _meta.json
        └── ...
```

## Path Rewriting

The server maps URL paths to GCS object keys:

| URL | GCS Key |
|-----|---------|
| `/` | `_root/index.html` |
| `/style.css` | `_root/style.css` |
| `/repos/my-project/` | `repos/my-project/index.html` |
| `/repos/my-project/setup/` | `repos/my-project/setup/index.html` |

Paths starting with `/repos/` pass through directly. All other paths get `_root/` prepended. See [Path Rewriting](../reference/path-rewriting.md) for details.

## Caching

The server maintains an in-memory LRU cache with configurable TTL and size limits. This reduces GCS round-trips and improves cold-start latency on Cloud Run.

Cache can be cleared via `POST /_admin/cache/purge`.

## Publish Workflow

Each repo will call folio's reusable GitHub Actions workflow (planned for Phase 3). The workflow will:

1. Checkout the repo
2. Install mkdocs via uv
3. Build the site (`uv run mkdocs build`)
4. Authenticate to GCP via Workload Identity Federation
5. Sync the built site to `gs://{bucket}/repos/{repo-name}/`
6. Write `_meta.json` with repo metadata

See [Caller Setup](../guides/caller-setup.md) for configuration.
