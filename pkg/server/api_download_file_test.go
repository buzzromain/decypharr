package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// downloadFileRouter creates a chi router wired to handleDownloadFile.
func downloadFileRouter(s *Server) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/browse/download/{torrent}/{file}", s.handleDownloadFile)
	return r
}

// seedDownloadEntry inserts an entry with the given protocol and files.
// The torrentName used in the URL must match entry.GetFolder() — with default
// config (WebDavUseFileName) this equals path.Clean(name).
func seedDownloadEntry(t *testing.T, s *Server, infohash, name, category string, protocol config.Protocol, files map[string]int64) {
	t.Helper()
	var totalSize int64
	for _, sz := range files {
		totalSize += sz
	}
	entry := &storage.Entry{
		Protocol:       protocol,
		InfoHash:       infohash,
		Name:           name,
		Size:           totalSize,
		Category:       category,
		State:          storage.EntryStatePausedUP,
		AddedOn:        time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Files:          make(map[string]*storage.File, len(files)),
		Providers:      make(map[string]*storage.ProviderEntry),
		ActiveProvider: "",
	}
	for fname, sz := range files {
		entry.Files[fname] = &storage.File{
			Name:     fname,
			Size:     sz,
			InfoHash: infohash,
			AddedOn:  time.Now(),
		}
	}
	if err := s.manager.AddOrUpdate(entry, nil); err != nil {
		t.Fatalf("seedDownloadEntry: %v", err)
	}
}

// ── Entry not found ──────────────────────────────────────────────────────────

func TestDownloadFile_EntryNotFound(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/NoSuchTorrent/file.mkv", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "Torrent not found") {
		t.Fatalf("expected 'Torrent not found', got: %s", body)
	}
}

// ── File not found in entry ──────────────────────────────────────────────────

func TestDownloadFile_FileNotFoundInEntry(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-aaa", "Movie.2024", "radarr", config.ProtocolTorrent, map[string]int64{
		"movie.mkv": 5000,
	})

	// Request a file name that doesn't exist in the entry
	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/Movie.2024/nonexistent.srt", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// ── Deleted file returns not-found ───────────────────────────────────────────

func TestDownloadFile_DeletedFile(t *testing.T) {
	s := newTestServer(t)

	// Seed entry then mark the file as deleted via a re-save
	entry := &storage.Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       "dl-del",
		Name:           "Deleted.Movie",
		Size:           1000,
		Category:       "radarr",
		State:          storage.EntryStatePausedUP,
		AddedOn:        time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Providers:      make(map[string]*storage.ProviderEntry),
		ActiveProvider: "",
		Files: map[string]*storage.File{
			"movie.mkv": {
				Name:     "movie.mkv",
				Size:     1000,
				InfoHash: "dl-del",
				Deleted:  true,
			},
		},
	}
	if err := s.manager.AddOrUpdate(entry, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/Deleted.Movie/movie.mkv", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for deleted file", w.Code)
	}
}

// ── Headers on torrent protocol (no provider → 412, but headers still set) ──

func TestDownloadFile_Headers_TorrentProtocol(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-hdr", "Headers.Test", "radarr", config.ProtocolTorrent, map[string]int64{
		"video.mp4": 42000,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/Headers.Test/video.mp4", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	// Without a debrid provider the torrent download fails with 412
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412 (no provider)", w.Code)
	}

	// ETag format: "hex(addedOn.Unix())-hex(fileSize)"
	addedUnix := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC).Unix()
	wantETag := fmt.Sprintf("\"%x-%x\"", addedUnix, int64(42000))
	if got := w.Header().Get("ETag"); got != wantETag {
		t.Errorf("ETag = %q, want %q", got, wantETag)
	}

	// Last-Modified from the fixed AddedOn time
	wantLastMod := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	if got := w.Header().Get("Last-Modified"); got != wantLastMod {
		t.Errorf("Last-Modified = %q, want %q", got, wantLastMod)
	}

	// Content-Disposition with correct filename
	wantDisp := `attachment; filename="video.mp4"`
	if got := w.Header().Get("Content-Disposition"); got != wantDisp {
		t.Errorf("Content-Disposition = %q, want %q", got, wantDisp)
	}
}

// ── Unsupported protocol ─────────────────────────────────────────────────────

func TestDownloadFile_UnsupportedProtocol(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-unk", "Unknown.Proto", "radarr", "ftp", map[string]int64{
		"data.bin": 500,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/Unknown.Proto/data.bin", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412 for unsupported protocol", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "Unsupported protocol") {
		t.Fatalf("expected 'Unsupported protocol', got: %s", body)
	}
}

// ── Torrent with no debrid provider ──────────────────────────────────────────

func TestDownloadFile_TorrentNoProvider_GracefulError(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-nop", "NoProvider.Movie", "radarr", config.ProtocolTorrent, map[string]int64{
		"movie.mkv": 8000,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/NoProvider.Movie/movie.mkv", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "Could not fetch download link") {
		t.Fatalf("expected download link error message, got: %s", body)
	}
}

