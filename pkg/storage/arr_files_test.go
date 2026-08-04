package storage

import (
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

// ReferencedInfoHashes drives the "no longer referenced" scan: an infohash
// missing from the set is reported as purgeable, so a silent regression here
// would offer live entries for deletion.
func TestReferencedInfoHashes(t *testing.T) {
	// NewStorage builds a logger, which loads the config; point it at a temp
	// directory so the test never touches a real config file.
	config.SetConfigPath(t.TempDir())

	s, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer s.Close()

	refs := []*ArrFile{
		{ArrName: "tv", ManagedPath: "/media/tv/a/s01e01.mkv", InfoHash: "AABB", FileName: "s01e01.mkv"},
		{ArrName: "tv", ManagedPath: "/media/tv/a/s01e02.mkv", InfoHash: "aabb", FileName: "s01e02.mkv"},
		{ArrName: "movies", ManagedPath: "/media/movies/b/b.mkv", InfoHash: "ccdd", FileName: "b.mkv"},
	}
	for _, r := range refs {
		if err := s.UpsertArrFile(r); err != nil {
			t.Fatalf("UpsertArrFile: %v", err)
		}
	}

	got, err := s.ReferencedInfoHashes()
	if err != nil {
		t.Fatalf("ReferencedInfoHashes: %v", err)
	}

	// Two files of the same torrent collapse to one hash, lowercased so the
	// caller can compare against entry infohashes without worrying about case.
	if len(got) != 2 {
		t.Fatalf("got %d hashes, want 2: %v", len(got), got)
	}
	for _, want := range []string{"aabb", "ccdd"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing %q", want)
		}
	}

	// Dropping one file of a torrent must keep the torrent referenced.
	if _, err := s.DeleteArrFile("/media/tv/a/s01e01.mkv"); err != nil {
		t.Fatalf("DeleteArrFile: %v", err)
	}
	got, _ = s.ReferencedInfoHashes()
	if _, ok := got["aabb"]; !ok {
		t.Error("torrent lost its reference while a sibling file remains")
	}
}
