package rar

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

func TestMain(m *testing.M) {
	// Set up a minimal config directory so config.Get() doesn't fail.
	tmp, err := os.MkdirTemp("", "rar-test-config-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	config.SetConfigPath(tmp)

	// Write minimal valid config to avoid interactive prompts
	cfgJSON := `{"download_folder":"/tmp/rar-test","debrids":[{"name":"test","provider":"realdebrid","api_key":"test"}]}`
	if err := os.WriteFile(filepath.Join(tmp, "config.json"), []byte(cfgJSON), 0644); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// --- File.Name() tests ---

func TestFile_Name(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"simple filename", "movie.mkv", "movie.mkv"},
		{"unix path", "dir/subdir/movie.mkv", "movie.mkv"},
		{"windows backslash", `dir\subdir\movie.mkv`, "movie.mkv"},
		{"mixed separators", `dir/subdir\movie.mkv`, "movie.mkv"},
		{"trailing separator", "dir/", ""},
		{"root only", "/", ""},
		{"empty path", "", ""},
		{"deep nesting", "a/b/c/d/e/f.txt", "f.txt"},
		{"dot file", ".hidden", ".hidden"},
		{"dot in dir", "dir/.hidden", ".hidden"},
		{"spaces in name", "dir/my file.mkv", "my file.mkv"},
		{"unicode filename", "dir/映画.mkv", "映画.mkv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{Path: tt.path}
			if got := f.Name(); got != tt.want {
				t.Errorf("File{Path: %q}.Name() = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// --- File.ByteRange() tests ---

func TestFile_ByteRange(t *testing.T) {
	tests := []struct {
		name           string
		dataOffset     int64
		compressedSize int64
		wantStart      int64
		wantEnd        int64
	}{
		{"normal range", 100, 50, 100, 149},
		{"zero offset", 0, 1024, 0, 1023},
		{"large offset", 1_000_000, 500_000, 1_000_000, 1_499_999},
		{"single byte", 42, 1, 42, 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{DataOffset: tt.dataOffset, CompressedSize: tt.compressedSize}
			r := f.ByteRange()
			if r[0] != tt.wantStart || r[1] != tt.wantEnd {
				t.Errorf("ByteRange() = [%d, %d], want [%d, %d]",
					r[0], r[1], tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// --- decodeUnicode() tests ---

func TestDecodeUnicode(t *testing.T) {
	tests := []struct {
		name        string
		ascii       string
		unicodeData []byte
		want        string
	}{
		{
			name:        "empty unicode data returns ascii",
			ascii:       "hello.txt",
			unicodeData: nil,
			want:        "hello.txt",
		},
		{
			name:        "empty unicode data bytes returns ascii",
			ascii:       "test.mkv",
			unicodeData: []byte{},
			want:        "test.mkv",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeUnicode(tt.ascii, tt.unicodeData)
			if got != tt.want {
				t.Errorf("decodeUnicode(%q, %v) = %q, want %q",
					tt.ascii, tt.unicodeData, got, tt.want)
			}
		})
	}
}

// --- ExtractFile() error paths ---

func TestExtractFile_DirectoryReturnsError(t *testing.T) {
	r := &Reader{
		File:  &HttpFile{},
		Files: make([]*File, 0),
	}
	dirFile := &File{
		Path:        "somedir",
		IsDirectory: true,
	}
	_, err := r.ExtractFile(dirFile)
	if err != ErrDirectoryExtractNotSupported {
		t.Errorf("ExtractFile(directory) = %v, want %v", err, ErrDirectoryExtractNotSupported)
	}
}

func TestExtractFile_CompressedReturnsError(t *testing.T) {
	r := &Reader{
		File:  &HttpFile{},
		Files: make([]*File, 0),
	}
	compressedFile := &File{
		Path:   "file.dat",
		Method: 0x31, // Not Store (0x30)
	}
	_, err := r.ExtractFile(compressedFile)
	if err != ErrCompressionNotSupported {
		t.Errorf("ExtractFile(compressed) = %v, want %v", err, ErrCompressionNotSupported)
	}
}

// --- parseFileHeader() tests ---

// buildFileHeader constructs a minimal valid RAR3 file header for testing.
// The header is structured as per RAR3 format specification.
func buildFileHeader(t *testing.T, opts fileHeaderOpts) []byte {
	t.Helper()

	nameBytes := []byte(opts.fileName)
	nameSize := uint16(len(nameBytes))

	// Compute flags
	flags := uint16(0)
	if opts.isDirectory {
		flags |= uint16(FlagDirectory)
	}
	if opts.hasData {
		flags |= uint16(FlagHasData)
	}

	// Fixed header: 7 bytes block header + 25 bytes file fields = 32 bytes + nameSize
	headSize := uint16(32 + nameSize)

	buf := make([]byte, headSize)
	// [0:2] = CRC (unused by parseFileHeader)
	buf[2] = BlockFile // headType
	binary.LittleEndian.PutUint16(buf[3:5], flags)
	binary.LittleEndian.PutUint16(buf[5:7], headSize)

	// [7:11] packSize (compressed)
	binary.LittleEndian.PutUint32(buf[7:11], opts.packSize)
	// [11:15] unpackSize
	binary.LittleEndian.PutUint32(buf[11:15], opts.unpackSize)
	// [15] fileOS
	buf[15] = 0
	// [16:20] CRC32
	binary.LittleEndian.PutUint32(buf[16:20], opts.crc)
	// [20:24] fileTime
	binary.LittleEndian.PutUint32(buf[20:24], 0)
	// [24] unpVer
	buf[24] = 29
	// [25] method
	buf[25] = opts.method
	// [26:28] nameSize
	binary.LittleEndian.PutUint16(buf[26:28], nameSize)
	// [28:32] fileAttr
	binary.LittleEndian.PutUint32(buf[28:32], 0)

	// filename at offset 32
	copy(buf[32:], nameBytes)

	return buf
}

type fileHeaderOpts struct {
	fileName    string
	packSize    uint32
	unpackSize  uint32
	method      byte
	crc         uint32
	isDirectory bool
	hasData     bool
}

func TestParseFileHeader_ValidFile(t *testing.T) {
	r := &Reader{Files: make([]*File, 0)}
	header := buildFileHeader(t, fileHeaderOpts{
		fileName:   "movie.mkv",
		packSize:   1024,
		unpackSize: 1024,
		method:     0x30,
		crc:        0xDEADBEEF,
		hasData:    true,
	})

	position := int64(100) // arbitrary archive position
	f, err := r.parseFileHeader(header, position)
	if err != nil {
		t.Fatalf("parseFileHeader() error: %v", err)
	}

	headSize := int(binary.LittleEndian.Uint16(header[5:7]))
	expectedDataOffset := position + int64(headSize)

	if f.Path != "movie.mkv" {
		t.Errorf("Path = %q, want %q", f.Path, "movie.mkv")
	}
	if f.Size != 1024 {
		t.Errorf("Size = %d, want 1024", f.Size)
	}
	if f.CompressedSize != 1024 {
		t.Errorf("CompressedSize = %d, want 1024", f.CompressedSize)
	}
	if f.Method != 0x30 {
		t.Errorf("Method = %#x, want 0x30", f.Method)
	}
	if f.CRC != 0xDEADBEEF {
		t.Errorf("CRC = %#x, want 0xDEADBEEF", f.CRC)
	}
	if f.IsDirectory {
		t.Error("IsDirectory = true, want false")
	}
	if f.DataOffset != expectedDataOffset {
		t.Errorf("DataOffset = %d, want %d", f.DataOffset, expectedDataOffset)
	}
}

func TestParseFileHeader_Directory(t *testing.T) {
	r := &Reader{Files: make([]*File, 0)}
	header := buildFileHeader(t, fileHeaderOpts{
		fileName:    "subdir",
		isDirectory: true,
	})

	f, err := r.parseFileHeader(header, 0)
	if err != nil {
		t.Fatalf("parseFileHeader() error: %v", err)
	}
	if !f.IsDirectory {
		t.Error("IsDirectory = false, want true")
	}
}

func TestParseFileHeader_PathWithDirectory(t *testing.T) {
	r := &Reader{Files: make([]*File, 0)}
	header := buildFileHeader(t, fileHeaderOpts{
		fileName:   "dir/subdir/file.txt",
		packSize:   100,
		unpackSize: 100,
		method:     0x30,
		hasData:    true,
	})

	f, err := r.parseFileHeader(header, 0)
	if err != nil {
		t.Fatalf("parseFileHeader() error: %v", err)
	}
	if f.Path != "dir/subdir/file.txt" {
		t.Errorf("Path = %q, want %q", f.Path, "dir/subdir/file.txt")
	}
	// Verify Name() extracts basename
	if f.Name() != "file.txt" {
		t.Errorf("Name() = %q, want %q", f.Name(), "file.txt")
	}
}

func TestParseFileHeader_TooShort(t *testing.T) {
	r := &Reader{Files: make([]*File, 0)}

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"6 bytes", make([]byte, 6)},
		{"7 bytes non-file block", func() []byte {
			b := make([]byte, 7)
			b[2] = BlockHeader // not BlockFile
			return b
		}()},
		{"31 bytes too short for file header", func() []byte {
			b := make([]byte, 31)
			b[2] = BlockFile
			return b
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.parseFileHeader(tt.data, 0)
			if err == nil {
				t.Error("expected error for short/invalid header, got nil")
			}
		})
	}
}

func TestParseFileHeader_NextOffset_WithAndWithoutData(t *testing.T) {
	r := &Reader{Files: make([]*File, 0)}
	position := int64(50)

	// File with data: nextOffset = dataOffset + packSize
	withData := buildFileHeader(t, fileHeaderOpts{
		fileName:   "a.dat",
		packSize:   512,
		unpackSize: 512,
		method:     0x30,
		hasData:    true,
	})
	fData, err := r.parseFileHeader(withData, position)
	if err != nil {
		t.Fatalf("with data: %v", err)
	}
	headSize := int64(binary.LittleEndian.Uint16(withData[5:7]))
	expectedNext := position + headSize + 512
	if fData.NextOffset != expectedNext {
		t.Errorf("with data: NextOffset = %d, want %d", fData.NextOffset, expectedNext)
	}

	// File without data: nextOffset = dataOffset
	withoutData := buildFileHeader(t, fileHeaderOpts{
		fileName:   "b.dat",
		packSize:   512,
		unpackSize: 512,
		method:     0x30,
		hasData:    false,
	})
	fNoData, err := r.parseFileHeader(withoutData, position)
	if err != nil {
		t.Fatalf("without data: %v", err)
	}
	headSizeNoData := int64(binary.LittleEndian.Uint16(withoutData[5:7]))
	if fNoData.NextOffset != position+headSizeNoData {
		t.Errorf("without data: NextOffset = %d, want %d", fNoData.NextOffset, position+headSizeNoData)
	}
}

// --- HTTP-based integration tests ---

// buildMinimalRAR3 constructs the smallest valid RAR3 archive with a single
// uncompressed (Store method) file. Returns the raw archive bytes.
func buildMinimalRAR3(t *testing.T, fileName string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	// 1. RAR3 marker (7 bytes)
	buf.Write(Rar3Marker)

	// 2. Archive header block
	archiveHeader := buildArchiveHeader()
	buf.Write(archiveHeader)

	// 3. File header block
	fileHeader := buildRAR3FileBlock(t, fileName, content)
	buf.Write(fileHeader)

	// 4. File data (uncompressed)
	buf.Write(content)

	// 5. End block
	endBlock := buildEndBlock()
	buf.Write(endBlock)

	return buf.Bytes()
}

func buildArchiveHeader() []byte {
	// Minimal archive header: type=0x73, no flags, size=7
	header := make([]byte, 7)
	// [0:2] CRC (doesn't matter for our parser)
	header[2] = BlockHeader
	// [3:5] flags = 0
	binary.LittleEndian.PutUint16(header[5:7], 7) // headSize = 7
	return header
}

func buildRAR3FileBlock(t *testing.T, fileName string, content []byte) []byte {
	t.Helper()
	nameBytes := []byte(fileName)
	nameSize := uint16(len(nameBytes))
	headSize := uint16(32 + nameSize)
	packSize := uint32(len(content))
	unpackSize := uint32(len(content))

	flags := uint16(FlagHasData)

	buf := make([]byte, headSize)
	buf[2] = BlockFile
	binary.LittleEndian.PutUint16(buf[3:5], flags)
	binary.LittleEndian.PutUint16(buf[5:7], headSize)
	binary.LittleEndian.PutUint32(buf[7:11], packSize)
	binary.LittleEndian.PutUint32(buf[11:15], unpackSize)
	buf[15] = 0 // OS
	buf[25] = 0x30 // Store method
	binary.LittleEndian.PutUint16(buf[26:28], nameSize)
	copy(buf[32:], nameBytes)

	return buf
}

func buildEndBlock() []byte {
	header := make([]byte, 7)
	header[2] = BlockEnd
	binary.LittleEndian.PutUint16(header[5:7], 7)
	return header
}

func serveRAR(t *testing.T, data []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			rangeHeader := r.Header.Get("Range")
			if rangeHeader == "" {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
				w.WriteHeader(http.StatusOK)
				w.Write(data)
				return
			}
			var start, end int64
			fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
			if start >= int64(len(data)) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if end >= int64(len(data)) {
				end = int64(len(data)) - 1
			}
			w.Header().Set("Content-Range",
				fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(data[start : end+1])
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewReader_ValidArchive(t *testing.T) {
	archive := buildMinimalRAR3(t, "test.txt", []byte("hello world"))
	srv := serveRAR(t, archive)

	reader, err := NewReader(srv.URL)
	if err != nil {
		t.Fatalf("NewReader() error: %v", err)
	}
	if reader.Marker != 0 {
		t.Errorf("Marker = %d, want 0", reader.Marker)
	}
}

func TestNewReader_InvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"garbage", []byte("this is not a RAR archive at all")},
		{"truncated marker", Rar3Marker[:4]},
		{"marker but no header", Rar3Marker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := serveRAR(t, tt.data)
			_, err := NewReader(srv.URL)
			if err == nil {
				t.Error("expected error for invalid data, got nil")
			}
		})
	}
}

func TestNewReader_MarkerNotAtStart(t *testing.T) {
	// RAR3 with some leading garbage before the marker
	leading := bytes.Repeat([]byte{0xFF}, 256)
	archive := buildMinimalRAR3(t, "test.txt", []byte("data"))
	data := append(leading, archive...)

	srv := serveRAR(t, data)
	reader, err := NewReader(srv.URL)
	if err != nil {
		t.Fatalf("NewReader() error: %v", err)
	}
	if reader.Marker != 256 {
		t.Errorf("Marker = %d, want 256", reader.Marker)
	}
}

func TestGetFiles_SingleFile(t *testing.T) {
	content := []byte("file content here")
	archive := buildMinimalRAR3(t, "hello.txt", content)
	srv := serveRAR(t, archive)

	reader, err := NewReader(srv.URL)
	if err != nil {
		t.Fatalf("NewReader() error: %v", err)
	}

	files, err := reader.GetFiles()
	if err != nil {
		t.Fatalf("GetFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("GetFiles() returned %d files, want 1", len(files))
	}

	f := files[0]
	if f.Path != "hello.txt" {
		t.Errorf("Path = %q, want %q", f.Path, "hello.txt")
	}
	if f.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", f.Size, len(content))
	}
	if f.Method != 0x30 {
		t.Errorf("Method = %#x, want 0x30", f.Method)
	}
	if f.IsDirectory {
		t.Error("IsDirectory = true, want false")
	}
}

func TestExtractFile_ViaHTTP(t *testing.T) {
	content := []byte("extracted content test 12345")
	archive := buildMinimalRAR3(t, "data.bin", content)
	srv := serveRAR(t, archive)

	reader, err := NewReader(srv.URL)
	if err != nil {
		t.Fatalf("NewReader() error: %v", err)
	}

	files, err := reader.GetFiles()
	if err != nil {
		t.Fatalf("GetFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	data, err := reader.ExtractFile(files[0])
	if err != nil {
		t.Fatalf("ExtractFile() error: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("ExtractFile() = %q, want %q", data, content)
	}
}

func TestGetFiles_CalledTwiceReturnsCached(t *testing.T) {
	archive := buildMinimalRAR3(t, "cached.txt", []byte("data"))
	srv := serveRAR(t, archive)

	reader, err := NewReader(srv.URL)
	if err != nil {
		t.Fatalf("NewReader() error: %v", err)
	}

	files1, err := reader.GetFiles()
	if err != nil {
		t.Fatalf("first GetFiles() error: %v", err)
	}
	files2, err := reader.GetFiles()
	if err != nil {
		t.Fatalf("second GetFiles() error: %v", err)
	}

	if len(files1) != len(files2) {
		t.Fatalf("file counts differ: %d vs %d", len(files1), len(files2))
	}
	// Same slice (cached)
	if &files1[0] != &files2[0] {
		t.Error("expected GetFiles to return cached results")
	}
}

func TestNewReader_ServerDown(t *testing.T) {
	// Use a URL that won't connect
	_, err := NewReader("http://127.0.0.1:1") // port 1 should fail
	if err == nil {
		t.Error("expected error when server is unreachable, got nil")
	}
}

func TestReadAt_EOF(t *testing.T) {
	data := []byte("short")
	srv := serveRAR(t, data)

	file := &HttpFile{
		URL:        srv.URL,
		client:     &http.Client{},
		FileSize:   int64(len(data)),
		MaxRetries: 0,
	}

	buf := make([]byte, 10)
	_, err := file.ReadAt(buf, int64(len(data)))
	if err != io.EOF {
		t.Errorf("ReadAt past end: err = %v, want io.EOF", err)
	}
}

func TestReadAt_EmptyBuffer(t *testing.T) {
	file := &HttpFile{
		URL:      "http://example.com",
		client:   &http.Client{},
		FileSize: 100,
	}
	n, err := file.ReadAt([]byte{}, 0)
	if n != 0 || err != nil {
		t.Errorf("ReadAt(empty) = (%d, %v), want (0, nil)", n, err)
	}
}
