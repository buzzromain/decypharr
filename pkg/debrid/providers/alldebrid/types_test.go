package alldebrid

import "testing"

func TestMagnetsUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantLen int
		wantErr bool
	}{
		{
			name:    "array payload",
			payload: `[{"id":1,"filename":"a.mkv"},{"id":2,"filename":"b.mkv"}]`,
			wantLen: 2,
		},
		{
			name:    "object payload",
			payload: `{"first":{"id":1,"filename":"a.mkv"},"second":{"id":2,"filename":"b.mkv"}}`,
			wantLen: 2,
		},
		{
			name:    "empty object",
			payload: `{}`,
			wantLen: 0,
		},
		{
			name:    "unsupported payload",
			payload: `"not-valid"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Magnets
			err := m.UnmarshalJSON([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(m) != tt.wantLen {
				t.Fatalf("len(magnets) = %d, want %d", len(m), tt.wantLen)
			}
		})
	}
}
