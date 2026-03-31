package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"cloud.google.com/go/storage"
)

// Config holds all server configuration parsed from environment variables.
type Config struct {
	Port             string
	GCSBucket        string
	AuthMode         string
	LoginPassword    string
	CookieHMACKey    string
	CookieMaxAge     time.Duration
	CookieSecure     bool
	CacheTTL         time.Duration
	CacheMaxMB       int
	CacheMaxObjectMB int
	RootPrefix       string
	ReposPrefix      string
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := validateConfig(cfg); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Resolve secrets from Secret Manager if configured.
	if err := resolveSecrets(ctx, cfg); err != nil {
		slog.Error("failed to resolve secrets", "error", err)
		os.Exit(1)
	}

	// Re-validate after secret resolution.
	if err := validateConfig(cfg); err != nil {
		slog.Error("invalid config after secret resolution", "error", err)
		os.Exit(1)
	}

	cache := NewCache(cfg.CacheMaxMB, cfg.CacheTTL)

	// Lazy GCS client — created on first request, not at startup.
	// This allows the server to start and serve /healthz without GCP credentials.
	bucket := NewLazyBucket(cfg.GCSBucket)

	hmacKey := []byte(cfg.CookieHMACKey)

	mux := http.NewServeMux()

	// Unauthenticated routes.
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /_login", handleLoginPage)
	mux.HandleFunc("POST /_login", newLoginSubmitHandler(cfg.LoginPassword, hmacKey, cfg.CookieMaxAge, cfg.CookieSecure))

	// Authenticated routes.
	authMW := newAuthMiddleware(hmacKey, cfg.AuthMode)
	mux.Handle("GET /_api/repos", authMW(newRepoDiscoveryHandler(bucket, cache, cfg.ReposPrefix)))
	mux.Handle("POST /_admin/cache/purge", authMW(http.HandlerFunc(newCachePurgeHandler(cache))))
	mux.Handle("GET /", authMW(newProxyHandler(bucket, cache, cfg.RootPrefix, cfg.ReposPrefix, cfg.CacheMaxObjectMB)))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		slog.Info("shutting down server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	slog.Info("starting server", "port", cfg.Port, "auth_mode", cfg.AuthMode, "bucket", cfg.GCSBucket)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{"status":"ok"}`)
}

func loadConfig() (*Config, error) {
	maxAge, err := time.ParseDuration(envOrDefault("COOKIE_MAX_AGE", "2160h"))
	if err != nil {
		return nil, fmt.Errorf("invalid COOKIE_MAX_AGE: %w", err)
	}

	cacheTTL, err := time.ParseDuration(envOrDefault("CACHE_TTL", "5m"))
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_TTL: %w", err)
	}

	cacheMaxMB, err := strconv.Atoi(envOrDefault("CACHE_MAX_MB", "128"))
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_MAX_MB: %w", err)
	}

	cacheMaxObjectMB, err := strconv.Atoi(envOrDefault("CACHE_MAX_OBJECT_MB", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_MAX_OBJECT_MB: %w", err)
	}

	cookieSecure, err := strconv.ParseBool(envOrDefault("COOKIE_SECURE", "true"))
	if err != nil {
		return nil, fmt.Errorf("invalid COOKIE_SECURE: %w", err)
	}

	return &Config{
		Port:             envOrDefault("PORT", "8080"),
		GCSBucket:        os.Getenv("GCS_BUCKET"),
		AuthMode:         envOrDefault("AUTH_MODE", authModePassword),
		LoginPassword:    os.Getenv("LOGIN_PASSWORD"),
		CookieHMACKey:    os.Getenv("COOKIE_HMAC_KEY"),
		CookieMaxAge:     maxAge,
		CookieSecure:     cookieSecure,
		CacheTTL:         cacheTTL,
		CacheMaxMB:       cacheMaxMB,
		CacheMaxObjectMB: cacheMaxObjectMB,
		RootPrefix:       envOrDefault("ROOT_PREFIX", "_root"),
		ReposPrefix:      envOrDefault("REPOS_PREFIX", "repos"),
	}, nil
}

