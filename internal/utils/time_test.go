package utils

import (
	"testing"
	"time"
)

// ── ParseDuration ─────────────────────────────────────────────────────────────

func TestParseDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		// Standard Go durations (fall through)
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"15s", 15 * time.Second, false},
		{"1h30m", 90 * time.Minute, false},

		// Extended: days
		{"1d", 24 * time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"10d", 240 * time.Hour, false},

		// Extended: weeks
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},

		// Extended: combined weeks+days
		{"1w2d", 9 * 24 * time.Hour, false},
		{"2w3d", 17 * 24 * time.Hour, false},

		// Extended: weeks+days+hours
		{"1w2d3h", 9*24*time.Hour + 3*time.Hour, false},

		// Extended: days+hours
		{"2d12h", 60 * time.Hour, false},

		// Whitespace trimmed
		{"  1h  ", time.Hour, false},

		// Errors
		{"", 0, true},
		{"invalid", 0, true},
		{"1x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ── CachedTime ────────────────────────────────────────────────────────────────

func TestCachedTime_InitialValues(t *testing.T) {
	t.Parallel()
	ct := NewCachedTime()
	before := time.Now()

	unix := ct.Unix()
	unixNs := ct.UnixNano()
	now := ct.Now()

	after := time.Now()

	if unix < before.Unix()-1 || unix > after.Unix()+1 {
		t.Errorf("CachedTime.Unix() = %d, expected near %d", unix, before.Unix())
	}
	if unixNs < before.UnixNano()-int64(time.Second) || unixNs > after.UnixNano()+int64(time.Second) {
		t.Error("CachedTime.UnixNano() out of range")
	}
	if now.Before(before.Add(-time.Second)) || now.After(after.Add(time.Second)) {
		t.Errorf("CachedTime.Now() = %v, expected between %v and %v", now, before, after)
	}
}

func TestCachedTime_StartStop(t *testing.T) {
	t.Parallel()
	ct := NewCachedTime()

	ct.Start()
	ct.Stop()

	// After stopping, the cached value should still be readable
	_ = ct.Unix()
	_ = ct.UnixNano()
	_ = ct.Now()
}

func TestCachedTime_StartIdempotent(t *testing.T) {
	t.Parallel()
	ct := NewCachedTime()
	// Calling Start multiple times should not panic
	ct.Start()
	ct.Start()
	ct.Stop()
}

func TestCachedTime_StopIdempotent(t *testing.T) {
	t.Parallel()
	ct := NewCachedTime()
	// Calling Stop when not running should not panic
	ct.Stop()
	ct.Stop()
}

func TestGlobalCachedTime_Functions(t *testing.T) {
	t.Parallel()
	before := time.Now()

	unix := NowUnix()
	now := Now()

	after := time.Now()

	if unix < before.Unix()-1 || unix > after.Unix()+1 {
		t.Errorf("NowUnix() = %d, expected near %d", unix, before.Unix())
	}
	if now.Before(before.Add(-time.Second)) || now.After(after.Add(time.Second)) {
		t.Errorf("Now() = %v out of expected range [%v, %v]", now, before, after)
	}
}
