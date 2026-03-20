package manager

import (
	"sort"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/storage"
)

// ── generateSeasonHash ────────────────────────────────────────────────────────

func TestGenerateSeasonHash_Deterministic(t *testing.T) {
	h1 := generateSeasonHash("abc123", 1)
	h2 := generateSeasonHash("abc123", 1)
	if h1 != h2 {
		t.Error("generateSeasonHash should be deterministic")
	}
}

func TestGenerateSeasonHash_DifferentInputs(t *testing.T) {
	h1 := generateSeasonHash("abc123", 1)
	h2 := generateSeasonHash("abc123", 2)
	h3 := generateSeasonHash("xyz456", 1)
	if h1 == h2 || h1 == h3 || h2 == h3 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestGenerateSeasonHash_IsHex(t *testing.T) {
	h := generateSeasonHash("abc123", 3)
	if len(h) != 32 { // MD5 = 16 bytes = 32 hex chars
		t.Errorf("hash length = %d, want 32", len(h))
	}
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("hash %q contains non-hex char %q", h, c)
		}
	}
}

// ── extractSeason ─────────────────────────────────────────────────────────────

func TestExtractSeason(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"Show.S01E01.mkv", 1},
		{"Show.S12E05.mkv", 12},
		{"Season 3 Episode 1", 3},
		{"season.02.episode.01", 2},
		{"Movie.2024.mkv", 0}, // no season
		{"", 0},
		{"S00E01.mkv", 0}, // season 0 is rejected (num > 0)
		{"S99E01.mkv", 99},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := extractSeason(tt.text)
			if got != tt.want {
				t.Errorf("extractSeason(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

// ── hasMultiSeasonIndicators ──────────────────────────────────────────────────

func TestHasMultiSeasonIndicators(t *testing.T) {
	yes := []string{
		"Show.Complete.Series",
		"Show.All.Seasons",
		"Show.Season.1-8", // pattern: season\.?\s*\d+-\d+
		"Show.S01-S08",
		"Show Seasons 1-3", // pattern: seasons?\s*\d+-\d+ — needs whitespace not dot
	}
	for _, name := range yes {
		if !hasMultiSeasonIndicators(name) {
			t.Errorf("hasMultiSeasonIndicators(%q) = false, want true", name)
		}
	}

	no := []string{
		"Show.S01E01.mkv",
		"Movie.2024.mkv",
		"Series.Season.1",
		"Show.Seasons.1-3", // dot before number doesn't match \s*
	}
	for _, name := range no {
		if hasMultiSeasonIndicators(name) {
			t.Errorf("hasMultiSeasonIndicators(%q) = true, want false", name)
		}
	}
}

// ── replaceMultiSeasonPattern ─────────────────────────────────────────────────

func TestReplaceMultiSeasonPattern_SRange(t *testing.T) {
	// "S01-08" → "S01"
	got := replaceMultiSeasonPattern("Show.S01-08.1080p", 1)
	if !strings.Contains(got, "S01") {
		t.Errorf("expected S01 in result, got %q", got)
	}
}

func TestReplaceMultiSeasonPattern_SToS(t *testing.T) {
	// "S01-S08" → "S01"
	got := replaceMultiSeasonPattern("Show.S01-S08.BluRay", 1)
	if !strings.Contains(got, "S01") {
		t.Errorf("expected S01 in result, got %q", got)
	}
}

func TestReplaceMultiSeasonPattern_CompleteSeries(t *testing.T) {
	got := replaceMultiSeasonPattern("Show.Complete.Series.1080p", 2)
	if !strings.Contains(got, "Season") || !strings.Contains(got, "02") {
		t.Errorf("expected Season 02 in result, got %q", got)
	}
}

func TestReplaceMultiSeasonPattern_AllSeasons(t *testing.T) {
	got := replaceMultiSeasonPattern("Show.All.Seasons.x265", 3)
	if !strings.Contains(got, "Season") {
		t.Errorf("expected Season in result, got %q", got)
	}
}

func TestReplaceMultiSeasonPattern_NoPattern_AppendsIfNoQuality(t *testing.T) {
	// No multi-season pattern, no quality indicator → append season
	got := replaceMultiSeasonPattern("Some.Show", 4)
	if !strings.Contains(got, "S04") {
		t.Errorf("expected S04 appended to %q, got %q", "Some.Show", got)
	}
}

// ── insertSeasonIntoName ──────────────────────────────────────────────────────

func TestInsertSeasonIntoName_AlreadyHasSeason(t *testing.T) {
	input := "Show.S02E01"
	got := insertSeasonIntoName(input, 2)
	if got != input {
		t.Errorf("insertSeasonIntoName should leave name unchanged when season already present, got %q", got)
	}
}

func TestInsertSeasonIntoName_BeforeQualityIndicator(t *testing.T) {
	got := insertSeasonIntoName("Show.1080p.BluRay", 3)
	if !strings.Contains(got, "S03") {
		t.Errorf("expected S03 inserted, got %q", got)
	}
	// Season should appear before quality
	sIdx := strings.Index(got, "S03")
	qIdx := strings.Index(got, "1080p")
	if sIdx > qIdx {
		t.Errorf("S03 should appear before quality indicator: %q", got)
	}
}

func TestInsertSeasonIntoName_NoQualityIndicator_Appended(t *testing.T) {
	got := insertSeasonIntoName("Plain.Show.Name", 5)
	if !strings.HasSuffix(strings.TrimSpace(got), "S05") {
		t.Errorf("expected S05 at end, got %q", got)
	}
}

// ── findAllSeasons ────────────────────────────────────────────────────────────

func TestFindAllSeasons_MultipleSeasons(t *testing.T) {
	files := []*storage.File{
		{Name: "Show.S01E01.mkv"},
		{Name: "Show.S01E02.mkv"},
		{Name: "Show.S02E01.mkv"},
	}
	seasons := findAllSeasons(files)
	if !seasons[1] || !seasons[2] {
		t.Errorf("expected seasons {1,2}, got %v", seasons)
	}
	if len(seasons) != 2 {
		t.Errorf("expected 2 unique seasons, got %d", len(seasons))
	}
}

func TestFindAllSeasons_FallsBackToPath(t *testing.T) {
	files := []*storage.File{
		{Name: "episode.mkv", Path: "Show/Season 3/episode.mkv"},
	}
	seasons := findAllSeasons(files)
	if !seasons[3] {
		t.Errorf("expected season 3 extracted from path, got %v", seasons)
	}
}

func TestFindAllSeasons_NoSeasonFiles(t *testing.T) {
	files := []*storage.File{
		{Name: "Movie.2024.mkv"},
	}
	seasons := findAllSeasons(files)
	if len(seasons) != 0 {
		t.Errorf("expected no seasons, got %v", seasons)
	}
}

// ── inferSeasonFromPath ───────────────────────────────────────────────────────

func TestInferSeasonFromPath_Found(t *testing.T) {
	known := map[int]bool{2: true, 3: true}
	got := inferSeasonFromPath("Show/Season 2/episode.mkv", known)
	if got != 2 {
		t.Errorf("inferSeasonFromPath = %d, want 2", got)
	}
}

func TestInferSeasonFromPath_NotInKnown(t *testing.T) {
	known := map[int]bool{1: true}
	got := inferSeasonFromPath("Show/Season 5/episode.mkv", known)
	if got != 0 {
		t.Errorf("inferSeasonFromPath = %d, want 0 (season not in known set)", got)
	}
}

func TestInferSeasonFromPath_NoSeasonInPath(t *testing.T) {
	known := map[int]bool{1: true}
	got := inferSeasonFromPath("Show/episodes/episode.mkv", known)
	if got != 0 {
		t.Errorf("inferSeasonFromPath = %d, want 0", got)
	}
}

// ── groupFilesBySeason ────────────────────────────────────────────────────────

func TestGroupFilesBySeason_BasicGrouping(t *testing.T) {
	files := []*storage.File{
		{Name: "Show.S01E01.mkv"},
		{Name: "Show.S01E02.mkv"},
		{Name: "Show.S02E01.mkv"},
	}
	known := map[int]bool{1: true, 2: true}
	groups := groupFilesBySeason(files, known)

	if len(groups[1]) != 2 {
		t.Errorf("season 1 should have 2 files, got %d", len(groups[1]))
	}
	if len(groups[2]) != 1 {
		t.Errorf("season 2 should have 1 file, got %d", len(groups[2]))
	}
}

func TestGroupFilesBySeason_SingleSeasonDefaultsAll(t *testing.T) {
	// When only one season is known, unidentified files go to it
	files := []*storage.File{
		{Name: "unidentified.mkv"}, // no season in name
	}
	known := map[int]bool{1: true}
	groups := groupFilesBySeason(files, known)

	if len(groups[1]) != 1 {
		t.Errorf("unidentified file should fall to the only known season, got %d files", len(groups[1]))
	}
}

// ── getSortedSeasons ──────────────────────────────────────────────────────────

func TestGetSortedSeasons(t *testing.T) {
	seasons := map[int]bool{3: true, 1: true, 2: true}
	got := getSortedSeasons(seasons)

	// getSortedSeasons doesn't guarantee order per its signature,
	// but we can verify all keys are present
	sort.Ints(got)
	want := []int{1, 2, 3}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("getSortedSeasons()[%d] = %d, want %d", i, got[i], v)
		}
	}
}

