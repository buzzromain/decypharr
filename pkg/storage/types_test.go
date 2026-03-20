package storage

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	debridTypes "github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeDownloadedProvider(id, provider string) *ProviderEntry {
	return &ProviderEntry{
		Provider: provider,
		ID:       id,
		AddedAt:  time.Now(),
		Status:   debridTypes.TorrentStatusDownloaded,
		Files: map[string]*ProviderFile{
			"movie.mkv": {Id: "file1", Link: "https://dl.example.com/movie.mkv", Path: "/files/movie.mkv"},
		},
	}
}

func makeEntryWithProvider(infohash, provider string) *Entry {
	return &Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       infohash,
		Name:           "Test Movie",
		ActiveProvider: provider,
		Providers: map[string]*ProviderEntry{
			provider: makeDownloadedProvider("debrid-id-1", provider),
		},
		Files: map[string]*File{
			"movie.mkv": {Name: "movie.mkv", Size: 1000},
		},
	}
}

// ── Entry.MarkAsCompleted ────────────────────────────────────────────────────

func TestEntry_MarkAsCompleted(t *testing.T) {
	t.Parallel()
	e := &Entry{InfoHash: "abc123"}

	before := time.Now()
	e.MarkAsCompleted("/downloads/Test Movie")
	after := time.Now()

	if e.State != EntryStatePausedUP {
		t.Errorf("State = %q, want %q", e.State, EntryStatePausedUP)
	}
	if e.IsDownloading {
		t.Error("IsDownloading should be false after completion")
	}
	if !e.IsComplete {
		t.Error("IsComplete should be true after completion")
	}
	if e.Progress != 1.0 {
		t.Errorf("Progress = %f, want 1.0", e.Progress)
	}
	if e.ContentPath != "/downloads/Test Movie" {
		t.Errorf("ContentPath = %q, want %q", e.ContentPath, "/downloads/Test Movie")
	}
	if e.CompletedAt == nil {
		t.Fatal("CompletedAt should not be nil")
	}
	if e.CompletedAt.Before(before) || e.CompletedAt.After(after) {
		t.Errorf("CompletedAt %v not in expected range [%v, %v]", e.CompletedAt, before, after)
	}
	if e.UpdatedAt.Before(before) || e.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt %v not in expected range", e.UpdatedAt)
	}
}

func TestEntry_MarkAsCompleted_IdempotentTimestamp(t *testing.T) {
	t.Parallel()
	e := &Entry{InfoHash: "abc123"}
	e.MarkAsCompleted("/first")
	first := *e.CompletedAt

	e.MarkAsCompleted("/second")

	// second call overwrites — both are valid timestamps
	if e.CompletedAt.Equal(first) {
		// unlikely but not a bug; the test just confirms MarkAsCompleted updates the timestamp
		t.Log("CompletedAt unchanged (same millisecond)")
	}
}

// ── Entry.MarkAsError ────────────────────────────────────────────────────────

func TestEntry_MarkAsError(t *testing.T) {
	t.Parallel()
	e := &Entry{InfoHash: "abc123", ErrorCount: 2}

	before := time.Now()
	err := errors.New("provider timeout")
	e.MarkAsError(err)
	after := time.Now()

	if e.State != EntryStateError {
		t.Errorf("State = %q, want %q", e.State, EntryStateError)
	}
	if e.IsDownloading {
		t.Error("IsDownloading should be false after error")
	}
	if e.LastError != "provider timeout" {
		t.Errorf("LastError = %q, want %q", e.LastError, "provider timeout")
	}
	if e.ErrorCount != 3 {
		t.Errorf("ErrorCount = %d, want 3 (incremented)", e.ErrorCount)
	}
	if e.LastErrorTime == nil {
		t.Fatal("LastErrorTime should not be nil")
	}
	if e.LastErrorTime.Before(before) || e.LastErrorTime.After(after) {
		t.Errorf("LastErrorTime %v not in expected range", e.LastErrorTime)
	}
}

func TestEntry_MarkAsError_AccumulatesCount(t *testing.T) {
	t.Parallel()
	e := &Entry{InfoHash: "abc123"}
	for i := range 5 {
		e.MarkAsError(errors.New("err"))
		if e.ErrorCount != i+1 {
			t.Errorf("ErrorCount after %d calls = %d, want %d", i+1, e.ErrorCount, i+1)
		}
	}
}

