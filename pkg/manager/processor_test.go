package manager

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/customerror"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/arr"
	debrid "github.com/sirrobot01/decypharr/pkg/debrid/common"
	debridTypes "github.com/sirrobot01/decypharr/pkg/debrid/types"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// injectClient stores a mock debrid client under the given name in the
// manager's internal clients map. Only callable from within package manager.
func injectClient(m *Manager, name string, c *mockDebridClient) {
	c.cfg.Name = name
	m.clients.Store(name, c)
}

// makeMagnet builds a minimal Magnet for import requests.
func makeMagnet(hash, name string) *utils.Magnet {
	return &utils.Magnet{
		InfoHash: hash,
		Name:     name,
		Link:     fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", hash, name),
	}
}

// makeImportReq returns a minimal ImportRequest suitable for SendToDebrid tests.
func makeImportReq(hash, name string) *ImportRequest {
	return &ImportRequest{
		Magnet:         makeMagnet(hash, name),
		Arr:            arr.New("sonarr", "", "", false, false, false, nil, "", ""),
		DownloadFolder: testDir,
		Action:         config.DownloadActionSymlink,
	}
}

// okTorrent returns a debrid Torrent that looks like a successful submission.
func okTorrent(hash, id string) *debridTypes.Torrent {
	return &debridTypes.Torrent{
		Id:       id,
		InfoHash: hash,
		Name:     hash,
		Files:    map[string]debridTypes.File{},
	}
}

// ── SendToDebrid ──────────────────────────────────────────────────────────────

// TestSendToDebrid_NoClients verifies the "no clients available" error when
// the manager has no registered debrid providers.
func TestSendToDebrid_NoClients(t *testing.T) {
	m := newTestManager(t)
	// clients map is empty by default (no debrid configured in test config)

	req := makeImportReq("aaaa0001aaaa0001aaaa0001aaaa0001aaaa0001", "test-torrent")
	_, err := m.SendToDebrid(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when no clients registered, got nil")
	}
}

// TestSendToDebrid_SubmitMagnetError verifies that a SubmitMagnet failure on
// all clients causes SendToDebrid to return an error.
func TestSendToDebrid_SubmitMagnetError(t *testing.T) {
	m := newTestManager(t)
	mc := &mockDebridClient{submitErr: errors.New("debrid unavailable")}
	injectClient(m, "rd", mc)

	req := makeImportReq("aaaa0002aaaa0002aaaa0002aaaa0002aaaa0002", "submit-err")
	_, err := m.SendToDebrid(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from failed SubmitMagnet, got nil")
	}
	mc.mu.Lock()
	calls := len(mc.submitCalls)
	mc.mu.Unlock()
	if calls != 1 {
		t.Errorf("SubmitMagnet call count = %d, want 1", calls)
	}
}

// TestSendToDebrid_SubmitReturnsNilTorrent verifies that a nil return from
// SubmitMagnet (no error) is treated as a failure.
func TestSendToDebrid_SubmitReturnsNilTorrent(t *testing.T) {
	m := newTestManager(t)
	mc := &mockDebridClient{submitResult: nil, submitErr: nil} // nil torrent, no error
	injectClient(m, "rd", mc)

	req := makeImportReq("aaaa0003aaaa0003aaaa0003aaaa0003aaaa0003", "nil-torrent")
	_, err := m.SendToDebrid(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when SubmitMagnet returns nil torrent, got nil")
	}
}

// TestSendToDebrid_CheckStatusError_DeletesTorrent verifies that when
// CheckStatus returns an error with a non-empty torrent ID, DeleteTorrent is
// called to clean up the dangling debrid entry.
func TestSendToDebrid_CheckStatusError_DeletesTorrent(t *testing.T) {
	m := newTestManager(t)
	const torrentID = "rd-torrent-id-9001"
	mc := &mockDebridClient{
		submitResult:    okTorrent("aaaa0004aaaa0004aaaa0004aaaa0004aaaa0004", torrentID),
		checkResult:     &debridTypes.Torrent{Id: torrentID}, // non-nil with ID → delete triggered
		checkErr:        errors.New("status check failed"),
		deleteTorrentCh: make(chan struct{}, 1),
	}
	injectClient(m, "rd", mc)

	req := makeImportReq("aaaa0004aaaa0004aaaa0004aaaa0004aaaa0004", "check-err")
	_, err := m.SendToDebrid(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from failed CheckStatus, got nil")
	}

	select {
	case <-mc.deleteTorrentCh:
	case <-time.After(time.Second):
		t.Fatal("DeleteTorrent was not called after CheckStatus error with non-empty torrent ID")
	}
	mc.mu.Lock()
	if mc.deleteTorrentCalls[0] != torrentID {
		t.Errorf("DeleteTorrent called with %q, want %q", mc.deleteTorrentCalls[0], torrentID)
	}
	mc.mu.Unlock()
}

