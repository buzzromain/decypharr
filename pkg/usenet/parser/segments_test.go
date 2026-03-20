package parser

import (
	"testing"

	"github.com/Tensai75/nzbparser"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// makeGroupWithMeta builds a FileGroup with explicit metadata (bypasses getMetadata heuristics).
func makeGroupWithMeta(baseName string, files []nzbparser.NzbFile, segSize, fileSize, lastFileSize int64) *FileGroup {
	return &FileGroup{
		BaseName: baseName,
		Files:    files,
		metadata: &fileAnalysisResult{
			segmentSize:  segSize,
			fileSize:     fileSize,
			lastFileSize: lastFileSize,
		},
	}
}

// ── getNZBSegments ────────────────────────────────────────────────────────────

func TestGetNZBSegments_NoSegments(t *testing.T) {
	group := makeGroupWithMeta("movie", []nzbparser.NzbFile{{}, {}}, 1000, 1000, 500)
	size, segs := getNZBSegments(0, nzbparser.NzbFile{}, group)
	if size != 0 || segs != nil {
		t.Errorf("empty segments: got size=%d segs=%v, want 0 nil", size, segs)
	}
}

func TestGetNZBSegments_SingleSegment_NonLastFile(t *testing.T) {
	// Single segment, not the last file in group → uses fileSize for last-segment calc
	seg := nzbparser.NzbSegment{Number: 1, Id: "msg-001", Bytes: 1_000_000}
	file := nzbparser.NzbFile{Segments: nzbparser.NzbSegments{seg}}

	// Group has 2 files, index 0 (non-last)
	files := []nzbparser.NzbFile{file, {Segments: nzbparser.NzbSegments{{Number: 1}}}}
	group := makeGroupWithMeta("movie", files, 970_000, 970_000, 500_000)

	totalSize, nzbSegs := getNZBSegments(0, file, group)

	if len(nzbSegs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(nzbSegs))
	}
	// With 1 segment total, it's the last segment.
	// fullSegsSize = segmentSize * 0 = 0
	// segSize = fileSize - 0 = 970_000
	if nzbSegs[0].MessageID != "msg-001" {
		t.Errorf("MessageID = %q, want msg-001", nzbSegs[0].MessageID)
	}
	if nzbSegs[0].StartOffset != 0 {
		t.Errorf("StartOffset = %d, want 0", nzbSegs[0].StartOffset)
	}
	if nzbSegs[0].Bytes != 970_000 {
		t.Errorf("Bytes = %d, want 970_000", nzbSegs[0].Bytes)
	}
	if totalSize != 970_000 {
		t.Errorf("totalSize = %d, want 970_000", totalSize)
	}
}

func TestGetNZBSegments_TwoSegments_OffsetProgression(t *testing.T) {
	// 2 segments: first uses segmentSize, last adjusts to fileSize
	seg1 := nzbparser.NzbSegment{Number: 1, Id: "msg-001", Bytes: 1_000_000}
	seg2 := nzbparser.NzbSegment{Number: 2, Id: "msg-002", Bytes: 500_000}
	file := nzbparser.NzbFile{Segments: nzbparser.NzbSegments{seg1, seg2}}

	const segSize = int64(970_000)
	const fileSize = int64(1_470_000) // segSize + 500_000 (last segment smaller)
	group := makeGroupWithMeta("movie", []nzbparser.NzbFile{file}, segSize, fileSize, fileSize)

	totalSize, nzbSegs := getNZBSegments(0, file, group)

	if len(nzbSegs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(nzbSegs))
	}
	// Segment 1: non-last → segmentSize
	if nzbSegs[0].Bytes != segSize {
		t.Errorf("seg[0].Bytes = %d, want %d", nzbSegs[0].Bytes, segSize)
	}
	if nzbSegs[0].StartOffset != 0 {
		t.Errorf("seg[0].StartOffset = %d, want 0", nzbSegs[0].StartOffset)
	}
	// Segment 2: last → fileSize - fullSegsSize = fileSize - segSize
	wantLastBytes := fileSize - segSize
	if nzbSegs[1].Bytes != wantLastBytes {
		t.Errorf("seg[1].Bytes = %d, want %d", nzbSegs[1].Bytes, wantLastBytes)
	}
	if nzbSegs[1].StartOffset != segSize {
		t.Errorf("seg[1].StartOffset = %d, want %d", nzbSegs[1].StartOffset, segSize)
	}
	if nzbSegs[1].EndOffset != nzbSegs[1].StartOffset+wantLastBytes-1 {
		t.Errorf("seg[1].EndOffset incorrect")
	}
	if totalSize != fileSize {
		t.Errorf("totalSize = %d, want %d", totalSize, fileSize)
	}
}

