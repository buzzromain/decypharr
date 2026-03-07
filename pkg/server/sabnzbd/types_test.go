package sabnzbd

import (
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/pkg/storage"
)

func TestMapStorageStateToSABStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state storage.TorrentState
		want  string
	}{
		{storage.EntryStateDownloading, StatusDownloading},
		{storage.EntryStatePausedDL, StatusPaused},
		{storage.EntryStatePausedUP, StatusCompleted},
		{storage.EntryStateError, StatusFailed},
		{storage.TorrentState("unknown"), StatusQueued},
		{storage.TorrentState(""), StatusQueued},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			t.Parallel()
			got := mapStorageStateToSABStatus(tt.state)
			if got != tt.want {
				t.Errorf("mapStorageStateToSABStatus(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 K"},
		{1024 * 1024, "1.00 M"},
		{1024 * 1024 * 1024, "1.00 G"},
		{1024 * 1024 * 1024 * 1024, "1.00 T"},
		{1536 * 1024, "1.50 M"}, // 1.5 MB
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestConvertToSABnzbdNZB_BasicMapping(t *testing.T) {
	t.Parallel()
	now := time.Now()
	entry := &storage.Entry{
		InfoHash:         "nzbhash123",
		Name:             "Test Show S01E01",
		OriginalFilename: "test-show-s01e01.nzb",
		Size:             1024 * 1024 * 500, // 500 MB
		Progress:         0.5,
		Speed:            1024 * 100, // 100 KB/s
		State:            storage.EntryStateDownloading,
		Category:         "sonarr",
		SavePath:         "/downloads",
		ContentPath:      "/downloads/Test Show S01E01",
		CreatedAt:        now,
		Files:            make(map[string]*storage.File),
		Tags:             []string{"hd", "encode"},
	}

	nzb := convertToSABnzbdNZB(entry)

	if nzb.NzoId != "nzbhash123" {
		t.Errorf("expected NzoId 'nzbhash123', got '%s'", nzb.NzoId)
	}
	if nzb.Name != "Test Show S01E01" {
		t.Errorf("expected Name 'Test Show S01E01', got '%s'", nzb.Name)
	}
	if nzb.Filename != "test-show-s01e01.nzb" {
		t.Errorf("expected Filename 'test-show-s01e01.nzb', got '%s'", nzb.Filename)
	}
	if nzb.Size != 1024*1024*500 {
		t.Errorf("expected Size %d, got %d", int64(1024*1024*500), nzb.Size)
	}
	if nzb.Percentage != 50 {
		t.Errorf("expected Percentage 50, got %f", nzb.Percentage)
	}
	if nzb.Status != StatusDownloading {
		t.Errorf("expected status Downloading, got %s", nzb.Status)
	}
	if nzb.Category != "sonarr" {
		t.Errorf("expected category sonarr, got %s", nzb.Category)
	}
	if nzb.Priority != PriorityNormal {
		t.Errorf("expected priority Normal, got %s", nzb.Priority)
	}
	if nzb.AddedOn != now.Unix() {
		t.Errorf("expected added_on %d, got %d", now.Unix(), nzb.AddedOn)
	}
	if nzb.CompletedOn != 0 {
		t.Errorf("expected completed_on 0, got %d", nzb.CompletedOn)
	}
}

func TestConvertToSABnzbdNZB_WithCompletedAt(t *testing.T) {
	t.Parallel()
	now := time.Now()
	completedAt := now.Add(-1 * time.Hour)
	entry := &storage.Entry{
		State:       storage.EntryStatePausedUP,
		CreatedAt:   now,
		CompletedAt: &completedAt,
		Files:       make(map[string]*storage.File),
	}

	nzb := convertToSABnzbdNZB(entry)
	if nzb.CompletedOn != completedAt.Unix() {
		t.Errorf("expected completed_on %d, got %d", completedAt.Unix(), nzb.CompletedOn)
	}
	if nzb.Status != StatusCompleted {
		t.Errorf("expected Completed status, got %s", nzb.Status)
	}
}

func TestConvertToSABnzbdNZB_TimeLeft(t *testing.T) {
	t.Parallel()
	// 100 bytes left at 10 bytes/sec = 10 seconds
	entry := &storage.Entry{
		Size:      200,
		Progress:  0.5, // 100 bytes left
		Speed:     10,
		CreatedAt: time.Now(),
		Files:     make(map[string]*storage.File),
	}

	nzb := convertToSABnzbdNZB(entry)
	if nzb.TimeLeft == "0:00:00" {
		t.Error("expected non-zero time left when downloading")
	}
}

func TestGetNZBFiles(t *testing.T) {
	t.Parallel()
	entry := &storage.Entry{
		InfoHash: "hash123",
		State:    storage.EntryStatePausedUP,
		Files: map[string]*storage.File{
			"episode.mkv": {Name: "episode.mkv", Size: 1024 * 1024 * 700},
			"deleted.nfo": {Name: "deleted.nfo", Size: 1024, Deleted: true},
			"subs.srt":    {Name: "subs.srt", Size: 50000},
		},
	}

	files := getNZBFiles(entry)

	// Should only include non-deleted files
	if len(files) != 2 {
		t.Fatalf("expected 2 files (excluding deleted), got %d", len(files))
	}

	names := make(map[string]bool)
	for _, f := range files {
		names[f.Filename] = true
		if f.Status != "finished" {
			t.Errorf("expected status 'finished' for completed entry, got '%s' for %s", f.Status, f.Filename)
		}
		if f.MBLeft != "0.00" {
			t.Errorf("expected MBLeft '0.00' for finished file, got '%s'", f.MBLeft)
		}
	}

	if names["deleted.nfo"] {
		t.Error("deleted.nfo should not be included in files")
	}
}

func TestGetNZBFiles_DownloadingStatus(t *testing.T) {
	t.Parallel()
	entry := &storage.Entry{
		InfoHash: "hash123",
		State:    storage.EntryStateDownloading,
		Files: map[string]*storage.File{
			"file.mkv": {Name: "file.mkv", Size: 1024 * 1024},
		},
	}

	files := getNZBFiles(entry)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != "active" {
		t.Errorf("expected status 'active' for downloading, got '%s'", files[0].Status)
	}
}
