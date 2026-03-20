package hybrid

import (
	"fmt"
	"testing"
)

func TestLRUCache_PutAndGet(t *testing.T) {
	t.Parallel()
	c := newLRUCache(10)

	c.Put("key1", []byte("value1"))
	got, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got) != "value1" {
		t.Errorf("expected value1, got %s", string(got))
	}
}

func TestLRUCache_GetMiss(t *testing.T) {
	t.Parallel()
	c := newLRUCache(10)

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	t.Parallel()
	c := newLRUCache(3)

	c.Put("a", []byte("1"))
	c.Put("b", []byte("2"))
	c.Put("c", []byte("3"))
	c.Put("d", []byte("4")) // Should evict "a" (oldest)

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}
	if _, ok := c.Get("d"); !ok {
		t.Error("expected 'd' to be present")
	}
	if c.Len() != 3 {
		t.Errorf("expected len 3, got %d", c.Len())
	}
}

func TestLRUCache_EvictionOrder(t *testing.T) {
	t.Parallel()
	c := newLRUCache(3)

	c.Put("a", []byte("1"))
	c.Put("b", []byte("2"))
	c.Put("c", []byte("3"))

	// Access "a" to move it to front
	c.Get("a")

	// Insert "d" — should evict "b" (least recently used), not "a"
	c.Put("d", []byte("4"))

	if _, ok := c.Get("b"); ok {
		t.Error("expected 'b' to be evicted (LRU)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("expected 'a' to still be present (recently accessed)")
	}
}

func TestLRUCache_PutOverwrite(t *testing.T) {
	t.Parallel()
	c := newLRUCache(3)

	c.Put("a", []byte("old"))
	c.Put("a", []byte("new"))

	got, ok := c.Get("a")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got) != "new" {
		t.Errorf("expected 'new', got '%s'", string(got))
	}
	if c.Len() != 1 {
		t.Errorf("expected len 1 after overwrite, got %d", c.Len())
	}
}

func TestLRUCache_Remove(t *testing.T) {
	t.Parallel()
	c := newLRUCache(10)

	c.Put("a", []byte("1"))
	c.Remove("a")

	if _, ok := c.Get("a"); ok {
		t.Error("expected cache miss after remove")
	}
	if c.Len() != 0 {
		t.Errorf("expected len 0, got %d", c.Len())
	}

	// Remove nonexistent key — should not panic
	c.Remove("nonexistent")
}

func TestLRUCache_Clear(t *testing.T) {
	t.Parallel()
	c := newLRUCache(10)

	c.Put("a", []byte("1"))
	c.Put("b", []byte("2"))
	c.Clear()

	if c.Len() != 0 {
		t.Errorf("expected len 0 after clear, got %d", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Error("expected miss after clear")
	}
}

func TestLRUCache_SetCapacity_Shrink(t *testing.T) {
	t.Parallel()
	c := newLRUCache(5)

	for i := range 5 {
		c.Put(fmt.Sprintf("k%d", i), []byte("v"))
	}

	c.SetCapacity(2)
	if c.Len() != 2 {
		t.Errorf("expected len 2 after shrink, got %d", c.Len())
	}
}

func TestLRUCache_SetCapacity_Grow(t *testing.T) {
	t.Parallel()
	c := newLRUCache(3)

	c.Put("a", []byte("1"))
	c.Put("b", []byte("2"))
	c.Put("c", []byte("3"))

	c.SetCapacity(10)
	if c.Len() != 3 {
		t.Errorf("expected len 3 after grow, got %d", c.Len())
	}
	// All items should still be accessible
	for _, k := range []string{"a", "b", "c"} {
		if _, ok := c.Get(k); !ok {
			t.Errorf("expected '%s' to still be present after grow", k)
		}
	}
}
