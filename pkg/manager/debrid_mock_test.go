package manager

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
	debridTypes "github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// mockDebridClient implements debrid.Client for unit tests.
// Configurable return values allow callers to exercise every code path
// in Manager methods that delegate to a debrid provider.
type mockDebridClient struct {
	mu  sync.Mutex
	cfg config.Debrid

	// SubmitMagnet
	submitResult *debridTypes.Torrent
	submitErr    error
	submitCalls  []*debridTypes.Torrent

	// CheckStatus
	checkResult  *debridTypes.Torrent
	checkErr     error
	checkBlock   <-chan struct{}
	checkEntered chan struct{} // closed/sent once CheckStatus is entered

	// GetTorrents
	torrentsResult   []*debridTypes.Torrent
	torrentsErr      error
	getTorrentsBlock <-chan struct{}

	// UpdateTorrent
	updateTorrentFn func(*debridTypes.Torrent) error

	// DeleteTorrent
	deleteTorrentErr   error
	deleteTorrentCalls []string
	deleteTorrentCh    chan struct{}

	// GetAvailableSlots
	availableSlots    int
	availableSlotsErr error

	// IsAvailable
	isAvailableResult map[string]bool
}

func (m *mockDebridClient) SubmitMagnet(tr *debridTypes.Torrent) (*debridTypes.Torrent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submitCalls = append(m.submitCalls, tr)
	if m.submitResult == nil {
		return nil, m.submitErr
	}
	cp := *m.submitResult
	cp.Files = make(map[string]debridTypes.File, len(m.submitResult.Files))
	for k, v := range m.submitResult.Files {
		cp.Files[k] = v
	}
	return &cp, m.submitErr
}

func (m *mockDebridClient) CheckStatus(tr *debridTypes.Torrent) (*debridTypes.Torrent, error) {
	if m.checkEntered != nil {
		select {
		case m.checkEntered <- struct{}{}:
		default:
		}
	}
	if m.checkBlock != nil {
		<-m.checkBlock
	}
	if m.checkResult == nil {
		return nil, m.checkErr
	}
	cp := *m.checkResult
	cp.Files = make(map[string]debridTypes.File, len(m.checkResult.Files))
	for k, v := range m.checkResult.Files {
		cp.Files[k] = v
	}
	return &cp, m.checkErr
}

func (m *mockDebridClient) GetDownloadLink(_ string, _ *debridTypes.File) (debridTypes.DownloadLink, error) {
	return debridTypes.DownloadLink{}, nil
}

func (m *mockDebridClient) DeleteTorrent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteTorrentCalls = append(m.deleteTorrentCalls, id)
	if m.deleteTorrentCh != nil {
		select {
		case m.deleteTorrentCh <- struct{}{}:
		default:
		}
	}
	return m.deleteTorrentErr
}

func (m *mockDebridClient) IsAvailable(infohashes []string) map[string]bool {
	if m.isAvailableResult != nil {
		return m.isAvailableResult
	}
	result := make(map[string]bool, len(infohashes))
	for _, h := range infohashes {
		result[h] = false
	}
	return result
}

func (m *mockDebridClient) UpdateTorrent(t *debridTypes.Torrent) error {
	if m.updateTorrentFn != nil {
		return m.updateTorrentFn(t)
	}
	return nil
}

func (m *mockDebridClient) GetTorrent(_ string) (*debridTypes.Torrent, error) { return nil, nil }

func (m *mockDebridClient) GetTorrents() ([]*debridTypes.Torrent, error) {
	if m.getTorrentsBlock != nil {
		<-m.getTorrentsBlock
	}
	return m.torrentsResult, m.torrentsErr
}

func (m *mockDebridClient) Config() config.Debrid { return m.cfg }

func (m *mockDebridClient) Logger() zerolog.Logger { return zerolog.Nop() }

func (m *mockDebridClient) RefreshDownloadLinks() error { return nil }

func (m *mockDebridClient) CheckFile(_ context.Context, _, _ string) error { return nil }

func (m *mockDebridClient) AccountManager() *account.Manager { return nil }

func (m *mockDebridClient) GetProfile() (*debridTypes.Profile, error) { return nil, nil }

func (m *mockDebridClient) GetAvailableSlots() (int, error) {
	return m.availableSlots, m.availableSlotsErr
}

func (m *mockDebridClient) SyncAccounts() {}

func (m *mockDebridClient) DeleteLink(_ debridTypes.DownloadLink) error { return nil }

func (m *mockDebridClient) SpeedTest(_ context.Context) debridTypes.SpeedTestResult {
	return debridTypes.SpeedTestResult{}
}

func (m *mockDebridClient) SupportsCheck() bool { return true }

// deleteTorrentCallCount returns the number of DeleteTorrent calls observed so
// far. Thread-safe.
func (m *mockDebridClient) deleteTorrentCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.deleteTorrentCalls)
}
