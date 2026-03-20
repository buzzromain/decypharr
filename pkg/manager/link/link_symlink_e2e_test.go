package link_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/storage"
)

// createSymlinks reproduces the core symlink creation algorithm from
// Downloader.processSymlink (pkg/manager/downloader.go). It walks sourceDir
// recursively, matches files by name against the entry's active files, and
// creates symlinks under libraryDir/<folder>/.
//
// Returns the list of created symlink paths.
func createSymlinks(entry *storage.Entry, sourceDir, libraryDir string) ([]string, error) {
	files := entry.GetActiveFiles()
	symlinkDir := filepath.Join(libraryDir, entry.Name)

	if err := os.MkdirAll(symlinkDir, os.ModePerm); err != nil {
		return nil, err
	}

	remaining := make(map[string]*storage.File, len(files))
	for _, f := range files {
		remaining[f.Name] = f
	}

	var created []string

	var walk func(string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, item := range entries {
			name := item.Name()
			fullPath := filepath.Join(dir, name)

			if file, ok := remaining[name]; ok {
				linkPath := filepath.Join(symlinkDir, file.Name)

				if err := os.Symlink(fullPath, linkPath); err == nil || os.IsExist(err) {
					if err == nil {
						created = append(created, linkPath)
					}
					delete(remaining, name)
				}
			} else if item.IsDir() {
				walk(fullPath)
			}
		}
	}

	walk(sourceDir)
	return created, nil
}

// setupSourceFiles creates real files inside sourceDir and returns the full
// paths. The files are placed inside sourceDir/<folderName>/ to mirror the
// mount directory layout used in production.
func setupSourceFiles(t *testing.T, sourceDir, folderName string, fileNames []string) []string {
	t.Helper()
	dir := filepath.Join(sourceDir, folderName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(fileNames))
	for _, name := range fileNames {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("content-"+name), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

// newTestEntry creates a minimal storage.Entry suitable for symlink tests.
func newTestEntry(name, infoHash string, fileNames []string) *storage.Entry {
	files := make(map[string]*storage.File, len(fileNames))
	for _, n := range fileNames {
		files[n] = &storage.File{
			Name:     n,
			Size:     1024,
			InfoHash: infoHash,
		}
	}
	return &storage.Entry{
		Name:     name,
		InfoHash: infoHash,
		Files:    files,
	}
}

// ---------- Tests ----------

func TestSymlinkCreation_SingleFile(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	entry := newTestEntry("My.Movie.2024", "abc123", []string{"movie.mkv"})
	setupSourceFiles(t, sourceDir, "My.Movie.2024", []string{"movie.mkv"})

	created, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 symlink, got %d", len(created))
	}

	linkPath := created[0]

	// Symlink exists
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink does not exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("path is not a symlink")
	}

	// Readlink resolves to expected source
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	expectedTarget := filepath.Join(sourceDir, "My.Movie.2024", "movie.mkv")
	if target != expectedTarget {
		t.Errorf("readlink = %q, want %q", target, expectedTarget)
	}

	// EvalSymlinks resolves to existing file
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}
	if resolved != expectedTarget {
		t.Errorf("EvalSymlinks = %q, want %q", resolved, expectedTarget)
	}

	// Target file is readable
	if _, err := os.Stat(resolved); err != nil {
		t.Fatalf("target file not accessible: %v", err)
	}
}

func TestSymlinkCreation_MultipleFiles(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	fileNames := []string{"episode.S01E01.mkv", "episode.S01E02.mkv", "episode.S01E03.mkv"}
	entry := newTestEntry("My.Show.S01", "def456", fileNames)
	setupSourceFiles(t, sourceDir, "My.Show.S01", fileNames)

	created, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 {
		t.Fatalf("expected 3 symlinks, got %d", len(created))
	}

	symlinkDir := filepath.Join(libraryDir, "My.Show.S01")
	for _, name := range fileNames {
		linkPath := filepath.Join(symlinkDir, name)

		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Errorf("symlink %s does not exist: %v", name, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a symlink", name)
			continue
		}

		resolved, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			t.Errorf("EvalSymlinks(%s) failed: %v", name, err)
			continue
		}
		expected := filepath.Join(sourceDir, "My.Show.S01", name)
		if resolved != expected {
			t.Errorf("EvalSymlinks(%s) = %q, want %q", name, resolved, expected)
		}
	}
}