func validateConfig(cfg *Config) error {
	if cfg.GCSBucket == "" {
		return fmt.Errorf("GCS_BUCKET is required")
	}
	if cfg.AuthMode != authModePassword && cfg.AuthMode != authModeNone {
		return fmt.Errorf("AUTH_MODE must be 'password' or 'none', got %q", cfg.AuthMode)
	}
	if cfg.AuthMode == authModePassword {
		if cfg.LoginPassword == "" && os.Getenv("LOGIN_PASSWORD_SECRET") == "" {
			return fmt.Errorf("AUTH_MODE=password requires LOGIN_PASSWORD or LOGIN_PASSWORD_SECRET")
		}
		if cfg.CookieHMACKey == "" && os.Getenv("COOKIE_HMAC_SECRET") == "" {
			return fmt.Errorf("AUTH_MODE=password requires COOKIE_HMAC_KEY or COOKIE_HMAC_SECRET")
		}
	}
	return nil
}

func resolveSecrets(ctx context.Context, cfg *Config) error {
	passwordSecret := os.Getenv("LOGIN_PASSWORD_SECRET")
	hmacSecret := os.Getenv("COOKIE_HMAC_SECRET")

	if passwordSecret == "" && hmacSecret == "" {
		return nil
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating secret manager client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if passwordSecret != "" {
		val, err := accessSecret(ctx, client, passwordSecret)
		if err != nil {
			return fmt.Errorf("resolving LOGIN_PASSWORD_SECRET: %w", err)
		}
		cfg.LoginPassword = val
		slog.Info("resolved login password from Secret Manager")
	}

	if hmacSecret != "" {
		val, err := accessSecret(ctx, client, hmacSecret)
		if err != nil {
			return fmt.Errorf("resolving COOKIE_HMAC_SECRET: %w", err)
		}
		cfg.CookieHMACKey = val
		slog.Info("resolved HMAC key from Secret Manager")
	}

	return nil
}

func accessSecret(ctx context.Context, client *secretmanager.Client, name string) (string, error) {
	// Append /versions/latest if no version is specified.
	if !strings.Contains(name, "/versions/") {
		name = name + "/versions/latest"
	}

	resp, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return "", err
	}
	return string(resp.Payload.Data), nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

const (
	authModePassword = "password"
	authModeNone     = "none"
)

// LazyBucket implements BucketReader with lazy GCS client initialization.
// The client is created on first request, allowing the server to start and
// serve /healthz without GCP credentials. Retries on transient init failures.
type LazyBucket struct {
	mu         sync.Mutex
	bucketName string
	client     *storage.Client
	bucket     *storage.BucketHandle
}

func NewLazyBucket(bucketName string) *LazyBucket {
	return &LazyBucket{bucketName: bucketName}
}

func (lb *LazyBucket) init(ctx context.Context) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if lb.client != nil {
		return nil
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotInitialized, err)
	}
	lb.client = client
	lb.bucket = client.Bucket(lb.bucketName)
	return nil
}

func (lb *LazyBucket) ReadObject(ctx context.Context, key string) ([]byte, string, error) {
	if err := lb.init(ctx); err != nil {
		return nil, "", err
	}
	return readGCSObject(ctx, lb.bucket, key)
}

func (lb *LazyBucket) ListPrefixes(ctx context.Context, prefix, delimiter string) ([]string, error) {
	if err := lb.init(ctx); err != nil {
		return nil, err
	}
	return listGCSPrefixes(ctx, lb.bucket, prefix, delimiter)
}

func (lb *LazyBucket) ReadObjectIfExists(ctx context.Context, key string) ([]byte, bool, error) {
	if err := lb.init(ctx); err != nil {
		return nil, false, err
	}
	return readGCSObjectIfExists(ctx, lb.bucket, key)
}
