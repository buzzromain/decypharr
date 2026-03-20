package storage

import (
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	debridTypes "github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// makeEntry builds a minimal valid Entry for storage tests.
func makeEntry(infohash, name string) *Entry {
	now := time.Now()
	return &Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       infohash,
		Name:           name,
		Category:       "radarr",
		ActiveProvider: "rd",
		State:          EntryStateDownloading,
		AddedOn:        now,
		CreatedAt:      now,
		Providers: map[string]*ProviderEntry{
			"rd": {
				Provider: "rd",
				ID:       "id-" + infohash,
				Status:   debridTypes.TorrentStatusDownloaded,
				Files:    map[string]*ProviderFile{"movie.mkv": {Id: "1", Link: "http://example.com/movie.mkv"}},
			},
		},
		Files: map[string]*File{"movie.mkv": {Name: "movie.mkv", Size: 1000, InfoHash: infohash}},
	}
}

// ── Storage.AddOrUpdate ───────────────────────────────────────────────────────

func TestAddOrUpdate_RoundTrip(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("hash-aou-1", "The Matrix")
	e.Tags = []string{"1080p", "x265"}
	e.LastError = "transient error"

	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}

	got, err := s.Get("hash-aou-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.InfoHash != "hash-aou-1" {
		t.Errorf("InfoHash = %q, want %q", got.InfoHash, "hash-aou-1")
	}
	if got.Name != "The Matrix" {
		t.Errorf("Name = %q, want %q", got.Name, "The Matrix")
	}
	if got.Category != "radarr" {
		t.Errorf("Category = %q, want %q", got.Category, "radarr")
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(got.Tags))
	}
}

func TestAddOrUpdate_OverwritesExisting(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("hash-aou-2", "Original Name")
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate (initial): %v", err)
	}

	e.Name = "Updated Name"
	e.Category = "sonarr"
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate (update): %v", err)
	}

	got, err := s.Get("hash-aou-2")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Name")
	}
	if got.Category != "sonarr" {
		t.Errorf("Category = %q, want %q", got.Category, "sonarr")
	}
}

func TestAddOrUpdate_SetsUpdatedAt(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("hash-aou-3", "Movie")
	// Proto serialization truncates timestamps to second precision; use a
	// 1-second-aligned window to avoid spurious failures.
	before := time.Now().Truncate(time.Second)
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}
	after := time.Now().Add(time.Second)

	got, err := s.Get("hash-aou-3")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UpdatedAt.Before(before) || got.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt %v not in range [%v, %v]", got.UpdatedAt, before, after)
	}
}

// ── Storage.BatchAddOrUpdate ──────────────────────────────────────────────────

func TestBatchAddOrUpdate_AllPersisted(t *testing.T) {
	s := newTestStorage(t)

	entries := []*Entry{
		makeEntry("batch-1", "Film A"),
		makeEntry("batch-2", "Film B"),
		makeEntry("batch-3", "Film C"),
	}

	if err := s.BatchAddOrUpdate(entries); err != nil {
		t.Fatalf("BatchAddOrUpdate: %v", err)
	}

	for _, e := range entries {
		got, err := s.Get(e.InfoHash)
		if err != nil {
			t.Errorf("Get(%s): %v", e.InfoHash, err)
			continue
		}
		if got.Name != e.Name {
			t.Errorf("Name = %q, want %q", got.Name, e.Name)
		}
	}
}

func TestBatchAddOrUpdate_EmptySlice(t *testing.T) {
	s := newTestStorage(t)
	if err := s.BatchAddOrUpdate(nil); err != nil {
		t.Errorf("BatchAddOrUpdate(nil): %v", err)
	}
	if err := s.BatchAddOrUpdate([]*Entry{}); err != nil {
		t.Errorf("BatchAddOrUpdate([]): %v", err)
	}
}

// ── Storage.Exists ────────────────────────────────────────────────────────────

func TestExists(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("exists-hash", "Movie")
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}

	found, err := s.Exists("exists-hash")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !found {
		t.Error("Exists() = false, want true for added entry")
	}

	missing, err := s.Exists("never-added")
	if err != nil {
		t.Fatalf("Exists (missing): %v", err)
	}
	if missing {
		t.Error("Exists() = true, want false for missing entry")
	}
}

