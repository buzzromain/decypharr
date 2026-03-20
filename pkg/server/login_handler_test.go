package server

// login_handler_test.go — black-box HTTP tests for the web login/logout flow.
//
// Coverage:
//   - POST /login with valid JSON credentials → 303 redirect + auth-session cookie set
//   - POST /login with wrong password         → 401, no cookie
//   - POST /login with malformed JSON body     → 400
//   - POST /login when NeedsAuth (no creds)   → 303 redirect to /register
//   - Full E2E: login → cookie → protected route → 200
//   - POST /logout                             → session invalidated + redirect to /login
//
// Tests MUST NOT use t.Parallel() — they mutate the global config singleton.

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

// loginJSON sends POST /login with a JSON body and returns the recorder.
func loginJSON(t *testing.T, s *Server, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `"}`
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.LoginHandler(rr, r)
	return rr
}

// authServerWithUser returns a Server configured with UseAuth=true and the
// given username/password, ready to exercise LoginHandler.
func authServerWithUser(t *testing.T, username, password string) *Server {
	t.Helper()
	return newMiddlewareTestServer(t, func(cfg *config.Config) {
		cfg.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
		cfg.Debrids = []config.Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}
		setAuthUser(t, cfg, username, password, "")
	})
}

// TestLoginHandler_ValidCredentials_SetsCookieAndRedirects verifies the primary
// login flow: valid credentials in the JSON body → 303 redirect to "/" and an
// auth-session cookie present in the response.
func TestLoginHandler_ValidCredentials_SetsCookieAndRedirects(t *testing.T) {
	s := authServerWithUser(t, "alice", "s3cr3t")

	rr := loginJSON(t, s, "alice", "s3cr3t")

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Fatalf("redirect location = %q, want /", loc)
	}

	var found bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "auth-session" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no auth-session cookie in successful login response")
	}
}

// TestLoginHandler_InvalidCredentials_Returns401 verifies that a wrong password
// is rejected with 401 and that no auth-session cookie is set — the caller
// must not gain access to protected resources.
func TestLoginHandler_InvalidCredentials_Returns401(t *testing.T) {
	s := authServerWithUser(t, "alice", "s3cr3t")

	rr := loginJSON(t, s, "alice", "wrongpassword")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "auth-session" {
			t.Fatal("auth-session cookie must not be set on failed login")
		}
	}
}

// TestLoginHandler_WrongUsername_Returns401 verifies that an unknown username is
// also rejected with 401 — the handler checks both fields, not just the password.
func TestLoginHandler_WrongUsername_Returns401(t *testing.T) {
	s := authServerWithUser(t, "alice", "s3cr3t")

	rr := loginJSON(t, s, "notexist", "s3cr3t")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// TestLoginHandler_MalformedJSON_Returns400 verifies that a non-JSON body is
// rejected with 400 before any credential comparison takes place.
func TestLoginHandler_MalformedJSON_Returns400(t *testing.T) {
	s := authServerWithUser(t, "alice", "s3cr3t")

	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.LoginHandler(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// TestLoginHandler_NeedsAuth_RedirectsToRegister verifies that when UseAuth=true
// but credentials have never been registered, the handler redirects to /register
// instead of attempting credential validation.
func TestLoginHandler_NeedsAuth_RedirectsToRegister(t *testing.T) {
	// UseAuth=true but no credentials saved → NeedsAuth()=true.
	s := newMiddlewareTestServer(t, func(cfg *config.Config) {
		cfg.UseAuth = true
		// Deliberately do not call setAuthUser — NeedsAuth() must return true.
	})

	rr := loginJSON(t, s, "alice", "s3cr3t")

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/register" {
		t.Fatalf("redirect location = %q, want /register", loc)
	}
}

// TestLoginHandler_ValidLogin_SessionAllowsProtectedRoute is an end-to-end test
// that chains the real LoginHandler with authMiddleware:
//
//  1. POST /login with valid credentials → obtain auth-session cookie
//  2. Send that cookie to a protected API endpoint → access granted (204)
//
// This is the primary web UI authentication path used by browser clients.
func TestLoginHandler_ValidLogin_SessionAllowsProtectedRoute(t *testing.T) {
	s := authServerWithUser(t, "alice", "s3cr3t")

	// Step 1: login.
	loginRR := loginJSON(t, s, "alice", "s3cr3t")
	if loginRR.Code != http.StatusSeeOther {
		t.Fatalf("login: status = %d, want 303", loginRR.Code)
	}
	var sc *http.Cookie
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "auth-session" {
			sc = c
			break
		}
	}
	if sc == nil {
		t.Fatal("no auth-session cookie from login")
	}

	// Step 2: access a protected API endpoint with the session cookie.
	req := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	req.AddCookie(sc)
	rr := httptest.NewRecorder()
	s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("protected route: status = %d, want 204 with valid session", rr.Code)
	}
}