func TestSymlinkCreation_NestedSourceDirectory(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	// Create nested structure: sourceDir/Show/Season 01/episode.mkv
	nestedDir := filepath.Join(sourceDir, "Show", "Season 01")
	if err := os.MkdirAll(nestedDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(nestedDir, "episode.mkv")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := newTestEntry("Show", "nested789", []string{"episode.mkv"})

	created, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 symlink, got %d", len(created))
	}

	resolved, err := filepath.EvalSymlinks(created[0])
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filePath {
		t.Errorf("resolved = %q, want %q", resolved, filePath)
	}
}

func TestSymlinkCreation_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		entryName string
		files     []string
		wantCount int
	}{
		{
			name:      "single video file",
			entryName: "Movie.2024",
			files:     []string{"movie.mkv"},
			wantCount: 1,
		},
		{
			name:      "multiple episode files",
			entryName: "Show.S01",
			files:     []string{"s01e01.mkv", "s01e02.mkv"},
			wantCount: 2,
		},
		{
			name:      "mixed media types",
			entryName: "Media.Pack",
			files:     []string{"video.mkv", "subtitle.srt", "nfo.nfo"},
			wantCount: 3,
		},
		{
			name:      "single file no extension",
			entryName: "NoExt",
			files:     []string{"datafile"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			libraryDir := t.TempDir()

			entry := newTestEntry(tt.entryName, "hash-"+tt.name, tt.files)
			setupSourceFiles(t, sourceDir, tt.entryName, tt.files)

			created, err := createSymlinks(entry, sourceDir, libraryDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(created) != tt.wantCount {
				t.Fatalf("got %d symlinks, want %d", len(created), tt.wantCount)
			}

			for _, linkPath := range created {
				info, err := os.Lstat(linkPath)
				if err != nil {
					t.Errorf("symlink missing: %v", err)
					continue
				}
				if info.Mode()&os.ModeSymlink == 0 {
					t.Errorf("%s is not a symlink", linkPath)
				}
				if _, err := filepath.EvalSymlinks(linkPath); err != nil {
					t.Errorf("EvalSymlinks failed for %s: %v", linkPath, err)
				}
			}
		})
	}
}

// TestSymlinkIdempotency verifies that calling createSymlinks twice does
// not create duplicate symlinks or return errors.
func TestSymlinkIdempotency(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	entry := newTestEntry("Idempotent.Movie", "idem123", []string{"movie.mkv", "subs.srt"})
	setupSourceFiles(t, sourceDir, "Idempotent.Movie", []string{"movie.mkv", "subs.srt"})

	// First call
	created1, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created1) != 2 {
		t.Fatalf("first call: expected 2 symlinks, got %d", len(created1))
	}

	// Second call — should not fail and should not create additional symlinks
	created2, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if len(created2) != 0 {
		t.Errorf("second call created %d new symlinks, expected 0 (idempotent)", len(created2))
	}

	// Verify only the original symlinks exist (no duplicates)
	symlinkDir := filepath.Join(libraryDir, "Idempotent.Movie")
	dirEntries, err := os.ReadDir(symlinkDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirEntries) != 2 {
		t.Errorf("expected 2 entries in symlink dir, got %d", len(dirEntries))
	}

	// Verify each is still a valid symlink
	for _, de := range dirEntries {
		linkPath := filepath.Join(symlinkDir, de.Name())
		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Errorf("Lstat(%s) failed: %v", de.Name(), err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a symlink after second call", de.Name())
		}
		if _, err := filepath.EvalSymlinks(linkPath); err != nil {
			t.Errorf("EvalSymlinks(%s) failed: %v", de.Name(), err)
		}
	}
}

// TestBrokenSymlinkDetection verifies that removing the source file makes the
// symlink broken, and that this is detectable via EvalSymlinks and Stat.
func TestBrokenSymlinkDetection(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	entry := newTestEntry("Broken.Show", "broken456", []string{"episode.mkv"})
	sourcePaths := setupSourceFiles(t, sourceDir, "Broken.Show", []string{"episode.mkv"})

	created, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 symlink, got %d", len(created))
	}

	linkPath := created[0]

	// Symlink works before removing source
	if _, err := filepath.EvalSymlinks(linkPath); err != nil {
		t.Fatalf("symlink should resolve before source removal: %v", err)
	}

	// Remove the source file
	if err := os.Remove(sourcePaths[0]); err != nil {
		t.Fatal(err)
	}

	// Symlink still exists (Lstat does not follow)
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatal("symlink entry should still exist after source removal")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("should still be a symlink")
	}

	// Readlink still returns the original target
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if target != sourcePaths[0] {
		t.Errorf("readlink = %q, want %q", target, sourcePaths[0])
	}

	// EvalSymlinks fails — broken symlink detected
	_, err = filepath.EvalSymlinks(linkPath)
	if err == nil {
		t.Fatal("EvalSymlinks should fail for broken symlink")
	}

	// Stat follows the symlink and should fail
	_, err = os.Stat(linkPath)
	if err == nil {
		t.Fatal("Stat should fail for broken symlink")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error, got: %v", err)
	}
}

