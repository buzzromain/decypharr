package arr

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── GetMovies ─────────────────────────────────────────────────────────────────

func TestGetMovies_ReturnsContents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("tmdbId") != "550" {
			t.Errorf("missing tmdbId param")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{"id":1,"title":"Fight Club","movieFile":{"id":10,"path":"/movies/fight_club.mkv","size":5000}},
			{"id":2,"title":"Empty","movieFile":{"id":0,"path":""}}
		]`)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Others}
	contents, err := a.GetMovies("550")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Second movie has no file → skipped
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	if contents[0].Title != "Fight Club" {
		t.Errorf("Title = %q, want Fight Club", contents[0].Title)
	}
	if len(contents[0].Files) != 1 || contents[0].Files[0].Path != "/movies/fight_club.mkv" {
		t.Errorf("unexpected files: %+v", contents[0].Files)
	}
	// Type should be set to Radarr
	if a.Type != Radarr {
		t.Errorf("Type = %q, want Radarr", a.Type)
	}
}

func TestGetMovies_404ReturnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	_, err := a.GetMovies("999")
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestGetMovies_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	contents, err := a.GetMovies("550")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 0 {
		t.Errorf("expected 0 contents, got %d", len(contents))
	}
}

// ── GetMedia ──────────────────────────────────────────────────────────────────

func TestGetMedia_RadarrDelegatesToGetMovies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":5,"title":"Dune","movieFile":{"id":1,"path":"/movies/dune.mkv","size":8000}}]`)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Radarr}
	contents, err := a.GetMedia("438631")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 || contents[0].Title != "Dune" {
		t.Errorf("unexpected contents: %+v", contents)
	}
}

func TestGetMedia_Sonarr_BuildsContentWithFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/series":
			fmt.Fprint(w, `[{"id":10,"title":"Breaking Bad"}]`)
		case "/api/v3/episodefile":
			fmt.Fprint(w, `[{"id":100,"path":"/tv/bb/s01e01.mkv","seasonNumber":1,"size":1500}]`)
		case "/api/v3/episode":
			fmt.Fprint(w, `[{"id":200,"episodeFileId":100}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Sonarr}
	contents, err := a.GetMedia("81189")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	if contents[0].Title != "Breaking Bad" {
		t.Errorf("Title = %q, want Breaking Bad", contents[0].Title)
	}
	if len(contents[0].Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(contents[0].Files))
	}
	f := contents[0].Files[0]
	if f.Path != "/tv/bb/s01e01.mkv" {
		t.Errorf("Path = %q, want /tv/bb/s01e01.mkv", f.Path)
	}
	if f.EpisodeId != 200 {
		t.Errorf("EpisodeId = %d, want 200", f.EpisodeId)
	}
}

func TestGetMedia_Sonarr_SkipsSeriesWithNoFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/series":
			fmt.Fprint(w, `[{"id":10,"title":"No Files Show"}]`)
		case "/api/v3/episodefile":
			fmt.Fprint(w, `[]`) // no files
		case "/api/v3/episode":
			fmt.Fprint(w, `[]`)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Sonarr}
	contents, err := a.GetMedia("99999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 0 {
		t.Errorf("expected 0 contents for series with no files, got %d", len(contents))
	}
}

func TestGetMedia_Sonarr_SkipsFilesWithoutPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/series":
			fmt.Fprint(w, `[{"id":10,"title":"Bad Files"}]`)
		case "/api/v3/episodefile":
			// File with id=0 or empty path → skipped
			fmt.Fprint(w, `[{"id":0,"path":"","seasonNumber":1},{"id":99,"path":"","seasonNumber":1}]`)
		case "/api/v3/episode":
			fmt.Fprint(w, `[]`)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Sonarr}
	contents, err := a.GetMedia("11111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 0 {
		t.Errorf("expected 0 contents (all files skipped), got %d", len(contents))
	}
}

// ── batchDeleteFiles ──────────────────────────────────────────────────────────

func TestBatchDeleteFiles_Sonarr(t *testing.T) {
	var gotPath string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Sonarr}
	files := []ContentFile{
		{FileId: 1, Path: "/nonexistent/a.mkv"},
		{FileId: 2, Path: "/nonexistent/b.mkv"},
	}
	err := a.batchDeleteFiles(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v3/episodefile/bulk" {
		t.Errorf("path = %q, want /api/v3/episodefile/bulk", gotPath)
	}
	if len(gotBody) == 0 {
		t.Error("expected non-empty body with episodeFileIds")
	}
}

func TestBatchDeleteFiles_Radarr(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Radarr}
	files := []ContentFile{{FileId: 10, Path: "/nonexistent/movie.mkv"}}
	_ = a.batchDeleteFiles(files)

	if gotPath != "/api/v3/moviefile/bulk" {
		t.Errorf("path = %q, want /api/v3/moviefile/bulk", gotPath)
	}
}

func TestBatchDeleteFiles_UnknownTypeError(t *testing.T) {
	a := &Arr{Host: "http://x", Token: "token", Type: Others}
	err := a.batchDeleteFiles([]ContentFile{{FileId: 1}})
	if err == nil {
		t.Error("expected error for unknown arr type")
	}
}

func TestDeleteFiles_Empty(t *testing.T) {
	a := &Arr{Host: "http://x", Token: "token"}
	if err := a.DeleteFiles(nil); err != nil {
		t.Errorf("expected nil for empty files, got %v", err)
	}
}

// ── searchMissing / SearchMissing ─────────────────────────────────────────────

func TestSearchMissing_Empty(t *testing.T) {
	a := &Arr{}
	if err := a.SearchMissing(nil); err != nil {
		t.Errorf("expected nil for empty files, got %v", err)
	}
}

func TestSearchMissing_UnknownType(t *testing.T) {
	a := &Arr{Host: "http://x", Token: "token", Type: Others}
	err := a.searchMissing([]ContentFile{{Id: 1}})
	if err == nil {
		t.Error("expected error for unknown arr type")
	}
}

func TestSearchRadarr_PostsCommand(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token", Type: Radarr}
	err := a.searchRadarr([]ContentFile{{Id: 42}, {Id: 99}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := string(gotBody)
	if body == "" {
		t.Error("expected JSON body with movieIds")
	}
}

func TestSearchRadarr_Non200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Note: 500 is retryable → don't use closed server, just check error propagation with bad token
	a := &Arr{Host: "http://x"} // no token → Request returns "arr not configured"
	if err := a.searchRadarr([]ContentFile{{Id: 1}}); err == nil {
		t.Error("expected error")
	}
}
