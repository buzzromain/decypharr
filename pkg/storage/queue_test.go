package storage

import (
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	debridTypes "github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// newTestStorage creates a real Storage backed by a temp dir for tests.
func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStorage(dir)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func makeQueuedEntry(infohash, category string) *Entry {
	now := time.Now()
	return &Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       infohash,
		Name:           "Movie " + infohash,
		Category:       category,
		ActiveProvider: "rd",
		State:          EntryStateDownloading,
		AddedOn:        now,
		CreatedAt:      now,
		Providers: map[string]*ProviderEntry{
			"rd": {
				Provider: "rd",
				ID:       "id-" + infohash,
				Status:   debridTypes.TorrentStatusDownloaded,
				Files:    map[string]*ProviderFile{"f.mkv": {Id: "1", Link: "http://x"}},
			},
		},
		Files: map[string]*File{"f.mkv": {Name: "f.mkv"}},
	}
}

// ── UpdateWhereQueued nil predicate ───────────────────────────────────────────

// TestUpdateWhereQueued_NilPredicate_UpdatesAllEntries demonstrates that
// UpdateWhereQueued with a nil predicate updates ALL queued entries.
//
// This is the underlying storage behavior that makes the bug in
// pkg/server/qbit/http.go handleSetCategory() possible:
// the handler builds a hash set but never assigns it to filterFunc,
// so filterFunc is always nil, causing ALL torrents to be recategorized
// regardless of the requested hashes.
func TestUpdateWhereQueued_NilPredicate_UpdatesAllEntries(t *testing.T) {
	s := newTestStorage(t)

	e1 := makeQueuedEntry("hash1", "sonarr")
	e2 := makeQueuedEntry("hash2", "radarr")
	e3 := makeQueuedEntry("hash3", "lidarr")

	for _, e := range []*Entry{e1, e2, e3} {
		if err := s.AddQueue(e); err != nil {
			t.Fatalf("AddQueue(%s): %v", e.InfoHash, err)
		}
	}

	// Simulate handleSetCategory bug: filterFunc is nil (should filter to hash1 only),
	// updateFunc sets category to "new-category".
	updateFunc := func(e *Entry) bool {
		e.Category = "new-category"
		return true
	}

	if err := s.UpdateWhereQueued(nil, updateFunc); err != nil {
		t.Fatalf("UpdateWhereQueued: %v", err)
	}

	// All three entries should now have "new-category" — not just hash1.
	for _, hash := range []string{"hash1", "hash2", "hash3"} {
		entry, err := s.GetQueued(hash)
		if err != nil {
			t.Fatalf("GetQueued(%s): %v", hash, err)
		}
		if entry.Category != "new-category" {
			t.Errorf("entry %s category = %q, want %q (nil predicate should update all)",
				hash, entry.Category, "new-category")
		}
	}

	// This confirms the handleSetCategory bug:
	// pkg/server/qbit/http.go handleSetCategory always passes nil filterFunc to
	// Queue.UpdateWhere, so it recategorizes EVERY torrent in the queue, ignoring
	// the hashes parameter sent by the *Arr client.
	t.Log("BUG DOCUMENTED: nil predicate in UpdateWhereQueued updates all entries. " +
		"handleSetCategory in pkg/server/qbit/http.go never assigns filterFunc, " +
		"causing all torrents to be recategorized regardless of requested hashes.")
}

// TestUpdateWhereQueued_WithPredicate_FiltersCorrectly demonstrates the
// correct behavior when a non-nil predicate is provided: only matching
// entries are updated.
func TestUpdateWhereQueued_WithPredicate_FiltersCorrectly(t *testing.T) {
	s := newTestStorage(t)

	e1 := makeQueuedEntry("hash1", "sonarr")
	e2 := makeQueuedEntry("hash2", "radarr")

	for _, e := range []*Entry{e1, e2} {
		if err := s.AddQueue(e); err != nil {
			t.Fatalf("AddQueue: %v", err)
		}
	}

	// Only update hash1
	filterFunc := func(e *Entry) bool { return e.InfoHash == "hash1" }
	updateFunc := func(e *Entry) bool {
		e.Category = "updated"
		return true
	}

	if err := s.UpdateWhereQueued(filterFunc, updateFunc); err != nil {
		t.Fatalf("UpdateWhereQueued: %v", err)
	}

	got1, _ := s.GetQueued("hash1")
	got2, _ := s.GetQueued("hash2")

	if got1.Category != "updated" {
		t.Errorf("hash1 category = %q, want %q", got1.Category, "updated")
	}
	if got2.Category != "radarr" {
		t.Errorf("hash2 category = %q, want %q (should be unchanged)", got2.Category, "radarr")
	}
}

