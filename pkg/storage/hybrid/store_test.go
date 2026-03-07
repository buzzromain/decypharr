package hybrid

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/sirrobot01/decypharr/internal/testutil"
)

func TestMain(m *testing.M) {
	_, cleanup := testutil.SetupTestConfigDir()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(Config{
		DataPath:            filepath.Join(dir, "store.log"),
		CacheSize:           100,
		CompactionThreshold: 0.2,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStore_NewAndClose(t *testing.T) {
	s := newTestStore(t)
	if s == nil {
		t.Fatal("store should not be nil")
	}
}

func TestStore_NewRequiresDataPath(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty DataPath")
	}
}

func TestStore_PutAndGet(t *testing.T) {
	s := newTestStore(t)

	err := s.Put("key1", []byte("hello"), &EntryMeta{Category: "sonarr", Provider: "rd"})
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}

	val, err := s.Get("key1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if string(val) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(val))
	}
}

func TestStore_PutOverwrite(t *testing.T) {
	s := newTestStore(t)

	s.Put("key1", []byte("old"), &EntryMeta{Category: "sonarr"})
	s.Put("key1", []byte("new"), &EntryMeta{Category: "radarr"})

	val, err := s.Get("key1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if string(val) != "new" {
		t.Errorf("expected 'new', got '%s'", string(val))
	}

	// Check that category was updated in index
	if keys := s.FilterByCategory("sonarr"); len(keys) != 0 {
		t.Errorf("expected no keys in sonarr, got %v", keys)
	}
	if keys := s.FilterByCategory("radarr"); len(keys) != 1 {
		t.Errorf("expected 1 key in radarr, got %d", len(keys))
	}
}

func TestStore_Delete(t *testing.T) {
	s := newTestStore(t)

	s.Put("key1", []byte("data"), nil)
	err := s.Delete("key1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if s.Exists("key1") {
		t.Error("key should not exist after delete")
	}

	_, err = s.Get("key1")
	if err == nil {
		t.Error("expected error when getting deleted key")
	}
}

func TestStore_DeleteNonExistent(t *testing.T) {
	s := newTestStore(t)

	err := s.Delete("nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent key")
	}
}

func TestStore_Exists(t *testing.T) {
	s := newTestStore(t)

	if s.Exists("key1") {
		t.Error("key should not exist before put")
	}

	s.Put("key1", []byte("data"), nil)
	if !s.Exists("key1") {
		t.Error("key should exist after put")
	}
}

func TestStore_Len(t *testing.T) {
	s := newTestStore(t)

	if s.Len() != 0 {
		t.Errorf("expected initial len 0, got %d", s.Len())
	}

	s.Put("a", []byte("1"), nil)
	s.Put("b", []byte("2"), nil)
	if s.Len() != 2 {
		t.Errorf("expected len 2, got %d", s.Len())
	}

	s.Delete("a")
	if s.Len() != 1 {
		t.Errorf("expected len 1, got %d", s.Len())
	}
}

func TestStore_Keys(t *testing.T) {
	s := newTestStore(t)

	s.Put("c", []byte("3"), nil)
	s.Put("a", []byte("1"), nil)
	s.Put("b", []byte("2"), nil)

	keys := s.Keys()
	sort.Strings(keys)
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("expected [a b c], got %v", keys)
	}
}

