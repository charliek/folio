package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// newAuthMiddleware returns a middleware that checks for a valid session cookie.
// If authMode is "none", all requests pass through without authentication.
func newAuthMiddleware(hmacKey []byte, authMode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authMode == authModeNone {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("_session")
			if err != nil || !validateCookie(cookie.Value, hmacKey) {
				loginURL := "/_login?next=" + url.QueryEscape(r.URL.RequestURI())
				http.Redirect(w, r, loginURL, http.StatusFound)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// validateCookie checks that a cookie value has valid format, signature, and expiry.
// Cookie format: <expiry_unix_timestamp>.<hmac_sha256_hex>
func validateCookie(value string, hmacKey []byte) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}

	expiryStr := parts[0]
	sig := parts[1]

	// Verify HMAC signature.
	expectedSig := computeHMAC(expiryStr, hmacKey)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return false
	}

	// Check expiry.
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return false
	}

	return time.Now().Unix() < expiry
}

// createCookie generates a signed session cookie value.
func createCookie(hmacKey []byte, maxAge time.Duration) string {
	expiry := time.Now().Add(maxAge).Unix()
	expiryStr := strconv.FormatInt(expiry, 10)
	sig := computeHMAC(expiryStr, hmacKey)
	return expiryStr + "." + sig
}

func computeHMAC(message string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// handleLoginPage renders the login form.
func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	next := r.URL.Query().Get("next")
	if !isValidRedirect(next) {
		next = "/"
	}
	renderLoginForm(w, next, "")
}

// newLoginSubmitHandler returns a handler for POST /_login.
func newLoginSubmitHandler(password string, hmacKey []byte, maxAge time.Duration, secure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			renderLoginForm(w, "/", "Invalid request.")
			return
		}

		submitted := r.FormValue("password")
		next := r.FormValue("next")
		if !isValidRedirect(next) {
			next = "/"
		}

		// Use constant-time comparison to prevent timing attacks.
		if subtle.ConstantTimeCompare([]byte(submitted), []byte(password)) != 1 {
			renderLoginForm(w, next, "Invalid password.")
			return
		}

		cookieValue := createCookie(hmacKey, maxAge)
		http.SetCookie(w, &http.Cookie{
			Name:     "_session",
			Value:    cookieValue,
			Path:     "/",
			MaxAge:   int(maxAge.Seconds()),
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, next, http.StatusFound)
	}
}

// isValidRedirect checks that a redirect target is a safe relative path.
// Rejects absolute URLs, protocol-relative URLs, and paths with traversal segments.
func isValidRedirect(target string) bool {
	if target == "" {
		return false
	}
	if !strings.HasPrefix(target, "/") {
		return false
	}
	if strings.HasPrefix(target, "//") {
		return false
	}
	// Reject path traversal segments but allow ".." in filenames (e.g., "file..v2.html").
	for _, seg := range strings.Split(target, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

func renderLoginForm(w http.ResponseWriter, next string, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if errMsg != "" {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	errorHTML := ""
	if errMsg != "" {
		errorHTML = fmt.Sprintf(`<p style="color:#d32f2f;margin-bottom:16px">%s</p>`, errMsg)
	}

	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Login — folio</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
         display: flex; align-items: center; justify-content: center;
         min-height: 100vh; background: #f5f5f5; }
  .card { background: #fff; padding: 40px; border-radius: 8px;
          box-shadow: 0 2px 8px rgba(0,0,0,0.1); width: 100%%; max-width: 380px; }
  h1 { font-size: 24px; margin-bottom: 24px; color: #333; }
  label { display: block; font-size: 14px; color: #555; margin-bottom: 6px; }
  input[type="password"] { width: 100%%; padding: 10px 12px; border: 1px solid #ddd;
                           border-radius: 4px; font-size: 16px; margin-bottom: 16px; }
  input[type="password"]:focus { outline: none; border-color: #3f51b5; }
  button { width: 100%%; padding: 10px; background: #3f51b5; color: #fff; border: none;
           border-radius: 4px; font-size: 16px; cursor: pointer; }
  button:hover { background: #303f9f; }
</style>
</head>
<body>
<div class="card">
  <h1>folio</h1>
  %s
  <form method="POST" action="/_login">
    <input type="hidden" name="next" value="%s">
    <label for="password">Password</label>
    <input type="password" id="password" name="password" autofocus required>
    <button type="submit">Sign in</button>
  </form>
</div>
</body>
</html>`, errorHTML, html.EscapeString(next))
}
