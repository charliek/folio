# Changelog

## v2026.04.19

- Rename prod Secret Manager secrets to the `{env}-{app}-{purpose}` convention: `folio-password` → `prod-folio-server-login-password`, `folio-hmac-key` → `prod-folio-server-hmac-key`.
- Add `deploy/cloud-run-service.yaml` as the checked-in source of truth for Cloud Run config (env vars, secret bindings, service account, resources).
- Switch `.github/workflows/deploy.yml` to render the manifest and run `gcloud run services replace` instead of `gcloud run services update --image`.
- Update README, docs, `infra/setup.sh`, and `CLAUDE.md` to match the new secret names and declarative-deploy model.
- Add reusable `publish-docs` workflow for caller repos and fix its `_meta.json` generation.
