package vfs

import (
	"io"
	"testing"
)

func TestStreamingFile_CloseIsIdempotentAndPreventsReads(t *testing.T) {
	item := &CacheItem{info: ItemInfo{Size: 128}}

	f := NewStreamingFile(item)
	if got := item.opens.Load(); got != 1 {
		t.Fatalf("opens after NewStreamingFile = %d, want 1", got)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("first close error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("second close error = %v", err)
	}
	if got := item.opens.Load(); got != 0 {
		t.Fatalf("opens after double close = %d, want 0", got)
	}

	buf := make([]byte, 16)
	if _, err := f.ReadAt(buf, 0); err == nil || err.Error() != "file closed" {
		t.Fatalf("ReadAt after close error = %v, want file closed", err)
	}
}

func TestStreamingFile_ReadAtBeyondEOF(t *testing.T) {
	item := &CacheItem{info: ItemInfo{Size: 10}}
	f := NewStreamingFile(item)
	t.Cleanup(func() { _ = f.Close() })

	buf := make([]byte, 4)
	n, err := f.ReadAt(buf, 10)
	if n != 0 {
		t.Fatalf("n = %d, want 0", n)
	}
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
}
