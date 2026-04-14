# API

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/_health` | No | Health check |
| `GET` | `/_login` | No | Login page |
| `POST` | `/_login` | No | Login submit |
| `GET` | `/_api/repos` | Yes | List published repos |
| `POST` | `/_admin/cache/purge` | Yes | Clear in-memory cache |
| `GET` | `/*` | Yes | Serve content from GCS |

## Health Check

```bash
curl http://localhost:8080/_health
```

```json
{"status":"ok"}
```

Always returns 200. Does not verify GCS connectivity.

## Repo Discovery

```bash
curl -b cookies.txt http://localhost:8080/_api/repos
```

```json
[
  {
    "name": "my-project",
    "description": "Project documentation",
    "last_published": "2026-03-31T14:22:00Z",
    "repo": "charliek/my-project",
    "url": "https://github.com/charliek/my-project"
  },
  {
    "name": "other-repo"
  }
]
```

Lists all repos by scanning GCS prefixes under `repos/`. Reads `_meta.json` for each repo if available. Repos without `_meta.json` appear with name only.

Response is cached with the same TTL as other content.

## Cache Purge

```bash
curl -X POST -b cookies.txt http://localhost:8080/_admin/cache/purge
```

```json
{"status":"ok"}
```

Clears all entries from the in-memory cache. Useful after publishing new content if you don't want to wait for TTL expiry.

## Authentication

All authenticated endpoints require a valid `_session` cookie. Without one, the server redirects to `/_login?next={original_path}`.

After successful login, the cookie is set with:

- `HttpOnly` — not accessible via JavaScript
- `Secure` — only sent over HTTPS (configurable)
- `SameSite=Lax` — basic CSRF protection
- Configurable max age (default 90 days)
