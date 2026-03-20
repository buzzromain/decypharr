package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestGetBrowseSortParams_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/browse", nil)
	sortBy, sortOrder := getBrowseSortParams(r)
	if sortBy != "name" {
		t.Errorf("sortBy = %q, want 'name'", sortBy)
	}
	if sortOrder != "asc" {
		t.Errorf("sortOrder = %q, want 'asc'", sortOrder)
	}
}

func TestGetBrowseSortParams_ValidFields(t *testing.T) {
	for _, field := range []string{"name", "size", "mod_time", "active_debrid"} {
		r := httptest.NewRequest(http.MethodGet, "/api/browse?sort_by="+field+"&sort_order=desc", nil)
		sortBy, sortOrder := getBrowseSortParams(r)
		if sortBy != field {
			t.Errorf("sortBy = %q, want %q", sortBy, field)
		}
		if sortOrder != "desc" {
			t.Errorf("sortOrder = %q, want 'desc'", sortOrder)
		}
	}
}

func TestGetBrowseSortParams_InvalidFallsBackToDefault(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/browse?sort_by=invalid_field&sort_order=invalid", nil)
	sortBy, sortOrder := getBrowseSortParams(r)
	if sortBy != "name" {
		t.Errorf("sortBy = %q, want 'name' (default)", sortBy)
	}
	if sortOrder != "asc" {
		t.Errorf("sortOrder = %q, want 'asc' (default for non-desc)", sortOrder)
	}
}

func TestSortBrowseEntries_DirsAlwaysFirst(t *testing.T) {
	entries := []BrowseEntry{
		{Name: "beta_file.mkv", IsDir: false, Size: 100},
		{Name: "alpha_dir", IsDir: true, Size: 0},
		{Name: "gamma_file.mkv", IsDir: false, Size: 200},
		{Name: "delta_dir", IsDir: true, Size: 0},
	}
	sortBrowseEntries(entries, "name", "asc")

	if !entries[0].IsDir || !entries[1].IsDir {
		t.Errorf("dirs not first: got %v, %v", entries[0].Name, entries[1].Name)
	}
	if entries[0].Name != "alpha_dir" {
		t.Errorf("first dir = %q, want 'alpha_dir'", entries[0].Name)
	}
	if entries[1].Name != "delta_dir" {
		t.Errorf("second dir = %q, want 'delta_dir'", entries[1].Name)
	}
	if entries[2].Name != "beta_file.mkv" {
		t.Errorf("first file = %q, want 'beta_file.mkv'", entries[2].Name)
	}
}

func TestSortBrowseEntries_SizeDescending(t *testing.T) {
	entries := []BrowseEntry{
		{Name: "small.mkv", Size: 100},
		{Name: "big.mkv", Size: 1000},
		{Name: "medium.mkv", Size: 500},
	}
	sortBrowseEntries(entries, "size", "desc")

	if entries[0].Name != "big.mkv" {
		t.Errorf("first = %q, want 'big.mkv'", entries[0].Name)
	}
	if entries[1].Name != "medium.mkv" {
		t.Errorf("second = %q, want 'medium.mkv'", entries[1].Name)
	}
	if entries[2].Name != "small.mkv" {
		t.Errorf("third = %q, want 'small.mkv'", entries[2].Name)
	}
}

func TestSortBrowseEntries_StableOnTies(t *testing.T) {
	entries := []BrowseEntry{
		{Name: "b.mkv", Size: 100, Path: "/b"},
		{Name: "a.mkv", Size: 100, Path: "/a"},
		{Name: "c.mkv", Size: 100, Path: "/c"},
	}
	sortBrowseEntries(entries, "size", "asc")

	if entries[0].Name != "a.mkv" {
		t.Errorf("first = %q, want 'a.mkv'", entries[0].Name)
	}
	if entries[1].Name != "b.mkv" {
		t.Errorf("second = %q, want 'b.mkv'", entries[1].Name)
	}
}

func TestBrowse_DefaultPaging(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/browse", nil)
	w := httptest.NewRecorder()
	s.handleBrowseMount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if resp.Page != 1 {
		t.Errorf("page = %d, want 1", resp.Page)
	}
	if resp.Limit != 50 {
		t.Errorf("limit = %d, want 50 (default)", resp.Limit)
	}
	if resp.CurrentDir != "/" {
		t.Errorf("current_dir = %q, want '/'", resp.CurrentDir)
	}
	if resp.Total < 5 {
		t.Errorf("total = %d, want >= 5 (static entries)", resp.Total)
	}
	if len(resp.Entries) == 0 {
		t.Error("entries should not be empty for mount browse")
	}
}

