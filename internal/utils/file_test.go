package utils

import (
	"testing"
)

// ── FormatSize ────────────────────────────────────────────────────────────────

func TestFormatSize(t *testing.T) {
	t.Parallel()
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	tests := []struct {
		bytes int64
		want  string
	}{
		// Bytes range
		{0, "0 bytes"},
		{1, "1 bytes"},
		{1023, "1023 bytes"},
		// KB range
		{KB, "1.00 KB"},
		{2 * KB, "2.00 KB"},
		{int64(1.5 * KB), "1.50 KB"},
		// MB range
		{MB, "1.00 MB"},
		{500 * MB, "500.00 MB"},
		// GB range
		{GB, "1.00 GB"},
		{int64(4.5 * float64(GB)), "4.50 GB"},
		// TB range
		{TB, "1.00 TB"},
		{2 * TB, "2.00 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := FormatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// ── PathUnescape ─────────────────────────────────────────────────────────────

func TestPathUnescape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		// Normal URL encoded paths
		{"/path/to/file", "/path/to/file"},
		{"/path%20with%20spaces", "/path with spaces"},
		{"/folder%2Ffile.mkv", "/folder/file.mkv"},
		// %25 should become %
		{"/path%25encoded", "/path%encoded"},
		// Plain text unchanged
		{"simple", "simple"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := PathUnescape(tt.input)
			if got != tt.want {
				t.Errorf("PathUnescape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