// TestDuplicateFileReferences verifies that when multiple entries reference the
// same source file name, only one symlink is created (no overwrites or errors).
func TestDuplicateFileReferences(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	// Two entries referencing the same file name
	entry1 := newTestEntry("Dup.Movie", "dup-aaa", []string{"movie.mkv"})
	entry2 := newTestEntry("Dup.Movie", "dup-bbb", []string{"movie.mkv"})

	setupSourceFiles(t, sourceDir, "Dup.Movie", []string{"movie.mkv"})

	// Create symlinks for first entry
	created1, err := createSymlinks(entry1, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created1) != 1 {
		t.Fatalf("first entry: expected 1 symlink, got %d", len(created1))
	}

	// Create symlinks for second entry (same folder, same file name)
	// The symlink already exists, so this should not create a new one.
	created2, err := createSymlinks(entry2, sourceDir, libraryDir)
	if err != nil {
		t.Fatalf("second entry should not error: %v", err)
	}
	if len(created2) != 0 {
		t.Errorf("second entry created %d symlinks, expected 0", len(created2))
	}

	// Only one symlink should exist
	symlinkDir := filepath.Join(libraryDir, "Dup.Movie")
	dirEntries, err := os.ReadDir(symlinkDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirEntries) != 1 {
		t.Errorf("expected exactly 1 symlink entry, got %d", len(dirEntries))
	}
}

// TestDeletedFilesExcluded verifies that files marked as Deleted in the entry
// are not symlinked.
func TestDeletedFilesExcluded(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	entry := newTestEntry("With.Deleted", "del789", []string{"keep.mkv", "remove.mkv"})
	entry.Files["remove.mkv"].Deleted = true

	setupSourceFiles(t, sourceDir, "With.Deleted", []string{"keep.mkv", "remove.mkv"})

	created, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 symlink (deleted excluded), got %d", len(created))
	}

	symlinkDir := filepath.Join(libraryDir, "With.Deleted")
	dirEntries, err := os.ReadDir(symlinkDir)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(dirEntries))
	for _, de := range dirEntries {
		names = append(names, de.Name())
	}
	sort.Strings(names)
	if len(names) != 1 || names[0] != "keep.mkv" {
		t.Errorf("expected only [keep.mkv], got %v", names)
	}
}

// TestNoSourceFiles verifies that when source files do not exist in the mount
// directory, no symlinks are created and no error occurs.
func TestNoSourceFiles(t *testing.T) {
	sourceDir := t.TempDir() // empty
	libraryDir := t.TempDir()

	entry := newTestEntry("Ghost.Movie", "ghost000", []string{"missing.mkv"})

	created, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 0 {
		t.Errorf("expected 0 symlinks for missing source, got %d", len(created))
	}

	// The library directory should still exist (it was created)
	symlinkDir := filepath.Join(libraryDir, "Ghost.Movie")
	if _, err := os.Stat(symlinkDir); err != nil {
		t.Errorf("symlink directory should exist even with no files: %v", err)
	}
}

// TestSymlinkTargetMatchesOriginalPath confirms that each symlink points
// exactly at the original source path — not a copy or indirect reference.
func TestSymlinkTargetMatchesOriginalPath(t *testing.T) {
	sourceDir := t.TempDir()
	libraryDir := t.TempDir()

	fileNames := []string{"a.mkv", "b.mkv"}
	entry := newTestEntry("Target.Check", "target123", fileNames)
	sourcePaths := setupSourceFiles(t, sourceDir, "Target.Check", fileNames)

	created, err := createSymlinks(entry, sourceDir, libraryDir)
	if err != nil {
		t.Fatal(err)
	}

	// Build expected mapping: filename -> source path
	expectedTargets := make(map[string]string)
	for _, sp := range sourcePaths {
		expectedTargets[filepath.Base(sp)] = sp
	}

	for _, linkPath := range created {
		name := filepath.Base(linkPath)
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Errorf("Readlink(%s) failed: %v", name, err)
			continue
		}
		want, ok := expectedTargets[name]
		if !ok {
			t.Errorf("unexpected symlink for %s", name)
			continue
		}
		if target != want {
			t.Errorf("symlink target for %s = %q, want %q", name, target, want)
		}
	}
}
