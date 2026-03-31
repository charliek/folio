# Path Rewriting

The server maps URL paths to GCS object keys using two rules:

1. Paths starting with `/repos/` pass through directly
2. All other paths get the root prefix (`_root/`) prepended
3. Paths ending in `/` get `index.html` appended

## Examples

| URL Path | GCS Key |
|----------|---------|
| `/` | `_root/index.html` |
| `/about/` | `_root/about/index.html` |
| `/style.css` | `_root/style.css` |
| `/css/main.css` | `_root/css/main.css` |
| `/repos/my-project/` | `repos/my-project/index.html` |
| `/repos/my-project/page.html` | `repos/my-project/page.html` |
| `/repos/my-project/css/style.css` | `repos/my-project/css/style.css` |
| `/repos/my-project/setup/` | `repos/my-project/setup/index.html` |

## Path Normalization

Before rewriting, paths are cleaned with Go's `path.Clean`:

- Double slashes (`//`) are collapsed
- Dot segments (`.`, `..`) are resolved
- Trailing dots are removed

This prevents path traversal attacks. After rewriting, the server validates that the resulting GCS key starts with either the root prefix or the repos prefix.

## Customization

The root and repos prefixes are configurable:

| Variable | Default | Description |
|----------|---------|-------------|
| `ROOT_PREFIX` | `_root` | GCS prefix for root site content |
| `REPOS_PREFIX` | `repos` | GCS prefix for repo doc sites |

The `_root/` prefix is never exposed in URLs. Users see `/` while the server reads from `_root/` in the bucket.
