package utils

import "testing"

func FuzzValidateURL(f *testing.F) {
	f.Add("http://localhost:8080")
	f.Add("https://example.org/path?q=1")
	f.Add("localhost:8787")
	f.Add("")
	f.Add("://bad")
	f.Add("http://")

	f.Fuzz(func(t *testing.T, in string) {
		_ = ValidateURL(in)
		_ = IsValidURL(in)
	})
}

func FuzzJoinURL(f *testing.F) {
	f.Add("http://localhost:8080", "api", "v1", "items?id=1")
	f.Add("https://example.org/base/", "a", "b", "c")
	f.Add("", "", "", "")

	f.Fuzz(func(t *testing.T, base, p1, p2, p3 string) {
		_, _ = JoinURL(base, p1, p2, p3)
	})
}