// ── DeleteWhereQueued nil predicate ───────────────────────────────────────────

// TestDeleteWhereQueued_NilPredicate_DeletesAll verifies that a nil predicate
// deletes every queued entry — analogous to the UpdateWhere nil behavior.
func TestDeleteWhereQueued_NilPredicate_DeletesAll(t *testing.T) {
	s := newTestStorage(t)

	for _, h := range []string{"a1", "a2", "a3"} {
		e := makeQueuedEntry(h, "cat")
		if err := s.AddQueue(e); err != nil {
			t.Fatalf("AddQueue: %v", err)
		}
	}

	if err := s.DeleteWhereQueued(nil, nil); err != nil {
		t.Fatalf("DeleteWhereQueued: %v", err)
	}

	for _, h := range []string{"a1", "a2", "a3"} {
		if _, err := s.GetQueued(h); err == nil {
			t.Errorf("expected entry %s to be deleted, but it still exists", h)
		}
	}
}

// ── FilterQueued nil filter ────────────────────────────────────────────────────

// TestFilterQueued_NilFilter_ReturnsAll verifies that a nil filter returns all entries.
func TestFilterQueued_NilFilter_ReturnsAll(t *testing.T) {
	s := newTestStorage(t)

	for _, h := range []string{"b1", "b2"} {
		e := makeQueuedEntry(h, "cat")
		if err := s.AddQueue(e); err != nil {
			t.Fatalf("AddQueue: %v", err)
		}
	}

	results, err := s.FilterQueued(nil)
	if err != nil {
		t.Fatalf("FilterQueued: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("FilterQueued(nil) returned %d entries, want 2", len(results))
	}
}

// ── AddQueue / GetQueued roundtrip ────────────────────────────────────────────

func TestAddAndGetQueued_RoundTrip(t *testing.T) {
	s := newTestStorage(t)

	e := makeQueuedEntry("roundtrip1", "sonarr")
	e.Tags = []string{"hd", "encode"}
	e.LastError = "previous error"

	if err := s.AddQueue(e); err != nil {
		t.Fatalf("AddQueue: %v", err)
	}

	got, err := s.GetQueued("roundtrip1")
	if err != nil {
		t.Fatalf("GetQueued: %v", err)
	}

	if got.InfoHash != "roundtrip1" {
		t.Errorf("InfoHash = %q, want %q", got.InfoHash, "roundtrip1")
	}
	if got.Category != "sonarr" {
		t.Errorf("Category = %q, want %q", got.Category, "sonarr")
	}
	if got.LastError != "previous error" {
		t.Errorf("LastError = %q, want %q", got.LastError, "previous error")
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(got.Tags))
	}
}

// TestDeleteQueued_NotFound verifies that deleting a non-existent entry does
// not panic.
func TestDeleteQueued_NotFound(t *testing.T) {
	s := newTestStorage(t)
	// deleting a key that doesn't exist should not error out or panic
	_ = s.DeleteQueued("doesnotexist", nil)
}

// TestUpdateQueue_Persists verifies that UpdateQueue persists the changes.
func TestUpdateQueue_Persists(t *testing.T) {
	s := newTestStorage(t)
	e := makeQueuedEntry("upd1", "sonarr")
	if err := s.AddQueue(e); err != nil {
		t.Fatalf("AddQueue: %v", err)
	}

	e.Category = "radarr"
	e.State = EntryStatePausedUP
	if err := s.UpdateQueue(e); err != nil {
		t.Fatalf("UpdateQueue: %v", err)
	}

	got, err := s.GetQueued("upd1")
	if err != nil {
		t.Fatalf("GetQueued: %v", err)
	}
	if got.Category != "radarr" {
		t.Errorf("Category = %q, want %q", got.Category, "radarr")
	}
	if got.State != EntryStatePausedUP {
		t.Errorf("State = %q, want %q", got.State, EntryStatePausedUP)
	}
}