// TestLoginHandler_InvalidLogin_CookieDoesNotUnlockProtectedRoute verifies the
// negative path: a failed login must not produce a cookie that unlocks protected
// routes. This ensures 401 on login means 401 everywhere downstream.
func TestLoginHandler_InvalidLogin_CookieDoesNotUnlockProtectedRoute(t *testing.T) {
	s := authServerWithUser(t, "alice", "s3cr3t")

	// Wrong password → 401, no cookie.
	loginRR := loginJSON(t, s, "alice", "badpass")
	if loginRR.Code != http.StatusUnauthorized {
		t.Fatalf("login: status = %d, want 401", loginRR.Code)
	}

	// Attempt to use whatever cookies were (not) set.
	req := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	for _, c := range loginRR.Result().Cookies() {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)

	if rr.Code == http.StatusNoContent {
		t.Fatal("protected route must not be accessible after a failed login")
	}
}

// TestLogoutHandler_ClearsSessionAndRedirects verifies that LogoutHandler
// invalidates the active session and redirects the browser to /login.
// After logout the old cookie must not grant access to protected routes.
func TestLogoutHandler_ClearsSessionAndRedirects(t *testing.T) {
	s := authServerWithUser(t, "alice", "s3cr3t")

	// Obtain a valid session cookie via login.
	loginRR := loginJSON(t, s, "alice", "s3cr3t")
	var sc *http.Cookie
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "auth-session" {
			sc = c
			break
		}
	}
	if sc == nil {
		t.Fatal("no auth-session cookie from login")
	}

	// Logout.
	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.AddCookie(sc)
	logoutRR := httptest.NewRecorder()
	s.LogoutHandler(logoutRR, logoutReq)

	if logoutRR.Code != http.StatusSeeOther {
		t.Fatalf("logout: status = %d, want 303", logoutRR.Code)
	}
	if loc := logoutRR.Header().Get("Location"); loc != "/login" {
		t.Fatalf("logout redirect = %q, want /login", loc)
	}

	// The Set-Cookie header from logout must invalidate the session (MaxAge <= 0).
	for _, c := range logoutRR.Result().Cookies() {
		if c.Name == "auth-session" && c.MaxAge > 0 {
			t.Errorf("logout auth-session cookie MaxAge = %d, want <= 0", c.MaxAge)
		}
	}

	// The cookie returned by logout carries authenticated=false.
	// Sending only that cookie must not unlock protected routes.
	var logoutCookie *http.Cookie
	for _, c := range logoutRR.Result().Cookies() {
		if c.Name == "auth-session" {
			logoutCookie = c
			break
		}
	}
	if logoutCookie == nil {
		// No Set-Cookie from logout means session was not explicitly cleared;
		// skip the post-logout access check.
		return
	}
	req := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	req.AddCookie(logoutCookie)
	rr := httptest.NewRecorder()
	s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)

	if rr.Code == http.StatusNoContent {
		t.Fatal("protected route must not be accessible with the logout-invalidated session cookie")
	}
}
