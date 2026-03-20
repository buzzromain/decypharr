package storage

import (
	"os"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/testutil"
)

func TestMain(m *testing.M) {
	_, cleanup := testutil.SetupTestConfigDir()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// ── mergeTags ─────────────────────────────────────────────────────────────────

func TestMergeTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		existing []string
		incoming []string
		wantLen  int
	}{
		{"both nil", nil, nil, 0},
		{"existing nil", nil, []string{"a", "b"}, 2},
		{"incoming nil", []string{"a", "b"}, nil, 2},
		{"no overlap", []string{"a", "b"}, []string{"c", "d"}, 4},
		{"full overlap", []string{"a", "b"}, []string{"a", "b"}, 2},
		{"partial overlap", []string{"a", "b"}, []string{"b", "c"}, 3},
		{"empty slices", []string{}, []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeTags(tt.existing, tt.incoming)
			if len(got) != tt.wantLen {
				t.Errorf("mergeTags() len = %d, want %d (got: %v)", len(got), tt.wantLen, got)
			}
			// Verify no duplicates
			seen := make(map[string]bool)
			for _, tag := range got {
				if seen[tag] {
					t.Errorf("mergeTags() returned duplicate tag: %s", tag)
				}
				seen[tag] = true
			}
		})
	}
}

// ── mergeFiles ────────────────────────────────────────────────────────────────

