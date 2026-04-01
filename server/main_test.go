package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Set required env vars.
	t.Setenv("GCS_BUCKET", "test-bucket")
	t.Setenv("LOGIN_PASSWORD", "pass")
	t.Setenv("COOKIE_HMAC_KEY", "key")

	// Clear any overrides by setting to empty (t.Setenv restores on cleanup).
	t.Setenv("PORT", "")
	t.Setenv("AUTH_MODE", "")
	t.Setenv("CACHE_TTL", "")
	t.Setenv("CACHE_MAX_MB", "")
	t.Setenv("CACHE_MAX_OBJECT_MB", "")
	t.Setenv("COOKIE_MAX_AGE", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("ROOT_PREFIX", "")
	t.Setenv("REPOS_PREFIX", "")

	cfg, err := loadConfig()
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "test-bucket", cfg.GCSBucket)
	assert.Equal(t, "password", cfg.AuthMode)
	assert.Equal(t, "pass", cfg.LoginPassword)
	assert.Equal(t, "key", cfg.CookieHMACKey)
	assert.Equal(t, true, cfg.CookieSecure)
	assert.Equal(t, 128, cfg.CacheMaxMB)
	assert.Equal(t, 10, cfg.CacheMaxObjectMB)
	assert.Equal(t, "_root", cfg.RootPrefix)
	assert.Equal(t, "repos", cfg.ReposPrefix)
}

func TestLoadConfig_CustomValues(t *testing.T) {
	t.Setenv("GCS_BUCKET", "my-bucket")
	t.Setenv("PORT", "9090")
	t.Setenv("AUTH_MODE", "none")
	t.Setenv("CACHE_TTL", "10m")
	t.Setenv("CACHE_MAX_MB", "256")
	t.Setenv("CACHE_MAX_OBJECT_MB", "20")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("ROOT_PREFIX", "_site")
	t.Setenv("REPOS_PREFIX", "projects")

	cfg, err := loadConfig()
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "my-bucket", cfg.GCSBucket)
	assert.Equal(t, "none", cfg.AuthMode)
	assert.Equal(t, false, cfg.CookieSecure)
	assert.Equal(t, 256, cfg.CacheMaxMB)
	assert.Equal(t, 20, cfg.CacheMaxObjectMB)
	assert.Equal(t, "_site", cfg.RootPrefix)
	assert.Equal(t, "projects", cfg.ReposPrefix)
}

func TestLoadConfig_InvalidDuration(t *testing.T) {
	t.Setenv("GCS_BUCKET", "bucket")
	t.Setenv("CACHE_TTL", "not-a-duration")

	_, err := loadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CACHE_TTL")
}

func TestValidateConfig_PasswordMode_MissingPassword(t *testing.T) {
	cfg := &Config{
		GCSBucket:     "bucket",
		AuthMode:      "password",
		LoginPassword: "",
		CookieHMACKey: "key",
	}
	// Also clear the SECRET env vars.
	t.Setenv("LOGIN_PASSWORD_SECRET", "")
	t.Setenv("COOKIE_HMAC_SECRET", "")

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LOGIN_PASSWORD")
}

func TestValidateConfig_PasswordMode_MissingHMAC(t *testing.T) {
	cfg := &Config{
		GCSBucket:     "bucket",
		AuthMode:      "password",
		LoginPassword: "pass",
		CookieHMACKey: "",
	}
	t.Setenv("COOKIE_HMAC_SECRET", "")

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "COOKIE_HMAC_KEY")
}

func TestValidateConfig_NoneMode(t *testing.T) {
	cfg := &Config{
		GCSBucket: "bucket",
		AuthMode:  "none",
	}

	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidAuthMode(t *testing.T) {
	cfg := &Config{
		GCSBucket: "bucket",
		AuthMode:  "oauth",
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_MODE")
}

func TestValidateConfig_MissingBucket(t *testing.T) {
	cfg := &Config{
		GCSBucket: "",
		AuthMode:  "none",
	}

	err := validateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GCS_BUCKET")
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	handleHealthz(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, `{"status":"ok"}`, rec.Body.String())
}
