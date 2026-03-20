package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// seedQueueEntry adds an entry to the manager's queue (not the main entry store).
// handleGetTorrents reads from Queue().ListFilter, which queries the queue store.
func seedQueueEntry(t *testing.T, s *Server, infohash, name, category string, size int64, state storage.TorrentState) {
	t.Helper()
	entry := &storage.Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       infohash,
		Name:           name,
		Size:           size,
		Category:       category,
		State:          state,
		AddedOn:        time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Files:          make(map[string]*storage.File),
		Providers:      make(map[string]*storage.ProviderEntry),
		ActiveProvider: "",
	}
	if err := s.manager.Queue().Add(entry); err != nil {
		t.Fatalf("seedQueueEntry: %v", err)
	}
}

// ── handleGetTorrents ────────────────────────────────────────────────────────

func TestGetTorrents_Empty(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	// Verify all required fields
	requiredFields := []string{"torrents", "total", "page", "limit", "total_pages", "has_prev", "has_next", "categories"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field %q", field)
		}
	}

	// torrents must be an array, not null
	torrents, ok := raw["torrents"].([]interface{})
	if !ok {
		t.Fatalf("torrents must be a JSON array, got %T", raw["torrents"])
	}
	if len(torrents) != 0 {
		t.Errorf("expected empty torrents, got %d", len(torrents))
	}

	// total should be 0
	if total, _ := raw["total"].(float64); total != 0 {
		t.Errorf("expected total=0, got %v", total)
	}

	// categories must be an array
	cats, ok := raw["categories"].([]interface{})
	if !ok {
		t.Fatalf("categories must be a JSON array, got %T", raw["categories"])
	}
	if len(cats) != 0 {
		t.Errorf("expected empty categories, got %d", len(cats))
	}
}

func TestGetTorrents_WithEntries(t *testing.T) {
	s := newTestServer(t)
	seedQueueEntry(t, s, "qt111", "Movie.2024", "radarr", 5000, storage.EntryStatePausedUP)
	seedQueueEntry(t, s, "qt222", "Show.S01E01", "sonarr", 3000, storage.EntryStatePausedUP)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	if total, _ := raw["total"].(float64); total != 2 {
		t.Errorf("expected total=2, got %v", total)
	}

	torrents := raw["torrents"].([]interface{})
	if len(torrents) != 2 {
		t.Fatalf("expected 2 torrents, got %d", len(torrents))
	}

	// Verify torrent entry field names are stable
	entry := torrents[0].(map[string]interface{})
	stableFields := []string{"info_hash", "name", "size", "category", "state", "protocol", "files", "providers", "active_provider"}
	for _, field := range stableFields {
		if _, ok := entry[field]; !ok {
			t.Errorf("missing stable field %q in torrent entry", field)
		}
	}
}

func TestGetTorrents_ResponseShape_FieldTypes(t *testing.T) {
	s := newTestServer(t)
	seedQueueEntry(t, s, "ft111", "TypeCheck", "radarr", 5000, storage.EntryStatePausedUP)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	// top-level field type assertions
	if _, ok := raw["total"].(float64); !ok {
		t.Errorf("total should be a number, got %T", raw["total"])
	}
	if _, ok := raw["page"].(float64); !ok {
		t.Errorf("page should be a number, got %T", raw["page"])
	}
	if _, ok := raw["limit"].(float64); !ok {
		t.Errorf("limit should be a number, got %T", raw["limit"])
	}
	if _, ok := raw["has_prev"].(bool); !ok {
		t.Errorf("has_prev should be a bool, got %T", raw["has_prev"])
	}
	if _, ok := raw["has_next"].(bool); !ok {
		t.Errorf("has_next should be a bool, got %T", raw["has_next"])
	}
	if _, ok := raw["torrents"].([]interface{}); !ok {
		t.Errorf("torrents should be an array, got %T", raw["torrents"])
	}
	if _, ok := raw["categories"].([]interface{}); !ok {
		t.Errorf("categories should be an array, got %T", raw["categories"])
	}
}

func TestGetTorrents_FilterByCategory(t *testing.T) {
	s := newTestServer(t)
	seedQueueEntry(t, s, "fc111", "RadarrMovie", "radarr", 5000, storage.EntryStatePausedUP)
	seedQueueEntry(t, s, "fc222", "SonarrShow", "sonarr", 3000, storage.EntryStatePausedUP)
	seedQueueEntry(t, s, "fc333", "RadarrMovie2", "radarr", 4000, storage.EntryStatePausedUP)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents?category=radarr", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	if total, _ := raw["total"].(float64); total != 2 {
		t.Errorf("expected total=2 for category=radarr, got %v", total)
	}

	torrents := raw["torrents"].([]interface{})
	for _, tr := range torrents {
		entry := tr.(map[string]interface{})
		if cat, _ := entry["category"].(string); cat != "radarr" {
			t.Errorf("expected category=radarr, got %q", cat)
		}
	}
}

