package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	json "github.com/bytedance/sonic"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/arr"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func TestMain(m *testing.M) {
	// Minimal package-level setup: set a base config path so config.Get()
	// can initialize. Per-test isolation is handled by newTestServer.
	dir, err := os.MkdirTemp("", "decypharr-server-testmain-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	config.SetConfigPath(dir)
	config.Get().UseAuth = false
	os.Exit(m.Run())
}

const (
	testSonarrToken = "sonarr-test-token"
	testRadarrToken = "radarr-test-token"
)

// newWebhookServer builds a test server with two configured ARRs (sonarr,
// radarr), each with a known token — handleArrWebhook authenticates every
// request against an ARR's own token, so tests exercising the dispatch path
// beyond name resolution need a real one to get past that gate.
func newWebhookServer(t *testing.T) *Server {
	t.Helper()
	s := newTestServer(t)
	s.manager.GetArrStorage().AddOrUpdate(arr.New("sonarr", "http://sonarr:8989", testSonarrToken, false, false, nil, "", "manual"))
	s.manager.GetArrStorage().AddOrUpdate(arr.New("radarr", "http://radarr:7878", testRadarrToken, false, false, nil, "", "manual"))
	return s
}

func postJSON(t *testing.T, s *Server, body interface{}, query, header string) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	target := "/webhooks/arr"
	if query != "" {
		target += "?" + query
	}
	r := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	if header != "" {
		r.Header.Set("X-Arr-Name", header)
	}
	w := httptest.NewRecorder()
	s.handleArrWebhook(w, r)
	return w
}

// arrWebhookAuthorized is the entire trust boundary for handleArrWebhook: this
// route sits outside authMiddleware, so a wrong answer here means anyone who can
// reach the port can impersonate any configured ARR and trigger debrid deletions
// (via HandleArrDelete's DownloadId fallback) just by guessing its name.
func TestArrWebhookAuthorized(t *testing.T) {
	configured := arr.New("radarr", "http://radarr:7878", "real-token-123", false, false, nil, "", "manual")

	cases := []struct {
		name  string
		a     *arr.Arr
		token string
		want  bool
	}{
		{"correct token", configured, "real-token-123", true},
		{"wrong token", configured, "guessed-token", false},
		{"empty token", configured, "", false},
		{"unknown arr (nil lookup)", nil, "real-token-123", false},
		{"unknown arr with no token guess", nil, "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := arrWebhookAuthorized(c.a, c.token); got != c.want {
				t.Errorf("arrWebhookAuthorized(%v, %q) = %v, want %v", c.a, c.token, got, c.want)
			}
		})
	}
}

// An ARR without a configured token can never have had a webhook registered for
// it (RegisterArrWebhooks skips those), so any request claiming to be it must be
// rejected outright — there is no legitimate token to compare against.
func TestArrWebhookAuthorized_NoTokenConfigured(t *testing.T) {
	untokened := arr.New("sonarr", "http://sonarr:8989", "", false, false, nil, "", "manual")
	if arrWebhookAuthorized(untokened, "") {
		t.Error("an ARR with no configured token must never authorize a webhook request")
	}
	if arrWebhookAuthorized(untokened, "anything") {
		t.Error("an ARR with no configured token must never authorize a webhook request")
	}
}

func TestHandleArrWebhook(t *testing.T) {
	tests := []struct {
		name     string
		body     interface{}
		query    string
		header   string
		wantCode int
	}{
		{
			name:     "invalid JSON body",
			body:     nil,
			query:    "",
			header:   "",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "Test event still requires auth",
			body:     arr.WebhookPayload{EventType: arr.EventTypeTest},
			query:    "arr=sonarr&token=" + testSonarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "Test event without a token is rejected",
			body:     arr.WebhookPayload{EventType: arr.EventTypeTest},
			query:    "arr=sonarr",
			header:   "",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "Download event arr query param",
			body:     arr.WebhookPayload{EventType: arr.EventTypeDownload, DownloadId: "hash1"},
			query:    "arr=sonarr&token=" + testSonarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "Download event instanceName in payload",
			body:     arr.WebhookPayload{EventType: arr.EventTypeDownload, InstanceName: "radarr"},
			query:    "token=" + testRadarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "Download event X-Arr-Name header",
			body:     arr.WebhookPayload{EventType: arr.EventTypeDownload},
			query:    "token=" + testSonarrToken,
			header:   "sonarr",
			wantCode: http.StatusOK,
		},
		{
			name:     "Download event no arr name returns 400",
			body:     arr.WebhookPayload{EventType: arr.EventTypeDownload},
			query:    "",
			header:   "",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "Download event with arr name but wrong token is rejected",
			body:     arr.WebhookPayload{EventType: arr.EventTypeDownload},
			query:    "arr=sonarr&token=wrong",
			header:   "",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "EpisodeFileDelete event",
			body:     arr.WebhookPayload{EventType: arr.EventTypeEpisodeFileDelete},
			query:    "arr=sonarr&token=" + testSonarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "MovieFileDelete event",
			body:     arr.WebhookPayload{EventType: arr.EventTypeMovieFileDelete},
			query:    "arr=radarr&token=" + testRadarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "Rename event",
			body:     arr.WebhookPayload{EventType: arr.EventTypeRename},
			query:    "arr=sonarr&token=" + testSonarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "SeriesDelete event",
			body:     arr.WebhookPayload{EventType: arr.EventTypeSeriesDelete},
			query:    "arr=sonarr&token=" + testSonarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "MovieDelete event",
			body:     arr.WebhookPayload{EventType: arr.EventTypeMovieDelete},
			query:    "arr=radarr&token=" + testRadarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
		{
			name:     "Unknown event type",
			body:     arr.WebhookPayload{EventType: "SomeUnknownEvent"},
			query:    "arr=sonarr&token=" + testSonarrToken,
			header:   "",
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newWebhookServer(t)

			var w *httptest.ResponseRecorder
			if tc.body == nil {
				r := httptest.NewRequest(http.MethodPost, "/webhooks/arr", bytes.NewReader([]byte("not-json{")))
				r.Header.Set("Content-Type", "application/json")
				w = httptest.NewRecorder()
				s.handleArrWebhook(w, r)
			} else {
				w = postJSON(t, s, tc.body, tc.query, tc.header)
			}

			if w.Code != tc.wantCode {
				t.Errorf("status = %d, want %d (body: %s)", w.Code, tc.wantCode, w.Body.String())
			}
		})
	}
}

