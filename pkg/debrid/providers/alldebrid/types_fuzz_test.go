package alldebrid

import "testing"

func FuzzMagnetsUnmarshalJSON(f *testing.F) {
	f.Add(`[]`)
	f.Add(`{}`)
	f.Add(`[{"id":1}]`)
	f.Add(`{"a":{"id":1}}`)
	f.Add(`"x"`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, payload string) {
		var m Magnets
		_ = m.UnmarshalJSON([]byte(payload))
	})
}
