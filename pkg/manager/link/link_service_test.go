package link

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/rs/zerolog"
	debrid "github.com/sirrobot01/decypharr/pkg/debrid/common"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestService(httpClient *http.Client, clients *xsync.Map[string, debrid.Client]) *Service {
	if clients == nil {
		clients = xsync.NewMap[string, debrid.Client]()
	}
	return New(
		clients,
		nil,
		func(_ context.Context, _ *storage.Entry) error { return nil },
		httpClient,
		1,
		zerolog.Nop(),
	)
}

func newTestEntry(provider, filename, fileLink string) *storage.Entry {
	return &storage.Entry{
		InfoHash:       "abc123",
		Name:           "Test.Show.S01",
		ActiveProvider: provider,
		Files: map[string]*storage.File{
			filename: {Name: filename, Size: 1024},
		},
		Providers: map[string]*storage.ProviderEntry{
			provider: {
				ID: "debrid-id-1",
				Files: map[string]*storage.ProviderFile{
					filename: {Link: fileLink},
				},
			},
		},
	}
}

// ── validateLink ──────────────────────────────────────────────────────────────

func TestValidateLink_Nil(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	err := s.validateLink(context.Background(), nil)
	if !IsPermanentErr(err) {
		t.Errorf("nil link: expected permanent error, got %v", err)
	}
}

func TestValidateLink_EmptyDownloadLink(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	link := &types.DownloadLink{Filename: "file.mkv", Link: "http://example.com"}
	// DownloadLink field is empty → Empty() == true
	err := s.validateLink(context.Background(), link)
	if !IsPermanentErr(err) {
		t.Errorf("empty download link: expected permanent error, got %v", err)
	}
}

func TestValidateLink_200OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTestService(srv.Client(), nil)
	link := &types.DownloadLink{DownloadLink: srv.URL + "/file.mkv"}
	if err := s.validateLink(context.Background(), link); err != nil {
		t.Errorf("expected nil for 200 OK, got %v", err)
	}
}

func TestValidateLink_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := newTestService(srv.Client(), nil)
	link := &types.DownloadLink{DownloadLink: srv.URL + "/file.mkv"}
	err := s.validateLink(context.Background(), link)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !IsPermanentErr(err) {
		t.Errorf("404: expected permanent error, got category %v", GetLinkError(err))
	}
}

func TestValidateLink_429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := newTestService(srv.Client(), nil)
	link := &types.DownloadLink{DownloadLink: srv.URL + "/file.mkv"}
	err := s.validateLink(context.Background(), link)
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
	le := GetLinkError(err)
	if le == nil || !le.ShouldRetry() {
		t.Errorf("429: expected retryable error, got %v", err)
	}
}

func TestValidateLink_XErrorHeader(t *testing.T) {
	// X-Error header overrides status code for error classification
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Error", "bandwidth_exceeded")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s := newTestService(srv.Client(), nil)
	link := &types.DownloadLink{DownloadLink: srv.URL + "/file.mkv"}
	err := s.validateLink(context.Background(), link)
	if err == nil {
		t.Fatal("expected error for X-Error: bandwidth_exceeded, got nil")
	}
	le := GetLinkError(err)
	if le == nil || !le.ShouldDisableAccount() {
		t.Errorf("bandwidth_exceeded: expected account issue error, got %v", err)
	}
}

func TestValidateLink_NetworkError(t *testing.T) {
	// Point to a closed server → network error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close()

	s := newTestService(&http.Client{}, nil)
	link := &types.DownloadLink{DownloadLink: url + "/file.mkv"}
	err := s.validateLink(context.Background(), link)
	if err == nil {
		t.Fatal("expected error for network failure, got nil")
	}
	le := GetLinkError(err)
	if le == nil || !le.ShouldRetry() {
		t.Errorf("network error: expected retryable error, got %v", err)
	}
}

// ── getPlacementFile ──────────────────────────────────────────────────────────

func TestGetPlacementFile_FileNotInEntry(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")
	_, err := s.getPlacementFile(entry, "other.mkv")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	le := GetLinkError(err)
	if le == nil || le.Code != "file_not_found" {
		t.Errorf("code = %q, want file_not_found", le.Code)
	}
}

func TestGetPlacementFile_PlacementNil(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := &storage.Entry{
		InfoHash:       "abc",
		ActiveProvider: "rd",
		Files:          map[string]*storage.File{"video.mkv": {Name: "video.mkv"}},
		Providers:      map[string]*storage.ProviderEntry{}, // no placement for "rd"
	}
	_, err := s.getPlacementFile(entry, "video.mkv")
	if err == nil {
		t.Fatal("expected error for nil placement")
	}
	le := GetLinkError(err)
	if le == nil || le.Code != "placement_not_found" {
		t.Errorf("code = %q, want placement_not_found", le.Code)
	}
}

func TestGetPlacementFile_Success(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")
	pf, err := s.getPlacementFile(entry, "video.mkv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pf.Link != "https://rd.com/link" {
		t.Errorf("Link = %q, want %q", pf.Link, "https://rd.com/link")
	}
}

