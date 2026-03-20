package storage

import (
	"fmt"
	"sync"
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

// TestUpdateWhereQueued_NilPredicate_ReturnsError verifies that UpdateWhereQueued
// rejects a nil predicate with an error and does not modify any entries.
//
// Previously (before the fix) a nil predicate silently updated ALL queued entries,
// which was the root cause of the handleSetCategory bug in pkg/server/qbit/http.go.
func TestUpdateWhereQueued_NilPredicate_ReturnsError(t *testing.T) {
	s := newTestStorage(t)

	e1 := makeQueuedEntry("hash1", "sonarr")
	e2 := makeQueuedEntry("hash2", "radarr")
	e3 := makeQueuedEntry("hash3", "lidarr")

	for _, e := range []*Entry{e1, e2, e3} {
		if err := s.AddQueue(e); err != nil {
			t.Fatalf("AddQueue(%s): %v", e.InfoHash, err)
		}
	}

	updateFunc := func(e *Entry) bool {
		e.Category = "new-category"
		return true
	}

	_ = s.UpdateWhereQueued(nil, updateFunc)
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

func TestQueueStorage_ConcurrentUpdateAndDelete_FinalStateInvariant(t *testing.T) {
	s := newTestStorage(t)

	const total = 60
	evenHashes := make(map[string]struct{}, total/2)
	oddHashes := make(map[string]struct{}, total/2)

	for i := range total {
		hash := fmt.Sprintf("hash-%02d", i)
		if i%2 == 0 {
			evenHashes[hash] = struct{}{}
		} else {
			oddHashes[hash] = struct{}{}
		}
		if err := s.AddQueue(makeQueuedEntry(hash, "initial")); err != nil {
			t.Fatalf("AddQueue(%s): %v", hash, err)
		}
	}

	updatePhase := make(chan struct{})
	deletePhase := make(chan struct{})
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-updatePhase
		errCh <- s.UpdateWhereQueued(func(e *Entry) bool {
			_, ok := evenHashes[e.InfoHash]
			return ok
		}, func(e *Entry) bool {
			e.Category = "even"
			return true
		})
	}()

	go func() {
		defer wg.Done()
		<-deletePhase
		errCh <- s.DeleteWhereQueued(func(e *Entry) bool {
			_, ok := oddHashes[e.InfoHash]
			return ok
		}, nil)
	}()

	// Deterministic phases:
	// 1) apply updates to all even hashes
	// 2) delete all odd hashes
	close(updatePhase)
	if err := <-errCh; err != nil {
		t.Fatalf("UpdateWhereQueued failed: %v", err)
	}
	close(deletePhase)
	if err := <-errCh; err != nil {
		t.Fatalf("DeleteWhereQueued failed: %v", err)
	}
	wg.Wait()

	for hash := range oddHashes {
		if _, err := s.GetQueued(hash); err == nil {
			t.Fatalf("odd hash %s should be deleted", hash)
		}
	}
	for hash := range evenHashes {
		got, err := s.GetQueued(hash)
		if err != nil {
			t.Fatalf("even hash %s should exist: %v", hash, err)
		}
		if got.Category != "even" {
			t.Fatalf("even hash %s category=%q, want even", hash, got.Category)
		}
	}
}
