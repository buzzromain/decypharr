package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAddContent_Multipart_Success(t *testing.T) {
	s := newTestServer(t)
	magnet := "magnet:?xt=urn:btih:da39a3ee5e6b4b0d3255bfef95601890afd80709&dn=TestFile"
	body, ct := buildMultipartForm(t, map[string]string{
		"urls":   magnet,
		"arr":    "sonarr",
		"action": "download",
	})

	r := httptest.NewRequest(http.MethodPost, "/api/add", body)
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	s.handleAddContent(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var results []map[string]interface{}
	decodeJSON(t, w, &results)
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if _, ok := results[0]["status"]; !ok {
		t.Error("result missing 'status' field")
	}
}

func TestHandleAddContent_MalformedJSON_Returns400(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/add",
		strings.NewReader(`{"url":"magnet:?xt=urn:btih:abc"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleAddContent(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleAddContent_InvalidMultipart_Returns400(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/add",
		strings.NewReader("this is not valid multipart data"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=nonexistent")
	w := httptest.NewRecorder()
	s.handleAddContent(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleAddContent_EmptyMultipart_ReturnsEmptyArray(t *testing.T) {
	s := newTestServer(t)
	body, ct := buildMultipartForm(t, map[string]string{})

	r := httptest.NewRequest(http.MethodPost, "/api/add", body)
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	s.handleAddContent(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var results []interface{}
	decodeJSON(t, w, &results)
	if len(results) != 0 {
		t.Fatalf("results len = %d, want 0; body: %s", len(results), w.Body.String())
	}
}

func TestHandleAddContent_ServiceError_MappedCorrectly(t *testing.T) {
	s := newTestServer(t)
	body, ct := buildMultipartForm(t, map[string]string{
		"urls": "not-a-valid-url",
	})

	r := httptest.NewRequest(http.MethodPost, "/api/add", body)
	r.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	s.handleAddContent(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var results []map[string]interface{}
	decodeJSON(t, w, &results)
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if status, _ := results[0]["status"].(string); status != "error" {
		t.Errorf("status = %q, want 'error'", status)
	}
	if errMsg, _ := results[0]["error"].(string); errMsg == "" {
		t.Error("error field should be non-empty for failed imports")
	}
}
