# Architecture

## Server Structure

All Go code lives in `server/`:

```text
server/
├── main.go       # Entry point, config, routing, lazy GCS client
├── auth.go       # Login page, cookie validation, auth middleware
├── proxy.go      # GCS proxy, path rewriting, repo discovery
├── cache.go      # In-memory LRU/TTL cache
├── go.mod
├── go.sum
└── Dockerfile
```

The server is a single `main` package with no internal sub-packages. This keeps the codebase simple for a focused tool.

## Key Interfaces

### BucketReader

```go
type BucketReader interface {
    ReadObject(ctx context.Context, key string) ([]byte, string, error)
    ListPrefixes(ctx context.Context, prefix, delimiter string) ([]string, error)
    ReadObjectIfExists(ctx context.Context, key string) ([]byte, bool, error)
}
```

All GCS operations go through this interface. The production implementation (`LazyBucket`) wraps `*storage.BucketHandle`. Tests use a mock.

## Authentication Flow

1. `authMiddleware` checks for `_session` cookie on every request
2. Cookie format: `<expiry_unix>.<hmac_sha256_hex>`
3. HMAC is computed over the expiry timestamp string
4. Validation: verify HMAC signature, then check `expiry > now`
5. Invalid or missing → `302` redirect to `/_login?next={path}`
6. Password comparison uses `subtle.ConstantTimeCompare`

## Cache Design

- LRU eviction using `container/list` from stdlib
- TTL-based expiration checked lazily on `Get()`
- Thread-safe via `sync.Mutex`
- Configurable max total size and max per-object size
- No background goroutines — expiration is lazy

## Lazy GCS Client

The GCS client is initialized on first request, not at startup. This allows:

- Server to start and serve `/healthz` without GCP credentials
- Health checks to work in CI and local dev
- Faster cold starts on Cloud Run

## Error Handling

| GCS Error | HTTP Status |
|-----------|-------------|
| Object not found | 404 Not Found |
| Permission denied / other | 502 Bad Gateway |
| Client not initialized | 503 Service Unavailable |

GCS error details are logged but not exposed to clients.

## Dependencies

- `cloud.google.com/go/storage` — GCS client
- `cloud.google.com/go/secretmanager` — Secret Manager (conditional)
- `net/http` — HTTP server (no framework)
- `log/slog` — Structured logging
- `container/list` — LRU linked list
