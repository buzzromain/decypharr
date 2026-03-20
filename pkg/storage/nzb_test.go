package storage

import (
	"fmt"
	"testing"
)

// ── NZB.GetFileByName ─────────────────────────────────────────────────────────

func TestNZB_GetFileByName(t *testing.T) {
	t.Parallel()

	nzb := &NZB{
		ID: "nzb-1",
		Files: []NZBFile{
			{Name: "movie.mkv", Size: 1000},
			{Name: "movie.nfo", Size: 200},
			{Name: "deleted.rar", Size: 500, IsDeleted: true},
		},
	}

	tests := []struct {
		name     string
		query    string
		wantNil  bool
		wantName string
	}{
		{
			name:     "existing file found",
			query:    "movie.mkv",
			wantNil:  false,
			wantName: "movie.mkv",
		},
		{
			name:     "second file found",
			query:    "movie.nfo",
			wantNil:  false,
			wantName: "movie.nfo",
		},
		{
			name:    "deleted file not returned",
			query:   "deleted.rar",
			wantNil: true,
		},
		{
			name:    "non-existent file returns nil",
			query:   "missing.mkv",
			wantNil: true,
		},
		{
			name:    "empty name returns nil",
			query:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := nzb.GetFileByName(tt.query)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetFileByName(%q) = %+v, want nil", tt.query, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("GetFileByName(%q) = nil, want non-nil", tt.query)
			}
			if got.Name != tt.wantName {
				t.Errorf("GetFileByName(%q).Name = %q, want %q", tt.query, got.Name, tt.wantName)
			}
		})
	}
}

func TestNZB_GetFileByName_ReturnsPointerToSliceElement(t *testing.T) {
	t.Parallel()
	// Verify the returned pointer points into the NZB.Files slice (not a copy)
	nzb := &NZB{
		Files: []NZBFile{{Name: "ep.mkv", Size: 100}},
	}
	got := nzb.GetFileByName("ep.mkv")
	if got == nil {
		t.Fatal("expected non-nil")
	}
	// Mutate through pointer and verify the slice element changed
	got.Size = 999
	if nzb.Files[0].Size != 999 {
		t.Error("GetFileByName did not return pointer into slice")
	}
}

func TestNZB_GetFileByName_EmptyFiles(t *testing.T) {
	t.Parallel()
	nzb := &NZB{ID: "empty"}
	if got := nzb.GetFileByName("any.mkv"); got != nil {
		t.Errorf("expected nil for empty files, got %+v", got)
	}
}

// ── NZB.MarkFileAsRemoved ─────────────────────────────────────────────────────

