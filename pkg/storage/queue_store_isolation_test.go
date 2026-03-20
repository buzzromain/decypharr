package storage

import (
	"fmt"
	"sync"
	"testing"
)

// ── AddQueue / GetQueued round-trip ───────────────────────────────────────────

func TestStorage_AddQueue_GetQueued_RoundTrip(t *testing.T) {
	s := newTestStorage(t)
	e := makeEntry("q-rt-1", "QueuedMovie")

	if err := s.AddQueue(e); err != nil {
		t.Fatalf("AddQueue: %v", err)
	}
	got, err := s.GetQueued("q-rt-1")
	if err != nil {
		t.Fatalf("GetQueued: %v", err)
	}
	if got.InfoHash != "q-rt-1" {
		t.Errorf("InfoHash = %q, want q-rt-1", got.InfoHash)
	}
}

// ── entries and queue are independent stores ──────────────────────────────────

// TestStorage_EntryAndQueue_AreIndependentStores verifies that an entry stored
// in both the entries store and the queue store is retrievable from each
// independently (writes do not cross-contaminate).
func TestStorage_EntryAndQueue_AreIndependentStores(t *testing.T) {
	s := newTestStorage(t)
	e := makeEntry("dual-hash", "DualMovie")

	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}
	if err := s.AddQueue(e); err != nil {
		t.Fatalf("AddQueue: %v", err)
	}

	if _, err := s.Get("dual-hash"); err != nil {
		t.Errorf("entries.Get error: %v", err)
	}
	if _, err := s.GetQueued("dual-hash"); err != nil {
		t.Errorf("GetQueued error: %v", err)
	}
}

// TestStorage_DeleteEntry_LeavesQueueIntact verifies that deleting an entry
// from the entries store does NOT automatically remove it from the queue store.
// The queue store is an independent store; callers must delete from it separately.
func TestStorage_DeleteEntry_LeavesQueueIntact(t *testing.T) {
	s := newTestStorage(t)
	e := makeEntry("del-keep-q", "DeleteTest")

	_ = s.AddOrUpdate(e)
	_ = s.AddQueue(e)

	if err := s.Delete("del-keep-q"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Entry should be gone from entries store.
	exists, err := s.Exists("del-keep-q")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("entry should be gone from entries store after Delete")
	}

	// Queue entry should still be present (different store, no cascading delete).
	if _, err := s.GetQueued("del-keep-q"); err != nil {
		t.Errorf("queue entry should survive entries deletion: %v", err)
	}
}

// TestStorage_DeleteQueued_RemovesFromQueueOnly verifies that DeleteQueued
// removes from the queue store but leaves the entries store untouched.
func TestStorage_DeleteQueued_RemovesFromQueueOnly(t *testing.T) {
	s := newTestStorage(t)
	e := makeEntry("delq-only", "DelQueueTest")

	_ = s.AddOrUpdate(e)
	_ = s.AddQueue(e)

	if err := s.DeleteQueued("delq-only", nil); err != nil {
		t.Fatalf("DeleteQueued: %v", err)
	}

	// Queue entry must be gone.
	if _, err := s.GetQueued("delq-only"); err == nil {
		t.Error("GetQueued should fail after DeleteQueued")
	}

	// Entries store must still contain the entry.
	got, err := s.Get("delq-only")
	if err != nil {
		t.Errorf("entries store should still contain entry: %v", err)
	}
	if got == nil {
		t.Error("entries.Get returned nil")
	}
}

// TestStorage_UpdateQueue_ReflectsChanges verifies that UpdateQueue persists
// mutations to a queued entry.
func TestStorage_UpdateQueue_ReflectsChanges(t *testing.T) {
	s := newTestStorage(t)
	e := makeEntry("upd-q", "OrigName")

	if err := s.AddQueue(e); err != nil {
		t.Fatalf("AddQueue: %v", err)
	}
	e.Name = "UpdatedName"
	if err := s.UpdateQueue(e); err != nil {
		t.Fatalf("UpdateQueue: %v", err)
	}
	got, err := s.GetQueued("upd-q")
	if err != nil {
		t.Fatalf("GetQueued: %v", err)
	}
	if got.Name != "UpdatedName" {
		t.Errorf("Name = %q, want UpdatedName", got.Name)
	}
}

