package torbox

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// --- GetTorrents ---

func TestProvider_GetTorrents_MapsRuntimeState(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrents/mylist" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		// Return data only on first request; empty on subsequent (pagination termination)
		if requestCount > 1 {
			fmt.Fprint(w, `{"success":true,"data":[]}`)
			return
		}
		fmt.Fprint(w, `{
			"success": true,
			"data": [
				{
					"id": 1, "hash": "aaa111", "name": "Movie.2024",
					"size": 1073741824, "progress": 1.0,
					"download_state": "completed", "download_finished": true,
					"download_speed": 0, "seeds": 5,
					"created_at": "2024-06-01T12:00:00Z",
					"files": [
						{"id": 10, "name": "Movie.2024/movie.mkv", "size": 1073741824, "short_name": "movie.mkv", "absolute_path": "Movie.2024/movie.mkv"}
					]
				},
				{
					"id": 2, "hash": "bbb222", "name": "Show.S01E01",
					"size": 524288000, "progress": 0.45,
					"download_state": "downloading", "download_finished": false,
					"download_speed": 5000000, "seeds": 12,
					"created_at": "2024-06-02T12:00:00Z",
					"files": [
						{"id": 20, "name": "Show.S01E01/ep01.mkv", "size": 524288000, "short_name": "ep01.mkv", "absolute_path": "Show.S01E01/ep01.mkv"}
					]
				},
				{
					"id": 3, "hash": "ccc333", "name": "Cached.Item",
					"size": 314572800, "progress": 1.0,
					"download_state": "cached", "download_finished": false,
					"download_speed": 0, "seeds": 0,
					"created_at": "2024-06-03T12:00:00Z",
					"files": []
				}
			]
		}`)
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	torrents, err := tb.GetTorrents()
	if err != nil {
		t.Fatalf("GetTorrents error: %v", err)
	}

	if len(torrents) != 3 {
		t.Fatalf("expected 3 torrents, got %d", len(torrents))
	}

	// First: download_finished=true → downloaded
	if torrents[0].Status != types.TorrentStatusDownloaded {
		t.Errorf("torrent 1 status: got %q, want %q", torrents[0].Status, types.TorrentStatusDownloaded)
	}
	if torrents[0].Name != "Movie.2024" {
		t.Errorf("torrent 1 name: got %q, want %q", torrents[0].Name, "Movie.2024")
	}
	if torrents[0].Progress != 100 {
		t.Errorf("torrent 1 progress: got %f, want 100", torrents[0].Progress)
	}
	if torrents[0].Debrid != "torbox" {
		t.Errorf("torrent 1 debrid: got %q, want %q", torrents[0].Debrid, "torbox")
	}
	// download_finished=true → files should have torbox:// links
	if len(torrents[0].Files) == 0 {
		t.Fatal("torrent 1 should have files")
	}
	for _, f := range torrents[0].Files {
		if f.Link == "" {
			t.Error("finished torrent file should have a link")
		}
	}

	// Second: downloading → downloading
	if torrents[1].Status != types.TorrentStatusDownloading {
		t.Errorf("torrent 2 status: got %q, want %q", torrents[1].Status, types.TorrentStatusDownloading)
	}
	// Files should have no link since not finished
	for _, f := range torrents[1].Files {
		if f.Link != "" {
			t.Error("downloading torrent file should not have a link")
		}
	}

	// Third: cached → downloaded
	if torrents[2].Status != types.TorrentStatusDownloaded {
		t.Errorf("torrent 3 status: got %q, want %q", torrents[2].Status, types.TorrentStatusDownloaded)
	}
}

func TestProvider_GetTorrents_StatusMapping(t *testing.T) {
	tests := []struct {
		state    string
		finished bool
		want     types.TorrentStatus
	}{
		{"downloading", false, types.TorrentStatusDownloading},
		{"completed", false, types.TorrentStatusDownloaded},
		{"cached", false, types.TorrentStatusDownloaded},
		{"uploading", false, types.TorrentStatusDownloaded},
		{"downloaded", false, types.TorrentStatusDownloaded},
		{"paused", false, types.TorrentStatusDownloading},
		{"downloading (queued)", false, types.TorrentStatusDownloading},
		{"stalled", false, types.TorrentStatusError},
		{"", true, types.TorrentStatusDownloaded},
	}

	tb := &Torbox{}
	for _, tt := range tests {
		got := tb.getTorboxStatus(tt.state, tt.finished)
		if got != tt.want {
			t.Errorf("getTorboxStatus(%q, %v): got %q, want %q", tt.state, tt.finished, got, tt.want)
		}
	}
}

// --- GetAvailableSlots ---

func TestProvider_GetAvailableSlots_ValidMapping(t *testing.T) {
	tests := []struct {
		name     string
		plan     int64
		wantSlot int
	}{
		{"essential", 1, 3},
		{"pro", 2, 10},
		{"standard", 3, 5},
		{"free/unknown", 99, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/user/me" {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"success":true,"data":{"id":1,"plan":%d,"email":"test@test.com","premium_expires_at":"2025-12-31T23:59:59Z"}}`, tt.plan)
					return
				}
				http.Error(w, "not found", http.StatusNotFound)
			}))
			defer srv.Close()

			tb := newTestTorbox(srv.URL, false)
			slots, err := tb.GetAvailableSlots()
			if err != nil {
				t.Fatalf("GetAvailableSlots error: %v", err)
			}
			if slots != tt.wantSlot {
				t.Errorf("slots: got %d, want %d", slots, tt.wantSlot)
			}
		})
	}
}

// --- Profile malformed payload ---

func TestProvider_AccountProfile_MalformedPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{totally broken json`))
	}))
	defer srv.Close()

	tb := &Torbox{
		Host:   srv.URL,
		APIKey: "tb-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "torbox"},
	}

	_, err := tb.GetProfile()
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestProvider_AccountProfile_NilData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true,"data":null}`)
	}))
	defer srv.Close()

	tb := &Torbox{
		Host:   srv.URL,
		APIKey: "tb-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "torbox"},
	}

	_, err := tb.GetProfile()
	if err == nil {
		t.Fatal("expected error on null data, got nil")
	}
}

// --- RefreshDownloadLinks is a no-op for Torbox (returns empty) ---

func TestProvider_RefreshDownloadLinks_NoOp(t *testing.T) {
	useTorrentFile := false
	tb := &Torbox{
		Host:   "http://unused",
		APIKey: "tb-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{
			Name:           "torbox",
			UseTorrentFile: &useTorrentFile,
		},
	}

	// fetchDownloadLinks returns empty, so RefreshLinks should succeed with no HTTP calls
	links, err := tb.fetchDownloadLinks(nil)
	if err != nil {
		t.Fatalf("fetchDownloadLinks error: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}
