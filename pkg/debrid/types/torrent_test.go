package types

import (
	"sync"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/utils"
)

// ── Torrent.GetSize ────────────────────────────────────────────────────────────

func TestTorrent_GetSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		size  int64
		bytes int64
		want  int64
	}{
		{"size non-zero returns size", 1000, 500, 1000},
		{"size zero returns bytes", 0, 500, 500},
		{"both zero returns zero", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tor := &Torrent{Size: tt.size, Bytes: tt.bytes}
			if got := tor.GetSize(); got != tt.want {
				t.Errorf("GetSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ── Torrent.GetFile ────────────────────────────────────────────────────────────

func TestTorrent_GetFile(t *testing.T) {
	t.Parallel()
	tor := &Torrent{
		Files: map[string]File{
			"video.mkv": {Name: "video.mkv", Size: 1000},
			"deleted.mkv": {Name: "deleted.mkv", Size: 500, Deleted: true},
		},
	}

	// Existing non-deleted file
	f, ok := tor.GetFile("video.mkv")
	if !ok {
		t.Fatal("expected to find video.mkv")
	}
	if f.Name != "video.mkv" {
		t.Errorf("unexpected file name: %s", f.Name)
	}

	// Deleted file → not found
	_, ok = tor.GetFile("deleted.mkv")
	if ok {
		t.Error("expected deleted file to not be found")
	}

	// Non-existent file
	_, ok = tor.GetFile("nonexistent.mkv")
	if ok {
		t.Error("expected non-existent file to not be found")
	}
}

// ── Torrent.GetFiles ───────────────────────────────────────────────────────────

func TestTorrent_GetFiles(t *testing.T) {
	t.Parallel()
	tor := &Torrent{
		Files: map[string]File{
			"a.mkv": {Name: "a.mkv", Size: 100},
			"b.mkv": {Name: "b.mkv", Size: 200, Deleted: true},
			"c.mkv": {Name: "c.mkv", Size: 300},
		},
	}

	files := tor.GetFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 non-deleted files, got %d", len(files))
	}
	for _, f := range files {
		if f.Deleted {
			t.Errorf("GetFiles() returned deleted file: %s", f.Name)
		}
	}
}

func TestTorrent_GetFiles_Empty(t *testing.T) {
	t.Parallel()
	tor := &Torrent{Files: map[string]File{}}
	if files := tor.GetFiles(); len(files) != 0 {
		t.Errorf("expected empty slice, got %v", files)
	}
}

// ── Torrent.Copy ──────────────────────────────────────────────────────────────

func TestTorrent_Copy(t *testing.T) {
	t.Parallel()
	magnet := &utils.Magnet{Name: "test", InfoHash: "abc123"}
	orig := &Torrent{
		Id:               "id1",
		InfoHash:         "abc123",
		Name:             "Test Torrent",
		Filename:         "test.torrent",
		OriginalFilename: "original.torrent",
		Size:             1000,
		Bytes:            900,
		Magnet:           magnet,
		Files: map[string]File{
			"file.mkv": {Name: "file.mkv", Size: 900},
		},
		Status:   TorrentStatusDownloaded,
		Added:    time.Now(),
		Progress: 100.0,
		Speed:    0,
		Seeders:  5,
		Links:    []string{"http://link1", "http://link2"},
		Debrid:   "realdebrid",
	}

	cp := orig.Copy()
	if cp == orig {
		t.Fatal("Copy() should return a different pointer")
	}
	if cp.Id != orig.Id {
		t.Errorf("Copy().Id = %q, want %q", cp.Id, orig.Id)
	}
	if cp.Name != orig.Name {
		t.Errorf("Copy().Name = %q, want %q", cp.Name, orig.Name)
	}
	if cp.Magnet != orig.Magnet {
		t.Error("Magnet pointer should be the same (shallow copy)")
	}

	// Mutating copy's Files should not affect original
	cp.Files["new.mkv"] = File{Name: "new.mkv"}
	if _, exists := orig.Files["new.mkv"]; exists {
		t.Error("modifying copy's Files should not affect original")
	}

	// Mutating copy's Links should not affect original
	cp.Links = append(cp.Links, "http://link3")
	if len(orig.Links) != 2 {
		t.Error("modifying copy's Links should not affect original")
	}
}

func TestTorrent_Copy_Concurrent(t *testing.T) {
	t.Parallel()
	tor := &Torrent{
		Id:   "id1",
		Name: "Test",
		Files: map[string]File{
			"f.mkv": {Name: "f.mkv"},
		},
		Links: []string{"http://link"},
	}

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp := tor.Copy()
			_ = cp.Id
		}()
	}
	wg.Wait()
}

// ── DownloadLink.Empty / Valid ─────────────────────────────────────────────────

func TestDownloadLink_Empty(t *testing.T) {
	t.Parallel()
	dl := DownloadLink{}
	if !dl.Empty() {
		t.Error("empty DownloadLink should be empty")
	}

	dl.DownloadLink = "http://example.com/file.mkv"
	if dl.Empty() {
		t.Error("non-empty DownloadLink should not be empty")
	}
}

func TestDownloadLink_Valid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		link      string
		wantErr   bool
	}{
		{"valid URL", "http://example.com/file.mkv", false},
		{"valid HTTPS", "https://cdn.example.com/path/file.mkv", false},
		{"empty link", "", true},
		{"invalid URL", "not-a-url", true},
		{"ftp URL", "ftp://server.com/file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dl := DownloadLink{DownloadLink: tt.link}
			err := dl.Valid()
			if (err != nil) != tt.wantErr {
				t.Errorf("DownloadLink.Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDownloadLink_String(t *testing.T) {
	t.Parallel()
	dl := DownloadLink{DownloadLink: "http://example.com/file.mkv"}
	if dl.String() != "http://example.com/file.mkv" {
		t.Errorf("String() = %q, want %q", dl.String(), "http://example.com/file.mkv")
	}
}

// ── Torrent.Cleanup ───────────────────────────────────────────────────────────

func TestTorrent_Cleanup_NoRemove(t *testing.T) {
	t.Parallel()
	tor := &Torrent{Filename: "/nonexistent/path/file.mkv"}
	// remove=false should be a no-op
	tor.Cleanup(false)
}

func TestTorrent_Cleanup_Remove_NonexistentFile(t *testing.T) {
	t.Parallel()
	tor := &Torrent{Filename: "/nonexistent/path/file.mkv"}
	// Should not panic even if file doesn't exist
	tor.Cleanup(true)
}

// ── TorrentStatus constants ───────────────────────────────────────────────────

func TestTorrentStatus_Values(t *testing.T) {
	t.Parallel()
	// Ensure constants have expected string values
	if TorrentStatusQueued != "queued" {
		t.Errorf("TorrentStatusQueued = %q, want %q", TorrentStatusQueued, "queued")
	}
	if TorrentStatusDownloading != "downloading" {
		t.Errorf("TorrentStatusDownloading = %q, want %q", TorrentStatusDownloading, "downloading")
	}
	if TorrentStatusDownloaded != "downloaded" {
		t.Errorf("TorrentStatusDownloaded = %q, want %q", TorrentStatusDownloaded, "downloaded")
	}
	if TorrentStatusError != "error" {
		t.Errorf("TorrentStatusError = %q, want %q", TorrentStatusError, "error")
	}
}
