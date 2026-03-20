package webdav

import (
	"testing"
)

// ── fastEscapePath ────────────────────────────────────────────────────────────

func TestFastEscapePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Unreserved chars pass through unchanged
		{"simple", "simple"},
		{"/path/to/file.mkv", "/path/to/file.mkv"},
		{"a-b_c.d~e", "a-b_c.d~e"},
		// Spaces encoded
		{"My Show/Season 1", "My%20Show/Season%201"},
		// Special chars encoded
		{"file&name.mkv", "file%26name.mkv"},
		{"100% done", "100%25%20done"},
		{"<tag>", "%3Ctag%3E"},
		// Slash preserved
		{"/Tv Shows/Breaking Bad/S01E01.mkv", "/Tv%20Shows/Breaking%20Bad/S01E01.mkv"},
		// Empty string
		{"", ""},
		// Digits and uppercase letters pass through
		{"ABC123", "ABC123"},
	}
	for _, tt := range tests {
		got := fastEscapePath(tt.input)
		if got != tt.want {
			t.Errorf("fastEscapePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── xmlEscape ─────────────────────────────────────────────────────────────────

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"a&b", "a&amp;b"},
		{"<tag>", "&lt;tag&gt;"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"it's", "it&apos;s"},
		{"a<b>&c\"d'e", "a&lt;b&gt;&amp;c&quot;d&apos;e"},
		{"", ""},
	}
	for _, tt := range tests {
		got := xmlEscape(tt.input)
		if got != tt.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── parseRange ────────────────────────────────────────────────────────────────

func TestParseRange(t *testing.T) {
	const size = int64(1000)

	t.Run("empty header returns nil", func(t *testing.T) {
		ranges, err := parseRange("", size)
		if err != nil || ranges != nil {
			t.Errorf("parseRange(\"\") = %v, %v; want nil, nil", ranges, err)
		}
	})

	t.Run("invalid prefix returns error", func(t *testing.T) {
		if _, err := parseRange("units=0-99", size); err == nil {
			t.Error("expected error for invalid prefix")
		}
	})

	t.Run("missing dash returns error", func(t *testing.T) {
		if _, err := parseRange("bytes=0100", size); err == nil {
			t.Error("expected error for missing dash")
		}
	})

	t.Run("normal range", func(t *testing.T) {
		ranges, err := parseRange("bytes=0-99", size)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 1 || ranges[0].start != 0 || ranges[0].end != 99 {
			t.Errorf("range = %+v, want [{0 99}]", ranges)
		}
	})

	t.Run("open-ended range", func(t *testing.T) {
		ranges, err := parseRange("bytes=500-", size)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 1 || ranges[0].start != 500 || ranges[0].end != size-1 {
			t.Errorf("range = %+v, want [{500 999}]", ranges)
		}
	})

	t.Run("suffix range", func(t *testing.T) {
		// bytes=-200 = last 200 bytes
		ranges, err := parseRange("bytes=-200", size)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 1 || ranges[0].start != 800 || ranges[0].end != 999 {
			t.Errorf("range = %+v, want [{800 999}]", ranges)
		}
	})

	t.Run("end clipped to size-1", func(t *testing.T) {
		ranges, err := parseRange("bytes=0-9999", size)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 1 || ranges[0].end != size-1 {
			t.Errorf("range.end = %d, want %d", ranges[0].end, size-1)
		}
	})

	t.Run("start beyond size is skipped", func(t *testing.T) {
		ranges, err := parseRange("bytes=1000-1999", size)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 0 {
			t.Errorf("expected empty ranges for start >= size, got %+v", ranges)
		}
	})

	t.Run("multiple ranges", func(t *testing.T) {
		ranges, err := parseRange("bytes=0-99,200-299", size)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 2 {
			t.Fatalf("expected 2 ranges, got %d", len(ranges))
		}
		if ranges[0].start != 0 || ranges[0].end != 99 {
			t.Errorf("ranges[0] = %+v, want {0 99}", ranges[0])
		}
		if ranges[1].start != 200 || ranges[1].end != 299 {
			t.Errorf("ranges[1] = %+v, want {200 299}", ranges[1])
		}
	})

	t.Run("inverted range returns error", func(t *testing.T) {
		if _, err := parseRange("bytes=500-100", size); err == nil {
			t.Error("expected error for inverted range (start > end)")
		}
	})

	t.Run("non-numeric start returns error", func(t *testing.T) {
		if _, err := parseRange("bytes=abc-100", size); err == nil {
			t.Error("expected error for non-numeric start")
		}
	})

	t.Run("non-numeric end returns error", func(t *testing.T) {
		if _, err := parseRange("bytes=0-abc", size); err == nil {
			t.Error("expected error for non-numeric end")
		}
	})

	t.Run("suffix larger than size is capped", func(t *testing.T) {
		// bytes=-9999 when size=1000: start=0, end=999
		ranges, err := parseRange("bytes=-9999", size)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ranges) != 1 || ranges[0].start != 0 || ranges[0].end != 999 {
			t.Errorf("range = %+v, want [{0 999}]", ranges)
		}
	})
}
