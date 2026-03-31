package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCookie(t *testing.T) {
	hmacKey := []byte("test-secret-key")

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "valid cookie",
			value: createCookie(hmacKey, 1*time.Hour),
			want:  true,
		},
		{
			name:  "expired cookie",
			value: createCookie(hmacKey, -1*time.Hour),
			want:  false,
		},
		{
			name:  "tampered HMAC",
			value: createCookie(hmacKey, 1*time.Hour)[:10] + "tampered",
			want:  false,
		},
		{
			name:  "wrong HMAC key",
			value: createCookie([]byte("different-key"), 1*time.Hour),
			want:  false,
		},
		{
			name:  "malformed - no dot",
			value: "nodothere",
			want:  false,
		},
		{
			name:  "malformed - empty",
			value: "",
			want:  false,
		},
		{
			name:  "malformed - non-numeric expiry",
			value: "notanumber.abc123",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateCookie(tt.value, hmacKey)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateCookie(t *testing.T) {
	hmacKey := []byte("test-key")
	cookie := createCookie(hmacKey, 24*time.Hour)

	parts := strings.SplitN(cookie, ".", 2)
	require.Len(t, parts, 2, "cookie should have two parts separated by dot")
	assert.NotEmpty(t, parts[0], "expiry should not be empty")
	assert.NotEmpty(t, parts[1], "HMAC should not be empty")

	// Cookie should be valid.
	assert.True(t, validateCookie(cookie, hmacKey))
}

func TestAuthMiddleware_None(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := newAuthMiddleware(nil, "none")
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/some/path", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_NoCookie(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := newAuthMiddleware([]byte("secret"), "password")
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/protected/page", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Equal(t, "/_login?next=%2Fprotected%2Fpage", location)
}

func TestAuthMiddleware_ValidCookie(t *testing.T) {
	hmacKey := []byte("secret")
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("protected content"))
	})

	mw := newAuthMiddleware(hmacKey, "password")
	wrapped := mw(handler)

	cookie := createCookie(hmacKey, 1*time.Hour)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "_session", Value: cookie})
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "protected content", rec.Body.String())
}

func TestAuthMiddleware_InvalidCookie(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := newAuthMiddleware([]byte("secret"), "password")
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/page", nil)
	req.AddCookie(&http.Cookie{Name: "_session", Value: "invalid.cookie"})
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
}

func TestHandleLoginPage(t *testing.T) {
	req := httptest.NewRequest("GET", "/_login?next=/docs/page", nil)
	rec := httptest.NewRecorder()
	handleLoginPage(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `<form`)
	assert.Contains(t, body, `name="password"`)
	assert.Contains(t, body, `value="/docs/page"`)
}

func TestHandleLoginPage_InvalidNext(t *testing.T) {
	req := httptest.NewRequest("GET", "/_login?next=https://evil.com", nil)
	rec := httptest.NewRecorder()
	handleLoginPage(rec, req)

	body := rec.Body.String()
	// Should default to /
	assert.Contains(t, body, `value="/"`)
}

func TestLoginSubmit_CorrectPassword(t *testing.T) {
	handler := newLoginSubmitHandler("secret123", []byte("hmac-key"), 24*time.Hour, false)

	form := url.Values{"password": {"secret123"}, "next": {"/dashboard"}}
	req := httptest.NewRequest("POST", "/_login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "_session", cookies[0].Name)
	assert.True(t, cookies[0].HttpOnly)
}

func TestLoginSubmit_WrongPassword(t *testing.T) {
	handler := newLoginSubmitHandler("secret123", []byte("hmac-key"), 24*time.Hour, false)

	form := url.Values{"password": {"wrong"}, "next": {"/page"}}
	req := httptest.NewRequest("POST", "/_login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid password")
}

func TestLoginSubmit_SecureCookie(t *testing.T) {
	handler := newLoginSubmitHandler("pass", []byte("key"), 1*time.Hour, true)

	form := url.Values{"password": {"pass"}, "next": {"/"}}
	req := httptest.NewRequest("POST", "/_login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler(rec, req)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].Secure)
}

func TestIsValidRedirect(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"valid root", "/", true},
		{"valid path", "/docs/page", true},
		{"valid deep path", "/repos/my-project/setup/", true},
		{"empty", "", false},
		{"absolute URL", "https://evil.com", false},
		{"protocol relative", "//evil.com", false},
		{"path traversal", "/docs/../../../etc/passwd", false},
		{"relative no slash", "relative/path", false},
		{"double dot in filename", "/docs/file..v2.html", true},
		{"traversal segment", "/docs/../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidRedirect(tt.target))
		})
	}
}