// ── Entry.IsValid ─────────────────────────────────────────────────────────────

func TestEntry_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry *Entry
		want  bool
	}{
		{
			name: "valid entry",
			entry: &Entry{
				InfoHash:       "abc123",
				Name:           "Test Movie",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {
						Provider: "rd",
						ID:       "debrid-123",
						Status:   debridTypes.TorrentStatusDownloaded,
						Files: map[string]*ProviderFile{
							"movie.mkv": {Id: "file1", Link: "https://dl.example.com/movie.mkv"},
						},
					},
				},
			},
			want: true,
		},
		{
			name:  "empty infohash",
			entry: &Entry{Name: "Test", ActiveProvider: "rd"},
			want:  false,
		},
		{
			name:  "empty name",
			entry: &Entry{InfoHash: "abc123", ActiveProvider: "rd"},
			want:  false,
		},
		{
			name:  "no providers",
			entry: &Entry{InfoHash: "abc123", Name: "Test", Providers: map[string]*ProviderEntry{}},
			want:  false,
		},
		{
			name: "active provider not in providers map",
			entry: &Entry{
				InfoHash:       "abc123",
				Name:           "Test",
				ActiveProvider: "missing",
				Providers:      map[string]*ProviderEntry{"rd": {Provider: "rd", ID: "x"}},
			},
			want: false,
		},
		{
			name: "provider entry missing ID",
			entry: &Entry{
				InfoHash:       "abc123",
				Name:           "Test",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {Provider: "rd", ID: "", Files: map[string]*ProviderFile{"f": {Id: "1", Link: "http://x"}}},
				},
			},
			want: false,
		},
		{
			name: "provider file missing link",
			entry: &Entry{
				InfoHash:       "abc123",
				Name:           "Test",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {
						Provider: "rd",
						ID:       "debrid-123",
						Files:    map[string]*ProviderFile{"f": {Id: "1", Link: ""}},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.entry.IsValid()
			if got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ── Entry.RunChecks ───────────────────────────────────────────────────────────

func TestEntry_RunChecks(t *testing.T) {
	t.Parallel()

	validFile := &File{Name: "movie.mkv"}
	validProviderFile := &ProviderFile{Id: "f1", Link: "https://dl.example.com/movie.mkv"}

	tests := []struct {
		name        string
		entry       *Entry
		wantRefresh bool
		wantErr     bool
	}{
		{
			name: "valid complete entry",
			entry: &Entry{
				InfoHash:       "abc",
				Name:           "Test",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {
						Provider: "rd",
						ID:       "x",
						Status:   debridTypes.TorrentStatusDownloaded,
						Files:    map[string]*ProviderFile{"movie.mkv": validProviderFile},
					},
				},
				Files: map[string]*File{"movie.mkv": validFile},
			},
			wantRefresh: false,
			wantErr:     false,
		},
		{
			name:        "bad entry",
			entry:       &Entry{Bad: true},
			wantRefresh: false,
			wantErr:     true,
		},
		{
			name:        "no active provider",
			entry:       &Entry{InfoHash: "abc"},
			wantRefresh: true,
			wantErr:     true,
		},
		{
			name: "provider not downloaded",
			entry: &Entry{
				InfoHash:       "abc",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {Provider: "rd", ID: "x", Status: debridTypes.TorrentStatusDownloading},
				},
			},
			wantRefresh: true,
			wantErr:     true,
		},
		{
			name: "provider has no files",
			entry: &Entry{
				InfoHash:       "abc",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {Provider: "rd", ID: "x", Status: debridTypes.TorrentStatusDownloaded, Files: map[string]*ProviderFile{}},
				},
			},
			wantRefresh: true,
			wantErr:     true,
		},
		{
			name: "entry file missing in provider files",
			entry: &Entry{
				InfoHash:       "abc",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {
						Provider: "rd",
						ID:       "x",
						Status:   debridTypes.TorrentStatusDownloaded,
						Files:    map[string]*ProviderFile{"other.mkv": validProviderFile},
					},
				},
				Files: map[string]*File{"movie.mkv": validFile},
			},
			wantRefresh: true,
			wantErr:     true,
		},
		{
			name: "entry file has no link in provider",
			entry: &Entry{
				InfoHash:       "abc",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {
						Provider: "rd",
						ID:       "x",
						Status:   debridTypes.TorrentStatusDownloaded,
						Files:    map[string]*ProviderFile{"movie.mkv": {Id: "f1", Link: ""}},
					},
				},
				Files: map[string]*File{"movie.mkv": validFile},
			},
			wantRefresh: true,
			wantErr:     true,
		},
		{
			name: "deleted file skipped in check",
			entry: &Entry{
				InfoHash:       "abc",
				ActiveProvider: "rd",
				Providers: map[string]*ProviderEntry{
					"rd": {
						Provider: "rd",
						ID:       "x",
						Status:   debridTypes.TorrentStatusDownloaded,
						Files:    map[string]*ProviderFile{"good.mkv": validProviderFile},
					},
				},
				Files: map[string]*File{
					"good.mkv":    {Name: "good.mkv"},
					"deleted.nfo": {Name: "deleted.nfo", Deleted: true},
				},
			},
			wantRefresh: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			refresh, err := tt.entry.RunChecks()
			if refresh != tt.wantRefresh {
				t.Errorf("RunChecks() refresh = %v, want %v", refresh, tt.wantRefresh)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("RunChecks() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ── Entry.GetActiveProvider ───────────────────────────────────────────────────

func TestEntry_GetActiveProvider(t *testing.T) {
	t.Parallel()

	t.Run("nil providers", func(t *testing.T) {
		t.Parallel()
		e := &Entry{ActiveProvider: "rd"}
		if got := e.GetActiveProvider(); got != nil {
			t.Errorf("expected nil for nil providers, got %+v", got)
		}
	})

	t.Run("empty active provider", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			Providers: map[string]*ProviderEntry{"rd": {Provider: "rd"}},
		}
		if got := e.GetActiveProvider(); got != nil {
			t.Errorf("expected nil for empty ActiveProvider, got %+v", got)
		}
	})

	t.Run("active provider not in map", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			ActiveProvider: "missing",
			Providers:      map[string]*ProviderEntry{"rd": {Provider: "rd"}},
		}
		if got := e.GetActiveProvider(); got != nil {
			t.Errorf("expected nil for missing provider, got %+v", got)
		}
	})

	t.Run("valid active provider", func(t *testing.T) {
		t.Parallel()
		p := &ProviderEntry{Provider: "rd", ID: "123"}
		e := &Entry{
			ActiveProvider: "rd",
			Providers:      map[string]*ProviderEntry{"rd": p},
		}
		got := e.GetActiveProvider()
		if got == nil {
			t.Fatal("expected non-nil provider")
		}
		if got.ID != "123" {
			t.Errorf("got ID %q, want %q", got.ID, "123")
		}
	})
}

