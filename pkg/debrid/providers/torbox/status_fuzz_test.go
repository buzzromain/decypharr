package torbox

import (
	"strings"
	"testing"
)

func FuzzGetTorboxStatus(f *testing.F) {
	tb := &Torbox{}
	f.Add("downloading")
	f.Add("completed")
	f.Add("")
	f.Add("x\x00y")
	f.Add(strings.Repeat("a", 256))

	f.Fuzz(func(t *testing.T, status string) {
		_ = tb.getTorboxStatus(status, false)
		_ = tb.getTorboxStatus(status, true)
	})
}
