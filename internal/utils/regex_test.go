package utils

import (
	"testing"
)

// ── IsMediaFile ───────────────────────────────────────────────────────────────

func TestIsMediaFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		// Known video extensions
		{"movie.mkv", true},
		{"movie.mp4", true},
		{"movie.avi", true},
		{"movie.ts", true},
		{"video.m2ts", true},
		{"video.webm", true},
		{"video.mov", true},
		// Known audio extensions
		{"audio.mp3", true},
		{"audio.flac", true},
		{"audio.opus", true},
		{"audio.wav", true},
		// Case insensitive
		{"movie.MKV", true},
		{"audio.MP3", true},
		// Unknown extensions
		{"archive.zip", false},
		{"document.pdf", false},
		{"data.nzb", false},
		{"script.sh", false},
		// No extension
		{"noextension", false},
		// Hidden file with extension
		{".hidden.mkv", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := IsMediaFile(tt.path)
			if got != tt.want {
				t.Errorf("IsMediaFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// ── RemoveInvalidChars ────────────────────────────────────────────────────────

func TestRemoveInvalidChars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		// Normal chars preserved
		{"hello world", "hello world"},
		{"movie (2023)", "movie (2023)"},
		{"Show.S01E01", "Show.S01E01"},
		// Control characters removed (< 32)
		{"hello\x00world", "helloworld"},
		{"tab\there", "tabhere"},
		// Forbidden chars removed: < > " \ | ? *
		{"movie<2023>", "movie2023"},
		{`file"name`, "filename"},
		{"path|split", "pathsplit"},
		{"wild?card", "wildcard"},
		{"multi*star", "multistar"},
		// Backslash removed (except as path separator on Windows, but on Linux it's not)
		// Forward slash (path separator) is preserved
		// Empty string
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := RemoveInvalidChars(tt.input)
			if got != tt.want {
				t.Errorf("RemoveInvalidChars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRemoveInvalidChars_PreservesNormalText(t *testing.T) {
	t.Parallel()
	// These should all pass through unchanged
	inputs := []string{
		"Hello, World!",
		"Movie.Title.2023.mkv",
		"Série française",
		"日本語",
		"123 Main St.",
	}
	for _, s := range inputs {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			got := RemoveInvalidChars(s)
			// All printable non-forbidden chars should survive
			if len(got) == 0 && len(s) > 0 {
				t.Errorf("RemoveInvalidChars(%q) removed all chars, got empty string", s)
			}
		})
	}
}