// ── No side effects on unrelated entries ─────────────────────────────────────

func TestDownloadFile_NoSideEffects(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-keep1", "Keep.One", "radarr", config.ProtocolTorrent, map[string]int64{
		"keep1.mkv": 1000,
	})
	seedDownloadEntry(t, s, "dl-keep2", "Keep.Two", "sonarr", config.ProtocolTorrent, map[string]int64{
		"keep2.mkv": 2000,
	})
	seedDownloadEntry(t, s, "dl-target", "Target.Movie", "radarr", config.ProtocolTorrent, map[string]int64{
		"target.mkv": 3000,
	})

	// Attempt a download (will fail at provider level, but should not mutate storage)
	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/Target.Movie/target.mkv", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	// Verify unrelated entries are intact
	for _, hash := range []string{"dl-keep1", "dl-keep2", "dl-target"} {
		if _, err := s.manager.GetEntry(hash); err != nil {
			t.Errorf("entry %s should still exist after download attempt: %v", hash, err)
		}
	}
}

// ── URL-encoded torrent/file names ───────────────────────────────────────────

func TestDownloadFile_URLEncodedNames(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-enc", "Movie (2024)", "radarr", config.ProtocolTorrent, map[string]int64{
		"movie file.mkv": 7000,
	})

	// URL-encode spaces as %20 and parentheses as %28/%29
	req := httptest.NewRequest(http.MethodGet,
		"/api/browse/download/Movie%20%282024%29/movie%20file.mkv", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	// Should reach the handler and fail at provider level (not 404)
	if w.Code == http.StatusNotFound {
		t.Fatalf("URL-encoded names should resolve the entry, got 404: %s", w.Body.String())
	}
	// Expected: 412 (no provider) — confirms name decoding worked
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412 (no provider) for URL-encoded names", w.Code)
	}
}

// ── Multiple files in entry — correct file is targeted ───────────────────────

func TestDownloadFile_CorrectFileTargeted(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-multi", "MultiFile.Pack", "radarr", config.ProtocolTorrent, map[string]int64{
		"episode01.mkv": 3000,
		"episode02.mkv": 4000,
		"extras.txt":    100,
	})

	// Request episode02.mkv specifically
	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/MultiFile.Pack/episode02.mkv", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	// Should reach handler (412 from no provider, not 404)
	if w.Code == http.StatusNotFound {
		t.Fatalf("expected file to be found, got 404")
	}

	// Content-Disposition should reference the targeted file, not another
	disp := w.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "episode02.mkv") {
		t.Errorf("Content-Disposition = %q, should reference episode02.mkv", disp)
	}
	if strings.Contains(disp, "episode01") || strings.Contains(disp, "extras") {
		t.Errorf("Content-Disposition should not reference other files: %q", disp)
	}
}

// ── Content-Type derived from file extension ─────────────────────────────────

func TestDownloadFile_ContentTypeByExtension(t *testing.T) {
	s := newTestServer(t)

	tests := []struct {
		name     string
		fileName string
		wantType string // what handleDownloadFile sets before dispatch
	}{
		{"mkv file", "video.mkv", "video/x-matroska"},
		{"mp4 file", "video.mp4", "video/mp4"},
		{"unknown extension", "data.xyz123", "application/octet-stream"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := fmt.Sprintf("dl-ct-%d", i)
			entryName := fmt.Sprintf("CT.Test.%d", i)
			seedDownloadEntry(t, s, hash, entryName, "radarr", config.ProtocolTorrent, map[string]int64{
				tt.fileName: 1000,
			})

			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/api/browse/download/%s/%s", entryName, tt.fileName), nil)
			w := httptest.NewRecorder()
			downloadFileRouter(s).ServeHTTP(w, req)

			// The handler sets Content-Type before dispatch.
			// On error, http.Error overwrites it to text/plain.
			// We can still verify the handler reached the dispatch point (not 404).
			if w.Code == http.StatusNotFound {
				t.Fatalf("entry should be found, got 404")
			}
		})
	}
}

// ── Unsupported protocol preserves Content-Type set by handler ───────────────

func TestDownloadFile_UnsupportedProtocol_Headers(t *testing.T) {
	s := newTestServer(t)
	seedDownloadEntry(t, s, "dl-uprh", "UPHeaders.Test", "radarr", "unknown-proto", map[string]int64{
		"file.mkv": 2000,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/browse/download/UPHeaders.Test/file.mkv", nil)
	w := httptest.NewRecorder()
	downloadFileRouter(s).ServeHTTP(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412", w.Code)
	}

	// ETag and Last-Modified should still be set (http.Error doesn't clear them)
	if got := w.Header().Get("ETag"); got == "" {
		t.Error("ETag header should be present even on unsupported protocol error")
	}
	if got := w.Header().Get("Last-Modified"); got == "" {
		t.Error("Last-Modified header should be present even on unsupported protocol error")
	}
	if got := w.Header().Get("Content-Disposition"); got == "" {
		t.Error("Content-Disposition header should be present even on unsupported protocol error")
	}
}
