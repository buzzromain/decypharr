package link

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/customerror"
	"github.com/sirrobot01/decypharr/internal/testutil"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
	debrid "github.com/sirrobot01/decypharr/pkg/debrid/common"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// TestMain initialises a minimal config so that storage.Entry.GetFolder()
// (which calls config.Get()) succeeds in tests that go through handleBadLink.
func TestMain(m *testing.M) {
	_, cleanup := testutil.SetupTestConfigDir()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// ── mock debrid client ────────────────────────────────────────────────────────

type mockClient struct {
	downloadLink types.DownloadLink
	downloadErr  error
	debridName   string
}

func (m *mockClient) GetDownloadLink(_ string, _ *types.File) (types.DownloadLink, error) {
	return m.downloadLink, m.downloadErr
}

func (m *mockClient) AccountManager() *account.Manager { return nil }

func (m *mockClient) Config() config.Debrid {
	return config.Debrid{Name: m.debridName}
}

func (m *mockClient) SubmitMagnet(_ *types.Torrent) (*types.Torrent, error) { return nil, nil }
func (m *mockClient) CheckStatus(_ *types.Torrent) (*types.Torrent, error)  { return nil, nil }
func (m *mockClient) DeleteTorrent(_ string) error                           { return nil }
func (m *mockClient) IsAvailable(_ []string) map[string]bool                 { return nil }
func (m *mockClient) UpdateTorrent(_ *types.Torrent) error                   { return nil }
func (m *mockClient) GetTorrent(_ string) (*types.Torrent, error)            { return nil, nil }
func (m *mockClient) GetTorrents() ([]*types.Torrent, error)                 { return nil, nil }
func (m *mockClient) Logger() zerolog.Logger                                  { return zerolog.Nop() }
func (m *mockClient) RefreshDownloadLinks() error                             { return nil }
func (m *mockClient) CheckFile(_ context.Context, _, _ string) error         { return nil }
func (m *mockClient) GetProfile() (*types.Profile, error)                    { return nil, nil }
func (m *mockClient) GetAvailableSlots() (int, error)                        { return 0, nil }
func (m *mockClient) SyncAccounts()                                           {}
func (m *mockClient) DeleteLink(_ types.DownloadLink) error                   { return nil }
func (m *mockClient) SpeedTest(_ context.Context) types.SpeedTestResult {
	return types.SpeedTestResult{}
}
func (m *mockClient) SupportsCheck() bool { return false }

var _ debrid.Client = (*mockClient)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func newClientsMap(provider string, client debrid.Client) *xsync.Map[string, debrid.Client] {
	m := xsync.NewMap[string, debrid.Client]()
	if client != nil {
		m.Store(provider, client)
	}
	return m
}

// newTestEntryNoLink creates an entry where the placement file has no Link and no Id.
func newTestEntryNoLink(provider, filename string) *storage.Entry {
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
					// Link="" and Id="" → fetchLink must call getPlacementFile which
					// will either refresh or error. Since no refresher is set, it returns
					// "no_refresher". To reach GetDownloadLink we need a non-empty link or id.
					// This entry is used for scenarios where we want getPlacementFile to error.
					filename: {},
				},
			},
		},
	}
}

// newTestEntryWithID creates an entry where placement file has an Id but no Link.
// This is what the task description calls "for fetchLink to reach GetDownloadLink".
func newTestEntryWithID(provider, filename, id string) *storage.Entry {
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
					filename: {Id: id},
				},
			},
		},
	}
}

// ── fetchLink tests ───────────────────────────────────────────────────────────

func TestFetchLink_FileNotFound(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")

	_, err := s.fetchLink(context.Background(), entry, "nonexistent.mkv")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	le := GetLinkError(err)
	if le == nil {
		t.Fatalf("expected a link.Error, got %v", err)
	}
	if le.Code != "file_not_found" {
		t.Errorf("code = %q, want %q", le.Code, "file_not_found")
	}
	if !le.IsPermanent() {
		t.Errorf("expected permanent error, got category %s", le.Category)
	}
}

func TestFetchLink_NoClientRegistered(t *testing.T) {
	// No client in the map — getClient will fail → "client_not_found"
	s := newTestService(&http.Client{}, nil)
	// Use an entry whose placement file has a valid Id so getPlacementFile succeeds
	entry := newTestEntryWithID("rd", "video.mkv", "file-id-1")

	_, err := s.fetchLink(context.Background(), entry, "video.mkv")
	if err == nil {
		t.Fatal("expected error when no client is registered, got nil")
	}
	le := GetLinkError(err)
	if le == nil {
		t.Fatalf("expected a link.Error, got %v", err)
	}
	if le.Code != "client_not_found" {
		t.Errorf("code = %q, want %q", le.Code, "client_not_found")
	}
	if !le.IsPermanent() {
		t.Errorf("expected permanent error, got category %s", le.Category)
	}
}

