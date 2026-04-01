package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBucket implements BucketReader for testing.
type mockBucket struct {
	objects  map[string]mockObject
	prefixes []string
}

type mockObject struct {
	data        []byte
	contentType string
}

func newMockBucket() *mockBucket {
	return &mockBucket{
		objects: make(map[string]mockObject),
	}
}

func (m *mockBucket) ReadObject(_ context.Context, key string) ([]byte, string, error) {
	obj, ok := m.objects[key]
	if !ok {
		return nil, "", storage.ErrObjectNotExist
	}
	return obj.data, obj.contentType, nil
}

func (m *mockBucket) ListPrefixes(_ context.Context, _, _ string) ([]string, error) {
	return m.prefixes, nil
}

func (m *mockBucket) ReadObjectIfExists(_ context.Context, key string) ([]byte, bool, error) {
	obj, ok := m.objects[key]
	if !ok {
		return nil, false, nil
	}
	return obj.data, true, nil
}

// errorBucket always returns an error (simulates GCS backend failure).
type errorBucket struct{}

func (e *errorBucket) ReadObject(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", fmt.Errorf("permission denied")
}

func (e *errorBucket) ListPrefixes(_ context.Context, _, _ string) ([]string, error) {
	return nil, fmt.Errorf("permission denied")
}

func (e *errorBucket) ReadObjectIfExists(_ context.Context, _ string) ([]byte, bool, error) {
	return nil, false, fmt.Errorf("permission denied")
}

// uninitializedBucket simulates a GCS client that hasn't been initialized.
type uninitializedBucket struct{}

func (u *uninitializedBucket) ReadObject(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", ErrNotInitialized
}

func (u *uninitializedBucket) ListPrefixes(_ context.Context, _, _ string) ([]string, error) {
	return nil, ErrNotInitialized
}

func (u *uninitializedBucket) ReadObjectIfExists(_ context.Context, _ string) ([]byte, bool, error) {
	return nil, false, ErrNotInitialized
}

func TestRewritePath(t *testing.T) {
	tests := []struct {
		name    string
		urlPath string
		want    string
		wantErr bool
	}{
		// Root content.
		{"root", "/", "_root/index.html", false},
		{"root about", "/about/", "_root/about/index.html", false},
		{"root css", "/style.css", "_root/style.css", false},
		{"root nested", "/css/main.css", "_root/css/main.css", false},
		{"root page", "/about", "_root/about", false},

		// Repos content.
		{"repo index", "/repos/my-site/", "repos/my-site/index.html", false},
		{"repo page", "/repos/my-site/page.html", "repos/my-site/page.html", false},
		{"repo css", "/repos/my-site/css/style.css", "repos/my-site/css/style.css", false},
		{"repo nested", "/repos/project/setup/", "repos/project/setup/index.html", false},

		// Path traversal (must be safe).
		{"traversal from repos", "/repos/../../../etc/passwd", "_root/etc/passwd", false},
		{"traversal from root", "/../etc/passwd", "_root/etc/passwd", false},
		{"double dot in root", "/about/../../etc/passwd", "_root/etc/passwd", false},

		// Normalization.
		{"double slashes", "//repos//site/", "repos/site/index.html", false},
		{"dot segments", "/repos/site/./page", "repos/site/page", false},
		{"trailing dot", "/repos/site/.", "repos/site", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rewritePath(tt.urlPath, "_root", "repos")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRewritePath_NeverEscapesPrefix(t *testing.T) {
	// Exhaustive traversal tests — result must always start with _root/ or repos/.
	paths := []string{
		"/repos/../../../etc/passwd",
		"/../etc/passwd",
		"/../../..",
		"/%2e%2e/etc/passwd",
		"/repos/../../secret",
		"/./repos/../admin",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			key, err := rewritePath(p, "_root", "repos")
			if err != nil {
				return // error is acceptable
			}
			assert.True(t,
				len(key) >= 5 && (key[:5] == "_root" || key[:5] == "repos"),
				"key %q must start with _root/ or repos/", key)
		})
	}
}

func TestProxyHandler_CacheHit(t *testing.T) {
	bucket := newMockBucket()
	cache := NewCache(1, 5*time.Minute)

	// Pre-populate cache.
	cache.Set("_root/index.html", []byte("<html>cached</html>"), "text/html")

	handler := newProxyHandler(bucket, cache, "_root", "repos", 10)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html", rec.Header().Get("Content-Type"))
	assert.Equal(t, "<html>cached</html>", rec.Body.String())
}

func TestProxyHandler_CacheMiss(t *testing.T) {
	bucket := newMockBucket()
	bucket.objects["_root/index.html"] = mockObject{
		data:        []byte("<html>from GCS</html>"),
		contentType: "text/html",
	}
	cache := NewCache(1, 5*time.Minute)

	handler := newProxyHandler(bucket, cache, "_root", "repos", 10)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "<html>from GCS</html>", rec.Body.String())

	// Should now be cached.
	_, _, ok := cache.Get("_root/index.html")
	assert.True(t, ok)
}

