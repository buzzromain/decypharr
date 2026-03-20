package config

import "testing"

func FuzzParseSize(f *testing.F) {
	f.Add("1GB")
	f.Add("700MB")
	f.Add("1024KB")
	f.Add("42")
	f.Add("")
	f.Add("bad")
	f.Add("1.5GB")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseSize(s)
	})
}

func FuzzParseBoolAndSamplePath(f *testing.F) {
	f.Add("true", "Movie.Sample.mkv")
	f.Add("1", "/a/b/trailer-clip.mp4")
	f.Add("false", "")
	f.Add("", "\x00\xff")

	f.Fuzz(func(t *testing.T, b, p string) {
		_ = parseBool(b)
		_ = isSample(p)
	})
}
