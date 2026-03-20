package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	json "github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// seedEntryWithFiles inserts a storage entry with multiple named files.
func seedEntryWithFiles(t *testing.T, s *Server, infohash, name, category string, files map[string]int64) {
	t.Helper()
	var totalSize int64
	for _, sz := range files {
		totalSize += sz
	}
	entry := &storage.Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       infohash,
		Name:           name,
		Size:           totalSize,
		Category:       category,
		State:          storage.EntryStatePausedUP,
		AddedOn:        time.Now(),
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
		t.Fatalf("seedEntryWithFiles: %v", err)
	}
}

// browseTorrentFilesRouter creates a chi router wired to handleBrowseTorrentFiles
// on both the two-segment and three-segment browse paths.
func browseTorrentFilesRouter(s *Server) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/browse/{group}/{torrent}", s.handleBrowseTorrentFiles)
	r.Get("/api/browse/{group}/{subgroup}/{torrent}", s.handleBrowseTorrentFiles)
	return r
}

// browseDeleteRouter creates a chi router wired to the delete endpoints.
func browseDeleteRouter(s *Server) *chi.Mux {
	r := chi.NewRouter()
	r.Delete("/api/browse/torrents/{id}", s.handleDeleteBrowseTorrent)
	r.Delete("/api/browse/torrents/batch", s.handleBatchDeleteBrowseTorrents)
	return r
}

// ── handleBrowseTorrentFiles ─────────────────────────────────────────────────

func TestBrowseTorrentFiles_Success(t *testing.T) {
	s := newTestServer(t)
	seedEntryWithFiles(t, s, "aaa111", "Movie.2024", "radarr", map[string]int64{
		"movie.mkv": 5000,
		"subs.srt":  200,
	})

	r := httptest.NewRequest(http.MethodGet, "/api/browse/__all__/Movie.2024", nil)
	w := httptest.NewRecorder()
	browseTorrentFilesRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 files, got %d", resp.Total)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}

	// Verify file entry fields are populated
	for _, e := range resp.Entries {
		if e.Name == "" {
			t.Error("entry Name must not be empty")
		}
		if e.Path == "" {
			t.Error("entry Path must not be empty")
		}
		if e.Size <= 0 {
			t.Errorf("entry Size should be positive, got %d", e.Size)
		}
		if e.ModTime == "" {
			t.Error("entry ModTime must not be empty")
		}
	}
}

func TestBrowseTorrentFiles_NotFound(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/browse/__all__/NonExistent.2024", nil)
	w := httptest.NewRecorder()
	browseTorrentFilesRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBrowseTorrentFiles_SearchFilter(t *testing.T) {
	s := newTestServer(t)
	seedEntryWithFiles(t, s, "bbb222", "Show.S01", "sonarr", map[string]int64{
		"episode01.mkv": 3000,
		"episode02.mkv": 3100,
		"trailer.mp4":   500,
	})

	r := httptest.NewRequest(http.MethodGet, "/api/browse/__all__/Show.S01?search=episode", nil)
	w := httptest.NewRecorder()
	browseTorrentFilesRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 matching files, got %d", resp.Total)
	}
	for _, e := range resp.Entries {
		if e.Name != "episode01.mkv" && e.Name != "episode02.mkv" {
			t.Errorf("unexpected file in search results: %s", e.Name)
		}
	}
}

func TestBrowseTorrentFiles_ResponseShape(t *testing.T) {
	s := newTestServer(t)
	seedEntryWithFiles(t, s, "ccc333", "Pack.2024", "radarr", map[string]int64{
		"file.mkv": 1000,
	})

	r := httptest.NewRequest(http.MethodGet, "/api/browse/__all__/Pack.2024", nil)
	w := httptest.NewRecorder()
	browseTorrentFilesRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Decode into raw map to verify field names and types
	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	requiredFields := []string{"entries", "total", "page", "limit", "total_pages", "current_dir", "parent_dir"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field %q in response", field)
		}
	}

	// entries must be an array, not null
	entries, ok := raw["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries must be a JSON array, got %T", raw["entries"])
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	// Verify entry field names
	entry := entries[0].(map[string]interface{})
	entryFields := []string{"name", "path", "size", "mod_time", "is_dir", "active_debrid"}
	for _, field := range entryFields {
		if _, ok := entry[field]; !ok {
			t.Errorf("missing required entry field %q", field)
		}
	}

	// current_dir and parent_dir must be strings with expected values
	if cd, _ := raw["current_dir"].(string); cd != "/__all__/Pack.2024" {
		t.Errorf("expected current_dir=/__all__/Pack.2024, got %q", cd)
	}
	if pd, _ := raw["parent_dir"].(string); pd != "/__all__" {
		t.Errorf("expected parent_dir=/__all__, got %q", pd)
	}
}

func TestBrowseTorrentFiles_EmptyPage(t *testing.T) {
	s := newTestServer(t)
	seedEntryWithFiles(t, s, "ddd444", "Small.Movie", "radarr", map[string]int64{
		"file.mkv": 1000,
	})

	// Page well beyond total
	r := httptest.NewRequest(http.MethodGet, "/api/browse/__all__/Small.Movie?page=9999", nil)
	w := httptest.NewRecorder()
	browseTorrentFilesRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	entries, ok := raw["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries must be a JSON array, got %T", raw["entries"])
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries for out-of-range page, got %d", len(entries))
	}
}

// ── handleDeleteBrowseTorrent ────────────────────────────────────────────────

func TestDeleteBrowseTorrent_Success(t *testing.T) {
	s := newTestServer(t)
	seedEntry(t, s, "del111", "ToDelete", "radarr", 1000)

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/del111", nil)
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, w, &resp)

	if success, _ := resp["success"].(bool); !success {
		t.Error("expected success=true")
	}
	if msg, _ := resp["message"].(string); msg == "" {
		t.Error("expected non-empty message")
	}

	// Verify the entry is actually gone from storage
	if _, err := s.manager.GetEntry("del111"); err == nil {
		t.Error("entry should have been deleted from storage")
	}
}

