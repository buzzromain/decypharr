package realdebrid

import "testing"

func FuzzAvailabilityResponseUnmarshalJSON(f *testing.F) {
	f.Add(`{}`)
	f.Add(`[]`)
	f.Add(`{"abc":{"rd":[]}}`)
	f.Add(`[{"abc":{"rd":[]}}]`)
	f.Add(`"bad"`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, payload string) {
		var res AvailabilityResponse
		_ = res.UnmarshalJSON([]byte(payload))
	})
}
