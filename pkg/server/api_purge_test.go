package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestPurgeLocalPreview_EmptyStorage(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/maintenance/purge/local", nil)
	w := httptest.NewRecorder()
	s.handlePurgeLocalPreview(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]interface{}
	decodeJSON(t, w, &resp)

	count, _ := resp["count"].(float64)
	if count != 0 {
		t.Errorf("count = %v, want 0", count)
	}
	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("'entries' is not an array")
	}
	if len(entries) != 0 {
		t.Errorf("entries len = %d, want 0", len(entries))
	}
}

func TestPurgeLocalPreview_ReturnsExpectedCandidates(t *testing.T) {
	s := newTestServer(t)

	seedEntry(t, s, "unmanaged1", "Orphan.Movie", "", 1000)
	seedEntry(t, s, "unmanaged2", "Orphan.Show", "", 2000)
	seedEntry(t, s, "managed1", "Managed.Movie", "radarr", 3000)

	r := httptest.NewRequest(http.MethodGet, "/api/maintenance/purge/local", nil)
	w := httptest.NewRecorder()
	s.handlePurgeLocalPreview(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, w, &resp)

	count, _ := resp["count"].(float64)
	if int(count) != 2 {
		t.Errorf("count = %v, want 2 (only unmanaged entries)", count)
	}

	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("'entries' is not an array")
	}
	if len(entries) != 2 {
		t.Errorf("entries len = %d, want 2", len(entries))
	}

	for i, e := range entries {
		entry, ok := e.(map[string]interface{})
		if !ok {
			t.Fatalf("entry[%d] is not an object", i)
		}
		for _, field := range []string{"infohash", "name", "size"} {
			if _, ok := entry[field]; !ok {
				t.Errorf("entry[%d] missing field %q", i, field)
			}
		}
	}
}

func TestPurgeLocalPreview_DoesNotMutateState(t *testing.T) {
	s := newTestServer(t)

	seedEntry(t, s, "orphan1", "Orphan.Movie", "", 1000)

	r := httptest.NewRequest(http.MethodGet, "/api/maintenance/purge/local", nil)
	w := httptest.NewRecorder()
	s.handlePurgeLocalPreview(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want %d", w.Code, http.StatusOK)
	}

	entry, err := s.manager.GetEntry("orphan1")
	if err != nil {
		t.Fatalf("GetEntry after preview: %v", err)
	}
	if entry == nil {
		t.Fatal("entry was deleted by preview — preview must not mutate state")
	}
}

func TestPurgeLocalExecute_RemovesExpectedItems(t *testing.T) {
	s := newTestServer(t)

	seedEntry(t, s, "orphanExec1", "Orphan.Exec.1", "", 1000)
	seedEntry(t, s, "orphanExec2", "Orphan.Exec.2", "", 2000)
	seedEntry(t, s, "managedExec1", "Managed.Exec.1", "radarr", 3000)

	previewR := httptest.NewRequest(http.MethodGet, "/api/maintenance/purge/local", nil)
	previewW := httptest.NewRecorder()
	s.handlePurgeLocalPreview(previewW, previewR)

	var previewResp map[string]interface{}
	decodeJSON(t, previewW, &previewResp)
	expectedDeleted := int(previewResp["count"].(float64))
	if expectedDeleted < 2 {
		t.Fatalf("expected at least 2 unmanaged entries, got %d", expectedDeleted)
	}

	r := httptest.NewRequest(http.MethodDelete, "/api/maintenance/purge/local", nil)
	w := httptest.NewRecorder()
	s.handlePurgeLocalExecute(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, w, &resp)

	deleted, _ := resp["deleted"].(float64)
	if int(deleted) != expectedDeleted {
		t.Errorf("deleted = %v, want %d (matching preview count)", deleted, expectedDeleted)
	}

	for _, hash := range []string{"orphanExec1", "orphanExec2"} {
		entry, _ := s.manager.GetEntry(hash)
		if entry != nil {
			t.Errorf("entry %q still exists after purge execute", hash)
		}
	}

	entry, err := s.manager.GetEntry("managedExec1")
	if err != nil || entry == nil {
		t.Error("managed entry was incorrectly purged")
	}
}

func TestPurgeProviderPreview_NoProvider_Returns500(t *testing.T) {
	s := newTestServer(t)

	chiR := chi.NewRouter()
	chiR.Get("/api/maintenance/purge/provider/{name}", s.handlePurgeProviderPreview)

	r := httptest.NewRequest(http.MethodGet, "/api/maintenance/purge/provider/nonexistent", nil)
	w := httptest.NewRecorder()
	chiR.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestPurgeProviderExecute_NoProvider_Returns500(t *testing.T) {
	s := newTestServer(t)

	chiR := chi.NewRouter()
	chiR.Delete("/api/maintenance/purge/provider/{name}", s.handlePurgeProviderExecute)

	r := httptest.NewRequest(http.MethodDelete, "/api/maintenance/purge/provider/nonexistent", nil)
	w := httptest.NewRecorder()
	chiR.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestPurgeProviderPreview_MissingName_Returns400(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodGet, "/api/maintenance/purge/provider/", nil)
	w := httptest.NewRecorder()

	chiR := chi.NewRouter()
	chiR.Get("/api/maintenance/purge/provider/{name}", s.handlePurgeProviderPreview)

	s.handlePurgeProviderPreview(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestPurgeLocalExecute_PartialFailure_SafeBehavior(t *testing.T) {
	s := newTestServer(t)

	for i := 0; i < 5; i++ {
		seedEntry(t, s, fmt.Sprintf("partial%d", i), fmt.Sprintf("Partial.%d", i), "", int64(1000*(i+1)))
	}

	r := httptest.NewRequest(http.MethodDelete, "/api/maintenance/purge/local", nil)
	w := httptest.NewRecorder()
	s.handlePurgeLocalExecute(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	decodeJSON(t, w, &resp)

	deleted, _ := resp["deleted"].(float64)
	if int(deleted) != 5 {
		t.Errorf("deleted = %v, want 5", deleted)
	}

	for i := 0; i < 5; i++ {
		entry, _ := s.manager.GetEntry(fmt.Sprintf("partial%d", i))
		if entry != nil {
			t.Errorf("entry partial%d still exists after purge", i)
		}
	}
}