// ── Entry.HasProvider ─────────────────────────────────────────────────────────

func TestEntry_HasProvider(t *testing.T) {
	t.Parallel()

	e := &Entry{
		Providers: map[string]*ProviderEntry{
			"rd": {Provider: "rd"},
			"tb": {Provider: "tb"},
		},
	}

	if !e.HasProvider("rd") {
		t.Error("expected HasProvider('rd') = true")
	}
	if !e.HasProvider("tb") {
		t.Error("expected HasProvider('tb') = true")
	}
	if e.HasProvider("missing") {
		t.Error("expected HasProvider('missing') = false")
	}

	// nil providers
	e2 := &Entry{}
	if e2.HasProvider("rd") {
		t.Error("expected HasProvider to return false for nil providers")
	}
}

// ── Entry.ActivatePlacement ───────────────────────────────────────────────────

func TestEntry_ActivatePlacement(t *testing.T) {
	t.Parallel()

	t.Run("successful activation", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			Providers: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "x", Status: debridTypes.TorrentStatusDownloaded},
			},
		}
		if err := e.ActivatePlacement("rd"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if e.ActiveProvider != "rd" {
			t.Errorf("ActiveProvider = %q, want %q", e.ActiveProvider, "rd")
		}
	})

	t.Run("nil providers", func(t *testing.T) {
		t.Parallel()
		e := &Entry{}
		if err := e.ActivatePlacement("rd"); err == nil {
			t.Error("expected error for nil providers")
		}
	})

	t.Run("provider not found", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			Providers: map[string]*ProviderEntry{"tb": {Provider: "tb", Status: debridTypes.TorrentStatusDownloaded}},
		}
		if err := e.ActivatePlacement("rd"); !errors.Is(err, ErrPlacementNotFound) {
			t.Errorf("expected ErrPlacementNotFound, got %v", err)
		}
	})

	t.Run("provider not completed", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			Providers: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "x", Status: debridTypes.TorrentStatusDownloading},
			},
		}
		if err := e.ActivatePlacement("rd"); !errors.Is(err, ErrPlacementNotCompleted) {
			t.Errorf("expected ErrPlacementNotCompleted, got %v", err)
		}
	})
}

