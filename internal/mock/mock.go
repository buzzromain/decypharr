package mock

import (
	"context"
	"sync"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/arr"
	"github.com/sirrobot01/decypharr/pkg/manager"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// MockService implements manager.Service for handler tests.
// Queue and Arr are backed by real in-memory storage (via manager.New).
// AddNewTorrent, AddNewNZB, GetEntry, and GetEntryByName are intercepted so
// tests can control their return values and observe their call arguments.
type MockService struct {
	real *manager.Manager // provides real Queue + Arr, never started
	mu   sync.Mutex       // protects recorded call slices

	// Configurable return values
	AddNewTorrentErr     error
	AddNewTorrentHook    func(context.Context, *manager.ImportRequest) // called before returning, if non-nil
	AddNewNZBID          string
	AddNewNZBErr         error
	GetEntryResult       *storage.Entry
	GetEntryErr          error
	GetEntryByNameResult *storage.Entry
	GetEntryByNameErr    error

	// Recorded calls — useful for assertion in tests.
	// Access via TorrentCallCount/NZBCallCount for thread safety.
	AddNewTorrentCalls []*manager.ImportRequest
	AddNewNZBCalls     []*manager.ImportRequest
}

// NewMockService returns a MockService wrapping a lightweight manager.Manager
// (no workers started, no debrid clients). The underlying storage is created
// in a temp directory that is cleaned up when the test ends.
func NewMockService(t testing.TB) *MockService {
	t.Helper()
	mgr := manager.New()
	t.Cleanup(func() { _ = mgr.Stop() })
	return &MockService{real: mgr}
}

// -- manager.Service delegation to real manager (Queue, Arr) --

func (m *MockService) Queue() *manager.Queue { return m.real.Queue() }
func (m *MockService) Arr() *arr.Storage     { return m.real.Arr() }

// -- Intercepted methods with configurable behaviour --

func (m *MockService) GetEntry(infohash string) (*storage.Entry, error) {
	return m.GetEntryResult, m.GetEntryErr
}

func (m *MockService) GetEntryByName(torrentName, filename string) (*storage.Entry, error) {
	return m.GetEntryByNameResult, m.GetEntryByNameErr
}

func (m *MockService) AddNewTorrent(ctx context.Context, req *manager.ImportRequest) error {
	m.mu.Lock()
	m.AddNewTorrentCalls = append(m.AddNewTorrentCalls, req)
	m.mu.Unlock()
	if m.AddNewTorrentHook != nil {
		m.AddNewTorrentHook(ctx, req)
	}
	return m.AddNewTorrentErr
}

func (m *MockService) AddNewNZB(ctx context.Context, req *manager.ImportRequest) (string, error) {
	m.mu.Lock()
	m.AddNewNZBCalls = append(m.AddNewNZBCalls, req)
	m.mu.Unlock()
	return m.AddNewNZBID, m.AddNewNZBErr
}

// TorrentCallCount returns the number of AddNewTorrent calls (thread-safe).
func (m *MockService) TorrentCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.AddNewTorrentCalls)
}

// TorrentCalls returns a snapshot of all AddNewTorrent call arguments (thread-safe).
func (m *MockService) TorrentCalls() []*manager.ImportRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*manager.ImportRequest, len(m.AddNewTorrentCalls))
	copy(cp, m.AddNewTorrentCalls)
	return cp
}

// NZBCallCount returns the number of AddNewNZB calls (thread-safe).
func (m *MockService) NZBCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.AddNewNZBCalls)
}

// NZBCalls returns a snapshot of all AddNewNZB call arguments (thread-safe).
func (m *MockService) NZBCalls() []*manager.ImportRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*manager.ImportRequest, len(m.AddNewNZBCalls))
	copy(cp, m.AddNewNZBCalls)
	return cp
}

// Stop tears down the underlying manager. Use in shutdown tests.
func (m *MockService) Stop() error { return m.real.Stop() }

// -- WebDAV stubs (not exercised by qbit/sabnzbd handlers) --

func (m *MockService) IsReady() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *MockService) RootInfo() *manager.FileInfo { return nil }
func (m *MockService) GetEntries() []manager.FileInfo { return nil }
func (m *MockService) GetEntryChildren(group string) (*manager.FileInfo, []manager.FileInfo) {
	return nil, nil
}
func (m *MockService) GetTorrentChildren(name string) (*manager.FileInfo, []manager.FileInfo) {
	return nil, nil
}
func (m *MockService) GetTorrentFile(_, _ string) (*manager.FileInfo, error) { return nil, nil }
func (m *MockService) RemoveEntry(_ *manager.FileInfo) error                  { return nil }
func (m *MockService) CopyEntry(_ *manager.FileInfo, _ string, _ bool) error  { return nil }
func (m *MockService) TrackStream(_ *storage.Entry, _, _ string) string        { return "" }
func (m *MockService) UntrackStream(_ string)                                  {}