func TestHandleArrWebhook_MissingArrNameErrorMessage(t *testing.T) {
	s := newWebhookServer(t)
	w := postJSON(t, s, arr.WebhookPayload{EventType: arr.EventTypeDownload}, "", "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "Cannot determine ARR name") {
		t.Fatalf("body = %q, want error guidance about arr name", w.Body.String())
	}
}

func TestHandleArrWebhook_ArrNamePrecedence_QueryOverPayloadOverHeader(t *testing.T) {
	// This test validates resolver precedence contract:
	// query `arr` > payload.instanceName > X-Arr-Name header. The query arr
	// ("sonarr") is the one that must be authenticated — its token is what's
	// supplied, and a mismatched payload/header name must not matter.
	s := newWebhookServer(t)
	w := postJSON(t, s, arr.WebhookPayload{
		EventType:    arr.EventTypeDownload,
		InstanceName: "payload-radarr",
	}, "arr=sonarr&token="+testSonarrToken, "header-lidarr")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}
}

// ── integration: handler → manager → storage side effects ────────────────────

// TestHandleArrWebhook_DownloadEvent_UpsertsArrMedia verifies the full dispatch
// chain: HTTP handler authenticates, resolves the arr name, calls
// HandleArrImport, which writes an ArrMedia into storage.
func TestHandleArrWebhook_DownloadEvent_UpsertsArrMedia(t *testing.T) {
	s := newWebhookServer(t)

	// Set DownloadFolder AFTER newWebhookServer so it isn't overwritten
	// by the per-test config setup.
	dlDir := t.TempDir()
	config.Get().DownloadFolder = dlDir

	managedPath := "/media/tv/SrvShow/s01e01.mkv"
	// SourceFolder must be under config.Get().DownloadFolder for HandleArrImport
	// to accept the event as a Decypharr-originated import.
	sourceFolder := filepath.Join(dlDir, "sonarr")

	w := postJSON(t, s, arr.WebhookPayload{
		EventType:    arr.EventTypeDownload,
		DownloadId:   "srvtesthash1",
		SourceFolder: sourceFolder,
		EpisodeFile:  &arr.WebhookFile{Path: managedPath},
	}, "arr=sonarr&token="+testSonarrToken, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}

	ref, err := s.manager.Storage().GetArrMedia(managedPath)
	if err != nil {
		t.Fatalf("GetArrMedia: %v", err)
	}
	if ref == nil {
		t.Fatal("ArrMedia not found in storage after Download webhook — dispatch chain broken")
	}
	if ref.InfoHash != "srvtesthash1" {
		t.Errorf("InfoHash = %q, want %q", ref.InfoHash, "srvtesthash1")
	}
}

// TestHandleArrWebhook_EpisodeFileDeleteEvent_RemovesArrMedia verifies that the
// EpisodeFileDelete dispatch path reaches storage and removes the ArrMedia.
func TestHandleArrWebhook_EpisodeFileDeleteEvent_RemovesArrMedia(t *testing.T) {
	s := newWebhookServer(t)

	managedPath := "/media/tv/SrvDeleteShow/s01e01.mkv"

	// Seed an ArrMedia directly so the delete has something to remove.
	if err := s.manager.Storage().UpsertArrMedia(&storage.ArrMedia{
		ArrName:     "sonarr",
		ManagedPath: managedPath,
		InfoHash:    "srvdeletehash",
		FileName:    "s01e01.mkv",
	}); err != nil {
		t.Fatalf("UpsertArrMedia: %v", err)
	}

	w := postJSON(t, s, arr.WebhookPayload{
		EventType:   arr.EventTypeEpisodeFileDelete,
		EpisodeFile: &arr.WebhookFile{Path: managedPath},
	}, "arr=sonarr&token="+testSonarrToken, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}

	got, err := s.manager.Storage().GetArrMedia(managedPath)
	if err != nil {
		t.Fatalf("GetArrMedia: %v", err)
	}
	if got != nil {
		t.Error("ArrMedia still present after EpisodeFileDelete webhook — delete dispatch broken")
	}
}

// A webhook claiming to be an unauthenticated/unknown ARR must never reach
// HandleArrDelete's DownloadId fallback — this is the exact exploit path the
// arrWebhookAuthorized gate exists to close.
func TestHandleArrWebhook_UnauthenticatedRequest_NeverReachesStorage(t *testing.T) {
	s := newWebhookServer(t)

	w := postJSON(t, s, arr.WebhookPayload{
		EventType:  arr.EventTypeEpisodeFileDelete,
		DownloadId: "shouldneverbedeleted",
	}, "arr=sonarr&token=wrong-token", "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
