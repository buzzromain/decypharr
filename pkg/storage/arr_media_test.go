package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestArrStore(t *testing.T) *Storage {
	t.Helper()
	s, err := NewStorage(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertArrMedia(t *testing.T) {
	s := newTestArrStore(t)
	ref := &ArrMedia{
		ArrName:     "sonarr",
		ManagedPath: "/tv/Show/s01e01.mkv",
		InfoHash:    "abc123",
		FileName:    "s01e01.mkv",
	}

	if err := s.UpsertArrMedia(ref); err != nil {
		t.Fatalf("UpsertArrMedia: %v", err)
	}

	if ref.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after upsert")
	}
}

func TestGetArrMedia(t *testing.T) {
	t.Run("retrieves stored record", func(t *testing.T) {
		s := newTestArrStore(t)
		ref := &ArrMedia{
			ArrName:     "radarr",
			ManagedPath: "/movies/Film/film.mkv",
			InfoHash:    "def456",
			FileName:    "film.mkv",
		}
		if err := s.UpsertArrMedia(ref); err != nil {
			t.Fatalf("UpsertArrMedia: %v", err)
		}

		got, err := s.GetArrMedia(ref.ManagedPath)
		if err != nil {
			t.Fatalf("GetArrMedia: %v", err)
		}
		if got == nil {
			t.Fatal("expected record, got nil")
		}
		if got.InfoHash != ref.InfoHash {
			t.Errorf("InfoHash = %q, want %q", got.InfoHash, ref.InfoHash)
		}
	})

	t.Run("returns nil nil for missing key", func(t *testing.T) {
		s := newTestArrStore(t)
		got, err := s.GetArrMedia("/nonexistent/path.mkv")
		if err != nil {
			t.Fatalf("GetArrMedia: unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

func TestDeleteArrMedia(t *testing.T) {
	t.Run("removes and returns the record", func(t *testing.T) {
		s := newTestArrStore(t)
		ref := &ArrMedia{
			ArrName:     "sonarr",
			ManagedPath: "/tv/Series/s02e01.mkv",
			InfoHash:    "ghi789",
			FileName:    "s02e01.mkv",
		}
		if err := s.UpsertArrMedia(ref); err != nil {
			t.Fatalf("UpsertArrMedia: %v", err)
		}

		deleted, err := s.DeleteArrMedia(ref.ManagedPath)
		if err != nil {
			t.Fatalf("DeleteArrMedia: %v", err)
		}
		if deleted == nil {
			t.Fatal("expected deleted record, got nil")
		}
		if deleted.InfoHash != ref.InfoHash {
			t.Errorf("InfoHash = %q, want %q", deleted.InfoHash, ref.InfoHash)
		}

		got, _ := s.GetArrMedia(ref.ManagedPath)
		if got != nil {
			t.Error("record should be absent after deletion")
		}
	})

	t.Run("returns nil for missing record", func(t *testing.T) {
		s := newTestArrStore(t)
		got, err := s.DeleteArrMedia("/does/not/exist.mkv")
		if err != nil {
			t.Fatalf("DeleteArrMedia: unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

// ReferencedInfoHashes drives the "no longer referenced" scan: an infohash
// missing from the set is reported as purgeable, so a silent regression here
// would offer live entries for deletion.
func TestReferencedInfoHashes(t *testing.T) {
	s := newTestArrStore(t)

	refs := []*ArrMedia{
		{ArrName: "tv", ManagedPath: "/media/tv/a/s01e01.mkv", InfoHash: "AABB", FileName: "s01e01.mkv"},
		{ArrName: "tv", ManagedPath: "/media/tv/a/s01e02.mkv", InfoHash: "aabb", FileName: "s01e02.mkv"},
		{ArrName: "movies", ManagedPath: "/media/movies/b/b.mkv", InfoHash: "ccdd", FileName: "b.mkv"},
	}
	for _, r := range refs {
		if err := s.UpsertArrMedia(r); err != nil {
			t.Fatalf("UpsertArrMedia: %v", err)
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
	if _, err := s.DeleteArrMedia("/media/tv/a/s01e01.mkv"); err != nil {
		t.Fatalf("DeleteArrMedia: %v", err)
	}
	got, _ = s.ReferencedInfoHashes()
	if _, ok := got["aabb"]; !ok {
		t.Error("torrent lost its reference while a sibling file remains")
	}
}

func TestFindArrMediaByInfoHash(t *testing.T) {
	s := newTestArrStore(t)
	hash := "findtesthash"
	files := []ArrMedia{
		{ArrName: "sonarr", ManagedPath: "/tv/A/ep1.mkv", InfoHash: hash, FileName: "ep1.mkv"},
		{ArrName: "sonarr", ManagedPath: "/tv/A/ep2.mkv", InfoHash: hash, FileName: "ep2.mkv"},
		{ArrName: "radarr", ManagedPath: "/movies/Other/other.mkv", InfoHash: "otherhash", FileName: "other.mkv"},
	}
	for i := range files {
		if err := s.UpsertArrMedia(&files[i]); err != nil {
			t.Fatalf("UpsertArrMedia: %v", err)
		}
	}

	t.Run("returns all records with same infohash", func(t *testing.T) {
		refs, err := s.FindArrMediaByInfoHash(hash)
		if err != nil {
			t.Fatalf("FindArrMediaByInfoHash: %v", err)
		}
		if len(refs) != 2 {
			t.Errorf("expected 2 records, got %d", len(refs))
		}
	})

	t.Run("case-insensitive infohash match", func(t *testing.T) {
		refs, err := s.FindArrMediaByInfoHash("FINDTESTHASH")
		if err != nil {
			t.Fatalf("FindArrMediaByInfoHash: %v", err)
		}
		if len(refs) != 2 {
			t.Errorf("expected 2 records with uppercase hash, got %d", len(refs))
		}
	})
}

func TestFindArrMediaByFolder(t *testing.T) {
	s := newTestArrStore(t)
	folder := "/tv/FolderTest"
	nested := []ArrMedia{
		{ArrName: "sonarr", ManagedPath: filepath.Join(folder, "s01e01.mkv"), InfoHash: "foldertest1", FileName: "s01e01.mkv"},
		{ArrName: "sonarr", ManagedPath: filepath.Join(folder, "sub", "s01e02.mkv"), InfoHash: "foldertest2", FileName: "s01e02.mkv"},
	}
	sibling := ArrMedia{ArrName: "sonarr", ManagedPath: "/tv/OtherFolder/ep.mkv", InfoHash: "siblinghash", FileName: "ep.mkv"}

	for i := range nested {
		if err := s.UpsertArrMedia(&nested[i]); err != nil {
			t.Fatalf("UpsertArrMedia: %v", err)
		}
	}
	if err := s.UpsertArrMedia(&sibling); err != nil {
		t.Fatalf("UpsertArrMedia: %v", err)
	}

	t.Run("returns nested records", func(t *testing.T) {
		refs, err := s.FindArrMediaByFolder("sonarr", folder)
		if err != nil {
			t.Fatalf("FindArrMediaByFolder: %v", err)
		}
		if len(refs) != 2 {
			t.Errorf("expected 2 nested records, got %d", len(refs))
		}
	})

	t.Run("excludes non-nested records", func(t *testing.T) {
		refs, err := s.FindArrMediaByFolder("sonarr", folder)
		if err != nil {
			t.Fatalf("FindArrMediaByFolder: %v", err)
		}
		for _, r := range refs {
			if r.InfoHash == sibling.InfoHash {
				t.Error("sibling record should not be included")
			}
		}
	})

	t.Run("case-insensitive folder prefix", func(t *testing.T) {
		refs, err := s.FindArrMediaByFolder("sonarr", "/TV/FOLDERTEST")
		if err != nil {
			t.Fatalf("FindArrMediaByFolder: %v", err)
		}
		if len(refs) != 2 {
			t.Errorf("expected 2 records with uppercase folder, got %d", len(refs))
		}
	})

	// A SeriesDelete/MovieDelete from one ARR must never sweep up another ARR's
	// tracked media, even when their library roots overlap (shared parent
	// folder, symlinked structure) — each ARR only ever reports its own
	// folder in these events.
	t.Run("scoped to the requesting arr", func(t *testing.T) {
		radarrSibling := ArrMedia{ArrName: "radarr", ManagedPath: filepath.Join(folder, "movie.mkv"), InfoHash: "radarrhash", FileName: "movie.mkv"}
		if err := s.UpsertArrMedia(&radarrSibling); err != nil {
			t.Fatalf("UpsertArrMedia: %v", err)
		}

		refs, err := s.FindArrMediaByFolder("sonarr", folder)
		if err != nil {
			t.Fatalf("FindArrMediaByFolder: %v", err)
		}
		for _, r := range refs {
			if r.ArrName != "sonarr" {
				t.Errorf("got a %s record from a sonarr-scoped query: %+v", r.ArrName, r)
			}
		}

		refs, err = s.FindArrMediaByFolder("radarr", folder)
		if err != nil {
			t.Fatalf("FindArrMediaByFolder: %v", err)
		}
		if len(refs) != 1 || refs[0].InfoHash != radarrSibling.InfoHash {
			t.Errorf("refs = %v, want exactly radarr's own file", refs)
		}
	})
}

func TestGetArrMediaLastEventDate(t *testing.T) {
	s := newTestArrStore(t)
	t.Run("returns zero and false when unset", func(t *testing.T) {
		got, ok := s.GetArrMediaLastEventDate("noarr")
		if ok {
			t.Error("expected ok=false for unset arr")
		}
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})
}

func TestSetAndGetArrMediaLastEventDate(t *testing.T) {
	s := newTestArrStore(t)
	arrName := "testarr"
	now := time.Now().UTC().Truncate(time.Nanosecond)

	if err := s.SetArrMediaLastEventDate(arrName, now); err != nil {
		t.Fatalf("SetArrMediaLastEventDate: %v", err)
	}

	got, ok := s.GetArrMediaLastEventDate(arrName)
	if !ok {
		t.Fatal("expected ok=true after set")
	}

	gotFormatted := got.UTC().Format(time.RFC3339Nano)
	wantFormatted := now.UTC().Format(time.RFC3339Nano)
	if gotFormatted != wantFormatted {
		t.Errorf("time round-trip: got %q, want %q", gotFormatted, wantFormatted)
	}
}

func TestNormalizePathViaGetArrMedia(t *testing.T) {
	s := newTestArrStore(t)
	ref := &ArrMedia{
		ArrName:     "sonarr",
		ManagedPath: "/TV/NormalizeTest/ep.mkv",
		InfoHash:    "normhash",
		FileName:    "ep.mkv",
	}
	if err := s.UpsertArrMedia(ref); err != nil {
		t.Fatalf("UpsertArrMedia: %v", err)
	}

	got, err := s.GetArrMedia("/tv/normalizetest/ep.mkv")
	if err != nil {
		t.Fatalf("GetArrMedia: %v", err)
	}
	if got == nil {
		t.Fatal("expected record via case-insensitive path, got nil")
	}
	if got.InfoHash != ref.InfoHash {
		t.Errorf("InfoHash = %q, want %q", got.InfoHash, ref.InfoHash)
	}
}

func TestUpsertArrMedia_MetadataRoundTrip(t *testing.T) {
	s := newTestArrStore(t)
	ref := &ArrMedia{
		ArrName:      "radarr",
		ManagedPath:  "/movies/Inception/Inception.mkv",
		InfoHash:     "inception123",
		FileName:     "Inception.mkv",
		Title:        "Inception",
		Year:         2010,
		MediaType:    "movie",
		Poster:       "https://image.tmdb.org/t/p/original/inception.jpg",
		TmdbId:       27205,
		ImdbId:       "tt1375666",
		Overview:     "A thief who steals corporate secrets...",
		Genres:       []string{"Action", "Science Fiction"},
		Quality:      "Remux-1080p",
		ReleaseGroup: "KENOBi3838",
	}

	if err := s.UpsertArrMedia(ref); err != nil {
		t.Fatalf("UpsertArrMedia: %v", err)
	}

	got, err := s.GetArrMedia(ref.ManagedPath)
	if err != nil {
		t.Fatalf("GetArrMedia: %v", err)
	}
	if got == nil {
		t.Fatal("expected record, got nil")
	}

	if got.Title != ref.Title {
		t.Errorf("Title = %q, want %q", got.Title, ref.Title)
	}
	if got.Year != ref.Year {
		t.Errorf("Year = %d, want %d", got.Year, ref.Year)
	}
	if got.TmdbId != ref.TmdbId {
		t.Errorf("TmdbId = %d, want %d", got.TmdbId, ref.TmdbId)
	}
	if got.Quality != ref.Quality {
		t.Errorf("Quality = %q, want %q", got.Quality, ref.Quality)
	}
	if got.ReleaseGroup != ref.ReleaseGroup {
		t.Errorf("ReleaseGroup = %q, want %q", got.ReleaseGroup, ref.ReleaseGroup)
	}
	if len(got.Genres) != len(ref.Genres) {
		t.Errorf("Genres len = %d, want %d", len(got.Genres), len(ref.Genres))
	}
}
