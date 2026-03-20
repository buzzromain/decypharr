package torbox

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestGetTorrent_DetailFieldMapping(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantID     string
		wantName   string
		wantBytes  int64
		wantProg   float64
		wantStatus types.TorrentStatus
		wantSpeed  int64
		wantSeed   int
		wantDebrid string
		wantFiles  int
	}{
		{
			name: "downloading in progress",
			payload: `{"success":true,"data":{
				"id":42,"name":"Movie.2026","size":8000,"progress":0.65,
				"download_state":"downloading","download_finished":false,
				"download_speed":5000,"seeds":12,"hash":"aabb",
				"created_at":"2024-06-15T12:00:00Z",
				"files":[{"id":1,"name":"root/movie.mkv","absolute_path":"/mnt/root/movie.mkv","size":7500}]
			}}`,
			wantID: "42", wantName: "Movie.2026", wantBytes: 8000,
			wantProg: 65, wantStatus: types.TorrentStatusDownloading,
			wantSpeed: 5000, wantSeed: 12, wantDebrid: "torbox", wantFiles: 1,
		},
		{
			name: "finished with download links",
			payload: `{"success":true,"data":{
				"id":99,"name":"Show.S01","size":20000,"progress":1.0,
				"download_state":"completed","download_finished":true,
				"download_speed":0,"seeds":5,"hash":"ccdd",
				"created_at":"2024-01-01T00:00:00Z",
				"files":[
					{"id":10,"name":"Show/ep1.mkv","absolute_path":"/mnt/Show/ep1.mkv","size":10000},
					{"id":11,"name":"Show/ep2.mkv","absolute_path":"/mnt/Show/ep2.mkv","size":10000}
				]
			}}`,
			wantID: "99", wantName: "Show.S01", wantBytes: 20000,
			wantProg: 100, wantStatus: types.TorrentStatusDownloaded,
			wantSpeed: 0, wantSeed: 5, wantDebrid: "torbox", wantFiles: 2,
		},
		{
			name: "error state",
			payload: `{"success":true,"data":{
				"id":7,"name":"Bad","size":100,"progress":0,
				"download_state":"some_unknown_state","download_finished":false,
				"download_speed":0,"seeds":0,"hash":"eeee",
				"created_at":"2024-01-01T00:00:00Z","files":[]
			}}`,
			wantID: "7", wantName: "Bad", wantBytes: 100,
			wantProg: 0, wantStatus: types.TorrentStatusError,
			wantSpeed: 0, wantSeed: 0, wantDebrid: "torbox", wantFiles: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.payload))
			}))
			defer srv.Close()

			tb := newTestTorbox(srv.URL, false)
			got, err := tb.GetTorrent("any")
			if err != nil {
				t.Fatalf("GetTorrent error: %v", err)
			}

			if got.Id != tt.wantID {
				t.Errorf("Id = %q, want %q", got.Id, tt.wantID)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
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
			if got.Debrid != tt.wantDebrid {
				t.Errorf("Debrid = %q, want %q", got.Debrid, tt.wantDebrid)
			}
			if len(got.Files) != tt.wantFiles {
				t.Errorf("Files count = %d, want %d", len(got.Files), tt.wantFiles)
			}
		})
	}
}

func TestGetTorrent_ProgressScaling(t *testing.T) {
	// Torbox reports progress as 0-1, Decypharr uses 0-100
	tests := []struct {
		progress string
		want     float64
	}{
		{"0", 0},
		{"0.5", 50},
		{"1.0", 100},
		{"0.123", 12.3},
	}

	for _, tt := range tests {
		t.Run(tt.progress, func(t *testing.T) {
			payload := `{"success":true,"data":{"id":1,"name":"X","size":1,"progress":` + tt.progress + `,"download_state":"downloading","download_finished":false,"download_speed":0,"seeds":0,"hash":"h","created_at":"2024-01-01T00:00:00Z","files":[]}}`
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(payload))
			}))
			defer srv.Close()

			tb := newTestTorbox(srv.URL, false)
			got, err := tb.GetTorrent("1")
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if got.Progress != tt.want {
				t.Errorf("Progress = %v, want %v", got.Progress, tt.want)
			}
		})
	}
}

func TestGetTorrent_DownloadFinishedGeneratesLinks(t *testing.T) {
	payload := `{"success":true,"data":{
		"id":55,"name":"Pack","size":10000,"progress":1.0,
		"download_state":"completed","download_finished":true,
		"download_speed":0,"seeds":0,"hash":"ffff",
		"created_at":"2024-01-01T00:00:00Z",
		"files":[{"id":7,"name":"root/file.mkv","absolute_path":"/mnt/root/file.mkv","size":10000}]
	}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	got, err := tb.GetTorrent("55")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	f, ok := got.Files["file.mkv"]
	if !ok {
		t.Fatal("missing file.mkv in Files")
	}
	// When download_finished=true, links are generated as "torbox://{torrentId}/{fileId}"
	if !strings.HasPrefix(f.Link, "torbox://") {
		t.Errorf("Link = %q, want torbox:// prefix", f.Link)
	}
	if f.Link != "torbox://55/7" {
		t.Errorf("Link = %q, want torbox://55/7", f.Link)
	}
}

func TestGetTorrent_NotFinishedNoLinks(t *testing.T) {
	payload := `{"success":true,"data":{
		"id":55,"name":"Pack","size":10000,"progress":0.5,
		"download_state":"downloading","download_finished":false,
		"download_speed":100,"seeds":1,"hash":"ffff",
		"created_at":"2024-01-01T00:00:00Z",
		"files":[{"id":7,"name":"root/file.mkv","absolute_path":"/mnt/root/file.mkv","size":10000}]
	}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	got, err := tb.GetTorrent("55")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	f, ok := got.Files["file.mkv"]
	if !ok {
		t.Fatal("missing file.mkv in Files")
	}
	if f.Link != "" {
		t.Errorf("Link = %q, want empty when not finished", f.Link)
	}
}

func TestGetTorrent_OriginalFilenameFromFirstFilePath(t *testing.T) {
	payload := `{"success":true,"data":{
		"id":1,"name":"Display.Name","size":1000,"progress":1.0,
		"download_state":"completed","download_finished":true,
		"download_speed":0,"seeds":0,"hash":"h",
		"created_at":"2024-01-01T00:00:00Z",
		"files":[{"id":1,"name":"actual_root/subfolder/file.mkv","absolute_path":"/mnt/actual_root/subfolder/file.mkv","size":1000}]
	}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	got, err := tb.GetTorrent("1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// OriginalFilename should be the first path component of the first file
	if got.OriginalFilename != "actual_root" {
		t.Errorf("OriginalFilename = %q, want actual_root", got.OriginalFilename)
	}
}

func TestGetTorrent_NilDataReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":null}`))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	_, err := tb.GetTorrent("1")
	if err == nil || !strings.Contains(err.Error(), "error getting torrent") {
		t.Fatalf("error = %v, want error getting torrent", err)
	}
}

func TestGetTorrent_Non2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	_, err := tb.GetTorrent("1")
	if err == nil || !strings.Contains(err.Error(), "Status: 403") {
		t.Fatalf("error = %v, want status 403", err)
	}
}