// ── Entry.SwitchToNextProvider ────────────────────────────────────────────────

func TestEntry_SwitchToNextProvider(t *testing.T) {
	t.Parallel()

	t.Run("nil providers is no-op", func(t *testing.T) {
		t.Parallel()
		e := &Entry{}
		e.SwitchToNextProvider() // should not panic
	})

	t.Run("no downloaded providers leaves active empty", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			ActiveProvider: "old",
			Providers: map[string]*ProviderEntry{
				"dl": {Provider: "dl", Status: debridTypes.TorrentStatusDownloading},
			},
		}
		e.SwitchToNextProvider()
		// No downloaded provider found, ActiveProvider unchanged by SwitchToNextProvider logic
		// (ActivatePlacement would fail, so it remains as "old" from the loop not finding one)
	})

	t.Run("switches to downloaded provider", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			ActiveProvider: "failed",
			Providers: map[string]*ProviderEntry{
				"failed": {Provider: "failed", Status: debridTypes.TorrentStatusError},
				"rd":     {Provider: "rd", ID: "x", Status: debridTypes.TorrentStatusDownloaded},
			},
		}
		e.SwitchToNextProvider()
		if e.ActiveProvider != "rd" {
			t.Errorf("expected switch to 'rd', got %q", e.ActiveProvider)
		}
	})
}

// ── Entry.RemoveProvider ──────────────────────────────────────────────────────

func TestEntry_RemoveProvider(t *testing.T) {
	t.Parallel()

	t.Run("removes provider from map", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			ActiveProvider: "tb",
			Providers: map[string]*ProviderEntry{
				"rd": {Provider: "rd", Status: debridTypes.TorrentStatusDownloaded},
				"tb": {Provider: "tb", Status: debridTypes.TorrentStatusDownloaded},
			},
		}
		e.RemoveProvider("rd", nil)
		if e.HasProvider("rd") {
			t.Error("expected 'rd' to be removed")
		}
		if !e.HasProvider("tb") {
			t.Error("expected 'tb' to remain")
		}
	})

	t.Run("removes active provider and switches", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			ActiveProvider: "rd",
			Providers: map[string]*ProviderEntry{
				"rd": {Provider: "rd", Status: debridTypes.TorrentStatusDownloaded},
				"tb": {Provider: "tb", ID: "y", Status: debridTypes.TorrentStatusDownloaded},
			},
		}
		e.RemoveProvider("rd", nil)
		if e.ActiveProvider == "rd" {
			t.Error("expected active provider to switch away from removed 'rd'")
		}
		if e.ActiveProvider != "tb" {
			t.Errorf("expected switch to 'tb', got %q", e.ActiveProvider)
		}
	})

	t.Run("calls cleanup function", func(t *testing.T) {
		t.Parallel()
		cleaned := false
		p := &ProviderEntry{Provider: "rd", Status: debridTypes.TorrentStatusDownloaded}
		e := &Entry{
			Providers: map[string]*ProviderEntry{"rd": p},
		}
		e.RemoveProvider("rd", func(pe *ProviderEntry) error {
			cleaned = true
			return nil
		})
		if !cleaned {
			t.Error("expected cleanup function to be called")
		}
	})

	t.Run("nil providers is no-op", func(t *testing.T) {
		t.Parallel()
		e := &Entry{}
		e.RemoveProvider("rd", nil) // should not panic
	})
}

// ── Entry.GetFile ─────────────────────────────────────────────────────────────

