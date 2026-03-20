package arr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── Import ────────────────────────────────────────────────────────────────────

func TestImport_FetchesAndPostsCommand(t *testing.T) {
	var postCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/manualimport":
			if r.URL.Query().Get("downloadId") != "dl-abc" {
				t.Errorf("missing downloadId param, got %q", r.URL.Query().Get("downloadId"))
			}
			// Return one importable file
			fmt.Fprint(w, `[{
				"path":"/downloads/show.s01e01.mkv",
				"folderName":"show",
				"seasonNumber":1,
				"episodes":[{"id":42}],
				"quality":{"quality":{"id":1,"name":"HDTV-720p","source":"tv","resolution":720},"revision":{"version":1,"real":0,"isRepack":false}},
				"languages":[{"id":1,"name":"English"}],
				"releaseGroup":"grp"
			}]`)
		case "/api/v3/command":
			postCalled = true
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	body, err := a.Import("dl-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != nil {
		_ = body.Close()
	}
	if !postCalled {
		t.Error("expected POST to /api/v3/command")
	}
}

func TestImport_EmptyManualImportStillPostsCommand(t *testing.T) {
	var postCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/manualimport":
			fmt.Fprint(w, `[]`) // no files to import
		case "/api/v3/command":
			postCalled = true
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	_, err := a.Import("dl-empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !postCalled {
		t.Error("command should be posted even with empty manualimport response")
	}
}

func TestImport_GetRequestError(t *testing.T) {
	// Empty token → Request returns "arr not configured" immediately
	a := &Arr{Host: "http://sonarr:8989"}
	_, err := a.Import("dl-x")
	if err == nil {
		t.Error("expected error for unconfigured arr")
	}
}

func TestImport_BuildsEpisodeIds(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/manualimport":
			// Return entry with 2 episodes
			fmt.Fprint(w, `[{
				"path":"/dl/ep.mkv",
				"folderName":"show",
				"seasonNumber":2,
				"episodes":[{"id":10},{"id":11}],
				"quality":{"quality":{"id":1,"name":"WEB","source":"web","resolution":1080},"revision":{"version":1,"real":0,"isRepack":false}},
				"languages":[]
			}]`)
		case "/api/v3/command":
			capturedBody, _ = readBody(r)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	_, _ = a.Import("dl-ep")

	body := string(capturedBody)
	// Both episode IDs should be in the payload
	if body == "" {
		t.Fatal("expected non-empty command body")
	}
	for _, id := range []string{"10", "11"} {
		if !contains(body, id) {
			t.Errorf("expected episode ID %s in body: %s", id, body)
		}
	}
}

func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	buf := make([]byte, 4096)
	n, _ := r.Body.Read(buf)
	return buf[:n], nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
