package arr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

// ── searchSonarr ──────────────────────────────────────────────────────────────

func TestSearchSonarr_PostsPerUniqueSeriesSeason(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v3/command" {
			callCount.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Sonarr}
	files := []ContentFile{
		{Id: 1, SeasonNumber: 1},
		{Id: 1, SeasonNumber: 1}, // duplicate series+season → deduplicated
		{Id: 1, SeasonNumber: 2},
		{Id: 2, SeasonNumber: 1},
	}
	if err := a.searchSonarr(files); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3 unique (seriesId-season) combos: 1-1, 1-2, 2-1
	if got := callCount.Load(); got != 3 {
		t.Errorf("expected 3 POST calls (unique series+season), got %d", got)
	}
}

func TestSearchSonarr_Non200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Sonarr}
	if err := a.searchSonarr([]ContentFile{{Id: 1, SeasonNumber: 1}}); err == nil {
		t.Error("expected error for non-200 response")
	}
}

func TestSearchSonarr_NotConfigured(t *testing.T) {
	a := &Arr{Host: "http://x"} // no token
	if err := a.searchSonarr([]ContentFile{{Id: 1, SeasonNumber: 1}}); err == nil {
		t.Error("expected error for unconfigured arr")
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestRefresh_PostsCommand(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v3/command" {
			called = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	a.Refresh() // return value is ignored by design
	if !called {
		t.Error("expected POST to /api/v3/command")
	}
}

// ── SyncToConfig ──────────────────────────────────────────────────────────────

func TestSyncToConfig_NewArrAddedToResult(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()
	s.AddOrUpdate(&Arr{
		Name:    "sonarr",
		Host:    "http://sonarr:8989",
		Token:   "mytoken",
		Cleanup: true,
		Source:  SourceManual,
	})

	result := s.SyncToConfig()

	var found *config.Arr
	for i := range result {
		if result[i].Name == "sonarr" {
			found = &result[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected 'sonarr' in SyncToConfig result")
	}
	if found.Host != "http://sonarr:8989" {
		t.Errorf("Host = %q, want http://sonarr:8989", found.Host)
	}
	if found.Token != "mytoken" {
		t.Errorf("Token = %q, want mytoken", found.Token)
	}
	if !found.Cleanup {
		t.Error("expected Cleanup=true")
	}
}

// ── SyncFromConfig ────────────────────────────────────────────────────────────

func TestSyncFromConfig_AddsNewAndKeepsExisting(t *testing.T) {
	// Not parallel: uses config.Get()
	// SyncFromConfig merges: arrs in config are added/updated,
	// existing arrs not in config are preserved.
	s := NewStorage()
	s.AddOrUpdate(&Arr{Name: "old", Host: "http://old:9090", Token: "t1"})

	cfgArrs := []config.Arr{
		{Name: "sonarr", Host: "http://sonarr:8989", Token: "t2"},
	}
	s.SyncFromConfig(cfgArrs)

	// "old" is not in config but existing → preserved
	if got := s.Get("old"); got == nil {
		t.Error("existing arr not in config should be preserved after SyncFromConfig")
	}
	// "sonarr" from config is added
	if got := s.Get("sonarr"); got == nil {
		t.Fatal("expected 'sonarr' after SyncFromConfig")
	}
}

func TestSyncFromConfig_PreservesExistingToken(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()
	// Pre-populate with a real token
	s.AddOrUpdate(&Arr{Name: "sonarr", Host: "http://sonarr:8989", Token: "real-token"})

	// Config has empty token → existing token should be kept (cmp.Or logic)
	cfgArrs := []config.Arr{
		{Name: "sonarr", Host: "http://sonarr:8989", Token: ""},
	}
	s.SyncFromConfig(cfgArrs)

	got := s.Get("sonarr")
	if got == nil {
		t.Fatal("expected sonarr after SyncFromConfig")
	}
	if got.Token != "real-token" {
		t.Errorf("Token = %q, want real-token (preserved from existing)", got.Token)
	}
}

func TestSyncFromConfig_PreservesUnmatchedExisting(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()
	s.AddOrUpdate(&Arr{Name: "sonarr", Host: "http://sonarr:8989", Token: "t1"})
	s.AddOrUpdate(&Arr{Name: "radarr", Host: "http://radarr:7878", Token: "t2"})

	// Config only mentions sonarr → radarr should be preserved (unmatch → added from existing)
	cfgArrs := []config.Arr{
		{Name: "sonarr", Host: "http://sonarr:8989", Token: "t1"},
	}
	s.SyncFromConfig(cfgArrs)

	if got := s.Get("radarr"); got == nil {
		t.Error("radarr should be preserved (not in config, kept from existing)")
	}
}

// ── Monitor ───────────────────────────────────────────────────────────────────

func TestMonitor_CallsCleanupQueueForEachArr(t *testing.T) {
	// Not parallel: uses config.Get()
	var cleanupCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/queue":
			cleanupCalls.Add(1)
			fmt.Fprint(w, `{"totalRecords":0,"page":1,"pageSize":200,"records":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	s := NewStorage()
	s.AddOrUpdate(&Arr{Name: "sonarr", Host: srv.URL, Token: "t1"})
	s.AddOrUpdate(&Arr{Name: "radarr", Host: srv.URL, Token: "t2"})

	s.Monitor()

	if got := cleanupCalls.Load(); got != 2 {
		t.Errorf("expected 2 CleanupQueue calls (one per arr), got %d", got)
	}
}

func TestMonitor_EmptyStorage(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()
	s.Monitor() // should not panic or block
}