func TestNZB_MarkFileAsRemoved(t *testing.T) {
	t.Parallel()

	t.Run("marks existing file as deleted", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{
			ID:    "nzb-1",
			Files: []NZBFile{{Name: "movie.mkv"}, {Name: "movie.nfo"}},
		}
		if err := nzb.MarkFileAsRemoved("movie.mkv"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !nzb.Files[0].IsDeleted {
			t.Error("expected Files[0].IsDeleted = true")
		}
		// Other file should be untouched
		if nzb.Files[1].IsDeleted {
			t.Error("expected Files[1].IsDeleted = false (untouched)")
		}
	})

	t.Run("marks second file when first exists", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{
			ID:    "nzb-2",
			Files: []NZBFile{{Name: "ep01.mkv"}, {Name: "ep02.mkv"}},
		}
		if err := nzb.MarkFileAsRemoved("ep02.mkv"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if nzb.Files[0].IsDeleted {
			t.Error("ep01.mkv should not be deleted")
		}
		if !nzb.Files[1].IsDeleted {
			t.Error("ep02.mkv should be deleted")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{
			ID:    "nzb-3",
			Files: []NZBFile{{Name: "movie.mkv"}},
		}
		err := nzb.MarkFileAsRemoved("nonexistent.rar")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("returns error for empty NZB", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{ID: "nzb-4"}
		err := nzb.MarkFileAsRemoved("any.mkv")
		if err == nil {
			t.Error("expected error for empty files list")
		}
	})

	t.Run("can mark already-deleted file", func(t *testing.T) {
		t.Parallel()
		// MarkFileAsRemoved matches by name, not by IsDeleted state
		nzb := &NZB{
			ID:    "nzb-5",
			Files: []NZBFile{{Name: "already.rar", IsDeleted: true}},
		}
		if err := nzb.MarkFileAsRemoved("already.rar"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !nzb.Files[0].IsDeleted {
			t.Error("file should still be deleted")
		}
	})
}

// ── NZBFile.GetCacheKey ───────────────────────────────────────────────────────

func TestNZBFile_GetCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fileName string
		size     int64
		want     string
	}{
		{
			name:     "standard mkv file",
			fileName: "movie.mkv",
			size:     1073741824,
			want:     "rar_movie.mkv_1073741824",
		},
		{
			name:     "rar archive",
			fileName: "release.rar",
			size:     500,
			want:     "rar_release.rar_500",
		},
		{
			name:     "zero size",
			fileName: "empty.nfo",
			size:     0,
			want:     "rar_empty.nfo_0",
		},
		{
			name:     "empty name",
			fileName: "",
			size:     100,
			want:     "rar__100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			nf := &NZBFile{Name: tt.fileName, Size: tt.size}
			got := nf.GetCacheKey()
			if got != tt.want {
				t.Errorf("GetCacheKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNZBFile_GetCacheKey_Format(t *testing.T) {
	t.Parallel()
	nf := &NZBFile{Name: "test.rar", Size: 42}
	want := fmt.Sprintf("rar_%s_%d", nf.Name, nf.Size)
	got := nf.GetCacheKey()
	if got != want {
		t.Errorf("GetCacheKey() = %q, want %q", got, want)
	}
}

// ── NZB.GetFiles ──────────────────────────────────────────────────────────────

func TestNZB_GetFiles(t *testing.T) {
	t.Parallel()

	t.Run("returns only non-deleted files", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{
			Files: []NZBFile{
				{Name: "ep01.mkv"},
				{Name: "ep02.mkv", IsDeleted: true},
				{Name: "ep03.mkv"},
			},
		}
		got := nzb.GetFiles()
		if len(got) != 2 {
			t.Fatalf("GetFiles() len = %d, want 2", len(got))
		}
		for _, f := range got {
			if f.IsDeleted {
				t.Errorf("GetFiles() returned deleted file: %s", f.Name)
			}
		}
	})

	t.Run("all deleted returns empty slice", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{
			Files: []NZBFile{
				{Name: "a.rar", IsDeleted: true},
				{Name: "b.rar", IsDeleted: true},
			},
		}
		got := nzb.GetFiles()
		if len(got) != 0 {
			t.Errorf("GetFiles() = %d entries, want 0", len(got))
		}
	})

	t.Run("no deleted returns all files", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{
			Files: []NZBFile{
				{Name: "a.mkv"},
				{Name: "b.mkv"},
				{Name: "c.mkv"},
			},
		}
		got := nzb.GetFiles()
		if len(got) != 3 {
			t.Errorf("GetFiles() len = %d, want 3", len(got))
		}
	})

	t.Run("empty NZB returns empty slice", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{}
		got := nzb.GetFiles()
		if len(got) != 0 {
			t.Errorf("GetFiles() len = %d, want 0", len(got))
		}
	})

	t.Run("returns copies not references to original slice", func(t *testing.T) {
		t.Parallel()
		nzb := &NZB{
			Files: []NZBFile{{Name: "a.mkv", Size: 100}},
		}
		got := nzb.GetFiles()
		if len(got) != 1 {
			t.Fatal("expected 1 file")
		}
		// Mutating the returned copy should not affect the original
		got[0].Size = 999
		if nzb.Files[0].Size == 999 {
			t.Error("GetFiles() returned slice shares backing array with original")
		}
	})
}

func TestNZB_GetFiles_AfterMarkAsRemoved(t *testing.T) {
	t.Parallel()
	nzb := &NZB{
		ID: "combo",
		Files: []NZBFile{
			{Name: "movie.mkv"},
			{Name: "movie.nfo"},
		},
	}
	if err := nzb.MarkFileAsRemoved("movie.nfo"); err != nil {
		t.Fatalf("MarkFileAsRemoved: %v", err)
	}
	got := nzb.GetFiles()
	if len(got) != 1 {
		t.Fatalf("GetFiles() after remove: got %d, want 1", len(got))
	}
	if got[0].Name != "movie.mkv" {
		t.Errorf("unexpected file: %s", got[0].Name)
	}
}