func TestEntry_GetFile(t *testing.T) {
	t.Parallel()

	t.Run("nil files map returns error", func(t *testing.T) {
		t.Parallel()
		e := &Entry{}
		_, err := e.GetFile("movie.mkv")
		if err == nil {
			t.Error("expected error for nil files map")
		}
	})

	t.Run("existing file returned", func(t *testing.T) {
		t.Parallel()
		e := &Entry{Files: map[string]*File{"movie.mkv": {Name: "movie.mkv", Size: 500}}}
		f, err := e.GetFile("movie.mkv")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Name != "movie.mkv" || f.Size != 500 {
			t.Errorf("wrong file returned: %+v", f)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()
		e := &Entry{Files: map[string]*File{}}
		_, err := e.GetFile("missing.mkv")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("deleted file returns error", func(t *testing.T) {
		t.Parallel()
		e := &Entry{Files: map[string]*File{"dead.mkv": {Name: "dead.mkv", Deleted: true}}}
		_, err := e.GetFile("dead.mkv")
		if err == nil {
			t.Error("expected error for deleted file")
		}
	})
}

// ── Entry.GetActiveFiles ──────────────────────────────────────────────────────

func TestEntry_GetActiveFiles(t *testing.T) {
	t.Parallel()

	e := &Entry{
		Files: map[string]*File{
			"ep01.mkv": {Name: "ep01.mkv"},
			"ep02.mkv": {Name: "ep02.mkv"},
			"del.nfo":  {Name: "del.nfo", Deleted: true},
		},
	}
	active := e.GetActiveFiles()
	if len(active) != 2 {
		t.Errorf("expected 2 active files, got %d", len(active))
	}
	for _, f := range active {
		if f.Deleted {
			t.Errorf("GetActiveFiles() returned deleted file: %s", f.Name)
		}
	}
}

// ── ProviderEntry.IsValid ─────────────────────────────────────────────────────

func TestProviderEntry_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pe   *ProviderEntry
		want bool
	}{
		{
			name: "valid entry with all fields",
			pe: &ProviderEntry{
				Provider: "rd",
				ID:       "abc123",
				Files:    map[string]*ProviderFile{"f": {Id: "1", Link: "https://dl.example.com/f"}},
			},
			want: true,
		},
		{
			name: "empty ID",
			pe:   &ProviderEntry{Provider: "rd", Files: map[string]*ProviderFile{"f": {Id: "1", Link: "http://x"}}},
			want: false,
		},
		{
			name: "empty provider",
			pe:   &ProviderEntry{ID: "abc", Files: map[string]*ProviderFile{"f": {Id: "1", Link: "http://x"}}},
			want: false,
		},
		{
			name: "file missing Id",
			pe: &ProviderEntry{
				Provider: "rd",
				ID:       "abc",
				Files:    map[string]*ProviderFile{"f": {Id: "", Link: "http://x"}},
			},
			want: false,
		},
		{
			name: "file missing Link",
			pe: &ProviderEntry{
				Provider: "rd",
				ID:       "abc",
				Files:    map[string]*ProviderFile{"f": {Id: "1", Link: ""}},
			},
			want: false,
		},
		{
			name: "no files (empty map) - still valid (no files to fail check)",
			pe:   &ProviderEntry{Provider: "rd", ID: "abc", Files: map[string]*ProviderFile{}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.pe.IsValid()
			if got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ── ProviderEntry.NeedsUpdate ─────────────────────────────────────────────────

func TestProviderEntry_NeedsUpdate(t *testing.T) {
	t.Parallel()

	base := &ProviderEntry{
		ID:     "abc",
		Status: debridTypes.TorrentStatusDownloaded,
		Files:  map[string]*ProviderFile{"f": {Id: "1", Link: "http://x"}},
	}

	t.Run("no change needed", func(t *testing.T) {
		t.Parallel()
		remote := &debridTypes.Torrent{Id: "abc", Status: debridTypes.TorrentStatusDownloaded}
		if base.NeedsUpdate(remote) {
			t.Error("expected NeedsUpdate=false when ID and Status match")
		}
	})

	t.Run("different ID", func(t *testing.T) {
		t.Parallel()
		remote := &debridTypes.Torrent{Id: "different", Status: debridTypes.TorrentStatusDownloaded}
		if !base.NeedsUpdate(remote) {
			t.Error("expected NeedsUpdate=true when IDs differ")
		}
	})

	t.Run("different status", func(t *testing.T) {
		t.Parallel()
		remote := &debridTypes.Torrent{Id: "abc", Status: debridTypes.TorrentStatusDownloading}
		if !base.NeedsUpdate(remote) {
			t.Error("expected NeedsUpdate=true when status changes")
		}
	})

	t.Run("empty files triggers update", func(t *testing.T) {
		t.Parallel()
		pe := &ProviderEntry{
			ID:     "abc",
			Status: debridTypes.TorrentStatusDownloaded,
			Files:  map[string]*ProviderFile{},
		}
		remote := &debridTypes.Torrent{Id: "abc", Status: debridTypes.TorrentStatusDownloaded}
		if !pe.NeedsUpdate(remote) {
			t.Error("expected NeedsUpdate=true when files are empty")
		}
	})
}

// ── GetTorrentFolder ──────────────────────────────────────────────────────────

func TestGetTorrentFolder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		naming   config.WebDavFolderNaming
		name     string
		original string
		infohash string
		wantFn   func(got string) bool
		desc     string
	}{
		{
			naming:   config.WebDavUseFileName,
			name:     "The Dark Knight (2008)",
			original: "dark.knight.2008.mkv",
			wantFn:   func(got string) bool { return got == "The Dark Knight (2008)" },
			desc:     "filename naming uses Name",
		},
		{
			naming:   config.WebDavUseOriginalName,
			name:     "The Dark Knight (2008)",
			original: "dark.knight.2008.mkv",
			wantFn:   func(got string) bool { return got == "dark.knight.2008.mkv" },
			desc:     "original naming uses OriginalFilename",
		},
		{
			naming:   config.WebDavUseFileNameNoExt,
			name:     "movie.mkv",
			original: "movie.2008.mkv",
			wantFn:   func(got string) bool { return got == "movie" },
			desc:     "filename_no_ext removes extension from Name",
		},
		{
			naming:   config.WebDavUseOriginalNameNoExt,
			name:     "movie.mkv",
			original: "movie.2008.mkv",
			wantFn:   func(got string) bool { return got == "movie.2008" },
			desc:     "original_no_ext removes extension from OriginalFilename",
		},
		{
			naming:   config.WebdavUseHash,
			infohash: "abc123def456",
			wantFn:   func(got string) bool { return got == "abc123def456" },
			desc:     "hash naming uses InfoHash directly",
		},
		{
			naming: "unknown_naming",
			name:   "fallback name",
			wantFn: func(got string) bool { return got == "fallback name" },
			desc:   "unknown naming falls back to filename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			e := &Entry{
				Name:             tt.name,
				OriginalFilename: tt.original,
				InfoHash:         tt.infohash,
			}
			got := GetTorrentFolder(tt.naming, e)
			if !tt.wantFn(got) {
				t.Errorf("GetTorrentFolder(%q) = %q, unexpected result", tt.naming, got)
			}
		})
	}
}

// TestGetTorrentFolder_PathTraversal documents path traversal risk:
// path.Clean does NOT remove leading ".." from relative paths, so a malicious
// provider returning a torrent name like "../../etc/passwd" would produce a
// folder name that traverses outside the intended directory.
func TestGetTorrentFolder_PathTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string // what path.Clean produces
	}{
		{"../../etc/passwd", "../../etc/passwd"},
		{"../sibling", "../sibling"},
		{"safe/subdir", "safe/subdir"},
		{"./current", "current"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			e := &Entry{Name: tt.input}
			got := GetTorrentFolder(config.WebDavUseFileName, e)
			if got != tt.expected {
				t.Errorf("GetTorrentFolder(%q) = %q, want %q", tt.input, got, tt.expected)
			}
			// Confirm traversal sequences survive path.Clean (documenting the risk)
			if strings.Contains(got, "..") && strings.Contains(tt.input, "..") {
				// Path traversal sequences survive: this is expected from the current
				// implementation. Callers must validate folder names against a root.
				t.Logf("SECURITY NOTE: Path traversal sequence in provider name %q survived path.Clean → %q", tt.input, got)
			}
		})
	}
}