func TestGetTorrents_FilterBySearch(t *testing.T) {
	s := newTestServer(t)
	seedQueueEntry(t, s, "fs111", "Awesome.Movie.2024", "radarr", 5000, storage.EntryStatePausedUP)
	seedQueueEntry(t, s, "fs222", "Boring.Show.2024", "sonarr", 3000, storage.EntryStatePausedUP)

	// Search by name substring
	r := httptest.NewRequest(http.MethodGet, "/api/torrents?search=awesome", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	if total, _ := raw["total"].(float64); total != 1 {
		t.Errorf("expected total=1 for search=awesome, got %v", total)
	}

	// Search by infohash
	r2 := httptest.NewRequest(http.MethodGet, "/api/torrents?search=fs222", nil)
	w2 := httptest.NewRecorder()
	s.handleGetTorrents(w2, r2)

	var raw2 map[string]interface{}
	decodeJSON(t, w2, &raw2)

	if total, _ := raw2["total"].(float64); total != 1 {
		t.Errorf("expected total=1 for search by hash, got %v", total)
	}
}

func TestGetTorrents_FilterByState(t *testing.T) {
	s := newTestServer(t)
	seedQueueEntry(t, s, "st111", "Paused", "radarr", 5000, storage.EntryStatePausedUP)
	seedQueueEntry(t, s, "st222", "Downloading", "radarr", 3000, storage.EntryStateDownloading)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents?state=pausedUP", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	if total, _ := raw["total"].(float64); total != 1 {
		t.Errorf("expected total=1 for state=pausedUP, got %v", total)
	}
}

func TestGetTorrents_Pagination(t *testing.T) {
	s := newTestServer(t)
	for i := 0; i < 5; i++ {
		hash := "pg" + string(rune('a'+i)) + "00"
		seedQueueEntry(t, s, hash, "Entry"+string(rune('A'+i)), "radarr", int64(1000*(i+1)), storage.EntryStatePausedUP)
	}

	// Request page 1 with limit 2
	r := httptest.NewRequest(http.MethodGet, "/api/torrents?page=1&limit=2", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	if total, _ := raw["total"].(float64); total != 5 {
		t.Errorf("expected total=5, got %v", total)
	}
	torrents := raw["torrents"].([]interface{})
	if len(torrents) != 2 {
		t.Errorf("expected 2 torrents on page 1, got %d", len(torrents))
	}
	if hasPrev, _ := raw["has_prev"].(bool); hasPrev {
		t.Error("page 1 should not have has_prev=true")
	}
	if hasNext, _ := raw["has_next"].(bool); !hasNext {
		t.Error("page 1 should have has_next=true")
	}

	// Request page 3 (last page with 1 item)
	r2 := httptest.NewRequest(http.MethodGet, "/api/torrents?page=3&limit=2", nil)
	w2 := httptest.NewRecorder()
	s.handleGetTorrents(w2, r2)

	var raw2 map[string]interface{}
	decodeJSON(t, w2, &raw2)

	torrents2 := raw2["torrents"].([]interface{})
	if len(torrents2) != 1 {
		t.Errorf("expected 1 torrent on last page, got %d", len(torrents2))
	}
	if hasPrev, _ := raw2["has_prev"].(bool); !hasPrev {
		t.Error("last page should have has_prev=true")
	}
	if hasNext, _ := raw2["has_next"].(bool); hasNext {
		t.Error("last page should not have has_next=true")
	}
}

func TestGetTorrents_Categories(t *testing.T) {
	s := newTestServer(t)
	seedQueueEntry(t, s, "cat111", "Movie1", "radarr", 5000, storage.EntryStatePausedUP)
	seedQueueEntry(t, s, "cat222", "Show1", "sonarr", 3000, storage.EntryStatePausedUP)
	seedQueueEntry(t, s, "cat333", "Movie2", "radarr", 4000, storage.EntryStatePausedUP)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	cats := raw["categories"].([]interface{})
	catSet := make(map[string]bool)
	for _, c := range cats {
		catSet[c.(string)] = true
	}

	if !catSet["radarr"] {
		t.Error("categories should include radarr")
	}
	if !catSet["sonarr"] {
		t.Error("categories should include sonarr")
	}
	if len(catSet) != 2 {
		t.Errorf("expected 2 unique categories, got %d", len(catSet))
	}
}

func TestGetTorrents_DefaultPaginationValues(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	// Default page=1, limit=20
	if page, _ := raw["page"].(float64); page != 1 {
		t.Errorf("expected default page=1, got %v", page)
	}
	if limit, _ := raw["limit"].(float64); limit != 20 {
		t.Errorf("expected default limit=20, got %v", limit)
	}
}

func TestGetTorrents_OutOfRangePage(t *testing.T) {
	s := newTestServer(t)
	seedQueueEntry(t, s, "oor111", "Single", "radarr", 1000, storage.EntryStatePausedUP)

	r := httptest.NewRequest(http.MethodGet, "/api/torrents?page=999", nil)
	w := httptest.NewRecorder()
	s.handleGetTorrents(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	torrents := raw["torrents"].([]interface{})
	if len(torrents) != 0 {
		t.Errorf("expected empty torrents for out-of-range page, got %d", len(torrents))
	}

	// total should still reflect all entries
	if total, _ := raw["total"].(float64); total != 1 {
		t.Errorf("total should be 1 even on out-of-range page, got %v", total)
	}
}
