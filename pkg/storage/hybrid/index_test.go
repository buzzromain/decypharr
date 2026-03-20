package hybrid

import (
	"errors"
	"sort"
	"testing"
)

func TestIndex_PutAndGet(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	entry := &IndexEntry{
		Offset:   100,
		Size:     50,
		Category: "sonarr",
		Provider: "realdebrid",
		Status:   "completed",
		Name:     "test-entry",
	}
	idx.Put("key1", entry)

	got := idx.Get("key1")
	if got == nil {
		t.Fatal("expected entry, got nil")
	}
	if got.Category != "sonarr" {
		t.Errorf("expected category sonarr, got %s", got.Category)
	}
	if got.Provider != "realdebrid" {
		t.Errorf("expected provider realdebrid, got %s", got.Provider)
	}
	if got.Offset != 100 {
		t.Errorf("expected offset 100, got %d", got.Offset)
	}
}

func TestIndex_GetMiss(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	if got := idx.Get("nonexistent"); got != nil {
		t.Errorf("expected nil for missing key, got %+v", got)
	}
}

func TestIndex_PutOverwrite(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("key1", &IndexEntry{Category: "sonarr", Provider: "rd"})
	idx.Put("key1", &IndexEntry{Category: "radarr", Provider: "tb"})

	got := idx.Get("key1")
	if got.Category != "radarr" {
		t.Errorf("expected category radarr after overwrite, got %s", got.Category)
	}

	// Old secondary indexes should be cleaned up
	if keys := idx.GetByCategory("sonarr"); len(keys) != 0 {
		t.Errorf("expected sonarr category to be empty after overwrite, got %v", keys)
	}
	if keys := idx.GetByCategory("radarr"); len(keys) != 1 {
		t.Errorf("expected 1 key in radarr category, got %d", len(keys))
	}
	if keys := idx.GetByProvider("rd"); len(keys) != 0 {
		t.Errorf("expected rd provider to be empty after overwrite, got %v", keys)
	}
}

func TestIndex_Delete(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("key1", &IndexEntry{Category: "sonarr"})
	idx.Delete("key1")

	if got := idx.Get("key1"); got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
	if idx.Len() != 0 {
		t.Errorf("expected len 0, got %d", idx.Len())
	}
}

func TestIndex_DeleteNonExistent(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	// Should not panic
	idx.Delete("nonexistent")
	if idx.Len() != 0 {
		t.Errorf("expected len 0, got %d", idx.Len())
	}
}

func TestIndex_Len(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	if idx.Len() != 0 {
		t.Errorf("expected initial len 0, got %d", idx.Len())
	}

	idx.Put("a", &IndexEntry{})
	idx.Put("b", &IndexEntry{})
	idx.Put("c", &IndexEntry{})
	if idx.Len() != 3 {
		t.Errorf("expected len 3, got %d", idx.Len())
	}

	idx.Delete("b")
	if idx.Len() != 2 {
		t.Errorf("expected len 2 after delete, got %d", idx.Len())
	}
}

func TestIndex_Keys(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("c", &IndexEntry{})
	idx.Put("a", &IndexEntry{})
	idx.Put("b", &IndexEntry{})

	keys := idx.Keys()
	sort.Strings(keys)
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("expected [a b c], got %v", keys)
	}
}

func TestIndex_ForEach(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("a", &IndexEntry{Name: "alpha"})
	idx.Put("b", &IndexEntry{Name: "beta"})
	idx.Put("c", &IndexEntry{Name: "gamma"})

	visited := make(map[string]string)
	err := idx.ForEach(func(key string, entry *IndexEntry) error {
		visited[key] = entry.Name
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(visited) != 3 {
		t.Errorf("expected 3 visited entries, got %d", len(visited))
	}
}

func TestIndex_ForEach_EarlyExit(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("a", &IndexEntry{})
	idx.Put("b", &IndexEntry{})
	idx.Put("c", &IndexEntry{})

	sentinel := errors.New("stop")
	err := idx.ForEach(func(key string, entry *IndexEntry) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestIndex_GetByCategory(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("a", &IndexEntry{Category: "sonarr"})
	idx.Put("b", &IndexEntry{Category: "radarr"})
	idx.Put("c", &IndexEntry{Category: "sonarr"})

	sonarr := idx.GetByCategory("sonarr")
	sort.Strings(sonarr)
	if len(sonarr) != 2 || sonarr[0] != "a" || sonarr[1] != "c" {
		t.Errorf("expected [a c], got %v", sonarr)
	}

	radarr := idx.GetByCategory("radarr")
	if len(radarr) != 1 || radarr[0] != "b" {
		t.Errorf("expected [b], got %v", radarr)
	}

	if keys := idx.GetByCategory("lidarr"); keys != nil {
		t.Errorf("expected nil for missing category, got %v", keys)
	}
}

func TestIndex_GetByProvider(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("a", &IndexEntry{Provider: "realdebrid"})
	idx.Put("b", &IndexEntry{Provider: "torbox"})
	idx.Put("c", &IndexEntry{Provider: "realdebrid"})

	rd := idx.GetByProvider("realdebrid")
	sort.Strings(rd)
	if len(rd) != 2 || rd[0] != "a" || rd[1] != "c" {
		t.Errorf("expected [a c], got %v", rd)
	}
}

func TestIndex_Categories(t *testing.T) {
	t.Parallel()
	idx := newIndex()

	idx.Put("a", &IndexEntry{Category: "sonarr"})
	idx.Put("b", &IndexEntry{Category: "radarr"})
	idx.Put("c", &IndexEntry{Category: "sonarr"})

	cats := idx.Categories()
	sort.Strings(cats)
	if len(cats) != 2 || cats[0] != "radarr" || cats[1] != "sonarr" {
		t.Errorf("expected [radarr sonarr], got %v", cats)
	}

	// Delete all sonarr entries, category should disappear
	idx.Delete("a")
	idx.Delete("c")
	cats = idx.Categories()
	if len(cats) != 1 || cats[0] != "radarr" {
		t.Errorf("expected [radarr] after deletes, got %v", cats)
	}
}
