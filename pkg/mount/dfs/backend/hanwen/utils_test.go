//go:build !windows

package hanwen

import (
	"testing"
)

func TestHashPath_Deterministic(t *testing.T) {
	// Same input must always produce same hash
	path := "movies/inception/inception.mkv"
	h1 := hashPath(path)
	h2 := hashPath(path)
	if h1 != h2 {
		t.Errorf("hashPath(%q) not deterministic: %d != %d", path, h1, h2)
	}
}

func TestHashPath_DifferentPaths(t *testing.T) {
	// Different paths should produce different hashes (collision is possible
	// but extremely unlikely for these simple cases)
	paths := []string{
		"movies/a.mkv",
		"movies/b.mkv",
		"series/a.mkv",
		"",
		"/",
		"movies",
	}
	seen := make(map[uint64]string)
	for _, p := range paths {
		h := hashPath(p)
		if prev, ok := seen[h]; ok {
			t.Errorf("hash collision: %q and %q both hash to %d", prev, p, h)
		}
		seen[h] = p
	}
}

func TestHashPath_MinimumValue(t *testing.T) {
	// hashPath guarantees result > 1 (inodes 0 and 1 are reserved in FUSE)
	tests := []string{"", "a", "/", "test/path", "\x00"}
	for _, p := range tests {
		h := hashPath(p)
		if h <= 1 {
			t.Errorf("hashPath(%q) = %d, must be > 1 (reserved inodes)", p, h)
		}
	}
}

func TestHashPath_LongPath(t *testing.T) {
	// Should not panic on very long paths
	long := make([]byte, 10000)
	for i := range long {
		long[i] = byte('a' + (i % 26))
	}
	h := hashPath(string(long))
	if h <= 1 {
		t.Errorf("hashPath(long) = %d, must be > 1", h)
	}
}
