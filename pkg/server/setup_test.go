package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

func TestSetupCompleteHandler_ValidationErrors(t *testing.T) {
	t.Run("invalid json returns explicit error", func(t *testing.T) {
		s := newMiddlewareTestServer(t, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewBufferString("{"))
		rr := httptest.NewRecorder()
		s.setupCompleteHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("Invalid request format")) {
			t.Fatalf("expected invalid-format error, got: %s", rr.Body.String())
		}
	})

	t.Run("missing both debrid and usenet is rejected", func(t *testing.T) {
		s := newMiddlewareTestServer(t, nil)
		reqBody := SetupCompleteRequest{}
		reqBody.Auth.SkipAuth = true
		reqBody.Debrid.Skip = true
		reqBody.Usenet.Skip = true
		reqBody.Download.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
		payload, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(payload))
		rr := httptest.NewRecorder()
		s.setupCompleteHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("Please configure at least one Debrid or Usenet provider")) {
			t.Fatalf("unexpected body: %s", rr.Body.String())
		}
	})

	t.Run("invalid debrid provider returns explicit error", func(t *testing.T) {
		s := newMiddlewareTestServer(t, nil)
		reqBody := SetupCompleteRequest{}
		reqBody.Auth.SkipAuth = true
		reqBody.Debrid.Provider = "invalid-provider"
		reqBody.Debrid.APIKey = "x"
		reqBody.Download.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
		reqBody.Mount.MountType = "dfs"
		reqBody.Mount.CacheDir = filepath.Join(config.GetMainPath(), "cache")
		payload, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(payload))
		rr := httptest.NewRecorder()
		s.setupCompleteHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("Invalid debrid provider")) {
			t.Fatalf("unexpected body: %s", rr.Body.String())
		}
	})
}

func TestSetupCompleteHandler_UsenetOnlyAppliesPredictableDefaults(t *testing.T) {
	s := newMiddlewareTestServer(t, nil)
	reqBody := SetupCompleteRequest{}
	reqBody.Auth.SkipAuth = true
	reqBody.Debrid.Skip = true
	reqBody.Usenet.Host = "news.example.org"
	reqBody.Usenet.Port = 119
	reqBody.Usenet.Username = "user"
	reqBody.Usenet.Password = "pass"
	reqBody.Download.DownloadFolder = filepath.Join(config.GetMainPath(), "downloads")
	reqBody.Mount.MountType = "dfs"
	reqBody.Mount.MountPath = filepath.Join(config.GetMainPath(), "mount")
	reqBody.Mount.CacheDir = filepath.Join(config.GetMainPath(), "cache")
	payload, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.setupCompleteHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	cfg := config.Get()
	if cfg.DownloadFolder != reqBody.Download.DownloadFolder {
		t.Fatalf("download folder = %q, want %q", cfg.DownloadFolder, reqBody.Download.DownloadFolder)
	}
	if len(cfg.Categories) == 0 || cfg.Categories[0] != "sonarr" {
		t.Fatalf("categories not defaulted as expected: %#v", cfg.Categories)
	}
	if cfg.MaxDownloads != 10 {
		t.Fatalf("max downloads = %d, want 10", cfg.MaxDownloads)
	}
	if cfg.Mount.DFS.ChunkSize == "" || cfg.Mount.DFS.ReadAheadSize == "" || cfg.Mount.DFS.CacheExpiry == "" {
		t.Fatalf("dfs defaults missing: chunk=%q readAhead=%q expiry=%q", cfg.Mount.DFS.ChunkSize, cfg.Mount.DFS.ReadAheadSize, cfg.Mount.DFS.CacheExpiry)
	}
}