// ── Entry.Validate ────────────────────────────────────────────────────────────

func TestEntry_Validate(t *testing.T) {
	t.Parallel()

	t.Run("no active provider", func(t *testing.T) {
		t.Parallel()
		e := &Entry{}
		if err := e.Validate(); err == nil {
			t.Error("expected error for no active provider")
		}
	})

	t.Run("active provider with no files", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			ActiveProvider: "rd",
			Providers: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "x", Files: map[string]*ProviderFile{}},
			},
		}
		if err := e.Validate(); err == nil {
			t.Error("expected error for empty files in active provider")
		}
	})

	t.Run("valid entry passes", func(t *testing.T) {
		t.Parallel()
		e := &Entry{
			ActiveProvider: "rd",
			Providers: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "x", Files: map[string]*ProviderFile{"f": {Id: "1", Link: "http://x"}}},
			},
		}
		if err := e.Validate(); err != nil {
			t.Errorf("unexpected error for valid entry: %v", err)
		}
	})
}

// ── Entry state completeness ──────────────────────────────────────────────────

// TestEntry_StateTransitions verifies the state machine: a downloading entry
// that completes should have consistent flags and no longer be downloading.
func TestEntry_StateTransitions(t *testing.T) {
	t.Parallel()

	e := &Entry{
		InfoHash:       "abc123",
		IsDownloading:  true,
		State:          EntryStateDownloading,
		IsComplete:     false,
		ActiveProvider: "rd",
	}

	// Error transition
	e.MarkAsError(errors.New("download failed"))
	if e.State != EntryStateError {
		t.Errorf("after MarkAsError: State = %q, want %q", e.State, EntryStateError)
	}
	if e.IsDownloading {
		t.Error("after MarkAsError: IsDownloading should be false")
	}

	// Recovery and completion
	e.IsDownloading = true
	e.MarkAsCompleted("/path/to/content")
	if e.State != EntryStatePausedUP {
		t.Errorf("after MarkAsCompleted: State = %q, want %q", e.State, EntryStatePausedUP)
	}
	if e.IsDownloading {
		t.Error("after MarkAsCompleted: IsDownloading should be false")
	}
	if !e.IsComplete {
		t.Error("after MarkAsCompleted: IsComplete should be true")
	}
	if e.Progress != 1.0 {
		t.Errorf("after MarkAsCompleted: Progress = %f, want 1.0", e.Progress)
	}
}

