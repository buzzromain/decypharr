package parser

import (
	"testing"

	"github.com/Tensai75/nzbparser"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func newTestParser() *NZBParser {
	return NewParser(nil, 1, zerolog.Nop())
}

func makeFile(segments ...int) nzbparser.NzbFile {
	segs := make(nzbparser.NzbSegments, len(segments))
	for i, b := range segments {
		segs[i] = nzbparser.NzbSegment{Bytes: b, Number: i + 1}
	}
	return nzbparser.NzbFile{Segments: segs}
}

// ── getBaseFilename ───────────────────────────────────────────────────────────

func TestGetBaseFilename(t *testing.T) {
	p := newTestParser()

	cases := []struct {
		in   string
		want string
	}{
		// empty
		{"", ""},
		// regular extension stripped
		{"movie.mkv", "movie"},
		{"show.S01E01.mp4", "show.S01E01"},
		// RAR part file (.part01.rar)
		{"archive.part01.rar", "archive"},
		{"archive.part12.rar", "archive"},
		// numbered extension (.7z.001, .r01)
		{"archive.7z.001", "archive"},
		{"archive.r01", "archive"},
		{"archive.r99", "archive"},
		// PAR2 volume
		{"archive.vol0+1.par2", "archive"},
		{"archive.vol10+20.PAR2", "archive"},
		// quotes stripped
		{`"movie.mkv"`, "movie"},
		// no extension
		{"plainname", "plainname"},
	}

	for _, tc := range cases {
		got := p.getBaseFilename(tc.in)
		if got != tc.want {
			t.Errorf("getBaseFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── detectFileType (method on NZBParser, extension-based) ────────────────────

func TestNZBParserDetectFileType(t *testing.T) {
	p := newTestParser()

	cases := []struct {
		filename string
		want     storage.NZBFileType
	}{
		// media files
		{"movie.mkv", storage.NZBFileTypeMedia},
		{"show.mp4", storage.NZBFileTypeMedia},
		{"audio.mp3", storage.NZBFileTypeMedia},
		{"show.S01E01.avi", storage.NZBFileTypeMedia},
		// RAR files
		{"archive.rar", storage.NZBFileTypeRar},
		{"archive.r00", storage.NZBFileTypeRar},
		{"archive.r99", storage.NZBFileTypeRar},
		{"archive.part01.rar", storage.NZBFileTypeRar},
		// PAR2
		{"repair.par2", storage.NZBFileTypePar2},
		{"repair.vol0+1.par2", storage.NZBFileTypePar2},
		// 7-zip
		{"archive.7z", storage.NZBFileTypeSevenZip},
		{"archive.7z.001", storage.NZBFileTypeSevenZip},
		// ZIP
		{"archive.zip", storage.NZBFileTypeZip},
		// ignored extensions
		{"info.nfo", storage.NZBFileTypeIgnore},
		{"checksum.sfv", storage.NZBFileTypeIgnore},
		{"cover.jpg", storage.NZBFileTypeIgnore},
		{"subs.srt", storage.NZBFileTypeIgnore},
		// unknown
		{"document.pdf", storage.NZBFileTypeUnknown},
		{"data.xyz", storage.NZBFileTypeUnknown},
		// case insensitive
		{"MOVIE.MKV", storage.NZBFileTypeMedia},
		{"ARCHIVE.RAR", storage.NZBFileTypeRar},
	}

	for _, tc := range cases {
		got := p.detectFileType(tc.filename)
		if got != tc.want {
			t.Errorf("detectFileType(%q) = %q, want %q", tc.filename, got, tc.want)
		}
	}
}

// ── detectFileTypeFromContent ─────────────────────────────────────────────────

func TestDetectFileTypeFromContent(t *testing.T) {
	p := newTestParser()

	cases := []struct {
		name string
		data []byte
		want storage.NZBFileType
	}{
		{"empty", []byte{}, storage.NZBFileTypeUnknown},
		// RAR 4.x
		{"rar4", []byte{'R', 'a', 'r', '!', 0x1A, 0x07, 0x00}, storage.NZBFileTypeRar},
		// RAR 5.x
		{"rar5", []byte{'R', 'a', 'r', '!', 0x1A, 0x07, 0x01, 0x00}, storage.NZBFileTypeRar},
		// ZIP
		{"zip", []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00}, storage.NZBFileTypeZip},
		// 7-zip
		{"7z", []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}, storage.NZBFileTypeSevenZip},
		// Matroska (MKV)
		{"mkv", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x00, 0x00}, storage.NZBFileTypeMedia},
		// MP4 (ftyp at offset 4)
		{"mp4", []byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p', 0x00}, storage.NZBFileTypeMedia},
		// AVI (RIFF...AVI )
		{
			"avi",
			append(append([]byte("RIFF"), 0x00, 0x00, 0x00, 0x00), []byte("AVI ")...),
			storage.NZBFileTypeMedia,
		},
		// MPEG-PS
		{"mpeg-ps", []byte{0x00, 0x00, 0x01, 0xBA, 0x00}, storage.NZBFileTypeMedia},
		// MPEG-ES
		{"mpeg-es", []byte{0x00, 0x00, 0x01, 0xB3, 0x00}, storage.NZBFileTypeMedia},
		// unknown random bytes
		{"unknown", []byte{0xDE, 0xAD, 0xBE, 0xEF}, storage.NZBFileTypeUnknown},
	}

	for _, tc := range cases {
		got := p.detectFileTypeFromContent(tc.data)
		if got != tc.want {
			t.Errorf("detectFileTypeFromContent(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// ── FileGroup.getMetadata ─────────────────────────────────────────────────────

func TestFileGroup_GetMetadata_NoFiles(t *testing.T) {
	g := &FileGroup{}
	m := g.getMetadata()
	if m.fileSize != 0 || m.segmentSize != 0 {
		t.Errorf("empty group: expected zero metadata, got %+v", m)
	}
}

func TestFileGroup_GetMetadata_CachedOnSecondCall(t *testing.T) {
	g := &FileGroup{
		Files: []nzbparser.NzbFile{
			makeFile(1000000),
			makeFile(1000000),
		},
	}
	m1 := g.getMetadata()
	m2 := g.getMetadata()
	if m1 != m2 {
		t.Error("getMetadata() should return the same cached pointer on repeated calls")
	}
}

func TestFileGroup_GetMetadata_ComputesSize(t *testing.T) {
	// reportedBytes = 1_000_000 → segmentSize ≈ 970_000, fileSize = segmentSize * numSegments
	g := &FileGroup{
		Files: []nzbparser.NzbFile{
			makeFile(1_000_000, 1_000_000), // 2 segments in first file
			makeFile(500_000),               // 1 segment in last file
		},
	}
	m := g.getMetadata()
	// segmentSize = int64(1_000_000 * 0.97) = 970_000
	wantSeg := int64(float64(1_000_000) * 0.97)
	if m.segmentSize != wantSeg {
		t.Errorf("segmentSize = %d, want %d", m.segmentSize, wantSeg)
	}
	// fileSize = segmentSize * 2 (2 segments in first file)
	wantFile := wantSeg * 2
	if m.fileSize != wantFile {
		t.Errorf("fileSize = %d, want %d", m.fileSize, wantFile)
	}
	// lastFileSize = segmentSize * 1 (1 segment in last file)
	wantLast := wantSeg * 1
	if m.lastFileSize != wantLast {
		t.Errorf("lastFileSize = %d, want %d", m.lastFileSize, wantLast)
	}
}

func TestFileGroup_GetMetadata_ZeroBytes_DefaultSegment(t *testing.T) {
	// Bytes=0 → falls back to 750_000 default
	g := &FileGroup{
		Files: []nzbparser.NzbFile{
			makeFile(0), // zero reported bytes
		},
	}
	m := g.getMetadata()
	const defaultBytes = int64(750_000)
	wantSeg := int64(float64(defaultBytes) * 0.97)
	if m.segmentSize != wantSeg {
		t.Errorf("zero bytes: segmentSize = %d, want %d", m.segmentSize, wantSeg)
	}
}

// ── mergeObfuscatedRarGroups ──────────────────────────────────────────────────

func makeRARGroup(name string, numFiles int) *FileGroup {
	files := make([]nzbparser.NzbFile, numFiles)
	for i := range files {
		files[i] = nzbparser.NzbFile{Number: i + 1, Segments: nzbparser.NzbSegments{{Bytes: 1000, Number: 1}}}
	}
	return &FileGroup{
		BaseName: name,
		Type:     storage.NZBFileTypeRar,
		Files:    files,
		Groups:   map[string]struct{}{},
	}
}

func TestMergeObfuscatedRarGroups_NoMerge_SingleGroup(t *testing.T) {
	p := newTestParser()
	// Only one single-file RAR group → no merge
	input := map[string]*FileGroup{
		"abc": makeRARGroup("abc", 1),
	}
	result := p.mergeObfuscatedRarGroups(input)
	if len(result) != 1 {
		t.Errorf("got %d groups, want 1", len(result))
	}
}

func TestMergeObfuscatedRarGroups_NoMerge_MultiFileGroup(t *testing.T) {
	p := newTestParser()
	// Multi-file RAR group (already properly grouped) → no merge
	input := map[string]*FileGroup{
		"archive": makeRARGroup("archive", 10),
	}
	result := p.mergeObfuscatedRarGroups(input)
	if len(result) != 1 {
		t.Errorf("got %d groups, want 1", len(result))
	}
	if len(result["archive"].Files) != 10 {
		t.Errorf("files = %d, want 10", len(result["archive"].Files))
	}
}

func TestMergeObfuscatedRarGroups_MergesSingleFileRarGroups(t *testing.T) {
	p := newTestParser()
	// 3 single-file RAR groups (obfuscated) → merged into 1
	input := map[string]*FileGroup{
		"rnd1": makeRARGroup("rnd1", 1),
		"rnd2": makeRARGroup("rnd2", 1),
		"rnd3": makeRARGroup("rnd3", 1),
	}
	// Assign different Numbers to verify sort order
	input["rnd1"].Files[0].Number = 3
	input["rnd2"].Files[0].Number = 1
	input["rnd3"].Files[0].Number = 2

	result := p.mergeObfuscatedRarGroups(input)
	if len(result) != 1 {
		t.Errorf("got %d groups, want 1", len(result))
	}
	for _, g := range result {
		if len(g.Files) != 3 {
			t.Errorf("merged group has %d files, want 3", len(g.Files))
		}
		// Files should be sorted by Number
		for i := 1; i < len(g.Files); i++ {
			if g.Files[i].Number < g.Files[i-1].Number {
				t.Errorf("files not sorted by Number at index %d: %d < %d",
					i, g.Files[i].Number, g.Files[i-1].Number)
			}
		}
		if g.Type != storage.NZBFileTypeRar {
			t.Errorf("merged type = %q, want Rar", g.Type)
		}
	}
}

func TestMergeObfuscatedRarGroups_KeepsNonRarGroups(t *testing.T) {
	p := newTestParser()
	// 2 single-file RAR + 1 media group → RAR groups merged, media kept separate
	media := &FileGroup{
		BaseName: "movie",
		Type:     storage.NZBFileTypeMedia,
		Files:    []nzbparser.NzbFile{makeFile(1000)},
		Groups:   map[string]struct{}{},
	}
	input := map[string]*FileGroup{
		"rnd1":  makeRARGroup("rnd1", 1),
		"rnd2":  makeRARGroup("rnd2", 1),
		"movie": media,
	}
	result := p.mergeObfuscatedRarGroups(input)
	// Should be 2: merged RAR + media
	if len(result) != 2 {
		t.Errorf("got %d groups, want 2", len(result))
	}
	if _, ok := result["movie"]; !ok {
		t.Error("media group should be preserved")
	}
}

// ── isRarFile ─────────────────────────────────────────────────────────────────

func TestIsRarFile(t *testing.T) {
	p := newTestParser()

	trueCase := []string{"archive.rar", "archive.r00", "archive.r01", "archive.r99", "archive.part01.rar"}
	for _, f := range trueCase {
		if !p.isRarFile(f) {
			t.Errorf("isRarFile(%q) = false, want true", f)
		}
	}

	falseCase := []string{"archive.zip", "movie.mkv", "archive.7z", "archive.par2", "archive.r00extra"}
	for _, f := range falseCase {
		if p.isRarFile(f) {
			t.Errorf("isRarFile(%q) = true, want false", f)
		}
	}
}