func TestProxyHandler_NotFound(t *testing.T) {
	bucket := newMockBucket()
	cache := NewCache(1, 5*time.Minute)

	handler := newProxyHandler(bucket, cache, "_root", "repos", 10)

	req := httptest.NewRequest("GET", "/nonexistent.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestProxyHandler_BackendError(t *testing.T) {
	bucket := &errorBucket{}
	cache := NewCache(1, 5*time.Minute)

	handler := newProxyHandler(bucket, cache, "_root", "repos", 10)

	req := httptest.NewRequest("GET", "/page.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestProxyHandler_Uninitialized(t *testing.T) {
	bucket := &uninitializedBucket{}
	cache := NewCache(1, 5*time.Minute)

	handler := newProxyHandler(bucket, cache, "_root", "repos", 10)

	req := httptest.NewRequest("GET", "/page.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestProxyHandler_ContentTypeFallback(t *testing.T) {
	bucket := newMockBucket()
	bucket.objects["_root/style.css"] = mockObject{
		data:        []byte("body { color: red }"),
		contentType: "", // empty — should fall back to mime detection
	}
	cache := NewCache(1, 5*time.Minute)

	handler := newProxyHandler(bucket, cache, "_root", "repos", 10)

	req := httptest.NewRequest("GET", "/style.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/css; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestProxyHandler_LargeObjectSkipsCache(t *testing.T) {
	bucket := newMockBucket()
	// Object larger than 1MB limit (maxObjectMB=1 in the test below).
	largeData := make([]byte, 2*1024*1024)
	bucket.objects["_root/large.bin"] = mockObject{
		data:        largeData,
		contentType: "application/octet-stream",
	}
	cache := NewCache(10, 5*time.Minute)

	handler := newProxyHandler(bucket, cache, "_root", "repos", 1) // 1MB max

	req := httptest.NewRequest("GET", "/large.bin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Should NOT be cached.
	_, _, ok := cache.Get("_root/large.bin")
	assert.False(t, ok, "large object should not be cached")
}

func TestRepoDiscovery(t *testing.T) {
	bucket := newMockBucket()
	bucket.prefixes = []string{"repos/shed/", "repos/prox/", "repos/bare/"}
	bucket.objects["repos/shed/_meta.json"] = mockObject{
		data: []byte(`{
			"name": "shed",
			"description": "Dev environments",
			"last_published": "2026-03-31T14:00:00Z",
			"repo": "charliek/shed",
			"url": "https://github.com/charliek/shed"
		}`),
		contentType: "application/json",
	}
	bucket.objects["repos/prox/_meta.json"] = mockObject{
		data: []byte(`{
			"name": "prox",
			"description": "Process manager"
		}`),
		contentType: "application/json",
	}
	// repos/bare/ has no _meta.json — should still appear with name only.

	cache := NewCache(1, 5*time.Minute)
	handler := newRepoDiscoveryHandler(bucket, cache, "repos")

	req := httptest.NewRequest("GET", "/_api/repos", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var repos []RepoInfo
	err := json.Unmarshal(rec.Body.Bytes(), &repos)
	require.NoError(t, err)
	require.Len(t, repos, 3)

	// Find each repo.
	repoMap := make(map[string]RepoInfo)
	for _, r := range repos {
		repoMap[r.Name] = r
	}

	assert.Equal(t, "Dev environments", repoMap["shed"].Description)
	assert.Equal(t, "charliek/shed", repoMap["shed"].Repo)
	assert.Equal(t, "Process manager", repoMap["prox"].Description)
	assert.Empty(t, repoMap["bare"].Description) // no _meta.json
}

func TestRepoDiscovery_Cached(t *testing.T) {
	bucket := newMockBucket()
	bucket.prefixes = []string{"repos/project/"}

	cache := NewCache(1, 5*time.Minute)
	handler := newRepoDiscoveryHandler(bucket, cache, "repos")

	// First request populates cache.
	req := httptest.NewRequest("GET", "/_api/repos", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Modify bucket — should still get cached result.
	bucket.prefixes = []string{"repos/project/", "repos/new/"}

	req2 := httptest.NewRequest("GET", "/_api/repos", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	var repos []RepoInfo
	err := json.Unmarshal(rec2.Body.Bytes(), &repos)
	require.NoError(t, err)
	assert.Len(t, repos, 1, "should return cached result with 1 repo")
}

func TestCachePurgeHandler(t *testing.T) {
	cache := NewCache(1, 5*time.Minute)
	cache.Set("key1", []byte("data"), "text/plain")
	assert.Equal(t, 1, cache.Len())

	handler := newCachePurgeHandler(cache)

	req := httptest.NewRequest("POST", "/_admin/cache/purge", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.Equal(t, 0, cache.Len())
}
