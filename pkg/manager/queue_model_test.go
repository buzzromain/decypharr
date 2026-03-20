package manager

import (
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/arr"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// newQueueOnly wraps the shared newTestQueue helper (returns Queue+Storage) for
// tests that only need the Queue.
func newQueueOnly(t *testing.T) *Queue {
	t.Helper()
	q, _ := newTestQueue(t)
	return q
}

// ── NewTorrentRequest ─────────────────────────────────────────────────────────

func TestNewTorrentRequest_BasicFields(t *testing.T) {
	mag := &utils.Magnet{InfoHash: "abc123"}
	a := &arr.Arr{Name: "sonarr"}
	req := NewTorrentRequest("rd", "/downloads", mag, a, config.DownloadActionSymlink, nil, false, "", ImportTypeQBit, false)

	if req == nil {
		t.Fatal("expected non-nil request")
	}
	if req.Id == "" {
		t.Error("expected non-empty ID")
	}
	if req.Status != "started" {
		t.Errorf("status: got %q, want started", req.Status)
	}
	if req.SelectedDebrid != "rd" {
		t.Errorf("SelectedDebrid: got %q, want rd", req.SelectedDebrid)
	}
	if req.DownloadFolder != "/downloads" {
		t.Errorf("DownloadFolder: got %q, want /downloads", req.DownloadFolder)
	}
	if req.Magnet != mag {
		t.Error("Magnet not set correctly")
	}
	if req.Type != ImportTypeQBit {
		t.Errorf("Type: got %q, want qbit", req.Type)
	}
}

func TestNewTorrentRequest_ArrDebridTakesPriority(t *testing.T) {
	// When arr.SelectedDebrid is set, it overrides the debrid argument
	a := &arr.Arr{SelectedDebrid: "torbox"}
	req := NewTorrentRequest("rd", "", &utils.Magnet{}, a, config.DownloadActionSymlink, nil, false, "", ImportTypeAPI, false)
	if req.SelectedDebrid != "torbox" {
		t.Errorf("SelectedDebrid: got %q, want torbox (arr should win)", req.SelectedDebrid)
	}
}

func TestNewTorrentRequest_UniqueIDs(t *testing.T) {
	a := &arr.Arr{}
	r1 := NewTorrentRequest("rd", "", &utils.Magnet{}, a, config.DownloadActionSymlink, nil, false, "", ImportTypeQBit, false)
	r2 := NewTorrentRequest("rd", "", &utils.Magnet{}, a, config.DownloadActionSymlink, nil, false, "", ImportTypeQBit, false)
	if r1.Id == r2.Id {
		t.Error("expected unique IDs across calls")
	}
}

// ── NewNZBRequest ─────────────────────────────────────────────────────────────

func TestNewNZBRequest_BasicFields(t *testing.T) {
	content := []byte("<nzb/>")
	a := &arr.Arr{Name: "radarr"}
	req := NewNZBRequest("movie.nzb", "/usenet", content, a, config.DownloadActionSymlink, "http://cb", ImportTypeSABnzbd, false)

	if req == nil {
		t.Fatal("expected non-nil request")
	}
	if req.Id == "" {
		t.Error("expected non-empty ID")
	}
	if req.Status != "started" {
		t.Errorf("status: got %q, want started", req.Status)
	}
	if req.SelectedDebrid != "usenet" {
		t.Errorf("SelectedDebrid: got %q, want usenet", req.SelectedDebrid)
	}
	if req.Name != "movie.nzb" {
		t.Errorf("Name: got %q, want movie.nzb", req.Name)
	}
	if string(req.NZBContent) != "<nzb/>" {
		t.Errorf("NZBContent: got %q, want <nzb/>", req.NZBContent)
	}
	if req.CallBackUrl != "http://cb" {
		t.Errorf("CallBackUrl: got %q, want http://cb", req.CallBackUrl)
	}
	if req.Type != ImportTypeSABnzbd {
		t.Errorf("Type: got %q, want sabnzbd", req.Type)
	}
}

// ── ListFilterFunc ────────────────────────────────────────────────────────────

func TestListFilterFunc_NoFilters_ReturnsNil(t *testing.T) {
	q := newQueueOnly(t)
	fn := q.ListFilterFunc("", config.ProtocolAll, "", nil)
	if fn != nil {
		t.Error("expected nil filter when no constraints given")
	}
}

func TestListFilterFunc_CategoryFilter(t *testing.T) {
	q := newQueueOnly(t)
	fn := q.ListFilterFunc("movies", config.ProtocolAll, "", nil)
	if fn == nil {
		t.Fatal("expected non-nil filter for category constraint")
	}

	match := &storage.Entry{Category: "movies", Protocol: config.ProtocolTorrent}
	noMatch := &storage.Entry{Category: "tv", Protocol: config.ProtocolTorrent}

	if !fn(match) {
		t.Error("entry with matching category should pass")
	}
	if fn(noMatch) {
		t.Error("entry with different category should not pass")
	}
}

func TestListFilterFunc_ProtocolFilter(t *testing.T) {
	q := newQueueOnly(t)
	fn := q.ListFilterFunc("", config.ProtocolTorrent, "", nil)
	if fn == nil {
		t.Fatal("expected non-nil filter for protocol constraint")
	}

	torrent := &storage.Entry{Protocol: config.ProtocolTorrent}
	nzb := &storage.Entry{Protocol: config.ProtocolNZB}

	if !fn(torrent) {
		t.Error("torrent entry should pass torrent filter")
	}
	if fn(nzb) {
		t.Error("NZB entry should not pass torrent filter")
	}
}

func TestListFilterFunc_HashFilter(t *testing.T) {
	q := newQueueOnly(t)
	fn := q.ListFilterFunc("", config.ProtocolAll, "", []string{"hash1", "hash2"})
	if fn == nil {
		t.Fatal("expected non-nil filter for hash constraint")
	}

	inSet := &storage.Entry{InfoHash: "hash1"}
	notInSet := &storage.Entry{InfoHash: "hash3"}

	if !fn(inSet) {
		t.Error("entry with hash in set should pass")
	}
	if fn(notInSet) {
		t.Error("entry with hash not in set should not pass")
	}
}

func TestListFilterFunc_StateFilter(t *testing.T) {
	q := newQueueOnly(t)
	fn := q.ListFilterFunc("", config.ProtocolAll, storage.EntryStateDownloading, nil)
	if fn == nil {
		t.Fatal("expected non-nil filter for state constraint")
	}

	downloading := &storage.Entry{State: storage.EntryStateDownloading}
	completed := &storage.Entry{State: storage.EntryStatePausedUP}

	if !fn(downloading) {
		t.Error("downloading entry should pass state filter")
	}
	if fn(completed) {
		t.Error("completed entry should not pass downloading state filter")
	}
}

func TestListFilterFunc_CombinedFilters(t *testing.T) {
	q := newQueueOnly(t)
	fn := q.ListFilterFunc("movies", config.ProtocolTorrent, storage.EntryStateDownloading, []string{"abc"})
	if fn == nil {
		t.Fatal("expected non-nil combined filter")
	}

	// All conditions met
	perfect := &storage.Entry{
		Category: "movies",
		Protocol: config.ProtocolTorrent,
		State:    storage.EntryStateDownloading,
		InfoHash: "abc",
	}
	if !fn(perfect) {
		t.Error("entry satisfying all conditions should pass")
	}

	// One condition fails (wrong category)
	wrongCat := &storage.Entry{
		Category: "tv",
		Protocol: config.ProtocolTorrent,
		State:    storage.EntryStateDownloading,
		InfoHash: "abc",
	}
	if fn(wrongCat) {
		t.Error("entry with wrong category should not pass")
	}
}
