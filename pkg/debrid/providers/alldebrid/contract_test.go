package alldebrid

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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/magnet/status" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// statusCode 4 = downloaded, 0-3 = downloading, others = error
		fmt.Fprint(w, `{
			"status": "success",
			"data": {
				"magnets": [
					{
						"id": 100, "filename": "Movie.2024.mkv", "size": 1073741824,
						"hash": "abc123", "statusCode": 4, "completionDate": 1717200000,
						"files": [
							{"n": "Movie.2024.mkv", "s": 1073741824, "l": "http://dl/movie.mkv"}
						]
					},
					{
						"id": 101, "filename": "Show.S01E01.mkv", "size": 524288000,
						"hash": "def456", "statusCode": 1, "completionDate": 0,
						"files": []
					},
					{
						"id": 102, "filename": "Error.Item", "size": 100,
						"hash": "ghi789", "statusCode": 9, "completionDate": 0,
						"files": []
					}
				]
			}
		}`)
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, false)
	torrents, err := ad.GetTorrents()
	if err != nil {
		t.Fatalf("GetTorrents error: %v", err)
	}

	if len(torrents) != 3 {
		t.Fatalf("expected 3 torrents, got %d", len(torrents))
	}

	// statusCode 4 → downloaded
	if torrents[0].Status != types.TorrentStatusDownloaded {
		t.Errorf("torrent 1 status: got %q, want %q", torrents[0].Status, types.TorrentStatusDownloaded)
	}
	if torrents[0].Name != "Movie.2024.mkv" {
		t.Errorf("torrent 1 name: got %q, want %q", torrents[0].Name, "Movie.2024.mkv")
	}
	if torrents[0].InfoHash != "abc123" {
		t.Errorf("torrent 1 hash: got %q, want %q", torrents[0].InfoHash, "abc123")
	}
	if torrents[0].Debrid != "alldebrid" {
		t.Errorf("torrent 1 debrid: got %q, want %q", torrents[0].Debrid, "alldebrid")
	}
	// Files should be mapped
	if len(torrents[0].Files) != 1 {
		t.Errorf("torrent 1 files: got %d, want 1", len(torrents[0].Files))
	}

	// statusCode 1 → downloading
	if torrents[1].Status != types.TorrentStatusDownloading {
		t.Errorf("torrent 2 status: got %q, want %q", torrents[1].Status, types.TorrentStatusDownloading)
	}

	// statusCode 9 → error
	if torrents[2].Status != types.TorrentStatusError {
		t.Errorf("torrent 3 status: got %q, want %q", torrents[2].Status, types.TorrentStatusError)
	}
}

func TestProvider_GetTorrents_MagnetsAsMap(t *testing.T) {
	// AllDebrid can return magnets as a map instead of array
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"status": "success",
			"data": {
				"magnets": {
					"0": {"id": 200, "filename": "MapItem.mkv", "size": 100, "hash": "map1", "statusCode": 4, "completionDate": 1717200000, "files": []}
				}
			}
		}`)
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, false)
	torrents, err := ad.GetTorrents()
	if err != nil {
		t.Fatalf("GetTorrents error: %v", err)
	}
	if len(torrents) != 1 {
		t.Fatalf("expected 1 torrent from map format, got %d", len(torrents))
	}
	if torrents[0].Id != "200" {
		t.Errorf("Id: got %q, want %q", torrents[0].Id, "200")
	}
}

// --- GetAvailableSlots ---

func TestProvider_GetAvailableSlots_ValidMapping(t *testing.T) {
	t.Run("no limit returns default", func(t *testing.T) {
		ad := newTestAD("http://unused", false)
		ad.config.Limit = 0

		slots, err := ad.GetAvailableSlots()
		if err != nil {
			t.Fatalf("GetAvailableSlots error: %v", err)
		}
		if slots != config.DefaultAvailableSlots {
			t.Errorf("slots: got %d, want %d", slots, config.DefaultAvailableSlots)
		}
	})

	t.Run("with limit counts magnets", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Return 3 magnets
			fmt.Fprint(w, `{
				"status": "success",
				"data": {
					"magnets": [
						{"id": 1, "filename": "a", "statusCode": 4, "files": []},
						{"id": 2, "filename": "b", "statusCode": 1, "files": []},
						{"id": 3, "filename": "c", "statusCode": 4, "files": []}
					]
				}
			}`)
		}))
		defer srv.Close()

		ad := newTestAD(srv.URL, false)
		ad.config.Limit = 10
		ad.config.MinimumFreeSlot = 2

		slots, err := ad.GetAvailableSlots()
		if err != nil {
			t.Fatalf("GetAvailableSlots error: %v", err)
		}
		// 10 - 3 - 2 = 5
		if slots != 5 {
			t.Errorf("slots: got %d, want 5", slots)
		}
	})

	t.Run("negative clamped to zero", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"status": "success",
				"data": {
					"magnets": [
						{"id": 1, "filename": "a", "statusCode": 4, "files": []},
						{"id": 2, "filename": "b", "statusCode": 4, "files": []},
						{"id": 3, "filename": "c", "statusCode": 4, "files": []},
						{"id": 4, "filename": "d", "statusCode": 4, "files": []},
						{"id": 5, "filename": "e", "statusCode": 4, "files": []}
					]
				}
			}`)
		}))
		defer srv.Close()

		ad := newTestAD(srv.URL, false)
		ad.config.Limit = 3
		ad.config.MinimumFreeSlot = 1

		slots, err := ad.GetAvailableSlots()
		if err != nil {
			t.Fatalf("GetAvailableSlots error: %v", err)
		}
		if slots != 0 {
			t.Errorf("slots: got %d, want 0 (clamped)", slots)
		}
	})
}

// --- Profile malformed payload ---

func TestProvider_AccountProfile_MalformedPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{broken json!!!`))
	}))
	defer srv.Close()

	ad := &AllDebrid{
		Host:   srv.URL,
		APIKey: "ad-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "alldebrid"},
	}

	_, err := ad.GetProfile()
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestProvider_AccountProfile_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"error","error":{"code":"AUTH_MISSING_APIKEY","message":"API key is missing"}}`)
	}))
	defer srv.Close()

	ad := &AllDebrid{
		Host:   srv.URL,
		APIKey: "ad-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "alldebrid"},
	}

	_, err := ad.GetProfile()
	if err == nil {
		t.Fatal("expected error on error status, got nil")
	}
}

func TestProvider_AccountProfile_ValidMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"status": "success",
			"data": {
				"user": {
					"username": "aduser",
					"email": "ad@example.com",
					"isPremium": true,
					"isTrial": false,
					"premiumUntil": 1767225599,
					"fidelityPoints": 42
				}
			}
		}`)
	}))
	defer srv.Close()

	ad := &AllDebrid{
		Host:   srv.URL,
		APIKey: "ad-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "alldebrid"},
	}

	profile, err := ad.GetProfile()
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}
	if profile.Username != "aduser" {
		t.Errorf("Username: got %q, want %q", profile.Username, "aduser")
	}
	if profile.Type != "premium" {
		t.Errorf("Type: got %q, want %q", profile.Type, "premium")
	}
	if profile.Points != 42 {
		t.Errorf("Points: got %d, want 42", profile.Points)
	}
}

// --- AccountSync is a no-op for AllDebrid ---

func TestProvider_AccountSync_NoOp(t *testing.T) {
	ad := &AllDebrid{}
	err := ad.syncAccount(nil)
	if err != nil {
		t.Fatalf("syncAccount should be no-op, got error: %v", err)
	}
}
