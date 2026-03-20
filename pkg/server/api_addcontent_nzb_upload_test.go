package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	json "github.com/bytedance/sonic"
	"github.com/sirrobot01/decypharr/pkg/manager"
)

// ── NZB file upload via handleAddContent ──────────────────────────────────────
//
// handleAddContent accepts NZB files through the "nzbFiles" multipart field.
// These tests verify the multipart-to-response pipeline: correct parsing of
// uploaded NZB files, proper response structure, and correct differentiation
// from malformed/broken multipart requests.
//
// Note: without a configured usenet backend, AddNewNZB returns an error per
// item, but the handler always returns HTTP 200 with per-item results. The
// HTTP-level success path (multipart parsing → structured JSON response) is
// the externally observable behavior under test.

// nzbUploadResult is the response item shape returned by handleAddContent.
// Defined here for decoding only — mirrors manager.ImportRequest's JSON tags.
type nzbUploadResult struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Type   string `json:"type"`
}

// postAddContentNZB posts one or more NZB files to /api/add using the
// "nzbFiles" multipart field name, matching handleAddContent's expected key.
func postAddContentNZB(t *testing.T, s *Server, files map[string][]byte, extraFields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mp := multipart.NewWriter(&body)

	for name, content := range files {
		fw, err := mp.CreateFormFile("nzbFiles", name)
		if err != nil {
			t.Fatalf("CreateFormFile(%s): %v", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("Write(%s): %v", name, err)
		}
	}
	for k, v := range extraFields {
		_ = mp.WriteField(k, v)
	}
	mp.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/add", &body)
	r.Header.Set("Content-Type", mp.FormDataContentType())
	w := httptest.NewRecorder()
	s.handleAddContent(w, r)
	return w
}

// validNZBPayload is a minimal well-formed NZB document for deterministic tests.
var validNZBPayload = []byte(`<?xml version="1.0" encoding="UTF-8"?>
<nzb xmlns="http://www.newzbin.com/DTD/2003/nzb">
  <file subject="test file" poster="p@test.com" date="1700000000">
    <groups><group>alt.binaries.test</group></groups>
    <segments><segment bytes="1024" number="1">seg@host</segment></segments>
  </file>
</nzb>`)

// TestAddContentNZBUpload_SingleFile verifies that a single NZB file upload
// via multipart is properly parsed and returns a structured JSON response.
func TestAddContentNZBUpload_SingleFile(t *testing.T) {
	s := newTestServer(t)
	w := postAddContentNZB(t, s, map[string][]byte{
		"show.s01e01.nzb": validNZBPayload,
	}, map[string]string{"arr": "sonarr"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var results []nzbUploadResult
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, w.Body.String())
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}

	r := results[0]
	if r.Name != "show.s01e01.nzb" {
		t.Errorf("result.name = %q, want show.s01e01.nzb", r.Name)
	}
}

// TestAddContentNZBUpload_MultipleFiles verifies that multiple NZB files in
// a single multipart request each produce a separate result item.
func TestAddContentNZBUpload_MultipleFiles(t *testing.T) {
	s := newTestServer(t)
	w := postAddContentNZB(t, s, map[string][]byte{
		"ep01.nzb": validNZBPayload,
		"ep02.nzb": validNZBPayload,
	}, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var results []nzbUploadResult
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, w.Body.String())
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}

	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	for _, want := range []string{"ep01.nzb", "ep02.nzb"} {
		if !names[want] {
			t.Errorf("missing result for %q; got names: %v", want, names)
		}
	}
}

// TestAddContentNZBUpload_ResponseStructure verifies that each result item
// carries all fields that downstream consumers (UI, callbacks) depend on.
func TestAddContentNZBUpload_ResponseStructure(t *testing.T) {
	s := newTestServer(t)
	w := postAddContentNZB(t, s, map[string][]byte{
		"movie.nzb": validNZBPayload,
	}, map[string]string{"arr": "radarr", "action": "download"})

	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result item")
	}

	item := results[0]
	requiredFields := []string{"name", "id", "status", "type"}
	for _, f := range requiredFields {
		if _, ok := item[f]; !ok {
			t.Errorf("result item missing %q field — required for consumer stability", f)
		}
	}
}

// TestAddContentNZBUpload_ContentType_IsJSON verifies the Content-Type header
// is always application/json for NZB upload responses, not text/plain.
func TestAddContentNZBUpload_ContentType_IsJSON(t *testing.T) {
	s := newTestServer(t)
	w := postAddContentNZB(t, s, map[string][]byte{
		"test.nzb": validNZBPayload,
	}, nil)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestAddContentNZBUpload_DiffersFromMalformedMultipart verifies that a
// valid NZB upload returns 200/JSON while a broken multipart returns 400.
// This ensures the success path is distinguishable from error paths.
func TestAddContentNZBUpload_DiffersFromMalformedMultipart(t *testing.T) {
	s := newTestServer(t)

	// Valid upload → 200
	wOK := postAddContentNZB(t, s, map[string][]byte{
		"valid.nzb": validNZBPayload,
	}, nil)
	if wOK.Code != http.StatusOK {
		t.Fatalf("valid upload: status = %d, want 200", wOK.Code)
	}

	// Broken multipart → 400
	r := httptest.NewRequest(http.MethodPost, "/api/add",
		strings.NewReader("not valid multipart"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=broken")
	wBad := httptest.NewRecorder()
	s.handleAddContent(wBad, r)
	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("broken multipart: status = %d, want 400", wBad.Code)
	}
}

// TestAddContentNZBUpload_MixedWithTorrents verifies that NZB files and
// torrent URLs can be submitted in the same request, with each producing
// its own result item.
func TestAddContentNZBUpload_MixedWithTorrents(t *testing.T) {
	s := newTestServer(t)

	var body bytes.Buffer
	mp := multipart.NewWriter(&body)
	// Add an NZB file
	fw, _ := mp.CreateFormFile("nzbFiles", "show.nzb")
	_, _ = fw.Write(validNZBPayload)
	// Add a torrent URL
	_ = mp.WriteField("urls", "magnet:?xt=urn:btih:da39a3ee5e6b4b0d3255bfef95601890afd80709&dn=TestFile")
	mp.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/add", &body)
	r.Header.Set("Content-Type", mp.FormDataContentType())
	w := httptest.NewRecorder()
	s.handleAddContent(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var results []*manager.ImportRequest
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, w.Body.String())
	}
	if len(results) < 2 {
		t.Fatalf("expected ≥2 results (NZB + torrent), got %d", len(results))
	}
}
