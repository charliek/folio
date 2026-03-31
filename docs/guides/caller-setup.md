# Caller Setup

Add documentation publishing to any repo by creating a GitHub Actions workflow that calls folio's reusable workflow.

## Prerequisites

- A GCS bucket configured for folio (see [GCP Setup](gcp-setup.md))
- Workload Identity Federation configured for GitHub Actions
- An mkdocs project with dependencies in `pyproject.toml`

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

jobs:
  publish:
    uses: charliek/folio/.github/workflows/publish-docs.yml@main
    with:
      gcs-bucket: my-docs-bucket
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
requires-python = ">=3.12"

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
site_url: 'https://docs.example.com/repos/my-project/'
docs_dir: docs
site_dir: site-build
dev_addr: '127.0.0.1:7070'

theme:
  name: material
```

Key conventions:

- `site_dir: site-build` (not the default `site/`)
- `site_url` should point to the folio domain path
- `dev_addr` on port 7070

## Workflow Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `repo-name` | Path under `/repos/` | Repository name |
| `docs-dir` | Directory containing `mkdocs.yml` | `.` |
| `builder` | Build tool: `mkdocs`, `static` | `mkdocs` |
| `python-version` | Python version for uv | `3.12` |
| `docs-group` | uv dependency group | `docs` |
| `site-dir` | Build output directory | `site-build` |
| `gcs-bucket` | Target GCS bucket | (required) |

## Starter Template

See `examples/mkdocs-minimal/` in the folio repo for a minimal mkdocs project you can copy.