func TestDeleteBrowseTorrent_NotFound(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/nonexistent999", nil)
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	// DeleteEntry returns error for missing entry → handler returns 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for missing entry, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteBrowseTorrent_NoSideEffects(t *testing.T) {
	s := newTestServer(t)
	seedEntry(t, s, "keep111", "KeepMe", "radarr", 1000)
	seedEntry(t, s, "del222", "DeleteMe", "radarr", 2000)
	seedEntry(t, s, "keep333", "KeepMeToo", "sonarr", 3000)

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/del222", nil)
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Deleted entry should be gone
	if _, err := s.manager.GetEntry("del222"); err == nil {
		t.Error("del222 should have been deleted")
	}

	// Unrelated entries must still exist
	if _, err := s.manager.GetEntry("keep111"); err != nil {
		t.Errorf("keep111 should still exist: %v", err)
	}
	if _, err := s.manager.GetEntry("keep333"); err != nil {
		t.Errorf("keep333 should still exist: %v", err)
	}
}

// ── handleBatchDeleteBrowseTorrents ──────────────────────────────────────────

func TestBatchDeleteBrowseTorrents_Success(t *testing.T) {
	s := newTestServer(t)
	seedEntry(t, s, "bat111", "Batch1", "radarr", 1000)
	seedEntry(t, s, "bat222", "Batch2", "radarr", 2000)
	seedEntry(t, s, "bat333", "Batch3", "sonarr", 3000)

	body, _ := json.Marshal(map[string]interface{}{
		"ids": []string{"bat111", "bat222"},
	})

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/batch", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, w, &resp)

	if success, _ := resp["success"].(bool); !success {
		t.Error("expected success=true")
	}
	if count, _ := resp["count"].(float64); count != 2 {
		t.Errorf("expected count=2, got %v", count)
	}

	// Verify deleted entries are gone
	if _, err := s.manager.GetEntry("bat111"); err == nil {
		t.Error("bat111 should be deleted")
	}
	if _, err := s.manager.GetEntry("bat222"); err == nil {
		t.Error("bat222 should be deleted")
	}

	// Unrelated entry must remain
	if _, err := s.manager.GetEntry("bat333"); err != nil {
		t.Errorf("bat333 should still exist: %v", err)
	}
}

func TestBatchDeleteBrowseTorrents_ResponseShape(t *testing.T) {
	s := newTestServer(t)
	seedEntry(t, s, "shp111", "Shape1", "radarr", 1000)

	body, _ := json.Marshal(map[string]interface{}{
		"ids": []string{"shp111"},
	})

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/batch", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	for _, field := range []string{"success", "message", "count"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field %q in batch delete response", field)
		}
	}
}

func TestBatchDeleteBrowseTorrents_EmptyIDs(t *testing.T) {
	s := newTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"ids": []string{},
	})

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/batch", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty IDs, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBatchDeleteBrowseTorrents_InvalidJSON(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/batch",
		bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestBatchDeleteBrowseTorrents_PartialInvalid_FailFast(t *testing.T) {
	s := newTestServer(t)
	seedEntry(t, s, "pf111", "PartialKeep", "radarr", 1000)

	// First ID is invalid, second exists. DeleteTorrents fails on first error.
	body, _ := json.Marshal(map[string]interface{}{
		"ids": []string{"nonexistent", "pf111"},
	})

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/batch", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	// Should fail because first ID doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for partial invalid batch, got %d: %s", w.Code, w.Body.String())
	}

	// The valid entry should NOT have been deleted (fail-fast means it was never reached)
	if _, err := s.manager.GetEntry("pf111"); err != nil {
		t.Errorf("pf111 should still exist after fail-fast: %v", err)
	}
}

func TestBatchDeleteBrowseTorrents_NoSideEffects(t *testing.T) {
	s := newTestServer(t)
	seedEntry(t, s, "ns111", "NoSide1", "radarr", 1000)
	seedEntry(t, s, "ns222", "NoSide2", "radarr", 2000)
	seedEntry(t, s, "ns333", "NoSide3", "sonarr", 3000)

	body, _ := json.Marshal(map[string]interface{}{
		"ids": []string{"ns111"},
	})

	r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/batch", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	browseDeleteRouter(s).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Only ns111 should be deleted
	if _, err := s.manager.GetEntry("ns111"); err == nil {
		t.Error("ns111 should be deleted")
	}
	if _, err := s.manager.GetEntry("ns222"); err != nil {
		t.Errorf("ns222 should still exist: %v", err)
	}
	if _, err := s.manager.GetEntry("ns333"); err != nil {
		t.Errorf("ns333 should still exist: %v", err)
	}
}

func TestBatchDeleteBrowseTorrents_Idempotent(t *testing.T) {
	s := newTestServer(t)
	seedEntry(t, s, "idem111", "Idempotent", "radarr", 1000)

	makeReq := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]interface{}{
			"ids": []string{"idem111"},
		})
		r := httptest.NewRequest(http.MethodDelete, "/api/browse/torrents/batch", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		browseDeleteRouter(s).ServeHTTP(w, r)
		return w
	}

	// First delete should succeed
	w1 := makeReq()
	if w1.Code != http.StatusOK {
		t.Fatalf("first delete: expected 200, got %d", w1.Code)
	}

	// Second delete of same ID should fail (entry already gone)
	w2 := makeReq()
	if w2.Code != http.StatusInternalServerError {
		t.Fatalf("second delete: expected 500 (entry gone), got %d", w2.Code)
	}
}
