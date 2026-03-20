package realdebrid

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/customerror"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestGetTorrent_DetailFieldMapping(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantID     string
		wantName   string
		wantOrig   string
		wantBytes  int64
		wantProg   float64
		wantStatus types.TorrentStatus
		wantSpeed  int64
		wantSeed   int
		wantLinks  int
		wantFiles  int
		wantDebrid string
	}{
		{
			name: "downloaded with files and links",
			payload: `{
				"id":"rd-1","filename":"Movie.2026","original_filename":"Movie.2026.orig",
				"bytes":5000,"progress":100,"status":"downloaded","speed":0,"seeders":15,
				"links":["http://dl/1","http://dl/2"],
				"files":[
					{"id":1,"path":"Movie.2026/movie.mkv","bytes":4500,"selected":1},
					{"id":2,"path":"Movie.2026/extras.mkv","bytes":500,"selected":0}
				],
				"added":"2024-06-15T12:00:00Z"
			}`,
			wantID: "rd-1", wantName: "Movie.2026", wantOrig: "Movie.2026.orig",
			wantBytes: 5000, wantProg: 100, wantStatus: "downloaded",
			wantSpeed: 0, wantSeed: 15, wantLinks: 2, wantFiles: 2,
			wantDebrid: "realdebrid",
		},
		{
			name: "downloading in progress",
			payload: `{
				"id":"rd-2","filename":"Show.S01","original_filename":"Show.S01",
				"bytes":10000,"progress":45.5,"status":"downloading","speed":1234,"seeders":8,
				"links":[],"files":[],
				"added":"2024-06-15T12:00:00Z"
			}`,
			wantID: "rd-2", wantName: "Show.S01", wantOrig: "Show.S01",
			wantBytes: 10000, wantProg: 45.5, wantStatus: "downloading",
			wantSpeed: 1234, wantSeed: 8, wantLinks: 0, wantFiles: 0,
			wantDebrid: "realdebrid",
		},
		{
			name: "waiting_files_selection",
			payload: `{
				"id":"rd-3","filename":"Pack","original_filename":"Pack",
				"bytes":20000,"progress":0,"status":"waiting_files_selection","speed":0,"seeders":0,
				"links":[],"files":[{"id":1,"path":"Pack/ep1.mkv","bytes":10000,"selected":0}],
				"added":"2024-01-01T00:00:00Z"
			}`,
			wantID: "rd-3", wantName: "Pack", wantOrig: "Pack",
			wantBytes: 20000, wantProg: 0, wantStatus: "waiting_files_selection",
			wantSpeed: 0, wantSeed: 0, wantLinks: 0, wantFiles: 1,
			wantDebrid: "realdebrid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.payload))
			}))
			defer srv.Close()

			rd := newTestRD(srv.URL, false)
			got, err := rd.GetTorrent("any")
			if err != nil {
				t.Fatalf("GetTorrent error: %v", err)
			}

			if got.Id != tt.wantID {
				t.Errorf("Id = %q, want %q", got.Id, tt.wantID)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.OriginalFilename != tt.wantOrig {
				t.Errorf("OriginalFilename = %q, want %q", got.OriginalFilename, tt.wantOrig)
			}
			if got.Bytes != tt.wantBytes {
				t.Errorf("Bytes = %d, want %d", got.Bytes, tt.wantBytes)
			}
			if got.Progress != tt.wantProg {
				t.Errorf("Progress = %v, want %v", got.Progress, tt.wantProg)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Speed != tt.wantSpeed {
				t.Errorf("Speed = %d, want %d", got.Speed, tt.wantSpeed)
			}
			if got.Seeders != tt.wantSeed {
				t.Errorf("Seeders = %d, want %d", got.Seeders, tt.wantSeed)
			}
			if len(got.Links) != tt.wantLinks {
				t.Errorf("Links count = %d, want %d", len(got.Links), tt.wantLinks)
			}
			if len(got.Files) != tt.wantFiles {
				t.Errorf("Files count = %d, want %d", len(got.Files), tt.wantFiles)
			}
			if got.Debrid != tt.wantDebrid {
				t.Errorf("Debrid = %q, want %q", got.Debrid, tt.wantDebrid)
			}
			if got.Filename != tt.wantName {
				t.Errorf("Filename = %q, want %q", got.Filename, tt.wantName)
			}
		})
	}
}

func TestGetTorrent_FilePathExtraction(t *testing.T) {
	payload := `{
		"id":"f1","filename":"Pack","original_filename":"Pack","bytes":10000,
		"progress":100,"status":"downloaded","links":["http://dl/1","http://dl/2"],
		"files":[
			{"id":10,"path":"Pack/Season1/ep1.mkv","bytes":5000,"selected":1},
			{"id":11,"path":"Pack/Season1/ep2.mkv","bytes":5000,"selected":1}
		],
		"added":"2024-06-15T12:00:00Z"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	got, err := rd.GetTorrent("f1")
	if err != nil {
		t.Fatalf("GetTorrent error: %v", err)
	}

	// Files are keyed by basename
	for _, name := range []string{"ep1.mkv", "ep2.mkv"} {
		f, ok := got.Files[name]
		if !ok {
			t.Errorf("missing expected file %q in Files map", name)
			continue
		}
		if f.TorrentId != "f1" {
			t.Errorf("file %q TorrentId = %q, want f1", name, f.TorrentId)
		}
		if f.Size == 0 {
			t.Errorf("file %q Size is zero", name)
		}
	}
}

func TestGetTorrent_NotFoundReturnsSpecificError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	_, err := rd.GetTorrent("missing")
	if !errors.Is(err, customerror.TorrentNotFoundError) {
		t.Fatalf("error = %v, want TorrentNotFoundError", err)
	}
}

func TestGetTorrent_Non2xxNon404ReturnsGenericError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	_, err := rd.GetTorrent("any")
	if err == nil || !strings.Contains(err.Error(), "Status: 403") {
		t.Fatalf("error = %v, want status 403 error", err)
	}
}

func TestGetTorrent_AddedTimeParsing(t *testing.T) {
	payload := `{
		"id":"t1","filename":"X","original_filename":"X","bytes":1,
		"progress":100,"status":"downloaded","links":[],"files":[],
		"added":"2024-06-15T14:30:00Z"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	got, err := rd.GetTorrent("t1")
	if err != nil {
		t.Fatalf("GetTorrent error: %v", err)
	}

	if got.Added.Year() != 2024 || got.Added.Month() != 6 || got.Added.Day() != 15 {
		t.Errorf("Added = %v, want 2024-06-15", got.Added)
	}
}

func TestGetTorrent_StatusPassedThrough(t *testing.T) {
	// RealDebrid passes status string directly — verify non-standard statuses are preserved
	tests := []struct {
		status string
		want   types.TorrentStatus
	}{
		{"downloaded", "downloaded"},
		{"downloading", "downloading"},
		{"waiting_files_selection", "waiting_files_selection"},
		{"compressing", "compressing"},
		{"error", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			payload := `{"id":"x","filename":"X","original_filename":"X","bytes":1,"progress":0,"status":"` + tt.status + `","links":[],"files":[],"added":"2024-01-01T00:00:00Z"}`
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(payload))
			}))
			defer srv.Close()

			rd := newTestRD(srv.URL, false)
			got, err := rd.GetTorrent("x")
			if err != nil {
				t.Fatalf("GetTorrent error: %v", err)
			}
			if got.Status != tt.want {
				t.Errorf("Status = %q, want %q", got.Status, tt.want)
			}
		})
	}
}
