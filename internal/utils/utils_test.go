package utils

import (
	"testing"
)

// ── ValidateURL ──────────────────────────────────────────────────────────────

func TestValidateURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		wantErr bool
	}{
		// Valid full URLs
		{"http://sonarr:8989", false},
		{"https://radarr.example.com", false},
		{"http://localhost:7878", false},
		{"http://192.168.1.10:8080", false},
		{"https://myserver.local/path/to/app", false},

		// Valid host:port without scheme
		{"sonarr:8989", false},
		{"192.168.1.1:7878", false},

		// Invalid
		{"", true},
		{"ftp://server.com", true}, // non http/https scheme
		{"just-a-hostname", true},  // no scheme, no port
		{"not a url", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			err := ValidateURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ── IsValidURL ───────────────────────────────────────────────────────────────

func TestIsValidURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"http://example.com", true},
		{"https://example.com/path", true},
		{"http://localhost:8080", true},
		{"https://192.168.1.1:9090/api", true},
		// Too short
		{"http://x", false},
		// Wrong scheme
		{"ftp://example.com", false},
		{"magnet:?xt=urn:btih:abc", false},
		// Empty / garbage
		{"", false},
		{"not-a-url", false},
		// Missing host after scheme
		{"http://", false},
		{"https://", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := IsValidURL(tt.input)
			if got != tt.want {
				t.Errorf("IsValidURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ── JoinURL ──────────────────────────────────────────────────────────────────

func TestJoinURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		base  string
		paths []string
		want  string
	}{
		{"http://sonarr:8989", []string{"api/v3/health"}, "http://sonarr:8989/api/v3/health"},
		{"http://sonarr:8989/", []string{"/api/v3/health"}, "http://sonarr:8989/api/v3/health"},
		{"http://radarr:7878", []string{"api", "v3", "movie"}, "http://radarr:7878/api/v3/movie"},
		// Query parameters preserved
		{
			"http://server:8989",
			[]string{"api/v3/history?downloadId=abc123"},
			"http://server:8989/api/v3/history?downloadId=abc123",
		},
		{
			"http://server:8989",
			[]string{"api/v3/queue?page=2&pageSize=100"},
			"http://server:8989/api/v3/queue?page=2&pageSize=100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got, err := JoinURL(tt.base, tt.paths...)
			if err != nil {
				t.Fatalf("JoinURL(%q, %v) unexpected error: %v", tt.base, tt.paths, err)
			}
			if got != tt.want {
				t.Errorf("JoinURL(%q, %v) = %q, want %q", tt.base, tt.paths, got, tt.want)
			}
		})
	}
}

// ── ParseRateLimit ───────────────────────────────────────────────────────────

func TestParseRateLimit(t *testing.T) {
	t.Parallel()
	// Valid formats return a non-nil limiter
	valid := []string{
		"100/minute",
		"100/minutes",
		"50/second",
		"50/seconds",
		"10/hour",
		"10/hours",
		"5/day",
		"5/days",
		"100/min",
		"50/sec",
		"10/hr",
	}
	for _, s := range valid {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			if ParseRateLimit(s) == nil {
				t.Errorf("ParseRateLimit(%q) = nil, want non-nil", s)
			}
		})
	}

	// Invalid formats return nil
	invalid := []string{
		"",
		"100",         // no unit
		"abc/minute",  // non-numeric count
		"0/second",    // zero count
		"-1/minute",   // negative count
		"100/week",    // unknown unit
		"100/",        // empty unit
		"/minute",     // missing count
	}
	for _, s := range invalid {
		t.Run("invalid:"+s, func(t *testing.T) {
			t.Parallel()
			if ParseRateLimit(s) != nil {
				t.Errorf("ParseRateLimit(%q) = non-nil, want nil", s)
			}
		})
	}
}

// ── RemoveItem ───────────────────────────────────────────────────────────────

func TestRemoveItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		slice  []string
		remove []string
		want   []string
	}{
		{"remove one", []string{"a", "b", "c"}, []string{"b"}, []string{"a", "c"}},
		{"remove multiple", []string{"a", "b", "c", "d"}, []string{"a", "c"}, []string{"b", "d"}},
		{"remove all", []string{"x"}, []string{"x"}, []string{}},
		{"remove nonexistent", []string{"a", "b"}, []string{"z"}, []string{"a", "b"}},
		{"remove from empty", []string{}, []string{"a"}, []string{}},
		{"no removal", []string{"a", "b"}, []string{}, []string{"a", "b"}},
		{"remove duplicate value", []string{"a", "b", "a"}, []string{"a"}, []string{"b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RemoveItem(tt.slice, tt.remove...)
			if len(got) != len(tt.want) {
				t.Fatalf("RemoveItem() len = %d, want %d (got %v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("RemoveItem()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ── Contains ─────────────────────────────────────────────────────────────────

func TestContains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		slice []string
		value string
		want  bool
	}{
		{[]string{"sonarr", "radarr"}, "sonarr", true},
		{[]string{"sonarr", "radarr"}, "lidarr", false},
		{[]string{}, "a", false},
		{nil, "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			if got := Contains(tt.slice, tt.value); got != tt.want {
				t.Errorf("Contains(%v, %q) = %v, want %v", tt.slice, tt.value, got, tt.want)
			}
		})
	}
}

// ── Mask ─────────────────────────────────────────────────────────────────────

func TestMask(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		// len > 12: first 8 + **** + last 4
		{"abcdefghijklmnop", "abcdefgh****mnop"},
		// len > 8 (and <= 12): first 4 + **** + last 2
		{"abcdefghij", "abcd****ij"},
		// len <= 8: ****
		{"short", "****"},
		{"12345678", "****"},
		{"", "****"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := Mask(tt.input)
			if got != tt.want {
				t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── RemoveExtension ──────────────────────────────────────────────────────────

func TestRemoveExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		// Known media extensions are stripped
		{"movie.mkv", "movie"},
		{"show.s01e01.mp4", "show.s01e01"},
		{"audio.flac", "audio"},
		// Unknown extensions are NOT stripped
		{"archive.zip", "archive.zip"},
		{"document.pdf", "document.pdf"},
		{"noextension", "noextension"},
		// Edge cases
		{"movie.MKV", "movie"}, // case-insensitive
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := RemoveExtension(tt.input)
			if got != tt.want {
				t.Errorf("RemoveExtension(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
