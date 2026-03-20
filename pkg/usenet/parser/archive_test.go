package parser

import (
	"testing"

	"github.com/Tensai75/nzbparser"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// ── getRARVolumeOrder ─────────────────────────────────────────────────────────

func TestGetRARVolumeOrder(t *testing.T) {
	tests := []struct {
		filename string
		want     int
	}{
		// Plain .rar = first volume
		{"archive.rar", 0},
		{"ARCHIVE.RAR", 0},
		// .part01.rar = 1, .part02.rar = 2
		{"archive.part01.rar", 1},
		{"archive.part02.rar", 2},
		{"archive.part10.rar", 10},
		// Old-style: .r00 = 1, .r01 = 2
		{"archive.r00", 1},
		{"archive.r01", 2},
		{"archive.r09", 10},
		// Unknown extension goes last
		{"archive.zip", 999999},
		{"archive.nfo", 999999},
		{"archive", 999999},
	}
	for _, tt := range tests {
		got := getRARVolumeOrder(tt.filename)
		if got != tt.want {
			t.Errorf("getRARVolumeOrder(%q) = %d, want %d", tt.filename, got, tt.want)
		}
	}
}

func TestGetRARVolumeOrder_Sorting(t *testing.T) {
	// Verify sort order is correct: .rar < .r00 < .r01 < .part02.rar
	cases := []struct{ a, b string }{
		{"show.rar", "show.r00"},
		{"show.r00", "show.r01"},
		{"show.part01.rar", "show.part02.rar"},
	}
	for _, c := range cases {
		if getRARVolumeOrder(c.a) >= getRARVolumeOrder(c.b) {
			t.Errorf("expected order(%q) < order(%q)", c.a, c.b)
		}
	}
}

// ── DetectFileType ────────────────────────────────────────────────────────────

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		want  string
	}{
		{"empty", []byte{}, "unknown"},
		{"rar4", []byte("Rar!\x1A\x07\x00"), "rar4"},
		{"rar5", []byte("Rar!\x1A\x07\x01\x00"), "rar5"},
		{"zip", []byte{0x50, 0x4B, 0x03, 0x04, 0x00}, "zip"},
		{"7z", []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}, "7z"},
		{"gzip", []byte{0x1F, 0x8B, 0x00}, "gzip"},
		{"pdf", []byte("%PDF-extra"), "pdf"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0x00}, "jpeg"},
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, "png"},
		{"gif87a", []byte("GIF87a"), "gif"},
		{"gif89a", []byte("GIF89a"), "gif"},
		{"mkv", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x00}, "mkv"},
		{"mp3 id3", []byte("ID3extra"), "mp3"},
		{"flac", []byte("fLaC\x00"), "flac"},
		{"unknown", []byte{0x00, 0x01, 0x02}, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFileType(tt.bytes)
			if got != tt.want {
				t.Errorf("DetectFileType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectFileType_RIFF(t *testing.T) {
	// WAV: RIFF....WAVE
	wav := make([]byte, 12)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	if got := DetectFileType(wav); got != "wav" {
		t.Errorf("wav: DetectFileType = %q, want %q", got, "wav")
	}

	// AVI: RIFF....AVI_
	avi := make([]byte, 12)
	copy(avi[0:4], "RIFF")
	copy(avi[8:12], "AVI ")
	if got := DetectFileType(avi); got != "avi" {
		t.Errorf("avi: DetectFileType = %q, want %q", got, "avi")
	}

	// WebP: RIFF....WEBP
	webp := make([]byte, 12)
	copy(webp[0:4], "RIFF")
	copy(webp[8:12], "WEBP")
	if got := DetectFileType(webp); got != "webp" {
		t.Errorf("webp: DetectFileType = %q, want %q", got, "webp")
	}
}

func TestDetectFileType_MP4(t *testing.T) {
	// ftyp at position 4
	mp4 := make([]byte, 12)
	copy(mp4[4:8], "ftyp")
	if got := DetectFileType(mp4); got != "mp4" {
		t.Errorf("mp4: DetectFileType = %q, want %q", got, "mp4")
	}
}

func TestDetectFileType_TAR(t *testing.T) {
	// ustar magic at offset 257
	tar := make([]byte, 265)
	copy(tar[257:262], "ustar")
	if got := DetectFileType(tar); got != "tar" {
		t.Errorf("tar: DetectFileType = %q, want %q", got, "tar")
	}
}

// ── NormalizeArchivePath ──────────────────────────────────────────────────────

func TestNormalizeArchivePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Normal path
		{"folder/file.mkv", "folder/file.mkv"},
		// Leading ./
		{"./folder/file.mkv", "folder/file.mkv"},
		// Leading /
		{"/folder/file.mkv", "folder/file.mkv"},
		// Backslashes normalized to forward slashes
		{`folder\sub\file.mkv`, "folder/sub/file.mkv"},
		// Whitespace trimmed
		{"  folder/file.mkv  ", "folder/file.mkv"},
		// Empty stays empty
		{"", ""},
		// Only dots/slashes
		{"./", ""},
		// Nested dots
		{"./sub/../file.mkv", "file.mkv"},
		// Already clean
		{"file.mkv", "file.mkv"},
	}
	for _, tt := range tests {
		got := NormalizeArchivePath(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeArchivePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── determineNZBName ──────────────────────────────────────────────────────────

func TestDetermineNZBName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		meta     map[string]string
		want     string
	}{
		{
			name:     "filename wins over meta",
			filename: "MyShow.S01E01.nzb",
			meta:     map[string]string{"Name": "OtherName", "title": "OtherTitle"},
			want:     "MyShow.S01E01",
		},
		{
			name:     "extension stripped from filename",
			filename: "Movie.2024.nzb",
			meta:     nil,
			want:     "Movie.2024",
		},
		{
			name:     "Name meta used when no filename",
			filename: "",
			meta:     map[string]string{"Name": "ShowFromMeta"},
			want:     "ShowFromMeta",
		},
		{
			name:     "title meta used as last resort",
			filename: "",
			meta:     map[string]string{"title": "TitleFallback"},
			want:     "TitleFallback",
		},
		{
			name:     "Name preferred over title",
			filename: "",
			meta:     map[string]string{"Name": "NameVal", "title": "TitleVal"},
			want:     "NameVal",
		},
		{
			name:     "all empty returns empty",
			filename: "",
			meta:     map[string]string{},
			want:     "",
		},
		{
			name:     "nil meta and no filename",
			filename: "",
			meta:     nil,
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineNZBName(tt.filename, tt.meta)
			if got != tt.want {
				t.Errorf("determineNZBName(%q, %v) = %q, want %q", tt.filename, tt.meta, got, tt.want)
			}
		})
	}
}