// TestStorage_DeleteQueued_WithCleanup verifies that the cleanup callback is
// called with the queued entry before deletion.
func TestStorage_DeleteQueued_WithCleanup(t *testing.T) {
	s := newTestStorage(t)
	e := makeEntry("dq-clean", "CleanupTest")
	_ = s.AddQueue(e)

	var cleanupCalled bool
	var cleanupEntry *Entry
	cleanup := func(entry *Entry) error {
		cleanupCalled = true
		cleanupEntry = entry
		return nil
	}

	if err := s.DeleteQueued("dq-clean", cleanup); err != nil {
		t.Fatalf("DeleteQueued with cleanup: %v", err)
	}

	if !cleanupCalled {
		t.Error("cleanup function should have been called")
	}
	if cleanupEntry == nil || cleanupEntry.InfoHash != "dq-clean" {
		t.Errorf("cleanup received wrong entry: %+v", cleanupEntry)
	}
}

// ── Concurrent cross-store operations ────────────────────────────────────────

// TestStorage_ConcurrentQueueAndEntry_NoCorruption verifies that concurrent
// AddOrUpdate + AddQueue operations across many goroutines do not corrupt
// either store — all entries are retrievable from both after settling.
func TestStorage_ConcurrentQueueAndEntry_NoCorruption(t *testing.T) {
	s := newTestStorage(t)

	const workers = 40
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		i := i
		infohash := fmt.Sprintf("con-%03d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			e := makeEntry(infohash, fmt.Sprintf("Movie%d", i))
			_ = s.AddOrUpdate(e)
			_ = s.AddQueue(e)
		}()
	}
	close(start)
	wg.Wait()

	for i := 0; i < workers; i++ {
		infohash := fmt.Sprintf("con-%03d", i)
		if _, err := s.Get(infohash); err != nil {
			t.Errorf("entries.Get(%s) error: %v", infohash, err)
		}
		if _, err := s.GetQueued(infohash); err != nil {
			t.Errorf("GetQueued(%s) error: %v", infohash, err)
		}
	}
}

// TestStorage_ConcurrentCreateAndDeleteQueue_NoOrphans verifies that
// concurrent AddQueue + DeleteQueued cycles leave no ghost entries.
func TestStorage_ConcurrentCreateAndDeleteQueue_NoOrphans(t *testing.T) {
	s := newTestStorage(t)

	const workers = 25
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		i := i
		infohash := fmt.Sprintf("cadq-%03d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			e := makeEntry(infohash, fmt.Sprintf("M%d", i))
			_ = s.AddQueue(e)
			_ = s.DeleteQueued(infohash, nil)
		}()
	}
	close(start)
	wg.Wait()

	// After all workers finish, no queued entry should remain.
	queued, err := s.FilterQueued(nil)
	if err != nil {
		t.Fatalf("FilterQueued: %v", err)
	}
	// We can't guarantee exact count (race between add and delete) but
	// we assert no entry that was created-then-deleted still appears.
	deletedInfohashes := make(map[string]struct{}, workers)
	for i := 0; i < workers; i++ {
		deletedInfohashes[fmt.Sprintf("cadq-%03d", i)] = struct{}{}
	}
	for _, q := range queued {
		if _, ok := deletedInfohashes[q.InfoHash]; ok {
			t.Errorf("found orphan queued entry after delete: %s", q.InfoHash)
		}
	}
}

// TestStorage_AtomicEntryPlusQueueCreate verifies the create+enqueue pattern:
// after both operations succeed the entry is visible in both stores,
// and a subsequent count includes both.
func TestStorage_AtomicEntryPlusQueueCreate(t *testing.T) {
	s := newTestStorage(t)

	e1 := makeEntry("atomic-1", "Film1")
	e2 := makeEntry("atomic-2", "Film2")

	for _, e := range []*Entry{e1, e2} {
		if err := s.AddOrUpdate(e); err != nil {
			t.Fatalf("AddOrUpdate(%s): %v", e.InfoHash, err)
		}
		if err := s.AddQueue(e); err != nil {
			t.Fatalf("AddQueue(%s): %v", e.InfoHash, err)
		}
	}

	count, err := s.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("entries count = %d, want 2", count)
	}

	queueEntries, err := s.FilterQueued(nil)
	if err != nil {
		t.Fatalf("FilterQueued: %v", err)
	}
	if len(queueEntries) != 2 {
		t.Errorf("queue count = %d, want 2", len(queueEntries))
	}
}
