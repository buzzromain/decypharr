package alldebrid

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestGetTorrent_FieldMapping(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantID     string
		wantName   string
		wantHash   string
		wantBytes  int64
		wantStatus types.TorrentStatus
		wantProg   float64
		wantSpeed  int64
		wantSeed   int
		wantDebrid string
		wantFiles  int
	}{
		{
			name: "downloaded with files",
			payload: `{"status":"success","data":{"magnets":{
				"id":42,"filename":"My.Show.S01","size":5000,"hash":"aabbccdd",
				"statusCode":4,"completionDate":1700000000,"seeders":10,
				"files":[{"n":"ep1.mkv","s":1000,"l":"http://dl/1"},{"n":"ep2.mkv","s":2000,"l":"http://dl/2"}]
			}}}`,
			wantID: "42", wantName: "My.Show.S01", wantHash: "aabbccdd",
			wantBytes: 5000, wantStatus: types.TorrentStatusDownloaded,
			wantProg: 100, wantSpeed: 0, wantSeed: 10, wantDebrid: "alldebrid",
			wantFiles: 2,
		},
		{
			name: "downloading in progress",
			payload: `{"status":"success","data":{"magnets":{
				"id":7,"filename":"Movie.2026","size":2000,"hash":"11223344",
				"statusCode":2,"downloaded":500,"downloadSpeed":999,"seeders":3,
				"completionDate":0
			}}}`,
			wantID: "7", wantName: "Movie.2026", wantHash: "11223344",
			wantBytes: 2000, wantStatus: types.TorrentStatusDownloading,
			wantProg: 25, wantSpeed: 999, wantSeed: 3, wantDebrid: "alldebrid",
			wantFiles: 0,
		},
		{
			name: "queued status code 0",
			payload: `{"status":"success","data":{"magnets":{
				"id":1,"filename":"Q","size":100,"hash":"qqqq",
				"statusCode":0,"downloaded":0,"downloadSpeed":0,"seeders":0,
				"completionDate":0
			}}}`,
			wantID: "1", wantName: "Q", wantHash: "qqqq",
			wantBytes: 100, wantStatus: types.TorrentStatusDownloading,
			wantProg: 0, wantSpeed: 0, wantSeed: 0, wantDebrid: "alldebrid",
			wantFiles: 0,
		},
		{
			name: "error status code 5",
			payload: `{"status":"success","data":{"magnets":{
				"id":9,"filename":"Bad","size":50,"hash":"eeee",
				"statusCode":5,"downloaded":0,"downloadSpeed":0,"seeders":0,
				"completionDate":0
			}}}`,
			wantID: "9", wantName: "Bad", wantHash: "eeee",
			wantBytes: 50, wantStatus: types.TorrentStatusError,
			wantProg: 0, wantSpeed: 0, wantSeed: 0, wantDebrid: "alldebrid",
			wantFiles: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/magnet/status" {
					t.Fatalf("path = %s, want /magnet/status", r.URL.Path)
				}
				_, _ = w.Write([]byte(tt.payload))
			}))
			defer srv.Close()

			ad := newTestAD(srv.URL, false)
			got, err := ad.GetTorrent("1")
			if err != nil {
				t.Fatalf("GetTorrent error: %v", err)
			}

			if got.Id != tt.wantID {
				t.Errorf("Id = %q, want %q", got.Id, tt.wantID)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.InfoHash != tt.wantHash {
				t.Errorf("InfoHash = %q, want %q", got.InfoHash, tt.wantHash)
			}
			if got.Bytes != tt.wantBytes {
				t.Errorf("Bytes = %d, want %d", got.Bytes, tt.wantBytes)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Progress != tt.wantProg {
				t.Errorf("Progress = %v, want %v", got.Progress, tt.wantProg)
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
			if got.Filename != tt.wantName {
				t.Errorf("Filename = %q, want %q", got.Filename, tt.wantName)
			}
			if got.OriginalFilename != tt.wantName {
				t.Errorf("OriginalFilename = %q, want %q", got.OriginalFilename, tt.wantName)
			}
			if len(got.Files) != tt.wantFiles {
				t.Errorf("Files count = %d, want %d", len(got.Files), tt.wantFiles)
			}
		})
	}
}

func TestGetTorrent_NestedFileFlattening(t *testing.T) {
	payload := `{"status":"success","data":{"magnets":{
		"id":10,"filename":"Pack","size":9000,"hash":"flat",
		"statusCode":4,"completionDate":1700000000,"seeders":1,
		"files":[
			{"n":"Season1","e":[
				{"n":"ep1.mkv","s":3000,"l":"http://dl/s1e1"},
				{"n":"ep2.mkv","s":3000,"l":"http://dl/s1e2"}
			]},
			{"n":"extra.mkv","s":3000,"l":"http://dl/extra"}
		]
	}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, false)
	got, err := ad.GetTorrent("10")
	if err != nil {
		t.Fatalf("GetTorrent error: %v", err)
	}

	if len(got.Files) != 3 {
		t.Fatalf("Files count = %d, want 3 (flattened from nested)", len(got.Files))
	}

	for _, name := range []string{"ep1.mkv", "ep2.mkv", "extra.mkv"} {
		f, ok := got.Files[name]
		if !ok {
			t.Errorf("missing expected file %q", name)
			continue
		}
		if f.TorrentId != "10" {
			t.Errorf("file %q TorrentId = %q, want 10", name, f.TorrentId)
		}
		if f.Link == "" {
			t.Errorf("file %q Link is empty", name)
		}
	}
}

func TestGetTorrent_Non2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, false)
	_, err := ad.GetTorrent("1")
	if err == nil || !strings.Contains(err.Error(), "Status: 403") {
		t.Fatalf("error = %v, want status 403", err)
	}
}

func TestGetTorrent_StatusCodeBoundaries(t *testing.T) {
	tests := []struct {
		code   int
		status types.TorrentStatus
	}{
		{0, types.TorrentStatusDownloading},
		{1, types.TorrentStatusDownloading},
		{2, types.TorrentStatusDownloading},
		{3, types.TorrentStatusDownloading},
		{4, types.TorrentStatusDownloaded},
		{5, types.TorrentStatusError},
		{-1, types.TorrentStatusError},
		{99, types.TorrentStatusError},
	}

	for _, tt := range tests {
		got := getAlldebridStatus(tt.code)
		if got != tt.status {
			t.Errorf("getAlldebridStatus(%d) = %q, want %q", tt.code, got, tt.status)
		}
	}
}