// ── sortRARVolumesByOrder ─────────────────────────────────────────────────────

func TestSortRARVolumesByOrder(t *testing.T) {
	files := []nzbparser.NzbFile{
		{Filename: "archive.r02"},
		{Filename: "archive.rar"},
		{Filename: "archive.r00"},
		{Filename: "archive.r01"},
	}
	sortRARVolumesByOrder(files, func(f nzbparser.NzbFile) string { return f.Filename })

	expected := []string{"archive.rar", "archive.r00", "archive.r01", "archive.r02"}
	for i, f := range files {
		if f.Filename != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, f.Filename, expected[i])
		}
	}
}

// ── wrapNZBFile ───────────────────────────────────────────────────────────────

func TestWrapNZBFile_Nil(t *testing.T) {
	_, err := wrapNZBFile(nil)
	if err == nil {
		t.Error("expected error for nil NZBFile")
	}
}

func TestWrapNZBFile_NonNil(t *testing.T) {
	f := &storage.NZBFile{Name: "test.mkv"}
	result, err := wrapNZBFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != f {
		t.Errorf("expected slice with original pointer, got %v", result)
	}
}

// ── fileMetaKey ───────────────────────────────────────────────────────────────

func TestFileMetaKey(t *testing.T) {
	if got := fileMetaKey(nzbparser.NzbFile{Number: 5, Subject: "subj"}); got != "n:5" {
		t.Errorf("Number priority: got %q, want n:5", got)
	}
	if got := fileMetaKey(nzbparser.NzbFile{Subject: "my subject"}); got != "s:my subject" {
		t.Errorf("Subject priority: got %q, want s:my subject", got)
	}
	f3 := nzbparser.NzbFile{Segments: nzbparser.NzbSegments{{Id: "msg-id-123", Number: 1}}}
	if got := fileMetaKey(f3); got != "m:msg-id-123" {
		t.Errorf("Segment ID: got %q, want m:msg-id-123", got)
	}
	if got := fileMetaKey(nzbparser.NzbFile{}); got != "" {
		t.Errorf("empty: got %q, want empty", got)
	}
}

// ── getGroupsList ─────────────────────────────────────────────────────────────

func TestGetGroupsList(t *testing.T) {
	groups := map[string]struct{}{
		"alt.binaries.test":  {},
		"alt.binaries.other": {},
	}
	result := getGroupsList(groups)
	if len(result) != 2 {
		t.Errorf("getGroupsList: got %d items, want 2", len(result))
	}
	seen := make(map[string]bool)
	for _, g := range result {
		seen[g] = true
	}
	for k := range groups {
		if !seen[k] {
			t.Errorf("missing group %q in result", k)
		}
	}
}

func TestGetGroupsList_Empty(t *testing.T) {
	if got := getGroupsList(nil); len(got) != 0 {
		t.Errorf("getGroupsList(nil) = %v, want empty", got)
	}
}

// ── determineExtension ────────────────────────────────────────────────────────

func TestDetermineExtension(t *testing.T) {
	t.Run("returns extension from first file", func(t *testing.T) {
		g := &FileGroup{Files: []nzbparser.NzbFile{{Filename: "movie.mkv"}}}
		if got := determineExtension(g); got != ".mkv" {
			t.Errorf("got %q, want .mkv", got)
		}
	})
	t.Run("skips empty filename uses next", func(t *testing.T) {
		g := &FileGroup{Files: []nzbparser.NzbFile{{Filename: ""}, {Filename: "movie.mp4"}}}
		if got := determineExtension(g); got != ".mp4" {
			t.Errorf("got %q, want .mp4", got)
		}
	})
	t.Run("empty group returns empty", func(t *testing.T) {
		if got := determineExtension(&FileGroup{}); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
