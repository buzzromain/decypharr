package cgofuse

import (
	"reflect"
	"testing"
)

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{"root", "/", nil},
		{"empty", "", nil},
		{"single component", "/movies", []string{"movies"}},
		{"two components", "/movies/file.mkv", []string{"movies", "file.mkv"}},
		{"three components", "/group/torrent/file.mkv", []string{"group", "torrent", "file.mkv"}},
		{"trailing slash", "/movies/", []string{"movies"}},
		{"double slash", "/movies//file.mkv", []string{"movies", "", "file.mkv"}},
		{"no leading slash", "movies/file.mkv", []string{"movies", "file.mkv"}},
		{"multiple trailing slashes", "/dir///", []string{"dir"}},
		{"dot path", "/.", []string{"."}},
		{"dotdot path", "/..", []string{".."}},
		{"traversal attempt", "/../etc/passwd", []string{"..", "etc", "passwd"}},
		{"spaces in path", "/my movies/my file.mkv", []string{"my movies", "my file.mkv"}},
		{"unicode path", "/映画/ファイル.mkv", []string{"映画", "ファイル.mkv"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPath(tt.path)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestSplitPath_RootReturnsNil verifies the critical behavior that
// root path "/" returns nil, which is used to identify root directory operations.
func TestSplitPath_RootReturnsNil(t *testing.T) {
	got := splitPath("/")
	if got != nil {
		t.Fatalf("splitPath('/') = %v, want nil (root detection depends on this)", got)
	}
}

// TestSplitPath_PreservesTraversalComponents documents that splitPath does NOT
// sanitize path traversal. This is a known behavior — traversal prevention
// relies on the manager layer returning ENOENT for invalid entries.
func TestSplitPath_PreservesTraversalComponents(t *testing.T) {
	parts := splitPath("/../../../etc/passwd")
	if len(parts) == 0 {
		t.Fatal("expected non-empty parts for traversal path")
	}
	// The first component should be ".." — splitPath does not filter it
	if parts[0] != ".." {
		t.Errorf("parts[0] = %q, want '..'", parts[0])
	}
}
