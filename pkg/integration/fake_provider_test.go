package integration

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
	debridTypes "github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// fakeDebridClient implements debrid.Client for integration tests.
// It returns pre-configured results without network access.
type fakeDebridClient struct {
	mu  sync.Mutex
	cfg config.Debrid

	submitResult *debridTypes.Torrent
	checkResult  *debridTypes.Torrent
	submitCalls  []*debridTypes.Torrent
}

func newFakeProvider(name, torrentName string, files map[string]debridTypes.File) *fakeDebridClient {
	return &fakeDebridClient{
		cfg: config.Debrid{Name: name, Provider: "realdebrid"},
		submitResult: &debridTypes.Torrent{
			Id:       "debrid-id-001",
			Debrid:   name,
			Status:   debridTypes.TorrentStatusDownloaded,
			Name:     torrentName,
			Files:    files,
			Progress: 100,
		},
		checkResult: &debridTypes.Torrent{
			Id:       "debrid-id-001",
			Debrid:   name,
			Status:   debridTypes.TorrentStatusDownloaded,
			Name:     torrentName,
			Files:    files,
			Progress: 100,
		},
	}
}

func (f *fakeDebridClient) SubmitMagnet(tr *debridTypes.Torrent) (*debridTypes.Torrent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submitCalls = append(f.submitCalls, tr)
	cp := *f.submitResult
	cp.Files = make(map[string]debridTypes.File, len(f.submitResult.Files))
	for k, v := range f.submitResult.Files {
		cp.Files[k] = v
	}
	return &cp, nil
}

func (f *fakeDebridClient) CheckStatus(_ *debridTypes.Torrent) (*debridTypes.Torrent, error) {
	cp := *f.checkResult
	cp.Files = make(map[string]debridTypes.File, len(f.checkResult.Files))
	for k, v := range f.checkResult.Files {
		cp.Files[k] = v
	}
	return &cp, nil
}

func (f *fakeDebridClient) submitCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.submitCalls)
}

// ── No-op implementations of remaining debrid.Client methods ────────────

func (f *fakeDebridClient) GetDownloadLink(_ string, _ *debridTypes.File) (debridTypes.DownloadLink, error) {
	return debridTypes.DownloadLink{}, nil
}

func (f *fakeDebridClient) DeleteTorrent(_ string) error                       { return nil }
func (f *fakeDebridClient) IsAvailable(hashes []string) map[string]bool        { return nil }
func (f *fakeDebridClient) UpdateTorrent(_ *debridTypes.Torrent) error         { return nil }
func (f *fakeDebridClient) GetTorrent(_ string) (*debridTypes.Torrent, error)  { return nil, nil }
func (f *fakeDebridClient) GetTorrents() ([]*debridTypes.Torrent, error)       { return nil, nil }
func (f *fakeDebridClient) Config() config.Debrid                              { return f.cfg }
func (f *fakeDebridClient) Logger() zerolog.Logger                             { return zerolog.Nop() }
func (f *fakeDebridClient) RefreshDownloadLinks() error                        { return nil }
func (f *fakeDebridClient) CheckFile(_ context.Context, _, _ string) error     { return nil }
func (f *fakeDebridClient) AccountManager() *account.Manager                   { return nil }
func (f *fakeDebridClient) GetProfile() (*debridTypes.Profile, error)          { return nil, nil }
func (f *fakeDebridClient) GetAvailableSlots() (int, error)                    { return 10, nil }
func (f *fakeDebridClient) SyncAccounts()                                      {}
func (f *fakeDebridClient) DeleteLink(_ debridTypes.DownloadLink) error        { return nil }
func (f *fakeDebridClient) SupportsCheck() bool                                { return true }

func (f *fakeDebridClient) SpeedTest(_ context.Context) debridTypes.SpeedTestResult {
	return debridTypes.SpeedTestResult{}
}