func TestStore_ForEach(t *testing.T) {
	s := newTestStore(t)

	s.Put("a", []byte("alpha"), nil)
	s.Put("b", []byte("beta"), nil)

	visited := make(map[string]string)
	err := s.ForEach(func(key string, value []byte) error {
		visited[key] = string(value)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}
	if len(visited) != 2 {
		t.Errorf("expected 2 entries, got %d", len(visited))
	}
	if visited["a"] != "alpha" || visited["b"] != "beta" {
		t.Errorf("unexpected values: %v", visited)
	}
}

func TestStore_FilterByCategory(t *testing.T) {
	s := newTestStore(t)

	s.Put("a", []byte("1"), &EntryMeta{Category: "sonarr"})
	s.Put("b", []byte("2"), &EntryMeta{Category: "radarr"})
	s.Put("c", []byte("3"), &EntryMeta{Category: "sonarr"})

	sonarr := s.FilterByCategory("sonarr")
	sort.Strings(sonarr)
	if len(sonarr) != 2 || sonarr[0] != "a" || sonarr[1] != "c" {
		t.Errorf("expected [a c], got %v", sonarr)
	}
}

func TestStore_FilterByProvider(t *testing.T) {
	s := newTestStore(t)

	s.Put("a", []byte("1"), &EntryMeta{Provider: "rd"})
	s.Put("b", []byte("2"), &EntryMeta{Provider: "tb"})
	s.Put("c", []byte("3"), &EntryMeta{Provider: "rd"})

	rd := s.FilterByProvider("rd")
	sort.Strings(rd)
	if len(rd) != 2 || rd[0] != "a" || rd[1] != "c" {
		t.Errorf("expected [a c], got %v", rd)
	}
}

func TestStore_CacheInvalidationOnWrite(t *testing.T) {
	s := newTestStore(t)

	s.Put("key1", []byte("old"), nil)

	// Read to populate cache
	val, _ := s.Get("key1")
	if string(val) != "old" {
		t.Fatalf("expected 'old', got '%s'", string(val))
	}

	// Overwrite
	s.Put("key1", []byte("new"), nil)

	// Read again — should get new value, not cached old
	val, _ = s.Get("key1")
	if string(val) != "new" {
		t.Errorf("expected 'new' after overwrite, got '%s'", string(val))
	}
}

func TestStore_NeedsCompaction(t *testing.T) {
	s := newTestStore(t)

	// Store with only live entries should not need compaction
	// (add some entries so liveSize is close to logSize)
	for i := range 5 {
		key := string(rune('a' + i))
		s.Put(key, make([]byte, 1000), nil)
	}
	if s.NeedsCompaction() {
		t.Error("store with only live entries should not need compaction")
	}

	// Add and delete many entries to build up dead space
	for i := range 20 {
		key := string(rune('a' + i%26))
		s.Put(key, make([]byte, 1000), nil)
	}
	for i := range 20 {
		key := string(rune('a' + i%26))
		s.Delete(key)
	}

	// Now add one live entry so we have mostly dead space
	s.Put("live", []byte("alive"), nil)
	if !s.NeedsCompaction() {
		t.Error("expected compaction needed after many deletes")
	}
}

func TestStore_Compact(t *testing.T) {
	s := newTestStore(t)

	// Add entries
	s.Put("keep1", []byte("data1"), &EntryMeta{Category: "sonarr"})
	s.Put("keep2", []byte("data2"), &EntryMeta{Category: "radarr"})
	s.Put("delete", []byte("gone"), nil)
	s.Delete("delete")

	sizeBefore := s.DiskSize()

	err := s.Compact()
	if err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	sizeAfter := s.DiskSize()
	if sizeAfter >= sizeBefore {
		t.Errorf("expected disk size to shrink after compaction: before=%d, after=%d", sizeBefore, sizeAfter)
	}

	// Live entries should survive
	val, err := s.Get("keep1")
	if err != nil || string(val) != "data1" {
		t.Errorf("keep1 missing after compaction: val=%s, err=%v", string(val), err)
	}
	val, err = s.Get("keep2")
	if err != nil || string(val) != "data2" {
		t.Errorf("keep2 missing after compaction: val=%s, err=%v", string(val), err)
	}

	// Category filter should still work
	if keys := s.FilterByCategory("sonarr"); len(keys) != 1 {
		t.Errorf("expected 1 sonarr key after compaction, got %d", len(keys))
	}
}

func TestStore_RecoveryAfterReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.log")

	// Create, write, close
	s1, err := New(Config{DataPath: path, CacheSize: 100})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	s1.Put("k1", []byte("v1"), &EntryMeta{Category: "sonarr"})
	s1.Put("k2", []byte("v2"), &EntryMeta{Category: "radarr"})
	s1.Put("k3", []byte("v3"), nil)
	s1.Delete("k3")
	s1.Close()

	// Reopen
	s2, err := New(Config{DataPath: path, CacheSize: 100})
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer s2.Close()

	if s2.Len() != 2 {
		t.Errorf("expected 2 entries after reopen, got %d", s2.Len())
	}

	val, err := s2.Get("k1")
	if err != nil || string(val) != "v1" {
		t.Errorf("k1 recovery failed: val=%s, err=%v", string(val), err)
	}

	if !s2.Exists("k2") {
		t.Error("k2 should exist after reopen")
	}
	if s2.Exists("k3") {
		t.Error("k3 should not exist after reopen (was deleted)")
	}

	// Secondary indexes should be recovered
	if keys := s2.FilterByCategory("sonarr"); len(keys) != 1 {
		t.Errorf("expected 1 sonarr key after recovery, got %d", len(keys))
	}
}

func TestStore_ConcurrentReadWrite(t *testing.T) {
	s := newTestStore(t)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			_ = s.Put(key, []byte("data"), &EntryMeta{Category: "test"})
			s.Get(key)
			s.Exists(key)
			if i%5 == 0 {
				s.Delete(key)
			}
		}(i)
	}
	wg.Wait()
}