// ── Storage.Get ───────────────────────────────────────────────────────────────

func TestGet_NotFound(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.Get("no-such-hash")
	if err == nil {
		t.Error("Get() expected error for missing entry, got nil")
	}
}

// ── Storage.List ─────────────────────────────────────────────────────────────

func TestList_NoFilter(t *testing.T) {
	s := newTestStorage(t)

	hashes := []string{"list-1", "list-2", "list-3"}
	for _, h := range hashes {
		if err := s.AddOrUpdate(makeEntry(h, "Film "+h)); err != nil {
			t.Fatalf("AddOrUpdate(%s): %v", h, err)
		}
	}

	all, err := s.List(nil)
	if err != nil {
		t.Fatalf("List(nil): %v", err)
	}
	if len(all) != len(hashes) {
		t.Errorf("List(nil) len = %d, want %d", len(all), len(hashes))
	}
}

func TestList_WithFilter(t *testing.T) {
	s := newTestStorage(t)

	e1 := makeEntry("filter-1", "Sonarr Show")
	e1.Category = "sonarr"
	e2 := makeEntry("filter-2", "Radarr Movie")
	e2.Category = "radarr"
	e3 := makeEntry("filter-3", "Another Sonarr Show")
	e3.Category = "sonarr"

	for _, e := range []*Entry{e1, e2, e3} {
		if err := s.AddOrUpdate(e); err != nil {
			t.Fatalf("AddOrUpdate: %v", err)
		}
	}

	sonarr, err := s.List(func(e *Entry) bool { return e.Category == "sonarr" })
	if err != nil {
		t.Fatalf("List(filter): %v", err)
	}
	if len(sonarr) != 2 {
		t.Errorf("List(sonarr) = %d, want 2", len(sonarr))
	}
	for _, e := range sonarr {
		if e.Category != "sonarr" {
			t.Errorf("unexpected category %q in filtered result", e.Category)
		}
	}
}

func TestList_Empty(t *testing.T) {
	s := newTestStorage(t)
	all, err := s.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("List() on empty storage = %d, want 0", len(all))
	}
}

// ── Storage.ForEach ───────────────────────────────────────────────────────────

func TestForEach_VisitsAllEntries(t *testing.T) {
	s := newTestStorage(t)

	hashes := []string{"fe-1", "fe-2", "fe-3"}
	for _, h := range hashes {
		if err := s.AddOrUpdate(makeEntry(h, "Film "+h)); err != nil {
			t.Fatalf("AddOrUpdate: %v", err)
		}
	}

	seen := make(map[string]bool)
	err := s.ForEach(func(e *Entry) error {
		seen[e.InfoHash] = true
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach: %v", err)
	}
	for _, h := range hashes {
		if !seen[h] {
			t.Errorf("ForEach did not visit entry %s", h)
		}
	}
}

func TestForEach_PropagatesError(t *testing.T) {
	s := newTestStorage(t)
	if err := s.AddOrUpdate(makeEntry("fe-err", "Film")); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}

	sentinel := func(e *Entry) error {
		return errSentinel
	}
	err := s.ForEach(sentinel)
	if err == nil {
		t.Error("ForEach should propagate errors from the callback")
	}
}

// errSentinel is a package-level sentinel error for testing.
var errSentinel = &sentinelErr{}

type sentinelErr struct{}

func (e *sentinelErr) Error() string { return "sentinel error" }

// ── Storage.Delete ────────────────────────────────────────────────────────────

func TestDelete_RemovesEntry(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("del-hash", "To Delete")
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}

	// Confirm it exists
	found, err := s.Exists("del-hash")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !found {
		t.Fatal("entry should exist before deletion")
	}

	if err := s.Delete("del-hash"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	gone, err := s.Exists("del-hash")
	if err != nil {
		t.Fatalf("Exists after delete: %v", err)
	}
	if gone {
		t.Error("entry still exists after Delete")
	}
}

func TestDelete_NonExistent_NoError(t *testing.T) {
	s := newTestStorage(t)
	// Deleting a non-existent entry should not panic or hard-fail
	_ = s.Delete("never-existed")
}

// ── Storage.Count ─────────────────────────────────────────────────────────────

