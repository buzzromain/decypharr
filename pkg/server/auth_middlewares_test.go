package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/sirrobot01/decypharr/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func newMiddlewareTestServer(t *testing.T, configure func(*config.Config)) *Server {
	t.Helper()

	config.Reset()
	base := t.TempDir()
	config.SetConfigPath(base)
	cfg := config.Get()
	if configure != nil {
		configure(cfg)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	return &Server{
		cookie: sessions.NewCookieStore([]byte("test-secret-123")),
	}
}

func setAuthUser(t *testing.T, cfg *config.Config, username, password, token string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg.UseAuth = true
	if err := cfg.SaveAuth(&config.Auth{Username: username, Password: string(hash), APIToken: token}); err != nil {
		t.Fatalf("save auth: %v", err)
	}
}

func sessionCookie(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	sess, err := s.cookie.Get(req, "auth-session")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	sess.Values["authenticated"] = true
	sess.Values["username"] = "alice"
	if err := sess.Save(req, rr); err != nil {
		t.Fatalf("save session: %v", err)
	}
	res := rr.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	return cookies[0]
}

func TestAuthMiddleware_ProtectedRoutesRequireAuth(t *testing.T) {
	t.Run("web request without session redirects to login", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
			cfg.Debrids = []config.Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}
			setAuthUser(t, cfg, "alice", "pw", "token-1")
		})

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		req := httptest.NewRequest(http.MethodGet, "/download", nil)
		rr := httptest.NewRecorder()
		s.authMiddleware(next).ServeHTTP(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
		}
		if loc := rr.Header().Get("Location"); loc != "/login" {
			t.Fatalf("location = %q, want /login", loc)
		}
	})

	t.Run("api request without auth returns explicit 401 json", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
			cfg.Debrids = []config.Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}
			setAuthUser(t, cfg, "alice", "pw", "token-1")
		})

		req := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
		rr := httptest.NewRecorder()
		s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("invalid json body: %v", err)
		}
		if _, ok := body["error"]; !ok {
			t.Fatalf("response missing error field: %v", body)
		}
	})

	t.Run("invalid cookie cannot bypass auth", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
			cfg.Debrids = []config.Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}
			setAuthUser(t, cfg, "alice", "pw", "token-1")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Cookie", "auth-session=corrupted")
		rr := httptest.NewRecorder()
		s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
		}
	})

	t.Run("valid session allows protected route", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
			cfg.Debrids = []config.Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}
			setAuthUser(t, cfg, "alice", "pw", "token-1")
		})
		cookie := sessionCookie(t, s)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rr.Code)
		}
	})

	t.Run("bearer api token allows access without session", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
			cfg.Debrids = []config.Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}
			setAuthUser(t, cfg, "alice", "pw", "token-abc")
		})

		req := httptest.NewRequest(http.MethodGet, "/api/repair/jobs", nil)
		req.Header.Set("Authorization", "Bearer token-abc")
		rr := httptest.NewRecorder()
		s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rr.Code)
		}
	})
}

func TestSetupRedirectMiddleware_Behavior(t *testing.T) {
	t.Run("incomplete setup redirects web routes", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.UseAuth = false
			cfg.DownloadFolder = "" // invalid setup on purpose
			cfg.Debrids = nil
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		s.setupRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", rr.Code)
		}
		if rr.Header().Get("Location") != "/setup" {
			t.Fatalf("location = %q, want /setup", rr.Header().Get("Location"))
		}
	})

	t.Run("incomplete setup blocks api with 503 json", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.UseAuth = false
			cfg.DownloadFolder = ""
			cfg.Debrids = nil
		})

		req := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
		rr := httptest.NewRecorder()
		s.setupRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Setup wizard must be completed") {
			t.Fatalf("expected setup error message, got %q", rr.Body.String())
		}
	})

	t.Run("setup api route bypasses setup gate", func(t *testing.T) {
		s := newMiddlewareTestServer(t, func(cfg *config.Config) {
			cfg.UseAuth = false
			cfg.DownloadFolder = ""
			cfg.Debrids = nil
		})

		req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", strings.NewReader("{}"))
		rr := httptest.NewRecorder()
		s.setupRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rr.Code)
		}
	})
}

func TestWebRoutes_PublicVsProtected(t *testing.T) {
	s := newMiddlewareTestServer(t, func(cfg *config.Config) {
		cfg.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
		cfg.Debrids = []config.Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}
		cfg.UseAuth = true
		setAuthUser(t, cfg, "alice", "pw", "token-1")
	})
	// Web handlers require templates parsed by New(); keep only routes not touching templates.
	routes := s.WebRoutes()

	versionReq := httptest.NewRequest(http.MethodGet, "/version", nil)
	versionRR := httptest.NewRecorder()
	routes.ServeHTTP(versionRR, versionReq)
	if versionRR.Code != http.StatusOK {
		t.Fatalf("/version status = %d, want 200", versionRR.Code)
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	protectedRR := httptest.NewRecorder()
	routes.ServeHTTP(protectedRR, protectedReq)
	if protectedRR.Code != http.StatusUnauthorized {
		t.Fatalf("/api/torrents status = %d, want 401", protectedRR.Code)
	}
}