func TestMergeFiles(t *testing.T) {
	t.Parallel()
	now := time.Now()
	older := now.Add(-time.Hour)

	tests := []struct {
		name     string
		existing map[string]*File
		incoming map[string]*File
		wantLen  int
		check    func(t *testing.T, result map[string]*File)
	}{
		{
			name:     "existing nil",
			existing: nil,
			incoming: map[string]*File{"a.mkv": {Name: "a.mkv"}},
			wantLen:  1,
		},
		{
			name:     "incoming nil",
			existing: map[string]*File{"a.mkv": {Name: "a.mkv"}},
			incoming: nil,
			wantLen:  1,
		},
		{
			name: "no overlap",
			existing: map[string]*File{
				"a.mkv": {Name: "a.mkv", AddedOn: older},
			},
			incoming: map[string]*File{
				"b.mkv": {Name: "b.mkv", AddedOn: now},
			},
			wantLen: 2,
		},
		{
			name: "incoming newer wins",
			existing: map[string]*File{
				"a.mkv": {Name: "a.mkv", Size: 100, AddedOn: older},
			},
			incoming: map[string]*File{
				"a.mkv": {Name: "a.mkv", Size: 200, AddedOn: now},
			},
			wantLen: 1,
			check: func(t *testing.T, result map[string]*File) {
				if result["a.mkv"].Size != 200 {
					t.Errorf("expected incoming (newer) file to win, got size %d", result["a.mkv"].Size)
				}
			},
		},
		{
			name: "existing newer wins",
			existing: map[string]*File{
				"a.mkv": {Name: "a.mkv", Size: 100, AddedOn: now},
			},
			incoming: map[string]*File{
				"a.mkv": {Name: "a.mkv", Size: 200, AddedOn: older},
			},
			wantLen: 1,
			check: func(t *testing.T, result map[string]*File) {
				if result["a.mkv"].Size != 100 {
					t.Errorf("expected existing (newer) file to win, got size %d", result["a.mkv"].Size)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeFiles(tt.existing, tt.incoming)
			if len(got) != tt.wantLen {
				t.Errorf("mergeFiles() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// ── mergeProviders ────────────────────────────────────────────────────────────

func TestMergeProviders(t *testing.T) {
	t.Parallel()
	now := time.Now()
	older := now.Add(-time.Hour)

	tests := []struct {
		name     string
		existing map[string]*ProviderEntry
		incoming map[string]*ProviderEntry
		wantLen  int
		check    func(t *testing.T, result map[string]*ProviderEntry)
	}{
		{
			name:     "existing nil",
			existing: nil,
			incoming: map[string]*ProviderEntry{"rd": {Provider: "rd"}},
			wantLen:  1,
		},
		{
			name:     "incoming nil",
			existing: map[string]*ProviderEntry{"rd": {Provider: "rd"}},
			incoming: nil,
			wantLen:  1,
		},
		{
			name: "no overlap",
			existing: map[string]*ProviderEntry{
				"rd": {Provider: "rd", AddedAt: older},
			},
			incoming: map[string]*ProviderEntry{
				"tb": {Provider: "tb", AddedAt: now},
			},
			wantLen: 2,
		},
		{
			name: "incoming newer wins",
			existing: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "old-id", AddedAt: older},
			},
			incoming: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "new-id", AddedAt: now},
			},
			wantLen: 1,
			check: func(t *testing.T, result map[string]*ProviderEntry) {
				if result["rd"].ID != "new-id" {
					t.Errorf("expected newer provider entry to win, got ID=%s", result["rd"].ID)
				}
			},
		},
		{
			name: "existing newer wins",
			existing: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "existing-id", AddedAt: now},
			},
			incoming: map[string]*ProviderEntry{
				"rd": {Provider: "rd", ID: "incoming-id", AddedAt: older},
			},
			wantLen: 1,
			check: func(t *testing.T, result map[string]*ProviderEntry) {
				if result["rd"].ID != "existing-id" {
					t.Errorf("expected existing (newer) provider entry to win, got ID=%s", result["rd"].ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeProviders(tt.existing, tt.incoming)
			if len(got) != tt.wantLen {
				t.Errorf("mergeProviders() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// ── selectActivePlacement ─────────────────────────────────────────────────────

func TestSelectActivePlacement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		existing *Entry
		incoming *Entry
		want     string
	}{
		{
			name:     "incoming has active provider",
			existing: &Entry{ActiveProvider: "existing"},
			incoming: &Entry{ActiveProvider: "incoming"},
			want:     "incoming",
		},
		{
			name:     "incoming empty, fall back to existing",
			existing: &Entry{ActiveProvider: "existing"},
			incoming: &Entry{ActiveProvider: ""},
			want:     "existing",
		},
		{
			name:     "both empty",
			existing: &Entry{ActiveProvider: ""},
			incoming: &Entry{ActiveProvider: ""},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := selectActivePlacement(tt.existing, tt.incoming)
			if got != tt.want {
				t.Errorf("selectActivePlacement() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ── HandleExistingEntryMerge ──────────────────────────────────────────────────

func TestHandleExistingEntryMerge_NZBReturnsIncoming(t *testing.T) {
	t.Parallel()
	existing := &Entry{
		InfoHash:       "existing",
		ActiveProvider: "rd",
		Tags:           []string{"existing-tag"},
	}
	incoming := &Entry{
		Protocol:       config.ProtocolNZB,
		InfoHash:       "incoming",
		ActiveProvider: "usenet",
	}

	result := HandleExistingEntryMerge(existing, incoming)
	if result != incoming {
		t.Error("NZB entry should return incoming as-is without merging")
	}
}

func TestHandleExistingEntryMerge_TorrentMerges(t *testing.T) {
	t.Parallel()
	now := time.Now()
	older := now.Add(-time.Hour)

	existing := &Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       "abc",
		ActiveProvider: "rd",
		Tags:           []string{"tag1"},
		Files: map[string]*File{
			"ep01.mkv": {Name: "ep01.mkv", Size: 1000, AddedOn: older},
		},
		Providers: map[string]*ProviderEntry{
			"rd": {Provider: "rd", ID: "rd-id", AddedAt: older},
		},
	}
	incoming := &Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       "abc",
		ActiveProvider: "",
		Tags:           []string{"tag2"},
		Files: map[string]*File{
			"ep02.mkv": {Name: "ep02.mkv", Size: 2000, AddedOn: now},
		},
		Providers: map[string]*ProviderEntry{
			"tb": {Provider: "tb", ID: "tb-id", AddedAt: now},
		},
	}

	result := HandleExistingEntryMerge(existing, incoming)

	// Files should be merged
	if _, ok := result.Files["ep01.mkv"]; !ok {
		t.Error("expected ep01.mkv from existing to be in result")
	}
	if _, ok := result.Files["ep02.mkv"]; !ok {
		t.Error("expected ep02.mkv from incoming to be in result")
	}

	// Active provider: incoming empty → fall back to existing
	if result.ActiveProvider != "rd" {
		t.Errorf("expected ActiveProvider=rd (from existing), got %q", result.ActiveProvider)
	}

	// Providers merged
	if _, ok := result.Providers["rd"]; !ok {
		t.Error("expected rd provider from existing")
	}
	if _, ok := result.Providers["tb"]; !ok {
		t.Error("expected tb provider from incoming")
	}

	// Tags merged (no duplicates)
	tagSet := make(map[string]bool)
	for _, tag := range result.Tags {
		tagSet[tag] = true
	}
	if !tagSet["tag1"] {
		t.Error("expected tag1 from existing in result tags")
	}
	if !tagSet["tag2"] {
		t.Error("expected tag2 from incoming in result tags")
	}
}
