package integration

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
	debridTypes "github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// statefulProvider is an enhanced fake debrid client that supports:
//   - Atomic call counting for SubmitMagnet and CheckStatus
//   - Dynamic result swapping (thread-safe) for simulating provider state changes
//   - Blocking via a channel for simulating slow providers
type statefulProvider struct {
	mu  sync.Mutex
	cfg config.Debrid

	// Submit behavior
	submitResult *debridTypes.Torrent
	submitErr    error
	submitCalls  atomic.Int64

	// CheckStatus behavior — results can be swapped atomically
	checkResults    []*debridTypes.Torrent // indexed by call number (0-based)
	checkFallback   *debridTypes.Torrent   // used when call count exceeds len(checkResults)
	checkCalls      atomic.Int64
	checkCalledCh   chan struct{}
	checkBlock      chan struct{} // if non-nil, CheckStatus blocks until closed
	checkErr        error

	// Delete tracking
	deleteCalls    []string
	deleteTorrentCh chan struct{}
}

// newStatefulProvider creates a provider that returns the same result for all calls.
func newStatefulProvider(name, torrentName string, files map[string]debridTypes.File, status debridTypes.TorrentStatus) *statefulProvider {
	result := &debridTypes.Torrent{
		Id:       "debrid-stateful-001",
		Debrid:   name,
		Status:   status,
		Name:     torrentName,
		Files:    files,
		Progress: progressForStatus(status),
	}
	return &statefulProvider{
		cfg:           config.Debrid{Name: name, Provider: "realdebrid"},
		submitResult:  result,
		checkFallback: result,
		checkCalledCh: make(chan struct{}, 32),
	}
}

// newDelayedProvider creates a provider where CheckStatus returns "downloading"
// for the first N calls, then "downloaded" for subsequent calls.
func newDelayedProvider(name, torrentName string, files map[string]debridTypes.File, downloadingCalls int) *statefulProvider {
	results := make([]*debridTypes.Torrent, downloadingCalls)
	for i := range results {
		results[i] = &debridTypes.Torrent{
			Id:       "debrid-delayed-001",
			Debrid:   name,
			Status:   debridTypes.TorrentStatusDownloading,
			Name:     torrentName,
			Files:    files,
			Progress: float64(i+1) * (100.0 / float64(downloadingCalls+1)),
		}
	}
	return &statefulProvider{
		cfg:          config.Debrid{Name: name, Provider: "realdebrid"},
		submitResult: results[0], // SubmitMagnet returns "downloading"
		checkResults: results,
		checkCalledCh: make(chan struct{}, 32),
		checkFallback: &debridTypes.Torrent{
			Id:       "debrid-delayed-001",
			Debrid:   name,
			Status:   debridTypes.TorrentStatusDownloaded,
			Name:     torrentName,
			Files:    files,
			Progress: 100,
		},
	}
}

// newBlockingProvider creates a provider where SubmitMagnet blocks until the
// provided channel is closed.
func newBlockingProvider(name string, block chan struct{}) *statefulProvider {
	return &statefulProvider{
		cfg: config.Debrid{Name: name, Provider: "realdebrid"},
		submitResult: &debridTypes.Torrent{
			Id:       "debrid-block-001",
			Debrid:   name,
			Status:   debridTypes.TorrentStatusDownloading,
			Name:     "blocked-torrent",
			Files:    map[string]debridTypes.File{},
			Progress: 0,
		},
		checkBlock:    block,
		checkCalledCh: make(chan struct{}, 32),
		checkFallback: &debridTypes.Torrent{
			Id:       "debrid-block-001",
			Debrid:   name,
			Status:   debridTypes.TorrentStatusDownloading,
			Name:     "blocked-torrent",
			Files:    map[string]debridTypes.File{},
			Progress: 50,
		},
	}
}

func progressForStatus(s debridTypes.TorrentStatus) float64 {
	if s == debridTypes.TorrentStatusDownloaded {
		return 100
	}
	return 0
}

func (p *statefulProvider) SubmitMagnet(_ *debridTypes.Torrent) (*debridTypes.Torrent, error) {
	p.submitCalls.Add(1)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.submitErr != nil {
		return nil, p.submitErr
	}
	return copyTorrent(p.submitResult), nil
}

func (p *statefulProvider) CheckStatus(_ *debridTypes.Torrent) (*debridTypes.Torrent, error) {
	if p.checkCalledCh != nil {
		select {
		case p.checkCalledCh <- struct{}{}:
		default:
		}
	}

	// Block if configured
	if p.checkBlock != nil {
		<-p.checkBlock
	}

	callNum := int(p.checkCalls.Add(1)) - 1 // 0-based

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.checkErr != nil {
		return nil, p.checkErr
	}

	if callNum < len(p.checkResults) {
		return copyTorrent(p.checkResults[callNum]), nil
	}
	return copyTorrent(p.checkFallback), nil
}

func (p *statefulProvider) GetSubmitCallCount() int64 {
	return p.submitCalls.Load()
}

func (p *statefulProvider) GetCheckCallCount() int64 {
	return p.checkCalls.Load()
}

func (p *statefulProvider) CheckCalledChan() <-chan struct{} {
	return p.checkCalledCh
}

// copyTorrent returns a shallow copy with a cloned Files map to prevent races.
func copyTorrent(t *debridTypes.Torrent) *debridTypes.Torrent {
	if t == nil {
		return nil
	}
	cp := *t
	cp.Files = make(map[string]debridTypes.File, len(t.Files))
	for k, v := range t.Files {
		cp.Files[k] = v
	}
	return &cp
}

// ── No-op implementations of remaining debrid.Client methods ────────────

func (p *statefulProvider) GetDownloadLink(_ string, _ *debridTypes.File) (debridTypes.DownloadLink, error) {
	return debridTypes.DownloadLink{}, nil
}

func (p *statefulProvider) DeleteTorrent(id string) error {
	p.mu.Lock()
	p.deleteCalls = append(p.deleteCalls, id)
	p.mu.Unlock()
	if p.deleteTorrentCh != nil {
		select {
		case p.deleteTorrentCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func (p *statefulProvider) IsAvailable(_ []string) map[string]bool        { return nil }
func (p *statefulProvider) UpdateTorrent(_ *debridTypes.Torrent) error     { return nil }
func (p *statefulProvider) GetTorrent(_ string) (*debridTypes.Torrent, error) { return nil, nil }
func (p *statefulProvider) GetTorrents() ([]*debridTypes.Torrent, error)   { return nil, nil }
func (p *statefulProvider) Config() config.Debrid                          { return p.cfg }
func (p *statefulProvider) Logger() zerolog.Logger                         { return zerolog.Nop() }
func (p *statefulProvider) RefreshDownloadLinks() error                    { return nil }
func (p *statefulProvider) CheckFile(_ context.Context, _, _ string) error { return nil }
func (p *statefulProvider) AccountManager() *account.Manager               { return nil }
func (p *statefulProvider) GetProfile() (*debridTypes.Profile, error)      { return nil, nil }
func (p *statefulProvider) GetAvailableSlots() (int, error)                { return 10, nil }
func (p *statefulProvider) SyncAccounts()                                  {}
func (p *statefulProvider) DeleteLink(_ debridTypes.DownloadLink) error    { return nil }
func (p *statefulProvider) SupportsCheck() bool                            { return true }

func (p *statefulProvider) SpeedTest(_ context.Context) debridTypes.SpeedTestResult {
	return debridTypes.SpeedTestResult{}
}
