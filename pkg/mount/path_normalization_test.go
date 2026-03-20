package mount_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 1. Redundant separator removal
//    filepath.Clean and filepath.Join collapse consecutive separators.
// ---------------------------------------------------------------------------

func TestRedundantSeparatorRemoval(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"double slash", "/media//movies/file.mkv", "/media/movies/file.mkv"},
		{"triple slash", "/media//movies///file.mkv", "/media/movies/file.mkv"},
		{"many slashes", "////media////movies////file.mkv", "/media/movies/file.mkv"},
		{"trailing slashes", "/media/movies/file.mkv///", "/media/movies/file.mkv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filepath.Clean(tt.in)
			if got != tt.want {
				t.Errorf("filepath.Clean(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestJoinNormalizesRedundantSeparators(t *testing.T) {
	got := filepath.Join("/media/", "/movies/", "//file.mkv")
	want := "/media/movies/file.mkv"
	if got != want {
		t.Errorf("filepath.Join result = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// 2. Current-directory (.) removal
// ---------------------------------------------------------------------------

func TestCurrentDirectoryRemoval(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"single dot", "/media/./movies/file.mkv", "/media/movies/file.mkv"},
		{"multiple dots", "/media/./movies/./file.mkv", "/media/movies/file.mkv"},
		{"leading dot", "/./media/movies/file.mkv", "/media/movies/file.mkv"},
		{"trailing dot", "/media/movies/./", "/media/movies"},
		{"just dot", ".", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filepath.Clean(tt.in)
			if got != tt.want {
				t.Errorf("filepath.Clean(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Path traversal protection
//    safeJoinUnderBase (pkg/mount/dfs/vfs/cache.go) uses filepath.Rel to
//    detect escapes. Here we verify the same pattern: after cleaning, a path
//    must remain under its base directory.
// ---------------------------------------------------------------------------

// safeJoinUnderBase mirrors the production implementation so the test is
// self-contained while exercising the exact same algorithm.
func safeJoinUnderBase(base string, parts ...string) (string, error) {
	if base == "" {
		return "", errEmptyBase
	}
	cleanBase := filepath.Clean(base)
	cur := cleanBase
	for _, part := range parts {
		if part == "" {
			continue
		}
		if filepath.IsAbs(part) {
			return "", &pathError{"absolute path component is not allowed: " + part}
		}
		cur = filepath.Join(cur, part)
	}

	rel, err := filepath.Rel(cleanBase, cur)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", &pathError{"path escapes base directory"}
	}
	return cur, nil
}

type pathError struct{ msg string }

func (e *pathError) Error() string { return e.msg }

var errEmptyBase = &pathError{"base path is empty"}

func TestTraversalProtection(t *testing.T) {
	base := "/media/movies"

	type rejCase struct {
		name  string
		parts []string
	}
	rejected := []rejCase{
		{"simple dotdot", []string{"../../etc/passwd"}},
		{"dotdot in middle", []string{"subdir", "../../etc/passwd"}},
		{"absolute injection", []string{"/etc/passwd"}},
		{"encoded dotdot", []string{"..", "..", "etc", "passwd"}},
		{"deep escape", []string{"a", "b", "c", "../../../..", "etc", "passwd"}},
	}
	for _, tt := range rejected {
		t.Run("reject/"+tt.name, func(t *testing.T) {
			_, err := safeJoinUnderBase(base, tt.parts...)
			if err == nil {
				t.Fatalf("expected traversal to be rejected for parts %v", tt.parts)
			}
		})
	}

	type accCase struct {
		name  string
		parts []string
		want  string
	}
	accepted := []accCase{
		{"simple file", []string{"file.mkv"}, "/media/movies/file.mkv"},
		{"nested file", []string{"sub", "file.mkv"}, "/media/movies/sub/file.mkv"},
		{"dot in name", []string{"my.movie", "file.mkv"}, "/media/movies/my.movie/file.mkv"},
		{"cancel out within bounds", []string{"sub", "..", "other", "file.mkv"}, "/media/movies/other/file.mkv"},
	}
	for _, tt := range accepted {
		t.Run("accept/"+tt.name, func(t *testing.T) {
			got, err := safeJoinUnderBase(base, tt.parts...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("safeJoinUnderBase(%q, %v) = %q, want %q", base, tt.parts, got, tt.want)
			}
		})
	}
}

func TestTraversalProtection_EmptyBase(t *testing.T) {
	_, err := safeJoinUnderBase("", "file.mkv")
	if err == nil {
		t.Fatal("expected error for empty base")
	}
}

// ---------------------------------------------------------------------------
// 4. Unicode paths
//    Verify that filepath operations preserve unicode and that real filesystem
//    operations succeed with unicode names.
// ---------------------------------------------------------------------------

func TestUnicodePaths(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"french accents", "/media/movies/épisode 01.mkv", "/media/movies/épisode 01.mkv"},
		{"japanese", "/media/映画/ファイル.mkv", "/media/映画/ファイル.mkv"},
		{"emoji", "/media/🎬/movie.mkv", "/media/🎬/movie.mkv"},
		{"mixed scripts", "/media/movies/Ñoño_日本語.mkv", "/media/movies/Ñoño_日本語.mkv"},
		{"unicode with dot removal", "/media/./映画/./file.mkv", "/media/映画/file.mkv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filepath.Clean(tt.in)
			if got != tt.want {
				t.Errorf("filepath.Clean(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUnicodePaths_Filesystem(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"épisode 01.mkv",
		"映画.mkv",
		"🎬.mkv",
		"café résumé.txt",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatalf("failed to create unicode file %q: %v", name, err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("failed to stat unicode file %q: %v", name, err)
			}
		})
	}
}

func TestUnicodePaths_TraversalProtection(t *testing.T) {
	base := "/media/映画"
	_, err := safeJoinUnderBase(base, "../../etc/passwd")
	if err == nil {
		t.Fatal("traversal with unicode base should still be rejected")
	}

	got, err := safeJoinUnderBase(base, "épisode 01.mkv")
	if err != nil {
		t.Fatalf("unicode filename under unicode base should succeed: %v", err)
	}
	want := "/media/映画/épisode 01.mkv"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// 5. Mixed path separators
//    On Unix, backslashes are valid filename chars. filepath.Clean only
//    normalizes the OS separator. We verify platform-correct behavior.
// ---------------------------------------------------------------------------

func TestMixedPathSeparators(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Run("windows normalizes backslash", func(t *testing.T) {
			got := filepath.Clean(`C:\media\movies\file.mkv`)
			want := `C:\media\movies\file.mkv`
			if got != want {
				t.Errorf("filepath.Clean = %q, want %q", got, want)
			}
		})
		t.Run("windows mixed separators", func(t *testing.T) {
			got := filepath.Clean(`C:\media/movies\file.mkv`)
			want := `C:\media\movies\file.mkv`
			if got != want {
				t.Errorf("filepath.Clean = %q, want %q", got, want)
			}
		})
	} else {
		t.Run("unix preserves backslash in names", func(t *testing.T) {
			// On Unix, backslash is a valid filename character, not a separator.
			input := `/media/movies/file\.mkv`
			got := filepath.Clean(input)
			if got != input {
				t.Errorf("filepath.Clean(%q) = %q, want input unchanged", input, got)
			}
		})
	}
}

func TestFromSlash_NormalizesPortably(t *testing.T) {
	// filepath.FromSlash converts forward slashes to OS separator.
	input := "media/movies/file.mkv"
	got := filepath.FromSlash(input)
	want := strings.ReplaceAll(input, "/", string(os.PathSeparator))
	if got != want {
		t.Errorf("filepath.FromSlash(%q) = %q, want %q", input, got, want)
	}
}

func TestToSlash_NormalizesPortably(t *testing.T) {
	input := filepath.Join("media", "movies", "file.mkv")
	got := filepath.ToSlash(input)
	want := "media/movies/file.mkv"
	if got != want {
		t.Errorf("filepath.ToSlash(%q) = %q, want %q", input, got, want)
	}
}

// ---------------------------------------------------------------------------
// 6. Canonical path verification
//    After Clean, the result must equal itself when cleaned again (idempotent)
//    and must never contain "//" or "/./".
// ---------------------------------------------------------------------------

func TestCanonicalPathIdempotence(t *testing.T) {
	paths := []string{
		"/media//movies///file.mkv",
		"/media/./movies/file.mkv",
		"/media/movies/../movies/file.mkv",
		"///media///",
		"/media/movies/épisode 01.mkv",
		"/media/映画/ファイル.mkv",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			first := filepath.Clean(p)
			second := filepath.Clean(first)
			if first != second {
				t.Errorf("not idempotent: Clean(%q)=%q, Clean again=%q", p, first, second)
			}
			if strings.Contains(first, "//") {
				t.Errorf("canonical path %q still contains //", first)
			}
			if strings.Contains(first, "/./") {
				t.Errorf("canonical path %q still contains /./", first)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 7. splitPath behavior (mirrors cgofuse splitPath logic)
//    Ensures the FUSE-layer path splitting handles edge cases.
// ---------------------------------------------------------------------------

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func TestSplitPathEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantNil  bool
		wantLen  int
		wantHas  string // substring that must appear in some component
	}{
		{"root", "/", true, 0, ""},
		{"empty", "", true, 0, ""},
		{"redundant slashes produce empties", "/a//b", false, 3, ""},
		{"traversal preserved", "/../etc/passwd", false, 3, ".."},
		{"unicode", "/映画/file.mkv", false, 2, "映画"},
		{"spaces", "/my movies/file.mkv", false, 2, "my movies"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPath(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len(splitPath(%q)) = %d, want %d; got=%v", tt.in, len(got), tt.wantLen, got)
			}
			if tt.wantHas != "" {
				found := false
				for _, c := range got {
					if strings.Contains(c, tt.wantHas) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("no component in %v contains %q", got, tt.wantHas)
				}
			}
		})
	}
}

// TestSplitPathWithClean shows that cleaning before splitting eliminates
// empty components from redundant slashes and removes traversal artifacts
// that stay within bounds.
func TestSplitPathWithClean(t *testing.T) {
	input := "/media//movies///file.mkv"
	cleaned := filepath.Clean(input)
	parts := splitPath(cleaned)
	want := []string{"media", "movies", "file.mkv"}
	if len(parts) != len(want) {
		t.Fatalf("parts = %v, want %v", parts, want)
	}
	for i, w := range want {
		if parts[i] != w {
			t.Errorf("parts[%d] = %q, want %q", i, parts[i], w)
		}
	}
}