func TestGetPlacementFile_MissingLink_NoRefresher(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	// placementFile exists but has no link/id
	entry := &storage.Entry{
		InfoHash:       "abc",
		ActiveProvider: "rd",
		Files:          map[string]*storage.File{"video.mkv": {Name: "video.mkv"}},
		Providers: map[string]*storage.ProviderEntry{
			"rd": {Files: map[string]*storage.ProviderFile{"video.mkv": {}}},
		},
	}
	_, err := s.getPlacementFile(entry, "video.mkv")
	if err == nil {
		t.Fatal("expected error when placement file has no link and no refresher")
	}
	le := GetLinkError(err)
	if le == nil || le.Code != "no_refresher" {
		t.Errorf("code = %q, want no_refresher", le.Code)
	}
}

func TestGetPlacementFile_RefresherError(t *testing.T) {
	refreshErr := errors.New("upstream unavailable")
	s := &Service{
		validated:      xsync.NewMap[string, error](),
		clients:        xsync.NewMap[string, debrid.Client](),
		entryRefresher: func(_ string) (*storage.Entry, error) { return nil, refreshErr },
		httpClient:     &http.Client{},
		logger:         zerolog.Nop(),
	}
	entry := &storage.Entry{
		InfoHash:       "abc",
		ActiveProvider: "rd",
		Files:          map[string]*storage.File{"video.mkv": {Name: "video.mkv"}},
		Providers: map[string]*storage.ProviderEntry{
			"rd": {Files: map[string]*storage.ProviderFile{"video.mkv": {}}},
		},
	}
	_, err := s.getPlacementFile(entry, "video.mkv")
	if err == nil {
		t.Fatal("expected error from failing refresher")
	}
	le := GetLinkError(err)
	if le == nil || le.Code != "refresh_failed" {
		t.Errorf("code = %q, want refresh_failed", le.Code)
	}
}

func TestGetPlacementFile_RefresherFileDisappeared(t *testing.T) {
	refreshed := &storage.Entry{
		InfoHash:       "abc",
		ActiveProvider: "rd",
		Files:          map[string]*storage.File{}, // file gone after refresh
		Providers:      map[string]*storage.ProviderEntry{"rd": {Files: map[string]*storage.ProviderFile{}}},
	}
	s := &Service{
		validated:      xsync.NewMap[string, error](),
		clients:        xsync.NewMap[string, debrid.Client](),
		entryRefresher: func(_ string) (*storage.Entry, error) { return refreshed, nil },
		httpClient:     &http.Client{},
		logger:         zerolog.Nop(),
	}
	entry := &storage.Entry{
		InfoHash:       "abc",
		ActiveProvider: "rd",
		Files:          map[string]*storage.File{"video.mkv": {Name: "video.mkv"}},
		Providers: map[string]*storage.ProviderEntry{
			"rd": {Files: map[string]*storage.ProviderFile{"video.mkv": {}}},
		},
	}
	_, err := s.getPlacementFile(entry, "video.mkv")
	le := GetLinkError(err)
	if le == nil || le.Code != "file_disappeared" {
		t.Errorf("code = %q, want file_disappeared", le.Code)
	}
}

// ── fetchAndValidate — validation cache ───────────────────────────────────────

func TestFetchAndValidate_CachedFailure_Permanent(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	dlURL := "https://cdn.example.com/file.mkv"
	permanentErr := NewPermanentError(errors.New("file deleted"), "404")
	s.validated.Store(dlURL, permanentErr)

	got, ok := s.validated.Load(dlURL)
	if !ok {
		t.Fatal("expected cached error to be retrievable")
	}
	le := GetLinkError(got)
	if le == nil || !le.IsPermanent() {
		t.Errorf("expected permanent cached error, got %v", got)
	}
}

// ── Clear ─────────────────────────────────────────────────────────────────────

func TestClear(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	s.validated.Store("https://cdn.example.com/a.mkv", nil)
	s.validated.Store("https://cdn.example.com/b.mkv", errors.New("err"))

	if s.validated.Size() != 2 {
		t.Fatalf("expected 2 entries before Clear, got %d", s.validated.Size())
	}
	s.Clear()
	if s.validated.Size() != 0 {
		t.Errorf("expected 0 entries after Clear, got %d", s.validated.Size())
	}
}

// ── invalidateAndRefetch ──────────────────────────────────────────────────────

func TestInvalidateAndRefetch_InvalidLink(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")

	// Empty Debrid → invalid link
	_, err := s.invalidateAndRefetch(context.Background(), entry, types.DownloadLink{
		Filename: "video.mkv",
	})
	if err == nil {
		t.Fatal("expected error for link with empty Debrid/Token")
	}
}

func TestInvalidateAndRefetch_ClientNotFound(t *testing.T) {
	s := newTestService(&http.Client{}, nil) // no clients registered
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")

	_, err := s.invalidateAndRefetch(context.Background(), entry, types.DownloadLink{
		Debrid:   "rd",
		Token:    "mytoken",
		Filename: "video.mkv",
	})
	if err == nil {
		t.Fatal("expected error when client not found")
	}
}

// ── helpers for assertions ────────────────────────────────────────────────────

func IsPermanentErr(err error) bool {
	le := GetLinkError(err)
	return le != nil && le.IsPermanent()
}
