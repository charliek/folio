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

```bash
gcloud run deploy folio-server \
  --image ghcr.io/charliek/folio:latest \
  --set-env-vars GCS_BUCKET=my-bucket \
  --set-secrets LOGIN_PASSWORD=folio-password:latest,COOKIE_HMAC_KEY=folio-hmac:latest \
  --min-instances 0 --max-instances 2 \
  --allow-unauthenticated
```

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