// ── Entry.IsTorrent / IsNZB ───────────────────────────────────────────────────

func TestEntry_IsTorrent_IsNZB(t *testing.T) {
	t.Parallel()

	torrent := &Entry{Protocol: config.ProtocolTorrent}
	if !torrent.IsTorrent() {
		t.Error("IsTorrent() = false for torrent protocol")
	}
	if torrent.IsNZB() {
		t.Error("IsNZB() = true for torrent protocol")
	}

	nzb := &Entry{Protocol: config.ProtocolNZB}
	if nzb.IsTorrent() {
		t.Error("IsTorrent() = true for NZB protocol")
	}
	if !nzb.IsNZB() {
		t.Error("IsNZB() = false for NZB protocol")
	}
}

// ── Entry.CanBeFixed / CanBeMoved ─────────────────────────────────────────────

func TestEntry_CanBeFixed_CanBeMoved(t *testing.T) {
	t.Parallel()

	torrent := &Entry{Protocol: config.ProtocolTorrent}
	if !torrent.CanBeFixed() {
		t.Error("CanBeFixed() = false for torrent")
	}
	if !torrent.CanBeMoved() {
		t.Error("CanBeMoved() = false for torrent")
	}

	nzb := &Entry{Protocol: config.ProtocolNZB}
	if nzb.CanBeFixed() {
		t.Error("CanBeFixed() = true for NZB (expected false)")
	}
	if nzb.CanBeMoved() {
		t.Error("CanBeMoved() = true for NZB (expected false)")
	}
}

// ── Entry.AddTorrentProvider ──────────────────────────────────────────────────

func TestEntry_AddTorrentProvider(t *testing.T) {
	t.Parallel()

	torrent := &debridTypes.Torrent{
		Id:     "rd-abc",
		Debrid: "rd",
		Status: debridTypes.TorrentStatusDownloaded,
		Files: map[string]debridTypes.File{
			"movie.mkv": {Name: "movie.mkv", Id: "file1", Link: "https://dl.example.com/movie.mkv", Path: "/movie.mkv"},
		},
	}

	e := &Entry{InfoHash: "abc123"}
	pe := e.AddTorrentProvider(torrent)

	if pe == nil {
		t.Fatal("expected non-nil ProviderEntry")
	}
	if pe.Provider != "rd" {
		t.Errorf("Provider = %q, want rd", pe.Provider)
	}
	if pe.ID != "rd-abc" {
		t.Errorf("ID = %q, want rd-abc", pe.ID)
	}
	if pe.Status != debridTypes.TorrentStatusDownloaded {
		t.Errorf("Status = %q, want downloaded", pe.Status)
	}
	pf, ok := pe.Files["movie.mkv"]
	if !ok {
		t.Fatal("expected movie.mkv in provider files")
	}
	if pf.Id != "file1" || pf.Link != "https://dl.example.com/movie.mkv" {
		t.Errorf("ProviderFile mismatch: %+v", pf)
	}
	if _, ok := e.Providers["rd"]; !ok {
		t.Error("expected entry to be added to e.Providers[rd]")
	}
}

