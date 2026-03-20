package repair

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/arr"
)

func TestFileIsSymlinked(t *testing.T) {
	dir := t.TempDir()

	// Regular file — not a symlink
	regular := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regular, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fileIsSymlinked(regular) {
		t.Error("regular file should not be detected as symlink")
	}

	// Symlink pointing to regular file
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if !fileIsSymlinked(link) {
		t.Error("symlink should be detected as symlink")
	}

	// Non-existent path
	if fileIsSymlinked(filepath.Join(dir, "nonexistent")) {
		t.Error("non-existent path should return false")
	}
}

func TestGetSymlinkTarget(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "target.mkv")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("absolute symlink resolves to target", func(t *testing.T) {
		link := filepath.Join(dir, "link.mkv")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		got := getSymlinkTarget(link)
		if got != filepath.Clean(target) {
			t.Errorf("getSymlinkTarget = %q, want %q", got, target)
		}
	})

	t.Run("relative symlink resolves to absolute path", func(t *testing.T) {
		link := filepath.Join(dir, "rel_link.mkv")
		if err := os.Symlink("target.mkv", link); err != nil {
			t.Fatal(err)
		}
		got := getSymlinkTarget(link)
		if got != filepath.Clean(target) {
			t.Errorf("getSymlinkTarget = %q, want %q", got, target)
		}
	})

	t.Run("regular file returns empty string", func(t *testing.T) {
		if got := getSymlinkTarget(target); got != "" {
			t.Errorf("getSymlinkTarget(regular) = %q, want empty", got)
		}
	})

	t.Run("non-existent path returns empty string", func(t *testing.T) {
		if got := getSymlinkTarget(filepath.Join(dir, "ghost")); got != "" {
			t.Errorf("getSymlinkTarget(nonexistent) = %q, want empty", got)
		}
	})

	t.Run("broken symlink still returns intended target path", func(t *testing.T) {
		link := filepath.Join(dir, "broken.mkv")
		if err := os.Symlink("missing.mkv", link); err != nil {
			t.Fatal(err)
		}
		got := getSymlinkTarget(link)
		want := filepath.Join(dir, "missing.mkv")
		if got != filepath.Clean(want) {
			t.Errorf("getSymlinkTarget(broken) = %q, want %q", got, want)
		}
	})
}

func TestGetSymlinkTarget_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission mode bits are not reliably enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission test is not reliable when running as root")
	}

	parent := t.TempDir()
	restricted := filepath.Join(parent, "restricted")
	if err := os.Mkdir(restricted, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(restricted, "blocked.mkv")
	if err := os.Symlink("target.mkv", link); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(restricted, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(restricted, 0o700) })

	if got := getSymlinkTarget(link); got != "" {
		t.Fatalf("expected empty target on permission denied, got %q", got)
	}
}

func TestCollectFiles_IncludesBrokenSymlinks(t *testing.T) {
	dir := t.TempDir()

	broken := filepath.Join(dir, "episode.mkv")
	if err := os.Symlink("missing-file.mkv", broken); err != nil {
		t.Fatal(err)
	}
	regular := filepath.Join(dir, "regular.mkv")
	if err := os.WriteFile(regular, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	media := arr.Content{
		Files: []arr.ContentFile{
			{Name: "broken", Path: broken},
			{Name: "regular", Path: regular},
		},
	}

	got := collectFiles(media)
	if len(got) != 1 {
		t.Fatalf("collectFiles returned %d parent(s), want 1", len(got))
	}

	targetDir := filepath.Clean(dir)
	files := got[targetDir]
	if len(files) != 1 {
		t.Fatalf("collectFiles returned %d files for parent, want 1", len(files))
	}
	if !files[0].IsSymlink {
		t.Fatal("expected collected file to be marked symlink")
	}
	if files[0].TargetPath != "missing-file.mkv" {
		t.Fatalf("TargetPath = %q, want missing-file.mkv", files[0].TargetPath)
	}
}
