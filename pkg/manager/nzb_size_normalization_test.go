package manager

import (
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/pkg/storage"
)

// ── ptrTime ───────────────────────────────────────────────────────────────────

func TestPtrTime(t *testing.T) {
	now := time.Now()
	p := ptrTime(now)
	if p == nil {
		t.Fatal("ptrTime should return non-nil pointer")
	}
	if !p.Equal(now) {
		t.Errorf("ptrTime = %v, want %v", *p, now)
	}
}

func TestPtrTime_IndependentCopy(t *testing.T) {
	t1 := time.Now()
	p := ptrTime(t1)
	// Mutating the original time doesn't affect the pointer
	t1 = t1.Add(time.Hour)
	if p.Equal(t1) {
		t.Error("ptrTime should store an independent copy, not a reference")
	}
}

// ── streamSizeFromSegments ────────────────────────────────────────────────────

func TestStreamSizeFromSegments_Empty(t *testing.T) {
	got := streamSizeFromSegments(nil)
	if got != 0 {
		t.Errorf("streamSizeFromSegments(nil) = %d, want 0", got)
	}
	got = streamSizeFromSegments([]storage.NZBSegment{})
	if got != 0 {
		t.Errorf("streamSizeFromSegments([]) = %d, want 0", got)
	}
}

func TestStreamSizeFromSegments_UsesMaxEndOffset(t *testing.T) {
	// When EndOffset is set, maxEnd takes priority over sum of Bytes
	segs := []storage.NZBSegment{
		{Bytes: 100, EndOffset: 199}, // EndOffset+1 = 200
		{Bytes: 100, EndOffset: 399}, // EndOffset+1 = 400
	}
	got := streamSizeFromSegments(segs)
	if got != 400 {
		t.Errorf("streamSizeFromSegments = %d, want 400 (max EndOffset+1)", got)
	}
}

func TestStreamSizeFromSegments_FallsBackToByteSum(t *testing.T) {
	// EndOffset=-1 → EndOffset+1=0, never > maxEnd=0, so sum path is taken
	segs := []storage.NZBSegment{
		{Bytes: 100, EndOffset: -1},
		{Bytes: 200, EndOffset: -1},
		{Bytes: 50, EndOffset: -1},
	}
	got := streamSizeFromSegments(segs)
	if got != 350 {
		t.Errorf("streamSizeFromSegments = %d, want 350 (sum of Bytes)", got)
	}
}

func TestStreamSizeFromSegments_ZeroBytesSkipped(t *testing.T) {
	// Bytes == 0 are excluded from sum; use EndOffset=-1 to force sum path
	segs := []storage.NZBSegment{
		{Bytes: 0, EndOffset: -1},
		{Bytes: 300, EndOffset: -1},
	}
	got := streamSizeFromSegments(segs)
	if got != 300 {
		t.Errorf("streamSizeFromSegments = %d, want 300", got)
	}
}

// ── normalizeNZBFileSizes ─────────────────────────────────────────────────────

func TestNormalizeNZBFileSizes_Nil(t *testing.T) {
	changed, total := normalizeNZBFileSizes(nil)
	if changed || total != 0 {
		t.Errorf("normalizeNZBFileSizes(nil) = (%v, %d), want (false, 0)", changed, total)
	}
}

// seg returns a segment with EndOffset=size-1 so streamSizeFromSegments returns exactly size.
func seg(size int64) storage.NZBSegment {
	return storage.NZBSegment{Bytes: size, EndOffset: size - 1}
}

func TestNormalizeNZBFileSizes_NoChange(t *testing.T) {
	// streamSize = 300 (EndOffset=299), file.Size = 300, TotalSize = 300 → no change
	nzb := &storage.NZB{
		TotalSize: 300,
		Files:     []storage.NZBFile{{Size: 300, Segments: []storage.NZBSegment{seg(300)}}},
	}
	changed, total := normalizeNZBFileSizes(nzb)
	if changed {
		t.Error("should not report changed when sizes already correct")
	}
	if total != 300 {
		t.Errorf("total = %d, want 300", total)
	}
}

func TestNormalizeNZBFileSizes_CorrectsTooLargeFileSize(t *testing.T) {
	// file.Size=9999 > streamSize=300 → corrected to 300
	nzb := &storage.NZB{
		TotalSize: 9999,
		Files:     []storage.NZBFile{{Size: 9999, Segments: []storage.NZBSegment{seg(300)}}},
	}
	changed, total := normalizeNZBFileSizes(nzb)
	if !changed {
		t.Error("should report changed when file size was too large")
	}
	if nzb.Files[0].Size != 300 {
		t.Errorf("corrected file size = %d, want 300", nzb.Files[0].Size)
	}
	if total != 300 {
		t.Errorf("total = %d, want 300", total)
	}
}

func TestNormalizeNZBFileSizes_DoesNotCorrectTooSmallFileSize(t *testing.T) {
	// file.Size=50 < streamSize=300 → NOT corrected (condition: file.Size > streamSize)
	nzb := &storage.NZB{
		TotalSize: 50,
		Files:     []storage.NZBFile{{Size: 50, Segments: []storage.NZBSegment{seg(300)}}},
	}
	changed, _ := normalizeNZBFileSizes(nzb)
	if changed {
		t.Error("should NOT correct file size upward when it is smaller than stream size")
	}
	if nzb.Files[0].Size != 50 {
		t.Errorf("file size should remain 50, got %d", nzb.Files[0].Size)
	}
}

func TestNormalizeNZBFileSizes_CorrectsTotalSize(t *testing.T) {
	// File sizes match their segments, but TotalSize is wrong (0 → 300)
	nzb := &storage.NZB{
		TotalSize: 0,
		Files: []storage.NZBFile{
			{Size: 100, Segments: []storage.NZBSegment{seg(100)}},
			{Size: 200, Segments: []storage.NZBSegment{seg(200)}},
		},
	}
	changed, total := normalizeNZBFileSizes(nzb)
	if !changed {
		t.Error("should report changed when TotalSize was wrong")
	}
	if total != 300 {
		t.Errorf("total = %d, want 300", total)
	}
	if nzb.TotalSize != 300 {
		t.Errorf("nzb.TotalSize = %d, want 300", nzb.TotalSize)
	}
}

func TestNormalizeNZBFileSizes_NoSegments_SizeKept(t *testing.T) {
	// streamSizeFromSegments returns 0 → file size is not changed
	nzb := &storage.NZB{
		TotalSize: 500,
		Files: []storage.NZBFile{
			{Size: 500, Segments: nil},
		},
	}
	changed, total := normalizeNZBFileSizes(nzb)
	if changed {
		t.Error("should not change file size when no segments")
	}
	if total != 500 {
		t.Errorf("total = %d, want 500", total)
	}
}