func TestCount(t *testing.T) {
	s := newTestStorage(t)

	n0, err := s.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n0 != 0 {
		t.Errorf("Count() on empty storage = %d, want 0", n0)
	}

	for i := range 3 {
		h := "count-hash-" + string(rune('a'+i))
		if err := s.AddOrUpdate(makeEntry(h, "Film")); err != nil {
			t.Fatalf("AddOrUpdate: %v", err)
		}
	}

	n3, err := s.Count()
	if err != nil {
		t.Fatalf("Count after 3 inserts: %v", err)
	}
	if n3 != 3 {
		t.Errorf("Count() = %d, want 3", n3)
	}

	if err := s.Delete("count-hash-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	n2, err := s.Count()
	if err != nil {
		t.Fatalf("Count after delete: %v", err)
	}
	if n2 != 2 {
		t.Errorf("Count() after delete = %d, want 2", n2)
	}
}

// ── Storage.GetEntryItems ─────────────────────────────────────────────────────

func TestGetEntryItems_ReflectsAddedEntries(t *testing.T) {
	s := newTestStorage(t)

	// Entry item keys are derived from GetFolder() which uses the config's
	// FolderNaming. In tests the config uses defaults (WebDavUseFileName).
	e1 := makeEntry("ei-hash-1", "Show S01")
	e2 := makeEntry("ei-hash-2", "Show S02")

	for _, e := range []*Entry{e1, e2} {
		if err := s.AddOrUpdate(e); err != nil {
			t.Fatalf("AddOrUpdate: %v", err)
		}
	}

	items := s.GetEntryItems()
	if len(items) == 0 {
		t.Error("GetEntryItems() returned empty map, want at least 1 entry")
	}
}

// ── Storage.UpdateEntryItem ───────────────────────────────────────────────────

func TestUpdateEntryItem_PopulatesItems(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("uei-hash", "Movie Title")
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}
	if err := s.UpdateEntryItem(e); err != nil {
		t.Fatalf("UpdateEntryItem: %v", err)
	}

	// The folder name drives the key; verify something was written
	items := s.GetEntryItems()
	if len(items) == 0 {
		t.Error("GetEntryItems() empty after UpdateEntryItem")
	}
}

// ── Storage.GetEntryItem ──────────────────────────────────────────────────────

func TestGetEntryItem_RoundTrip(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("gei-hash", "Interstellar")
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}

	folderName := e.GetFolder()
	item, err := s.GetEntryItem(folderName)
	if err != nil {
		t.Fatalf("GetEntryItem(%q): %v", folderName, err)
	}
	if item == nil {
		t.Fatal("GetEntryItem returned nil")
	}
	if item.Name != folderName {
		t.Errorf("EntryItem.Name = %q, want %q", item.Name, folderName)
	}
}

func TestGetEntryItem_NotFound(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.GetEntryItem("no-such-folder")
	if err == nil {
		t.Error("GetEntryItem() expected error for missing item, got nil")
	}
}

// ── Storage.ForEachEntryItem ──────────────────────────────────────────────────

func TestForEachEntryItem_VisitsAll(t *testing.T) {
	s := newTestStorage(t)

	// Add two entries with different names so they produce distinct EntryItems
	e1 := makeEntry("feei-1", "Movie Alpha")
	e2 := makeEntry("feei-2", "Movie Beta")
	for _, e := range []*Entry{e1, e2} {
		if err := s.AddOrUpdate(e); err != nil {
			t.Fatalf("AddOrUpdate: %v", err)
		}
	}

	count := 0
	err := s.ForEachEntryItem(func(item *EntryItem) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachEntryItem: %v", err)
	}
	if count == 0 {
		t.Error("ForEachEntryItem visited 0 items, expected at least 1")
	}
}

func TestForEachEntryItem_PropagatesError(t *testing.T) {
	s := newTestStorage(t)

	e := makeEntry("feei-err", "Film Error")
	if err := s.AddOrUpdate(e); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}

	called := 0
	err := s.ForEachEntryItem(func(item *EntryItem) error {
		called++
		return errSentinel
	})
	if err == nil {
		t.Error("ForEachEntryItem should propagate error from callback")
	}
	if called != 1 {
		t.Errorf("callback called %d times, want 1 (stop on first error)", called)
	}
}