func TestBrowse_CustomPaging(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/browse?page=2&limit=2", nil)
	w := httptest.NewRecorder()
	s.handleBrowseMount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if resp.Page != 2 {
		t.Errorf("page = %d, want 2", resp.Page)
	}
	if resp.Limit != 2 {
		t.Errorf("limit = %d, want 2", resp.Limit)
	}
	if len(resp.Entries) > 2 {
		t.Errorf("entries count = %d, want <= 2", len(resp.Entries))
	}
	expectedPages := (resp.Total + 2 - 1) / 2
	if resp.TotalPages != expectedPages {
		t.Errorf("total_pages = %d, want %d", resp.TotalPages, expectedPages)
	}
}

func TestBrowse_Sorting(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/browse?sort_by=name&sort_order=desc", nil)
	w := httptest.NewRecorder()
	s.handleBrowseMount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if len(resp.Entries) < 2 {
		t.Skip("not enough entries to verify sort")
	}

	dirs := make([]string, 0)
	for _, e := range resp.Entries {
		if e.IsDir {
			dirs = append(dirs, e.Name)
		}
	}
	for i := 1; i < len(dirs); i++ {
		if strings.ToLower(dirs[i-1]) < strings.ToLower(dirs[i]) {
			t.Errorf("dirs not sorted desc: %q < %q", dirs[i-1], dirs[i])
		}
	}
}

func TestBrowse_InvalidPaging_ClampsToDefaults(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/browse?page=-1&limit=-5", nil)
	w := httptest.NewRecorder()
	s.handleBrowseMount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if resp.Page != 1 {
		t.Errorf("page = %d, want 1 (clamped)", resp.Page)
	}
	if resp.Limit != 50 {
		t.Errorf("limit = %d, want 50 (clamped)", resp.Limit)
	}
}

func TestBrowse_LimitClampedTo100(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/browse?limit=500", nil)
	w := httptest.NewRecorder()
	s.handleBrowseMount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if resp.Limit != 50 {
		t.Errorf("limit = %d, want 50 (clamped from >100)", resp.Limit)
	}
}

func TestBrowse_EmptyResultShape(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/browse?page=9999", nil)
	w := httptest.NewRecorder()
	s.handleBrowseMount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var raw map[string]interface{}
	decodeJSON(t, w, &raw)

	requiredFields := []string{"entries", "total", "page", "limit", "total_pages", "current_dir"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field %q in response", field)
		}
	}

	entries, ok := raw["entries"].([]interface{})
	if !ok {
		t.Fatal("'entries' is not an array")
	}
	if len(entries) != 0 {
		t.Errorf("entries len = %d, want 0 for out-of-range page", len(entries))
	}
}

func TestBrowseGroup_EmptyGroup(t *testing.T) {
	s := newTestServer(t)

	chiR := chi.NewRouter()
	chiR.Get("/api/browse/{group}", s.handleBrowseGroup)

	r := httptest.NewRequest(http.MethodGet, "/api/browse/__all__", nil)
	w := httptest.NewRecorder()
	chiR.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)
	if resp.CurrentDir != "/__all__" {
		t.Errorf("current_dir = %q, want '/__all__'", resp.CurrentDir)
	}
	if resp.ParentDir != "/" {
		t.Errorf("parent_dir = %q, want '/'", resp.ParentDir)
	}
}

func TestBrowseGroup_WithSeededEntries(t *testing.T) {
	s := newTestServer(t)

	seedEntry(t, s, "aaa111", "Movie.2024", "radarr", 1000)
	seedEntry(t, s, "bbb222", "Show.S01E01", "sonarr", 2000)
	seedEntry(t, s, "ccc333", "Another.Movie", "radarr", 500)

	s.manager.RefreshEntries(true)

	chiR := chi.NewRouter()
	chiR.Get("/api/browse/{group}", s.handleBrowseGroup)

	r := httptest.NewRequest(http.MethodGet, "/api/browse/__all__?sort_by=name&sort_order=asc", nil)
	w := httptest.NewRecorder()
	chiR.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp BrowseResponse
	decodeJSON(t, w, &resp)

	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}

	if len(resp.Entries) >= 2 {
		for i := 1; i < len(resp.Entries); i++ {
			if strings.ToLower(resp.Entries[i-1].Name) > strings.ToLower(resp.Entries[i].Name) {
				t.Errorf("entries not sorted asc: %q > %q", resp.Entries[i-1].Name, resp.Entries[i].Name)
			}
		}
	}
}