func TestGetNZBSegments_LastFileSizeMismatch_FallsBackToEstimate(t *testing.T) {
	// If fileSize is wildly different from expected, fall back to 0.97 * Bytes
	seg1 := nzbparser.NzbSegment{Number: 1, Id: "msg-001", Bytes: 1_000_000}
	seg2 := nzbparser.NzbSegment{Number: 2, Id: "msg-002", Bytes: 200_000}
	file := nzbparser.NzbFile{Segments: nzbparser.NzbSegments{seg1, seg2}}

	const segSize = int64(970_000)
	// fileSize is tiny (2) → diff vs expected (~1_940_000) >> 1.5*segSize → mismatch
	group := makeGroupWithMeta("movie", []nzbparser.NzbFile{file}, segSize, 2, 2)

	_, nzbSegs := getNZBSegments(0, file, group)

	// Last segment should fall back to 0.97 * Bytes
	wantFallback := int64(float64(200_000) * 0.97)
	if nzbSegs[1].Bytes != wantFallback {
		t.Errorf("fallback: seg[1].Bytes = %d, want %d", nzbSegs[1].Bytes, wantFallback)
	}
}

func TestGetNZBSegments_GroupName(t *testing.T) {
	seg := nzbparser.NzbSegment{Number: 1, Id: "msg-001", Bytes: 1_000}
	file := nzbparser.NzbFile{Segments: nzbparser.NzbSegments{seg}}
	group := makeGroupWithMeta("my.movie", []nzbparser.NzbFile{file}, 970, 970, 970)

	_, nzbSegs := getNZBSegments(0, file, group)
	if nzbSegs[0].Group != "my.movie" {
		t.Errorf("Group = %q, want my.movie", nzbSegs[0].Group)
	}
}

// ── groupProcessedFiles ───────────────────────────────────────────────────────

func TestGroupProcessedFiles_IgnoreTypeSkipped(t *testing.T) {
	p := newTestParser()
	items := []contentResult{
		{
			file:         nzbparser.NzbFile{Basefilename: "info"},
			fileType:     storage.NZBFileTypeIgnore,
			actualFilename: "info.nfo",
		},
	}
	groups := p.groupProcessedFiles(items)
	if len(groups) != 0 {
		t.Errorf("ignored files should produce 0 groups, got %d", len(groups))
	}
}

func TestGroupProcessedFiles_SameBasenameGrouped(t *testing.T) {
	p := newTestParser()
	// Two files with same Basefilename → same group
	items := []contentResult{
		{
			file:         nzbparser.NzbFile{Basefilename: "movie", Number: 1},
			fileType:     storage.NZBFileTypeMedia,
			actualFilename: "",
		},
		{
			file:         nzbparser.NzbFile{Basefilename: "movie", Number: 2},
			fileType:     storage.NZBFileTypeMedia,
			actualFilename: "",
		},
	}
	groups := p.groupProcessedFiles(items)
	if len(groups) != 1 {
		t.Errorf("same basefilename should produce 1 group, got %d", len(groups))
	}
	if g, ok := groups["movie"]; !ok || len(g.Files) != 2 {
		t.Errorf("group 'movie' should have 2 files")
	}
}

func TestGroupProcessedFiles_DifferentBasenameSeparateGroups(t *testing.T) {
	p := newTestParser()
	items := []contentResult{
		{
			file:         nzbparser.NzbFile{Basefilename: "movie-a", Number: 1},
			fileType:     storage.NZBFileTypeMedia,
			actualFilename: "",
		},
		{
			file:         nzbparser.NzbFile{Basefilename: "movie-b", Number: 1},
			fileType:     storage.NZBFileTypeRar,
			actualFilename: "",
		},
	}
	groups := p.groupProcessedFiles(items)
	if len(groups) != 2 {
		t.Errorf("different basenames: expected 2 groups, got %d", len(groups))
	}
}

func TestGroupProcessedFiles_ActualFilenameUsedAsGroupKey(t *testing.T) {
	p := newTestParser()
	// actualFilename differs from file.Filename → use getBaseFilename(actualFilename)
	items := []contentResult{
		{
			file:           nzbparser.NzbFile{Filename: "random123", Basefilename: "random123", Number: 1},
			fileType:       storage.NZBFileTypeRar,
			actualFilename: "archive.part01.rar",
		},
		{
			file:           nzbparser.NzbFile{Filename: "random456", Basefilename: "random456", Number: 2},
			fileType:       storage.NZBFileTypeRar,
			actualFilename: "archive.part02.rar",
		},
	}
	groups := p.groupProcessedFiles(items)
	// Both map to "archive" via getBaseFilename
	if len(groups) != 1 {
		t.Errorf("expected 1 group keyed by 'archive', got %d: %v", len(groups), groupKeys(groups))
	}
	if g, ok := groups["archive"]; !ok || len(g.Files) != 2 {
		t.Errorf("group 'archive' should have 2 files")
	}
}

func TestGroupProcessedFiles_UnknownTypeInferredFromActualFilename(t *testing.T) {
	p := newTestParser()
	items := []contentResult{
		{
			file:           nzbparser.NzbFile{Filename: "obfuscated", Basefilename: "obfuscated", Number: 1},
			fileType:       storage.NZBFileTypeUnknown,
			actualFilename: "show.mkv", // inferred → Media
		},
	}
	groups := p.groupProcessedFiles(items)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	for _, g := range groups {
		if g.Type != storage.NZBFileTypeMedia {
			t.Errorf("type should be inferred as Media, got %q", g.Type)
		}
	}
}

func groupKeys(m map[string]*FileGroup) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
