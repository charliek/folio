# Quick Start

## Pull the Image

```bash
docker pull ghcr.io/charliek/folio:latest
```

## Run Locally

```bash
docker run -p 8080:8080 \
  -e GCS_BUCKET=your-bucket \
  -e LOGIN_PASSWORD=your-password \
  -e COOKIE_HMAC_KEY=your-secret-key \
  -e COOKIE_SECURE=false \
  ghcr.io/charliek/folio:latest
```

Visit `http://localhost:8080` to see the login page.

!!! note
    Set `COOKIE_SECURE=false` for local HTTP development. In production behind HTTPS, leave it at the default (`true`).

## Deploy to Cloud Run

Once the [GCP resources](../guides/gcp-setup.md) are provisioned, deploy by
applying the checked-in service manifest:

```bash
sed "s|__IMAGE__|ghcr.io/charliek/folio:latest|" deploy/cloud-run-service.yaml \
  | gcloud run services replace - --region us-central1
```

`deploy/cloud-run-service.yaml` holds the env vars, secret bindings,
service account, and resource limits. To deploy new image tags in
CI, use the `Deploy` GitHub Actions workflow, which renders the
manifest and runs `gcloud run services replace`.

See the [GCP Setup Guide](../guides/gcp-setup.md) for full provisioning instructions.

## Disable Authentication

If you handle auth externally (Tailscale, VPN, Cloud Run IAP):

```bash
docker run -p 8080:8080 \
  -e GCS_BUCKET=your-bucket \
  -e AUTH_MODE=none \
  ghcr.io/charliek/folio:latest
```

## Next Steps

- [How It Works](how-it-works.md) — Understand the architecture
- [Configuration](../reference/configuration.md) — All environment variables
- [Caller Setup](../guides/caller-setup.md) — Publish docs from your repos