func TestFetchLink_PlacementMissing(t *testing.T) {
	// Client registered, but entry has no placement for the active provider
	clients := newClientsMap("rd", &mockClient{debridName: "rd"})
	s := newTestService(&http.Client{}, clients)
	entry := &storage.Entry{
		InfoHash:       "abc123",
		Name:           "Test.Show.S01",
		ActiveProvider: "rd",
		Files: map[string]*storage.File{
			"video.mkv": {Name: "video.mkv", Size: 1024},
		},
		// Providers map exists but has no "rd" key
		Providers: map[string]*storage.ProviderEntry{},
	}

	_, err := s.fetchLink(context.Background(), entry, "video.mkv")
	if err == nil {
		t.Fatal("expected error when placement is nil, got nil")
	}
	le := GetLinkError(err)
	if le == nil {
		t.Fatalf("expected a link.Error, got %v", err)
	}
	// getPlacementFile is called first and it will return "placement_not_found"
	if le.Code != "placement_not_found" {
		t.Errorf("code = %q, want %q", le.Code, "placement_not_found")
	}
	if !le.IsPermanent() {
		t.Errorf("expected permanent error, got category %s", le.Category)
	}
}

func TestFetchLink_Success(t *testing.T) {
	// Full happy path: valid entry + registered client that returns a download link
	expected := types.DownloadLink{
		Debrid:       "rd",
		Token:        "tok",
		Filename:     "video.mkv",
		Link:         "https://rd.com/link",
		DownloadLink: "https://cdn.rd.com/video.mkv",
	}
	mc := &mockClient{downloadLink: expected, debridName: "rd"}
	clients := newClientsMap("rd", mc)
	s := newTestService(&http.Client{}, clients)
	// Entry with a valid Id so getPlacementFile succeeds without a refresher call
	entry := newTestEntryWithID("rd", "video.mkv", "file-id-1")

	got, err := s.fetchLink(context.Background(), entry, "video.mkv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DownloadLink != expected.DownloadLink {
		t.Errorf("DownloadLink = %q, want %q", got.DownloadLink, expected.DownloadLink)
	}
}

// ── handleBadLink tests ───────────────────────────────────────────────────────

func TestHandleBadLink_NonHosterUnavailableError(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")
	origErr := errors.New("some random error")
	dl := types.DownloadLink{Filename: "video.mkv", Link: "https://rd.com/link"}

	_, got := s.handleBadLink(context.Background(), origErr, entry, dl)
	if got == nil {
		t.Fatal("expected error to be returned, got nil")
	}
	if !errors.Is(got, origErr) {
		t.Errorf("expected original error %v, got %v", origErr, got)
	}
}

func TestHandleBadLink_EntryAlreadyBad(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")
	entry.Bad = true // mark as already bad
	dl := types.DownloadLink{Filename: "video.mkv", Link: "https://rd.com/link"}

	_, err := s.handleBadLink(context.Background(), customerror.HosterUnavailableError, entry, dl)
	if err == nil {
		t.Fatal("expected error for already-bad entry, got nil")
	}
	if !strings.Contains(err.Error(), "marked as bad") {
		t.Errorf("expected 'marked as bad' in error message, got: %v", err)
	}
}

// ── GetLink integration tests ─────────────────────────────────────────────────

func TestGetLink_FileNotFound(t *testing.T) {
	s := newTestService(&http.Client{}, nil)
	entry := newTestEntry("rd", "video.mkv", "https://rd.com/link")

	_, err := s.GetLink(context.Background(), entry, "does_not_exist.mkv")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestGetLink_Success_ValidatesLink(t *testing.T) {
	// httptest server that returns 200 for HEAD (validates the download link)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Mock client returns a DownloadLink pointing to the test server
	mc := &mockClient{
		debridName: "rd",
		downloadLink: types.DownloadLink{
			Debrid:       "rd",
			Token:        "tok",
			Filename:     "video.mkv",
			Link:         "https://rd.com/link",
			DownloadLink: srv.URL + "/video.mkv",
		},
	}
	clients := newClientsMap("rd", mc)
	s := &Service{
		validated:    xsync.NewMap[string, error](),
		clients:      clients,
		entryRefresher: nil,
		repairer:     func(_ context.Context, _ *storage.Entry) error { return nil },
		httpClient:   srv.Client(),
		retries:      1,
		logger:       zerolog.Nop(),
	}
	// Entry with a valid Id so getPlacementFile finds it directly
	entry := newTestEntryWithID("rd", "video.mkv", "file-id-1")

	got, err := s.GetLink(context.Background(), entry, "video.mkv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DownloadLink != srv.URL+"/video.mkv" {
		t.Errorf("DownloadLink = %q, want %q", got.DownloadLink, srv.URL+"/video.mkv")
	}
}

// ── disableLinkAccount tests ──────────────────────────────────────────────────

func TestDisableLinkAccount_ClientNotFound(t *testing.T) {
	// No clients registered — disableLinkAccount should log and return without panic
	s := newTestService(&http.Client{}, nil)
	dl := types.DownloadLink{
		Debrid: "rd",
		Token:  "some-token",
	}
	linkErr := &Error{
		Err:      errors.New("bandwidth exceeded"),
		Category: CategoryAccountIssue,
		Code:     "bandwidth_exceeded",
	}

	// Must not panic
	s.disableLinkAccount(dl, linkErr)
}
