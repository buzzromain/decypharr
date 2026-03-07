package qbit

import (
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/pkg/storage"
)

func TestConvertToQBitTorrent_BasicMapping(t *testing.T) {
	t.Parallel()
	now := time.Now()
	entry := &storage.Entry{
		InfoHash:       "abc123",
		Name:           "Test Movie",
		Size:           1000000,
		Progress:       0.5,
		Speed:          50000,
		Seeders:        10,
		State:          storage.EntryStateDownloading,
		Category:       "radarr",
		SavePath:       "/downloads",
		ContentPath:    "/downloads/Test Movie",
		ActiveProvider: "realdebrid",
		Magnet:         "magnet:?xt=urn:btih:abc123",
		CreatedAt:      now,
		Files:          make(map[string]*storage.File),
	}

	torrent := convertToQBitTorrentTorrent(entry)

	if torrent.Hash != "abc123" {
		t.Errorf("expected hash 'abc123', got '%s'", torrent.Hash)
	}
	if torrent.Name != "Test Movie" {
		t.Errorf("expected name 'Test Movie', got '%s'", torrent.Name)
	}
	if torrent.Size != 1000000 {
		t.Errorf("expected size 1000000, got %d", torrent.Size)
	}
	if torrent.Progress != 0.5 {
		t.Errorf("expected progress 0.5, got %f", torrent.Progress)
	}
	if torrent.Dlspeed != 50000 {
		t.Errorf("expected dlspeed 50000, got %d", torrent.Dlspeed)
	}
	if torrent.NumSeeds != 10 {
		t.Errorf("expected 10 seeds, got %d", torrent.NumSeeds)
	}
	if torrent.State != storage.EntryStateDownloading {
		t.Errorf("expected downloading state, got %s", torrent.State)
	}
	if torrent.Category != "radarr" {
		t.Errorf("expected category radarr, got %s", torrent.Category)
	}
	if torrent.SavePath != "/downloads" {
		t.Errorf("expected save_path /downloads, got %s", torrent.SavePath)
	}
	if torrent.Debrid != "realdebrid" {
		t.Errorf("expected debrid realdebrid, got %s", torrent.Debrid)
	}
	if torrent.MagnetURI != "magnet:?xt=urn:btih:abc123" {
		t.Errorf("expected magnet URI, got %s", torrent.MagnetURI)
	}
	if torrent.AddedOn != now.Unix() {
		t.Errorf("expected added_on %d, got %d", now.Unix(), torrent.AddedOn)
	}
}

func TestConvertToQBitTorrent_AmountLeft(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		size     int64
		progress float64
		wantLeft int64
	}{
		{"not started", 1000, 0.0, 1000},
		{"halfway", 1000, 0.5, 500},
		{"complete", 1000, 1.0, 0},
		{"75 percent", 10000, 0.75, 2500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := &storage.Entry{
				Size:      tt.size,
				Progress:  tt.progress,
				CreatedAt: time.Now(),
				Files:     make(map[string]*storage.File),
			}
			torrent := convertToQBitTorrentTorrent(entry)
			if torrent.AmountLeft != tt.wantLeft {
				t.Errorf("expected amount_left %d, got %d", tt.wantLeft, torrent.AmountLeft)
			}
		})
	}
}

func TestConvertToQBitTorrent_EmptyFiles(t *testing.T) {
	t.Parallel()
	entry := &storage.Entry{
		CreatedAt: time.Now(),
		Files:     make(map[string]*storage.File),
	}
	torrent := convertToQBitTorrentTorrent(entry)
	if len(torrent.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(torrent.Files))
	}
}

func TestGetTorrentFiles(t *testing.T) {
	t.Parallel()
	entry := &storage.Entry{
		Files: map[string]*storage.File{
			"movie.mkv":     {Name: "movie.mkv", Size: 5000000},
			"subtitles.srt": {Name: "subtitles.srt", Size: 50000},
		},
	}

	files := getTorrentFiles(entry)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Verify each file has correct fields (order is non-deterministic from map)
	names := make(map[string]int64)
	for _, f := range files {
		names[f.Name] = f.Size
	}
	if names["movie.mkv"] != 5000000 {
		t.Errorf("expected movie.mkv size 5000000, got %d", names["movie.mkv"])
	}
	if names["subtitles.srt"] != 50000 {
		t.Errorf("expected subtitles.srt size 50000, got %d", names["subtitles.srt"])
	}

	// Indices should be sequential starting from 0
	indices := make(map[int]bool)
	for _, f := range files {
		indices[f.Index] = true
	}
	if !indices[0] || !indices[1] {
		t.Errorf("expected indices 0 and 1, got %v", indices)
	}
}
