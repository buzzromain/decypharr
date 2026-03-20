package realdebrid

import "testing"

func TestAvailabilityResponseUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantLen int
		wantErr bool
	}{
		{
			name:    "object payload",
			payload: `{"abc":{"rd":[{"1":{"filename":"f","filesize":10}}]}}`,
			wantLen: 1,
		},
		{
			name:    "array payload",
			payload: `[{"abc":{"rd":[{"1":{"filename":"f","filesize":10}}]}}]`,
			wantLen: 1,
		},
		{
			name:    "empty array payload",
			payload: `[]`,
			wantLen: 0,
		},
		{
			name:    "invalid payload",
			payload: `"bad"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var res AvailabilityResponse
			err := res.UnmarshalJSON([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(res) != tt.wantLen {
				t.Fatalf("len(response) = %d, want %d", len(res), tt.wantLen)
			}
		})
	}
}

func TestHosterUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantRd  int
		wantErr bool
	}{
		{
			name:    "object payload",
			payload: `{"rd":[{"1":{"filename":"a.mkv","filesize":100}}]}`,
			wantRd:  1,
		},
		{
			name:    "empty array payload treated as empty hoster",
			payload: `[]`,
			wantRd:  0,
		},
		{
			name:    "invalid payload",
			payload: `"oops"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h Hoster
			err := h.UnmarshalJSON([]byte(tt.payload))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(h.Rd) != tt.wantRd {
				t.Fatalf("len(hoster.rd) = %d, want %d", len(h.Rd), tt.wantRd)
			}
		})
	}
}
