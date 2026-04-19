# Configuration

All configuration is via environment variables.

## Server

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `8080` |
| `GCS_BUCKET` | GCS bucket name | (required) |
| `AUTH_MODE` | Auth mode: `password`, `none` | `password` |
| `ROOT_PREFIX` | Bucket prefix for root site | `_root` |
| `REPOS_PREFIX` | Bucket prefix for repo sites | `repos` |

## Authentication

| Variable | Description | Default |
|----------|-------------|---------|
| `LOGIN_PASSWORD` | Login password (plaintext) | — |
| `LOGIN_PASSWORD_SECRET` | Secret Manager resource name | — |
| `COOKIE_HMAC_KEY` | HMAC signing key (plaintext) | — |
| `COOKIE_HMAC_SECRET` | Secret Manager resource name | — |
| `COOKIE_MAX_AGE` | Cookie expiry duration | `2160h` (90 days) |
| `COOKIE_SECURE` | Set Secure flag on cookie | `true` |

When `AUTH_MODE=password`, either `LOGIN_PASSWORD` or `LOGIN_PASSWORD_SECRET` is required, and either `COOKIE_HMAC_KEY` or `COOKIE_HMAC_SECRET` is required.

If both a plaintext variable and its `*_SECRET` counterpart are set, Secret Manager takes precedence.

Set `AUTH_MODE=none` to disable authentication entirely. Useful when auth is handled externally (Tailscale, VPN, Cloud Run IAP).

## Cache

| Variable | Description | Default |
|----------|-------------|---------|
| `CACHE_TTL` | Cache entry time-to-live | `5m` |
| `CACHE_MAX_MB` | Maximum cache size in MB | `128` |
| `CACHE_MAX_OBJECT_MB` | Max size of a single cached object in MB | `10` |

Objects larger than `CACHE_MAX_OBJECT_MB` are served directly from GCS without caching.

## Secret Manager

The server supports two modes for secrets:

- **Direct environment variables** — Set `LOGIN_PASSWORD` and `COOKIE_HMAC_KEY` directly. Simple, good for local dev.
- **Secret Manager resource names** — Set `LOGIN_PASSWORD_SECRET` and `COOKIE_HMAC_SECRET` to Secret Manager resource names (e.g., `projects/my-project/secrets/prod-folio-server-login-password`). The server resolves them at startup. Recommended for production.

If no `/versions/` suffix is provided, `/versions/latest` is appended automatically.

## Security Considerations

- The login endpoint has no built-in rate limiting. For production deployments, add rate limiting at the infrastructure level (Cloud Armor, reverse proxy, or Cloud Run concurrency limits).
- Set `COOKIE_SECURE=false` only for local HTTP development. In production behind HTTPS, leave it at the default (`true`).
- Rotate the HMAC key to invalidate all sessions.
