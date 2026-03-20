package arr

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ── Request ───────────────────────────────────────────────────────────────────

func TestRequest_NotConfigured(t *testing.T) {
	a := &Arr{}
	_, err := a.Request("GET", "/api/v3/health", nil, nil)
	if err == nil {
		t.Fatal("expected error for empty token/host")
	}
}

func TestRequest_200OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "testtoken" {
			t.Errorf("expected X-Api-Key header 'testtoken', got %q", r.Header.Get("X-Api-Key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "testtoken"}
	resp, err := a.Request("GET", "/api/v3/health", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRequest_UnmarshalsJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"sonarr","type":"sonarr"}`))
	}))
	defer srv.Close()

	type result struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	var res result
	a := &Arr{Host: srv.URL, Token: "token"}
	_, err := a.Request("GET", "/api/v3/status", nil, &res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Name != "sonarr" {
		t.Errorf("Name = %q, want 'sonarr'", res.Name)
	}
}

func TestRequest_401DoesNotRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "badtoken"}
	resp, err := a.Request("GET", "/api/v3/health", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if calls != 1 {
		t.Errorf("401 should not trigger retry: server called %d times", calls)
	}
}

// ── Validate ──────────────────────────────────────────────────────────────────

func TestValidate_NotConfigured(t *testing.T) {
	a := &Arr{}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for unconfigured arr")
	}
}

func TestValidate_InvalidURL(t *testing.T) {
	a := &Arr{Host: "not-a-url", Token: "token"}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestValidate_200OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Name: "sonarr"}
	if err := a.Validate(); err != nil {
		t.Errorf("unexpected error for 200 OK: %v", err)
	}
}

func TestValidate_404AcceptedForLidarr(t *testing.T) {
	// Some Arr apps return 404 on /api/v3/health — still considered valid
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Name: "lidarr"}
	if err := a.Validate(); err != nil {
		t.Errorf("unexpected error for 404 (Lidarr): %v", err)
	}
}

func TestValidate_401Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "badtoken", Name: "sonarr"}
	if err := a.Validate(); err == nil {
		t.Error("expected error for 401 response")
	}
}

// ── ContentFile.Delete ────────────────────────────────────────────────────────

func TestContentFile_Delete_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episode.mkv")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &ContentFile{Path: path}
	f.Delete()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestContentFile_Delete_NonExistent(t *testing.T) {
	// Should not panic on missing file
	f := &ContentFile{Path: "/tmp/does-not-exist-xyz.mkv"}
	f.Delete() // no panic expected
}
