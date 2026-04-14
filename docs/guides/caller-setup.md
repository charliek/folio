# Caller Setup

Add documentation publishing to any repo by creating a GitHub Actions workflow that calls folio's reusable workflow.

## Prerequisites

- A GCS bucket configured for folio (see [GCP Setup](gcp-setup.md))
- Workload Identity Federation configured for GitHub Actions
- An mkdocs project with dependencies in `pyproject.toml`

## GitHub Configuration

Add these to your repo's Settings > Secrets and variables > Actions:

**Secrets:**

- `GCP_WIF_PROVIDER` — Workload Identity Federation provider path
- `GCP_SA_EMAIL` — CI service account email

**Variables:**

- `GCS_BUCKET` — Target GCS bucket name

## Workflow File

Create `.github/workflows/publish.yml` in your repo:

```yaml
name: Publish Docs
on:
  push:
    branches: [main]
    paths:
      - 'docs/**'
      - 'mkdocs.yml'
      - 'pyproject.toml'
  workflow_dispatch:

jobs:
  publish:
    uses: charliek/folio/.github/workflows/publish-docs.yml@main
    with:
      gcs-bucket: ${{ vars.GCS_BUCKET }}
    secrets:
      GCP_WORKLOAD_IDENTITY_PROVIDER: ${{ secrets.GCP_WIF_PROVIDER }}
      GCP_SERVICE_ACCOUNT: ${{ secrets.GCP_SA_EMAIL }}
```

## Project Setup

Your repo needs a `pyproject.toml` with mkdocs dependencies:

```toml
[project]
name = "my-project-docs"
version = "0.1.0"
requires-python = ">=3.13"

[dependency-groups]
docs = [
    "mkdocs>=1.6.1",
    "mkdocs-material>=9.6.14",
    "pymdown-extensions>=10.15",
]
```

And an `mkdocs.yml`:

```yaml
site_name: my-project
site_description: 'Short description for the repo index'
docs_dir: docs
site_dir: site-build
dev_addr: '127.0.0.1:7070'

theme:
  name: material
```

Key conventions:

- `site_dir: site-build` (not the default `site/`)
- `site_description` is used in `_meta.json` for the repo index
- `dev_addr` on port 7070

## Workflow Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `gcs-bucket` | Target GCS bucket | (required) |
| `repo-name` | Path under `/repos/` | Repository name |
| `docs-dir` | Directory containing `mkdocs.yml` | `.` |
| `python-version` | Python version for uv | `3.13` |
| `docs-group` | uv dependency group | `docs` |
| `site-dir` | Build output directory | `site-build` |
| `description` | Override for `_meta.json` description | `site_description` from `mkdocs.yml` |

## What the Workflow Publishes

The reusable workflow:

1. Builds the mkdocs site
2. Writes `_meta.json` with repo metadata (name, description, timestamp, GitHub link)
3. Syncs the built site to `gs://{bucket}/repos/{repo-name}/`

The `_meta.json` file is read by the folio server's `/_api/repos` endpoint to populate the repo index.

## Starter Template

See `examples/caller-workflow.yml` in the folio repo for a complete example.
