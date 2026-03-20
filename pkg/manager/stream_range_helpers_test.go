package manager

import (
	"errors"
	"fmt"
	"net"
	"testing"
)

// ── normalizeStreamRange ──────────────────────────────────────────────────────

func TestNormalizeStreamRange(t *testing.T) {
	tests := []struct {
		name      string
		size      int64
		start     int64
		end       int64
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{"zero_size_errors", 0, 0, 10, 0, 0, true},
		{"negative_size_errors", -1, 0, 10, 0, 0, true},
		{"start_beyond_size_errors", 100, 150, 200, 0, 0, true},
		{"end_before_start_errors", 100, 50, 20, 0, 0, true},
		{"negative_start_clamped_to_zero", 100, -5, 50, 0, 50, false},
		{"end_minus_one_becomes_size_minus_one", 100, 0, -1, 0, 99, false},
		{"end_at_size_clamped", 100, 0, 100, 0, 99, false},
		{"end_beyond_size_clamped", 100, 0, 999, 0, 99, false},
		{"valid_range_unchanged", 1000, 100, 200, 100, 200, false},
		{"zero_start_end_zero", 100, 0, 0, 0, 0, false},
		{"start_equals_end", 100, 50, 50, 50, 50, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := normalizeStreamRange(tt.size, tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if gotStart != tt.wantStart {
					t.Errorf("start: got %d, want %d", gotStart, tt.wantStart)
				}
				if gotEnd != tt.wantEnd {
					t.Errorf("end: got %d, want %d", gotEnd, tt.wantEnd)
				}
			}
		})
	}
}

// ── buildContentRange ─────────────────────────────────────────────────────────

func TestBuildContentRange(t *testing.T) {
	tests := []struct {
		start, end, total int64
		want              string
	}{
		{0, 99, 100, "bytes 0-99/100"},
		{0, 0, 1, "bytes 0-0/1"},
		{500, 999, 10000, "bytes 500-999/10000"},
		// end < start → '*' wildcard
		{100, 50, 200, "bytes 100-*/200"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d-%d/%d", tt.start, tt.end, tt.total), func(t *testing.T) {
			got := buildContentRange(tt.start, tt.end, tt.total)
			if got != tt.want {
				t.Errorf("buildContentRange(%d,%d,%d) = %q, want %q", tt.start, tt.end, tt.total, got, tt.want)
			}
		})
	}
}

// ── buildHTTPRange ────────────────────────────────────────────────────────────

func TestBuildHTTPRange(t *testing.T) {
	tests := []struct {
		start, end int64
		want       string
	}{
		{0, 99, "bytes=0-99"},
		{500, 999, "bytes=500-999"},
		// end < start → no end value
		{100, 50, "bytes=100-"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d-%d", tt.start, tt.end), func(t *testing.T) {
			got := buildHTTPRange(tt.start, tt.end)
			if got != tt.want {
				t.Errorf("buildHTTPRange(%d,%d) = %q, want %q", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

// ── isConnectionError ─────────────────────────────────────────────────────────

type fakeNetError struct{ msg string }

func (e *fakeNetError) Error() string   { return e.msg }
func (e *fakeNetError) Timeout() bool   { return false }
func (e *fakeNetError) Temporary() bool { return true }

// Ensure fakeNetError implements net.Error at compile time.
var _ net.Error = (*fakeNetError)(nil)

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"regular_error", errors.New("something went wrong"), false},
		{"EOF", errors.New("EOF"), true},
		{"connection_reset", errors.New("read: connection reset by peer"), true},
		{"broken_pipe", errors.New("write: broken pipe"), true},
		{"connection_refused", errors.New("dial tcp: connection refused"), true},
		{"net_error", &fakeNetError{"timeout"}, true},
		{"wrapped_net_error", fmt.Errorf("outer: %w", &fakeNetError{"inner"}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConnectionError(tt.err)
			if got != tt.want {
				t.Errorf("isConnectionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
