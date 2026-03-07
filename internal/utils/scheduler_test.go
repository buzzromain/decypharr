package utils

import (
	"testing"
)

// ── ConvertToJobDef ───────────────────────────────────────────────────────────

func TestConvertToJobDef_Duration(t *testing.T) {
	t.Parallel()
	valid := []string{
		"30s",
		"5m",
		"1h",
		"2h30m",
		"24h",
	}
	for _, s := range valid {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			_, err := ConvertToJobDef(s)
			if err != nil {
				t.Errorf("ConvertToJobDef(%q) unexpected error: %v", s, err)
			}
		})
	}
}

func TestConvertToJobDef_ClockTime(t *testing.T) {
	t.Parallel()
	valid := []string{
		"04:00",
		"12:30",
		"00:00",
		"23:59",
	}
	for _, s := range valid {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			_, err := ConvertToJobDef(s)
			if err != nil {
				t.Errorf("ConvertToJobDef(%q) unexpected error: %v", s, err)
			}
		})
	}
}

func TestConvertToJobDef_CronExpression(t *testing.T) {
	t.Parallel()
	valid := []string{
		"* * * * *",       // every minute
		"0 * * * *",       // every hour
		"0 0 * * *",       // daily at midnight
		"30 4 * * 1",      // every Monday at 04:30
		"0 0 1 1 *",       // yearly
	}
	for _, s := range valid {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			_, err := ConvertToJobDef(s)
			if err != nil {
				t.Errorf("ConvertToJobDef(%q) unexpected error: %v", s, err)
			}
		})
	}
}

func TestConvertToJobDef_Invalid(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"",
		"not-a-schedule",
		"99:99",           // invalid clock time
		"24:00",           // invalid hour
		"12:60",           // invalid minute
		"abc",
	}
	for _, s := range invalid {
		t.Run("invalid:"+s, func(t *testing.T) {
			t.Parallel()
			_, err := ConvertToJobDef(s)
			if err == nil {
				t.Errorf("ConvertToJobDef(%q) expected error, got nil", s)
			}
		})
	}
}

// ── parseClockTime ────────────────────────────────────────────────────────────

func TestParseClockTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		wantOk  bool
		wantH   int
		wantM   int
	}{
		{"04:00", true, 4, 0},
		{"12:30", true, 12, 30},
		{"00:00", true, 0, 0},
		{"23:59", true, 23, 59},
		// Invalid
		{"24:00", false, 0, 0},   // hour out of range
		{"12:60", false, 0, 0},   // minute out of range
		{"-1:00", false, 0, 0},   // negative
		{"12:30:00", false, 0, 0},// too many parts
		{"12", false, 0, 0},      // too few parts
		{"aa:bb", false, 0, 0},   // non-numeric
		{"", false, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, ok := parseClockTime(tt.input)
			if ok != tt.wantOk {
				t.Errorf("parseClockTime(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
				return
			}
			if tt.wantOk {
				if got.Hour() != tt.wantH || got.Minute() != tt.wantM {
					t.Errorf("parseClockTime(%q) = %02d:%02d, want %02d:%02d",
						tt.input, got.Hour(), got.Minute(), tt.wantH, tt.wantM)
				}
			}
		})
	}
}
