# Roadmap

## Phase 1: Scaffold, Build & Ship the Open-Source Project ✅

A presentable, functional open-source repo with a working Go server container image on GHCR.

- [x] Go server: GCS proxy, path rewriting, password auth, in-memory cache, repo discovery, health check
- [x] Unit tests for auth, routing, cache, config
- [x] Dockerfile (multi-stage, distroless)
- [x] Documentation site (mkdocs Material, 10 pages)
- [x] CI/CD: test+lint, GitHub Pages, GHCR image on tags
- [x] Examples, infra script, CLAUDE.md
- [x] README with architecture overview and quick start

## Phase 2: GCP Infrastructure & Private Deployment

Stand up the private instance on a custom domain.

- Provision GCS bucket, Secret Manager secrets (login password, HMAC key)
- Set up Workload Identity Federation for GitHub Actions → GCP
- Create `deploy-server.yml` workflow (push to `server/` → build, push to GHCR, deploy to Cloud Run)
- Deploy to Cloud Run, set up domain mapping and DNS
- Verify end-to-end: upload test HTML to GCS, visit the domain, see login → content

## Phase 3: Root Site & Reusable Workflow

Wire up the documentation publishing pipeline.

- Create `publish-root.yml` workflow (push to `docs/` → build mkdocs, sync to `gs://{bucket}/_root/`)
- Create `publish-docs.yml` reusable workflow with `_meta.json` write step
- Add `/_api/repos` endpoint integration with the root landing page
- Build the root landing page to fetch `/_api/repos` and render a dynamic repo index
- Verify: push a docs change → root site updates on the private domain

## Phase 4: Onboard Repos

Start publishing real documentation from private repos.

- Add caller workflow to first repo (e.g., shed)
- Verify it appears under `/repos/shed/` on the private domain
- Confirm `_meta.json` is written and `/_api/repos` returns the entry
- Onboard additional repos (prox, envsecrets, etc.)
- Iterate on the root landing page design as the repo list grows

## Future Considerations

- Additional builders: Sphinx, Hugo, plain HTML
- Multi-user auth: multiple passwords or OAuth2/OIDC allowlist
- Webhook cache invalidation: publish workflows POST to `/_admin/cache/purge`
- Build status badges on the root landing page
- Tailscale deployment guide as an alternative auth approach
- Non-GCP backends: S3 or R2 as storage alternatives