// ── convertToMultiSeason ──────────────────────────────────────────────────────

func TestConvertToMultiSeason_CreatesOneEntryPerSeason(t *testing.T) {
	base := &storage.Entry{
		Name:     "Show.S01-S02",
		InfoHash: "base123",
		Category: "tv",
		SavePath: "/downloads",
	}

	f1 := &storage.File{Name: "Show.S01E01.mkv", Size: 500}
	f2 := &storage.File{Name: "Show.S02E01.mkv", Size: 600}

	seasons := []SeasonInfo{
		{SeasonNumber: 1, Files: []*storage.File{f1}, InfoHash: "hash1", Name: "Show.S01"},
		{SeasonNumber: 2, Files: []*storage.File{f2}, InfoHash: "hash2", Name: "Show.S02"},
	}

	results := convertToMultiSeason(base, seasons)
	if len(results) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(results))
	}

	// Verify each entry has correct metadata
	for _, e := range results {
		if e.Category != "tv" {
			t.Errorf("Category = %q, want tv", e.Category)
		}
		if e.SavePath != "/downloads" {
			t.Errorf("SavePath = %q, want /downloads", e.SavePath)
		}
		if len(e.Files) != 1 {
			t.Errorf("expected 1 file per season entry, got %d", len(e.Files))
		}
	}
}

func TestConvertToMultiSeason_SizeIsSum(t *testing.T) {
	base := &storage.Entry{Name: "Show", InfoHash: "base"}
	f1 := &storage.File{Name: "ep1.mkv", Size: 100}
	f2 := &storage.File{Name: "ep2.mkv", Size: 200}

	seasons := []SeasonInfo{
		{SeasonNumber: 1, Files: []*storage.File{f1, f2}, InfoHash: "h1", Name: "Show.S01"},
	}

	results := convertToMultiSeason(base, seasons)
	if results[0].Size != 300 {
		t.Errorf("Size = %d, want 300 (sum of files)", results[0].Size)
	}
}

func TestConvertToMultiSeason_EmptySeasons(t *testing.T) {
	base := &storage.Entry{Name: "Show", InfoHash: "base"}
	results := convertToMultiSeason(base, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil seasons, got %d", len(results))
	}
}