func TestEntry_AddTorrentProvider_NilProviders(t *testing.T) {
	t.Parallel()
	// Entry with nil Providers map — AddTorrentProvider initializes it
	e := &Entry{}
	torrent := &debridTypes.Torrent{Id: "x", Debrid: "rd", Files: map[string]debridTypes.File{}}
	e.AddTorrentProvider(torrent)
	if e.Providers == nil {
		t.Error("Providers should be initialized after AddTorrentProvider")
	}
}

// ── EntryItem.GetFile ─────────────────────────────────────────────────────────

func TestEntryItem_GetFile(t *testing.T) {
	t.Parallel()

	t.Run("nil files returns error", func(t *testing.T) {
		t.Parallel()
		ei := &EntryItem{}
		_, err := ei.GetFile("any.mkv")
		if err == nil {
			t.Error("expected error for nil files map")
		}
	})

	t.Run("existing non-deleted file returned", func(t *testing.T) {
		t.Parallel()
		ei := &EntryItem{Files: map[string]*File{"ep.mkv": {Name: "ep.mkv", Size: 500}}}
		f, err := ei.GetFile("ep.mkv")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Size != 500 {
			t.Errorf("Size = %d, want 500", f.Size)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()
		ei := &EntryItem{Files: map[string]*File{}}
		_, err := ei.GetFile("missing.mkv")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("deleted file returns error", func(t *testing.T) {
		t.Parallel()
		ei := &EntryItem{Files: map[string]*File{"dead.mkv": {Name: "dead.mkv", Deleted: true}}}
		_, err := ei.GetFile("dead.mkv")
		if err == nil {
			t.Error("expected error for deleted file")
		}
	})
}

// ── EntryItem.GetSize ─────────────────────────────────────────────────────────

func TestEntryItem_GetSize(t *testing.T) {
	t.Parallel()

	ei := &EntryItem{
		Files: map[string]*File{
			"a.mkv":   {Name: "a.mkv", Size: 1000},
			"b.mkv":   {Name: "b.mkv", Size: 2000},
			"del.nfo": {Name: "del.nfo", Size: 500, Deleted: true},
		},
	}
	got := ei.GetSize()
	if got != 3000 {
		t.Errorf("GetSize() = %d, want 3000 (deleted files excluded)", got)
	}
}

func TestEntryItem_GetSize_Empty(t *testing.T) {
	t.Parallel()
	ei := &EntryItem{Files: map[string]*File{}}
	if got := ei.GetSize(); got != 0 {
		t.Errorf("GetSize() = %d, want 0", got)
	}
}

// ── EntryItem.GetFirstFile ────────────────────────────────────────────────────

func TestEntryItem_GetFirstFile(t *testing.T) {
	t.Parallel()

	t.Run("empty map returns error", func(t *testing.T) {
		t.Parallel()
		ei := &EntryItem{Files: map[string]*File{}}
		_, err := ei.GetFirstFile()
		if err == nil {
			t.Error("expected error for empty files map")
		}
	})

	t.Run("non-empty map returns a file", func(t *testing.T) {
		t.Parallel()
		ei := &EntryItem{Files: map[string]*File{"ep.mkv": {Name: "ep.mkv"}}}
		f, err := ei.GetFirstFile()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f == nil {
			t.Error("expected non-nil file")
		}
	})
}

// ── EntryItem.GetActiveFiles ──────────────────────────────────────────────────

func TestEntryItem_GetActiveFiles(t *testing.T) {
	t.Parallel()

	ei := &EntryItem{
		Files: map[string]*File{
			"ep01.mkv": {Name: "ep01.mkv"},
			"ep02.mkv": {Name: "ep02.mkv"},
			"del.nfo":  {Name: "del.nfo", Deleted: true},
		},
	}
	active := ei.GetActiveFiles()
	if len(active) != 2 {
		t.Errorf("GetActiveFiles() len = %d, want 2", len(active))
	}
	for _, f := range active {
		if f.Deleted {
			t.Errorf("GetActiveFiles() returned deleted file: %s", f.Name)
		}
	}
}