// TestSendToDebrid_CheckStatusError_NoID_DoesNotDelete verifies that when
// CheckStatus returns nil (or a zero-ID torrent), DeleteTorrent is NOT called.
func TestSendToDebrid_CheckStatusError_NoID_DoesNotDelete(t *testing.T) {
	m := newTestManager(t)
	mc := &mockDebridClient{
		submitResult:    okTorrent("aaaa0005aaaa0005aaaa0005aaaa0005aaaa0005", "rd-id-5"),
		checkResult:     nil, // nil → condition `torrent != nil && torrent.Id != ""` is false
		checkErr:        errors.New("check failed"),
		deleteTorrentCh: make(chan struct{}, 1),
	}
	injectClient(m, "rd", mc)

	req := makeImportReq("aaaa0005aaaa0005aaaa0005aaaa0005aaaa0005", "check-nil")
	_, err := m.SendToDebrid(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	select {
	case <-mc.deleteTorrentCh:
		t.Fatal("DeleteTorrent should not be called when CheckStatus returns nil torrent")
	case <-time.After(50 * time.Millisecond):
	}
	if mc.deleteTorrentCallCount() != 0 {
		t.Errorf("DeleteTorrent called %d times, want 0 (nil torrent from CheckStatus)", mc.deleteTorrentCallCount())
	}
}

// TestSendToDebrid_Success verifies the happy path: a torrent returned by
// CheckStatus is propagated back to the caller unchanged.
func TestSendToDebrid_Success(t *testing.T) {
	m := newTestManager(t)
	const hash = "aaaa0006aaaa0006aaaa0006aaaa0006aaaa0006"
	want := okTorrent(hash, "rd-id-6")
	want.Name = "My.Show.S01E01"
	mc := &mockDebridClient{
		submitResult: okTorrent(hash, "rd-id-6"),
		checkResult:  want,
	}
	injectClient(m, "rd", mc)

	req := makeImportReq(hash, "My.Show.S01E01")
	got, err := m.SendToDebrid(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Id != want.Id {
		t.Errorf("torrent ID = %q, want %q", got.Id, want.Id)
	}
}

// TestSendToDebrid_FallbackToSecondClient verifies that when the first client
// fails, the second client is tried and its result is used.
func TestSendToDebrid_FallbackToSecondClient(t *testing.T) {
	m := newTestManager(t)
	const hash = "aaaa0007aaaa0007aaaa0007aaaa0007aaaa0007"

	failing := &mockDebridClient{submitErr: errors.New("first client down")}
	working := &mockDebridClient{
		submitResult: okTorrent(hash, "rd2-id-7"),
		checkResult:  okTorrent(hash, "rd2-id-7"),
	}
	injectClient(m, "rd1", failing)
	injectClient(m, "rd2", working)

	req := makeImportReq(hash, "fallback-test")
	got, err := m.SendToDebrid(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Id != "rd2-id-7" {
		t.Errorf("got torrent from wrong client: ID = %q", got.Id)
	}
}

// ── AddNewTorrent ─────────────────────────────────────────────────────────────

// TestAddNewTorrent_Success verifies that a successful submission adds the
// entry to the queue with the correct InfoHash and Category.
func TestAddNewTorrent_Success(t *testing.T) {
	m := newTestManager(t)
	const hash = "bbbb0001bbbb0001bbbb0001bbbb0001bbbb0001"
	mc := &mockDebridClient{
		submitResult: okTorrent(hash, "rd-id-b1"),
		checkResult:  okTorrent(hash, "rd-id-b1"),
	}
	injectClient(m, "rd", mc)

	req := makeImportReq(hash, "Great.Show.S01E01")
	if err := m.AddNewTorrent(context.Background(), req); err != nil {
		t.Fatalf("AddNewTorrent: %v", err)
	}
	t.Cleanup(func() { _ = m.queue.Delete(hash, nil) })

	entry, err := m.queue.GetTorrent(hash)
	if err != nil {
		t.Fatalf("entry not found in queue after AddNewTorrent: %v", err)
	}
	if entry.Category != "sonarr" {
		t.Errorf("category = %q, want sonarr", entry.Category)
	}
	if entry.State != storage.EntryStateDownloading {
		t.Errorf("state = %q, want %q", entry.State, storage.EntryStateDownloading)
	}
	if entry.Status != debridTypes.TorrentStatusDownloading {
		t.Errorf("status = %q, want %q", entry.Status, debridTypes.TorrentStatusDownloading)
	}
}

// TestAddNewTorrent_TooManyActiveDownloads verifies that the
// "too_many_active_downloads" error causes the request to be re-queued
// instead of returning an error.
func TestAddNewTorrent_TooManyActiveDownloads(t *testing.T) {
	m := newTestManager(t)
	const hash = "bbbb0002bbbb0002bbbb0002bbbb0002bbbb0002"
	mc := &mockDebridClient{
		submitErr: customerror.TooManyActiveDownloadsError,
	}
	injectClient(m, "rd", mc)

	req := makeImportReq(hash, "Queued.Show.S01E01")
	req.Arr = arr.New("sonarr", "", "", false, false, false, nil, "", "")
	req.Magnet = makeMagnet(hash, "Queued.Show.S01E01")

	if err := m.AddNewTorrent(context.Background(), req); err != nil {
		t.Fatalf("expected nil error on TooManyActiveDownloads (request re-queued), got: %v", err)
	}
	t.Cleanup(func() { m.queue.DeleteRequest(req.Id) })

	if req.Status != "queued" {
		t.Errorf("request status = %q, want queued", req.Status)
	}
}

// TestAddNewTorrent_DebridError propagates the error to the caller when the
// provider returns a non-retryable failure.
func TestAddNewTorrent_DebridError(t *testing.T) {
	m := newTestManager(t)
	const hash = "bbbb0003bbbb0003bbbb0003bbbb0003bbbb0003"
	mc := &mockDebridClient{submitErr: errors.New("fatal provider error")}
	injectClient(m, "rd", mc)

	req := makeImportReq(hash, "Error.Show.S01E01")
	if err := m.AddNewTorrent(context.Background(), req); err == nil {
		t.Fatal("expected error from AddNewTorrent, got nil")
	}
}

// ── FilterDebrid ──────────────────────────────────────────────────────────────

// TestFilterDebrid_All returns all registered clients when predicate is always
// true.
func TestFilterDebrid_All(t *testing.T) {
	m := newTestManager(t)
	injectClient(m, "rd1", &mockDebridClient{})
	injectClient(m, "rd2", &mockDebridClient{})

	got := m.FilterDebrid(func(_ debrid.Client) bool { return true })
	if len(got) != 2 {
		t.Errorf("FilterDebrid(all) = %d clients, want 2", len(got))
	}
}

// TestFilterDebrid_ByName returns only the client whose name matches.
func TestFilterDebrid_ByName(t *testing.T) {
	m := newTestManager(t)
	injectClient(m, "rd1", &mockDebridClient{})
	injectClient(m, "rd2", &mockDebridClient{})

	got := m.FilterDebrid(func(c debrid.Client) bool {
		return c.Config().Name == "rd1"
	})
	if len(got) != 1 {
		t.Errorf("FilterDebrid(name=rd1) = %d clients, want 1", len(got))
	}
	if got[0].Config().Name != "rd1" {
		t.Errorf("client name = %q, want rd1", got[0].Config().Name)
	}
}

// TestFilterDebrid_None returns empty when predicate is always false.
func TestFilterDebrid_None(t *testing.T) {
	m := newTestManager(t)
	injectClient(m, "rd1", &mockDebridClient{})

	got := m.FilterDebrid(func(_ debrid.Client) bool { return false })
	if len(got) != 0 {
		t.Errorf("FilterDebrid(none) = %d clients, want 0", len(got))
	}
}

// ── GetUnmanagedProviderTorrents ──────────────────────────────────────────────

// TestGetUnmanagedProviderTorrents_ProviderNotFound verifies the error when no
// client is registered under the given name.
func TestGetUnmanagedProviderTorrents_ProviderNotFound(t *testing.T) {
	m := newTestManager(t)

	_, err := m.GetUnmanagedProviderTorrents("ghost-provider")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

// TestGetUnmanagedProviderTorrents_AllManaged verifies that torrents already in
// the storage queue are not returned as unmanaged.
func TestGetUnmanagedProviderTorrents_AllManaged(t *testing.T) {
	m := newTestManager(t)
	const hash = "cccc0001cccc0001cccc0001cccc0001cccc0001"
	mc := &mockDebridClient{
		torrentsResult: []*debridTypes.Torrent{{Id: "t1", InfoHash: hash, Name: "managed"}},
	}
	injectClient(m, "rd", mc)

	// Put the same hash into storage so it's "managed".
	entry := makeEntry(hash, "sonarr", config.ProtocolTorrent)
	if err := m.queue.Add(entry); err != nil {
		t.Fatalf("queue.Add: %v", err)
	}
	t.Cleanup(func() { _ = m.queue.Delete(hash, nil) })

	got, err := m.GetUnmanagedProviderTorrents("rd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 unmanaged torrents (hash is in queue), got %d", len(got))
	}
}

// TestGetUnmanagedProviderTorrents_SomeUnmanaged verifies that only torrents
// absent from storage are returned.
func TestGetUnmanagedProviderTorrents_SomeUnmanaged(t *testing.T) {
	m := newTestManager(t)
	const managedHash = "cccc0002cccc0002cccc0002cccc0002cccc0002"
	const orphanHash = "cccc0003cccc0003cccc0003cccc0003cccc0003"

	mc := &mockDebridClient{
		torrentsResult: []*debridTypes.Torrent{
			{Id: "t-managed", InfoHash: managedHash, Name: "managed"},
			{Id: "t-orphan", InfoHash: orphanHash, Name: "orphan"},
		},
	}
	injectClient(m, "rd", mc)

	managed := makeEntry(managedHash, "sonarr", config.ProtocolTorrent)
	if err := m.queue.Add(managed); err != nil {
		t.Fatalf("queue.Add: %v", err)
	}
	t.Cleanup(func() { _ = m.queue.Delete(managedHash, nil) })

	got, err := m.GetUnmanagedProviderTorrents("rd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 unmanaged torrent, got %d", len(got))
	}
	if got[0].InfoHash != orphanHash {
		t.Errorf("unmanaged hash = %q, want %q", got[0].InfoHash, orphanHash)
	}
}

// ── PurgeUnmanagedProviderTorrents ───────────────────────────────────────────

// TestPurgeUnmanagedProviderTorrents_DeletesOrphans verifies that DeleteTorrent
// is called for each unmanaged torrent and the count is returned correctly.
func TestPurgeUnmanagedProviderTorrents_DeletesOrphans(t *testing.T) {
	m := newTestManager(t)
	orphans := []*debridTypes.Torrent{
		{Id: "orphan-id-1", InfoHash: "dddd0001dddd0001dddd0001dddd0001dddd0001", Name: "orphan1"},
		{Id: "orphan-id-2", InfoHash: "dddd0002dddd0002dddd0002dddd0002dddd0002", Name: "orphan2"},
	}
	mc := &mockDebridClient{torrentsResult: orphans}
	injectClient(m, "rd", mc)

	count, err := m.PurgeUnmanagedProviderTorrents("rd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("purged count = %d, want 2", count)
	}
	if mc.deleteTorrentCallCount() != 2 {
		t.Errorf("DeleteTorrent calls = %d, want 2", mc.deleteTorrentCallCount())
	}
}

// TestPurgeUnmanagedProviderTorrents_DeleteError continues after a DeleteTorrent
// failure and still returns the count of successful deletions.
func TestPurgeUnmanagedProviderTorrents_DeleteError(t *testing.T) {
	m := newTestManager(t)
	mc := &mockDebridClient{
		torrentsResult: []*debridTypes.Torrent{
			{Id: "fail-id-1", InfoHash: "eeee0001eeee0001eeee0001eeee0001eeee0001", Name: "fail1"},
		},
		deleteTorrentErr: errors.New("provider rejected delete"),
	}
	injectClient(m, "rd", mc)

	count, err := m.PurgeUnmanagedProviderTorrents("rd")
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	// Delete failed → 0 successfully purged
	if count != 0 {
		t.Errorf("purged count = %d, want 0 (all deletes failed)", count)
	}
}

func TestProcessQueuedTorrent_CheckStatusErrorMarksEntryError(t *testing.T) {
	m := newTestManager(t)
	const hash = "ffff0001ffff0001ffff0001ffff0001ffff0001"
	mc := &mockDebridClient{
		checkErr: errors.New("provider unavailable"),
	}
	injectClient(m, "rd", mc)

	entry := makeEntry(hash, "sonarr", config.ProtocolTorrent)
	entry.State = storage.EntryStateDownloading
	entry.ActiveProvider = "rd"
	entry.Magnet = makeMagnet(hash, "Queued.Show.S01E01").Link
	entry.Providers = map[string]*storage.ProviderEntry{
		"rd": {
			Provider: "rd",
			ID:       "rd-id-queued",
			AddedAt:  time.Now(),
		},
	}
	if err := m.queue.Add(entry); err != nil {
		t.Fatalf("queue.Add: %v", err)
	}
	t.Cleanup(func() { _ = m.queue.Delete(hash, nil) })

	m.processQueuedTorrent(entry)

	got, err := m.queue.GetTorrent(hash)
	if err != nil {
		t.Fatalf("queue.GetTorrent: %v", err)
	}
	if got.State != storage.EntryStateError {
		t.Fatalf("entry.State = %q, want %q", got.State, storage.EntryStateError)
	}
	if got.ErrorCount == 0 {
		t.Fatal("entry.ErrorCount = 0, want > 0")
	}
}

func TestProcessAction_MergesExistingEntryAndPreservesProviders(t *testing.T) {
	m := newTestManager(t)
	const hash = "ffff0002ffff0002ffff0002ffff0002ffff0002"

	existing := &storage.Entry{
		InfoHash:         hash,
		Name:             "same.name.mkv",
		OriginalFilename: "same.name.mkv",
		Protocol:         config.ProtocolTorrent,
		SavePath:         testDir,
		Category:         "sonarr",
		Action:           config.DownloadActionNone,
		ActiveProvider:   "rd",
		Providers: map[string]*storage.ProviderEntry{
			"rd": {Provider: "rd", ID: "rd-old", AddedAt: time.Now().Add(-time.Hour)},
		},
		Files: map[string]*storage.File{
			"old.mkv": {Name: "old.mkv", InfoHash: hash, AddedOn: time.Now().Add(-time.Hour)},
		},
		Tags: []string{"existing"},
	}
	if err := m.AddOrUpdate(existing, nil); err != nil {
		t.Fatalf("AddOrUpdate(existing): %v", err)
	}
	t.Cleanup(func() { _ = m.DeleteEntry(hash, false) })

	incoming := &storage.Entry{
		InfoHash:         hash,
		Name:             "same.name.mkv",
		OriginalFilename: "same.name.mkv",
		Protocol:         config.ProtocolTorrent,
		SavePath:         testDir,
		Category:         "sonarr",
		Action:           config.DownloadActionNone,
		ActiveProvider:   "tb",
		Providers: map[string]*storage.ProviderEntry{
			"tb": {Provider: "tb", ID: "tb-new", AddedAt: time.Now()},
		},
		Files: map[string]*storage.File{
			"new.mkv": {Name: "new.mkv", InfoHash: hash, AddedOn: time.Now()},
		},
		Tags: []string{"incoming"},
	}

	m.processAction(incoming)

	got, err := m.GetEntry(hash)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if got == nil {
		t.Fatal("GetEntry returned nil")
	}
	if _, ok := got.Providers["rd"]; !ok {
		t.Fatal("provider rd missing after merge")
	}
	if _, ok := got.Providers["tb"]; !ok {
		t.Fatal("provider tb missing after merge")
	}
	if _, ok := got.Files["old.mkv"]; !ok {
		t.Fatal("file old.mkv missing after merge")
	}
	if _, ok := got.Files["new.mkv"]; !ok {
		t.Fatal("file new.mkv missing after merge")
	}
	if got.State != storage.EntryStatePausedUP {
		t.Fatalf("entry.State = %q, want %q", got.State, storage.EntryStatePausedUP)
	}
}

func TestProcessAction_IdempotentForDuplicateInvocation(t *testing.T) {
	m := newTestManager(t)
	const hash = "ffff0003ffff0003ffff0003ffff0003ffff0003"
	entry := &storage.Entry{
		InfoHash:         hash,
		Name:             "dup.invocation.mkv",
		OriginalFilename: "dup.invocation.mkv",
		Protocol:         config.ProtocolTorrent,
		SavePath:         testDir,
		Category:         "sonarr",
		Action:           config.DownloadActionNone,
		ActiveProvider:   "rd",
		Providers: map[string]*storage.ProviderEntry{
			"rd": {Provider: "rd", ID: "rd-dup", AddedAt: time.Now()},
		},
		Files: map[string]*storage.File{
			"dup.invocation.mkv": {Name: "dup.invocation.mkv", InfoHash: hash, AddedOn: time.Now()},
		},
		Tags: []string{"dedupe"},
	}
	t.Cleanup(func() { _ = m.DeleteEntry(hash, false) })

	m.processAction(entry)
	m.processAction(entry)

	got, err := m.GetEntry(hash)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if len(got.Providers) != 1 {
		t.Fatalf("providers len = %d, want 1", len(got.Providers))
	}
	if len(got.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(got.Files))
	}
	if len(got.Tags) != 1 {
		t.Fatalf("tags len = %d, want 1", len(got.Tags))
	}
}
