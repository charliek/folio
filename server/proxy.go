package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// BucketReader abstracts GCS bucket operations for testability.
type BucketReader interface {
	ReadObject(ctx context.Context, key string) ([]byte, string, error)
	ListPrefixes(ctx context.Context, prefix, delimiter string) ([]string, error)
	ReadObjectIfExists(ctx context.Context, key string) ([]byte, bool, error)
}

// ErrNotInitialized is returned when the GCS client has not been initialized.
var ErrNotInitialized = errors.New("GCS client not initialized")

// rewritePath maps a URL path to a GCS object key.
// Paths are cleaned to prevent traversal, then routed:
//   - /repos/* passes through to GCS directly
//   - All other paths get rootPrefix prepended
//   - Paths ending in / get index.html appended
func rewritePath(urlPath, rootPrefix, reposPrefix string) (string, error) {
	// Clean the path to resolve .., double slashes, etc.
	cleaned := path.Clean(urlPath)
	if cleaned == "." {
		cleaned = "/"
	}
	// Ensure leading slash.
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}

	var key string
	reposPrefixSlash := "/" + reposPrefix + "/"

	if strings.HasPrefix(cleaned, reposPrefixSlash) {
		// Repos path — pass through, strip leading slash.
		key = cleaned[1:]
	} else {
		// Root content — prepend root prefix.
		key = rootPrefix + cleaned
	}

	// Append index.html for directory-like paths.
	// Check against the original URL to preserve trailing slash intent.
	if strings.HasSuffix(urlPath, "/") || key == rootPrefix || cleaned == "/" {
		if !strings.HasSuffix(key, "/") {
			key += "/"
		}
		key += "index.html"
	}

	// Validate the key stays within expected prefixes.
	if !strings.HasPrefix(key, rootPrefix+"/") && !strings.HasPrefix(key, reposPrefix+"/") {
		return "", fmt.Errorf("path %q rewrites to invalid key %q", urlPath, key)
	}

	return key, nil
}

// newProxyHandler returns a handler that proxies requests to GCS.
func newProxyHandler(bucket BucketReader, cache *Cache, rootPrefix, reposPrefix string, maxObjectMB int) http.Handler {
	maxObjectBytes := int64(maxObjectMB) * 1024 * 1024

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := rewritePath(r.URL.Path, rootPrefix, reposPrefix)
		if err != nil {
			slog.Warn("invalid path", "path", r.URL.Path, "error", err)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// Check cache first.
		if data, contentType, ok := cache.Get(key); ok {
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(data)
			return
		}

		// Fetch from GCS.
		data, contentType, err := bucket.ReadObject(r.Context(), key)
		if err != nil {
			handleGCSError(w, err, key)
			return
		}

		// Detect content type if not set.
		if contentType == "" {
			ext := path.Ext(key)
			contentType = mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		}

		// Cache if under size limit.
		if int64(len(data)) <= maxObjectBytes {
			cache.Set(key, data, contentType)
		}

		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(data)
	})
}

// RepoInfo represents metadata for a published documentation repo.
type RepoInfo struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	LastPublished string `json:"last_published,omitempty"`
	Repo          string `json:"repo,omitempty"`
	URL           string `json:"url,omitempty"`
}

// newRepoDiscoveryHandler returns a handler for GET /_api/repos.
func newRepoDiscoveryHandler(bucket BucketReader, cache *Cache, reposPrefix string) http.Handler {
	cacheKey := "_api/repos"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check cache.
		if data, contentType, ok := cache.Get(cacheKey); ok {
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(data)
			return
		}

		// List repo prefixes.
		prefixes, err := bucket.ListPrefixes(r.Context(), reposPrefix+"/", "/")
		if err != nil {
			handleGCSError(w, err, reposPrefix)
			return
		}

		repos := make([]RepoInfo, 0, len(prefixes))
		for _, prefix := range prefixes {
			// prefix is like "repos/my-project/"
			name := strings.TrimPrefix(prefix, reposPrefix+"/")
			name = strings.TrimSuffix(name, "/")
			if name == "" {
				continue
			}

			info := RepoInfo{Name: name}

			// Try to read _meta.json for additional metadata.
			metaKey := reposPrefix + "/" + name + "/_meta.json"
			if metaData, exists, err := bucket.ReadObjectIfExists(r.Context(), metaKey); err == nil && exists {
				var meta RepoInfo
				if json.Unmarshal(metaData, &meta) == nil {
					info = meta
					if info.Name == "" {
						info.Name = name
					}
				}
			}

			repos = append(repos, info)
		}

		data, err := json.Marshal(repos)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		contentType := "application/json"
		cache.Set(cacheKey, data, contentType)

		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(data)
	})
}

// newCachePurgeHandler returns a handler for POST /_admin/cache/purge.
func newCachePurgeHandler(cache *Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		cache.Purge()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	}
}

func handleGCSError(w http.ResponseWriter, err error, key string) {
	if errors.Is(err, storage.ErrObjectNotExist) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if errors.Is(err, ErrNotInitialized) {
		slog.Error("GCS client not initialized", "key", key)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	slog.Error("GCS error", "key", key, "error", err)
	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}

// GCS helper functions used by LazyBucket.

// maxReadBytes is the upper bound for reading a single GCS object into memory (100MB).
const maxReadBytes = 100 * 1024 * 1024

func readGCSObject(ctx context.Context, bucket *storage.BucketHandle, key string) ([]byte, string, error) {
	obj := bucket.Object(key)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = reader.Close() }()

	// Read up to maxReadBytes+1 to detect oversized objects.
	data, err := io.ReadAll(io.LimitReader(reader, maxReadBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxReadBytes {
		return nil, "", fmt.Errorf("object %q exceeds maximum size of %d bytes", key, maxReadBytes)
	}

	return data, reader.Attrs.ContentType, nil
}

func listGCSPrefixes(ctx context.Context, bucket *storage.BucketHandle, prefix, delimiter string) ([]string, error) {
	it := bucket.Objects(ctx, &storage.Query{
		Prefix:    prefix,
		Delimiter: delimiter,
	})

	var prefixes []string
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		if attrs.Prefix != "" {
			prefixes = append(prefixes, attrs.Prefix)
		}
	}
	return prefixes, nil
}

func readGCSObjectIfExists(ctx context.Context, bucket *storage.BucketHandle, key string) ([]byte, bool, error) {
	data, _, err := readGCSObject(ctx, bucket, key)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}
