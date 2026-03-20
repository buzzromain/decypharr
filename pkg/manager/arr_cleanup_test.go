package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/arr"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

var (
	testDir string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "decypharr-manager-test-*")
	if err != nil {
		panic(err)
	}
	testDir = dir

	config.SetConfigPath(dir)
	config.Get().UseAuth = false
	config.Get().DownloadFolder = dir

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	mgr := New()
	t.Cleanup(func() { _ = mgr.Stop() })
	return mgr
}

// ── isDecypharrSource ──────────────────────────────────────────────────────────

func TestIsDecypharrSource(t *testing.T) {
	tests := []struct {
		name           string
		sourceFolder   string
		downloadFolder string
		want           bool
	}{
		{
			name:           "child of downloadFolder",
			sourceFolder:   "/downloads/sonarr/show",
			downloadFolder: "/downloads",
			want:           true,
		},
		{
			name:           "equals downloadFolder",
			sourceFolder:   "/downloads",
			downloadFolder: "/downloads",
			want:           false,
		},
		{
			name:           "sibling folder",
			sourceFolder:   "/other/sonarr",
			downloadFolder: "/downloads",
			want:           false,
		},
		{
			name:           "empty sourceFolder",
			sourceFolder:   "",
			downloadFolder: "/downloads",
			want:           false,
		},
		{
			name:           "empty downloadFolder",
			sourceFolder:   "/downloads/show",
			downloadFolder: "",
			want:           false,
		},
		{
			name:           "downloadFolder with trailing slash normalized",
			sourceFolder:   "/downloads/show",
			downloadFolder: "/downloads/",
			want:           true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isDecypharrSource(tc.sourceFolder, tc.downloadFolder)
			if got != tc.want {
				t.Errorf("isDecypharrSource(%q, %q) = %v, want %v",
					tc.sourceFolder, tc.downloadFolder, got, tc.want)
			}
		})
	}
}

// ── HandleArrImport ────────────────────────────────────────────────────────────

func TestHandleArrImport_MatchingSourceFolder(t *testing.T) {
	mgr := newTestManager(t)

	managedPath := filepath.Join("/media/tv", "Show", "s01e01.mkv")
	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeDownload,
		DownloadId:   "ABCDEF1234",
		SourceFolder: filepath.Join(testDir, "sonarr"),
		EpisodeFile:  &arr.WebhookFile{Path: managedPath},
	}

	mgr.HandleArrImport("sonarr", payload)

	ref, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if ref == nil {
		t.Fatal("expected ArrFile to be upserted, got nil")
	}
	if ref.InfoHash != "abcdef1234" {
		t.Errorf("InfoHash = %q, want %q", ref.InfoHash, "abcdef1234")
	}
}

func TestHandleArrImport_NonMatchingSourceFolder(t *testing.T) {
	mgr := newTestManager(t)

	managedPath := filepath.Join("/media/tv", "Show", "s01e02.mkv")
	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeDownload,
		DownloadId:   "BBBBBB0000",
		SourceFolder: "/some/other/folder/not/under/downloads",
		EpisodeFile:  &arr.WebhookFile{Path: managedPath},
	}

	mgr.HandleArrImport("sonarr", payload)

	ref, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if ref != nil {
		t.Error("expected nil ArrFile for non-matching sourceFolder")
	}
}

func TestHandleArrImport_EmptySourceFolderNoStoredEntry(t *testing.T) {
	mgr := newTestManager(t)

	managedPath := filepath.Join("/media/tv", "Show", "s01e03.mkv")
	payload := &arr.WebhookPayload{
		EventType:   arr.EventTypeDownload,
		DownloadId:  "CCCCCC1111",
		EpisodeFile: &arr.WebhookFile{Path: managedPath},
	}

	mgr.HandleArrImport("sonarr", payload)

	ref, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if ref != nil {
		t.Error("expected nil ArrFile when sourceFolder empty and infohash not in storage")
	}
}

// ── HandleArrDelete ────────────────────────────────────────────────────────────

func TestHandleArrDelete_RemovesExistingFile(t *testing.T) {
	mgr := newTestManager(t)

	managedPath := filepath.Join("/media/tv", "Show", "s02e01.mkv")
	ref := &storage.ArrFile{
		ArrName:     "sonarr",
		ManagedPath: managedPath,
		InfoHash:    "deletehash",
		FileName:    "s02e01.mkv",
	}
	if err := mgr.storage.UpsertArrFile(ref); err != nil {
		t.Fatalf("UpsertArrFile: %v", err)
	}

	payload := &arr.WebhookPayload{
		EventType:   arr.EventTypeEpisodeFileDelete,
		EpisodeFile: &arr.WebhookFile{Path: managedPath},
	}
	mgr.HandleArrDelete("sonarr", payload)

	got, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if got != nil {
		t.Error("expected ArrFile to be deleted")
	}
}

func TestHandleArrDelete_NilRefEmptyDownloadId_NoPanic(t *testing.T) {
	mgr := newTestManager(t)

	payload := &arr.WebhookPayload{
		EventType:   arr.EventTypeEpisodeFileDelete,
		DownloadId:  "",
		EpisodeFile: &arr.WebhookFile{Path: "/nonexistent/file.mkv"},
	}

	mgr.HandleArrDelete("sonarr", payload)
}

// ── HandleArrRename ────────────────────────────────────────────────────────────

func TestHandleArrRename_MigratesPath(t *testing.T) {
	mgr := newTestManager(t)

	oldPath := filepath.Join("/media/tv", "Show", "old_name.mkv")
	newPath := filepath.Join("/media/tv", "Show", "new_name.mkv")

	ref := &storage.ArrFile{
		ArrName:     "sonarr",
		ManagedPath: oldPath,
		InfoHash:    "renamehash",
		FileName:    "old_name.mkv",
	}
	if err := mgr.storage.UpsertArrFile(ref); err != nil {
		t.Fatalf("UpsertArrFile: %v", err)
	}

	payload := &arr.WebhookPayload{
		EventType: arr.EventTypeRename,
		RenamedEpisodeFiles: []arr.WebhookRename{
			{PreviousPath: oldPath, Path: newPath},
		},
	}
	mgr.HandleArrRename("sonarr", payload)

	old, err := mgr.storage.GetArrFile(oldPath)
	if err != nil {
		t.Fatalf("GetArrFile old: %v", err)
	}
	if old != nil {
		t.Error("old path should be removed after rename")
	}

	newRef, err := mgr.storage.GetArrFile(newPath)
	if err != nil {
		t.Fatalf("GetArrFile new: %v", err)
	}
	if newRef == nil {
		t.Fatal("expected ArrFile at new path, got nil")
	}
	if newRef.InfoHash != ref.InfoHash {
		t.Errorf("InfoHash = %q, want %q", newRef.InfoHash, ref.InfoHash)
	}
}

// ── HandleArrSeriesDelete ─────────────────────────────────────────────────────

func TestHandleArrSeriesDelete_DeletedFilesFalse_NoOp(t *testing.T) {
	mgr := newTestManager(t)

	managedPath := filepath.Join("/media/tv", "Series", "s01e01.mkv")
	ref := &storage.ArrFile{
		ArrName:     "sonarr",
		ManagedPath: managedPath,
		InfoHash:    "serieskeephash",
		FileName:    "s01e01.mkv",
	}
	if err := mgr.storage.UpsertArrFile(ref); err != nil {
		t.Fatalf("UpsertArrFile: %v", err)
	}
	t.Cleanup(func() { _, _ = mgr.storage.DeleteArrFile(managedPath) })

	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeSeriesDelete,
		DeletedFiles: false,
		Series:       &arr.WebhookSeries{Path: filepath.Join("/media/tv", "Series")},
	}
	mgr.HandleArrSeriesDelete("sonarr", payload)

	got, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if got == nil {
		t.Error("ArrFile should remain when deletedFiles=false")
	}
}

// ── HandleArrMovieDelete ──────────────────────────────────────────────────────

func TestHandleArrMovieDelete_NilMovie_NoPanic(t *testing.T) {
	mgr := newTestManager(t)

	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeMovieDelete,
		DeletedFiles: true,
		Movie:        nil,
	}

	mgr.HandleArrMovieDelete("radarr", payload)
}

// ── test helpers ──────────────────────────────────────────────────────────────

// seedEntry inserts a minimal storage Entry so deleteOrphanedEntry can locate
// and remove it. Having an Entry in storage is required for the orphan-cleanup
// path to execute; without it deleteOrphanedEntry returns early.
func seedEntry(t *testing.T, mgr *Manager, infohash string) {
	t.Helper()
	entry := &storage.Entry{
		InfoHash:  infohash,
		Name:      "test-entry-" + infohash,
		Providers: make(map[string]*storage.ProviderEntry),
		Files:     make(map[string]*storage.File),
	}
	if err := mgr.storage.AddOrUpdate(entry); err != nil {
		t.Fatalf("seedEntry(%q): %v", infohash, err)
	}
}

// registerArrWithAllowDelete registers a stub arr instance with AllowDelete=true
// so the HandleArrDelete / folder-delete paths respect that flag.
func registerArrWithAllowDelete(mgr *Manager, name string) {
	a := arr.New(name, "http://localhost:18888", "test-token", false, false, true, nil, "", "manual")
	mgr.GetArrStorage().AddOrUpdate(a)
}

// ── handleArrFolderDelete + AllowDelete=true: entry cleanup integration ────────

// TestHandleArrFolderDelete_AllowDeleteTrue_DeletesLastEntryReference verifies
// the combined path that was previously untested:
//
//	HandleArrSeriesDelete (AllowDelete=true)
//	  → handleArrFolderDelete
//	    → DeleteArrFile (all files under folder)
//	    → deleteOrphanedEntry (last reference gone → Entry deleted)
//
// Both halves were already tested in isolation; this test proves the join works.
func TestHandleArrFolderDelete_AllowDeleteTrue_DeletesLastEntryReference(t *testing.T) {
	mgr := newTestManager(t)
	registerArrWithAllowDelete(mgr, "sonarr")

	const infohash = "folder-entry-last-ref"
	seriesPath := filepath.Join("/media/tv", "FolderDelEntryShow")

	seedEntry(t, mgr, infohash)
	for _, name := range []string{"s01e01.mkv", "s01e02.mkv"} {
		p := filepath.Join(seriesPath, name)
		if err := mgr.storage.UpsertArrFile(&storage.ArrFile{
			ArrName:     "sonarr",
			ManagedPath: p,
			InfoHash:    infohash,
			FileName:    name,
		}); err != nil {
			t.Fatalf("UpsertArrFile(%q): %v", p, err)
		}
	}

	mgr.HandleArrSeriesDelete("sonarr", &arr.WebhookPayload{
		EventType:    arr.EventTypeSeriesDelete,
		DeletedFiles: true,
		Series:       &arr.WebhookSeries{Path: seriesPath},
	})

	// All ArrFiles under the folder must be gone.
	for _, name := range []string{"s01e01.mkv", "s01e02.mkv"} {
		p := filepath.Join(seriesPath, name)
		got, err := mgr.storage.GetArrFile(p)
		if err != nil {
			t.Fatalf("GetArrFile(%q): %v", p, err)
		}
		if got != nil {
			t.Errorf("ArrFile at %q should be deleted", p)
		}
	}

	// Entry must be gone: it had no remaining ArrFile references.
	if _, err := mgr.GetEntry(infohash); err == nil {
		t.Error("Entry should be deleted when folder delete removes its last ArrFile reference")
	}
}

// TestHandleArrFolderDelete_AllowDeleteTrue_PreservesSharedEntry verifies that
// the Entry is NOT deleted when an ArrFile outside the deleted folder still
// references the same infohash. This is the negative control for the above test.
func TestHandleArrFolderDelete_AllowDeleteTrue_PreservesSharedEntry(t *testing.T) {
	mgr := newTestManager(t)
	registerArrWithAllowDelete(mgr, "sonarr")

	const infohash = "folder-entry-shared-ref"
	seriesPath := filepath.Join("/media/tv", "FolderSharedEntryShow")

	// ArrFile inside the folder – will be deleted by SeriesDelete.
	folderFile := filepath.Join(seriesPath, "s01e01.mkv")
	// ArrFile outside the folder – references the same infohash, must survive.
	outsideFile := filepath.Join("/media/tv", "FolderSharedEntryShowOther", "s02e01.mkv")

	seedEntry(t, mgr, infohash)
	for _, p := range []string{folderFile, outsideFile} {
		if err := mgr.storage.UpsertArrFile(&storage.ArrFile{
			ArrName:     "sonarr",
			ManagedPath: p,
			InfoHash:    infohash,
			FileName:    filepath.Base(p),
		}); err != nil {
			t.Fatalf("UpsertArrFile(%q): %v", p, err)
		}
	}
	t.Cleanup(func() {
		_, _ = mgr.storage.DeleteArrFile(outsideFile)
		_ = mgr.storage.Delete(infohash)
	})

	mgr.HandleArrSeriesDelete("sonarr", &arr.WebhookPayload{
		EventType:    arr.EventTypeSeriesDelete,
		DeletedFiles: true,
		Series:       &arr.WebhookSeries{Path: seriesPath},
	})

	// The folder ArrFile must be gone.
	if got, _ := mgr.storage.GetArrFile(folderFile); got != nil {
		t.Error("ArrFile inside deleted folder should be removed")
	}

	// The outside ArrFile must survive.
	if got, _ := mgr.storage.GetArrFile(outsideFile); got == nil {
		t.Error("ArrFile outside deleted folder must not be removed")
	}

	// Entry must survive: the outside ArrFile still references it.
	if _, err := mgr.GetEntry(infohash); err != nil {
		t.Error("Entry must be preserved when a sibling ArrFile outside the folder still references it")
	}
}

// ── HandleArrSeriesDelete (active path) ───────────────────────────────────────

func TestHandleArrSeriesDelete_DeletedFilesTrue_DeletesArrFiles(t *testing.T) {
	mgr := newTestManager(t)

	seriesPath := filepath.Join("/media/tv", "SeriesT1")
	paths := []string{
		filepath.Join(seriesPath, "s01e01.mkv"),
		filepath.Join(seriesPath, "s01e02.mkv"),
	}
	for _, p := range paths {
		if err := mgr.storage.UpsertArrFile(&storage.ArrFile{
			ArrName:     "sonarr",
			ManagedPath: p,
			InfoHash:    "series-del-hash",
			FileName:    filepath.Base(p),
		}); err != nil {
			t.Fatalf("UpsertArrFile: %v", err)
		}
	}

	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeSeriesDelete,
		DeletedFiles: true,
		Series:       &arr.WebhookSeries{Path: seriesPath},
	}
	mgr.HandleArrSeriesDelete("sonarr", payload)

	for _, p := range paths {
		got, err := mgr.storage.GetArrFile(p)
		if err != nil {
			t.Fatalf("GetArrFile(%q): %v", p, err)
		}
		if got != nil {
			t.Errorf("ArrFile at %q should be deleted after SeriesDelete, still present", p)
		}
	}
}

// ── HandleArrMovieDelete (active path) ────────────────────────────────────────

func TestHandleArrMovieDelete_DeletedFilesTrue_DeletesArrFiles(t *testing.T) {
	mgr := newTestManager(t)

	movieFolder := filepath.Join("/media/movies", "MovieT2")
	managedPath := filepath.Join(movieFolder, "film.mkv")

	if err := mgr.storage.UpsertArrFile(&storage.ArrFile{
		ArrName:     "radarr",
		ManagedPath: managedPath,
		InfoHash:    "movie-del-hash",
		FileName:    "film.mkv",
	}); err != nil {
		t.Fatalf("UpsertArrFile: %v", err)
	}

	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeMovieDelete,
		DeletedFiles: true,
		Movie:        &arr.WebhookMovie{FolderPath: movieFolder},
	}
	mgr.HandleArrMovieDelete("radarr", payload)

	got, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if got != nil {
		t.Error("ArrFile should be deleted after MovieDelete with deletedFiles=true")
	}
}

// ── deleteOrphanedEntry – last file triggers entry removal ────────────────────

func TestHandleArrDelete_AllowDelete_LastFileDeletesEntry(t *testing.T) {
	mgr := newTestManager(t)
	registerArrWithAllowDelete(mgr, "sonarr")

	const infohash = "orphan-hash"
	managedPath := filepath.Join("/media/tv", "OrphanShow", "s03e01.mkv")

	// Seed a storage Entry and a single ArrFile referencing it.
	seedEntry(t, mgr, infohash)
	if err := mgr.storage.UpsertArrFile(&storage.ArrFile{
		ArrName:     "sonarr",
		ManagedPath: managedPath,
		InfoHash:    infohash,
		FileName:    "s03e01.mkv",
	}); err != nil {
		t.Fatalf("UpsertArrFile: %v", err)
	}

	payload := &arr.WebhookPayload{
		EventType:   arr.EventTypeEpisodeFileDelete,
		EpisodeFile: &arr.WebhookFile{Path: managedPath},
	}
	mgr.HandleArrDelete("sonarr", payload)

	// ArrFile must be gone.
	arrFile, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if arrFile != nil {
		t.Error("ArrFile should be deleted after EpisodeFileDelete")
	}

	// Entry must be gone because no ArrFiles reference it any more.
	if _, err := mgr.GetEntry(infohash); err == nil {
		t.Error("Entry should be deleted when it has no remaining ArrFile references")
	}
}

// ── deleteOrphanedEntry – sibling ArrFile protects the entry ─────────────────

func TestHandleArrDelete_AllowDelete_SiblingFilePreservesEntry(t *testing.T) {
	mgr := newTestManager(t)
	registerArrWithAllowDelete(mgr, "sonarr")

	const infohash = "sibling-hash"
	ep1 := filepath.Join("/media/tv", "SiblingShow", "s01e01.mkv")
	ep2 := filepath.Join("/media/tv", "SiblingShow", "s01e02.mkv")

	seedEntry(t, mgr, infohash)
	for _, p := range []string{ep1, ep2} {
		if err := mgr.storage.UpsertArrFile(&storage.ArrFile{
			ArrName:     "sonarr",
			ManagedPath: p,
			InfoHash:    infohash,
			FileName:    filepath.Base(p),
		}); err != nil {
			t.Fatalf("UpsertArrFile(%q): %v", p, err)
		}
	}
	t.Cleanup(func() {
		_, _ = mgr.storage.DeleteArrFile(ep2)
		_ = mgr.storage.Delete(infohash)
	})

	// Delete only ep1.
	payload := &arr.WebhookPayload{
		EventType:   arr.EventTypeEpisodeFileDelete,
		EpisodeFile: &arr.WebhookFile{Path: ep1},
	}
	mgr.HandleArrDelete("sonarr", payload)

	// ep1 must be removed.
	got, err := mgr.storage.GetArrFile(ep1)
	if err != nil {
		t.Fatalf("GetArrFile ep1: %v", err)
	}
	if got != nil {
		t.Error("ep1 ArrFile should be deleted")
	}

	// ep2 (sibling) must still exist.
	sibling, err := mgr.storage.GetArrFile(ep2)
	if err != nil {
		t.Fatalf("GetArrFile ep2: %v", err)
	}
	if sibling == nil {
		t.Error("sibling ep2 ArrFile must not be deleted")
	}

	// Entry must survive because ep2 still references it.
	if _, err := mgr.GetEntry(infohash); err != nil {
		t.Error("Entry must be preserved while a sibling ArrFile still references it")
	}
}

// ── HandleArrDelete – DownloadId fallback ─────────────────────────────────────

func TestHandleArrDelete_FallbackToDownloadId_DeletesEntry(t *testing.T) {
	mgr := newTestManager(t)
	registerArrWithAllowDelete(mgr, "sonarr")

	const infohash = "fallback-hash"

	// Seed a storage Entry but NO ArrFile for the managed path,
	// which is the scenario where the webhook was configured after the initial import.
	seedEntry(t, mgr, infohash)

	payload := &arr.WebhookPayload{
		EventType:   arr.EventTypeEpisodeFileDelete,
		DownloadId:  infohash,
		EpisodeFile: &arr.WebhookFile{Path: "/media/tv/FallbackShow/ep.mkv"},
	}
	mgr.HandleArrDelete("sonarr", payload)

	// Entry must be cleaned up via the DownloadId fallback.
	if _, err := mgr.GetEntry(infohash); err == nil {
		t.Error("Entry should be deleted via DownloadId fallback when no ArrFile was stored")
	}
}

// ── HandleArrImport – idempotency ─────────────────────────────────────────────

func TestHandleArrImport_Idempotent(t *testing.T) {
	mgr := newTestManager(t)

	managedPath := filepath.Join("/media/tv", "IdempShow", "s01e01.mkv")
	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeDownload,
		DownloadId:   "idemp-hash",
		SourceFolder: filepath.Join(testDir, "sonarr-idemp"),
		EpisodeFile:  &arr.WebhookFile{Path: managedPath},
	}

	mgr.HandleArrImport("sonarr", payload)
	mgr.HandleArrImport("sonarr", payload)

	// Exactly one ArrFile must exist for this path.
	ref, err := mgr.storage.GetArrFile(managedPath)
	if err != nil {
		t.Fatalf("GetArrFile: %v", err)
	}
	if ref == nil {
		t.Fatal("expected ArrFile after idempotent import, got nil")
	}

	// Confirm only one record exists for this infohash in the arrFiles store.
	refs, err := mgr.storage.FindArrFilesByInfoHash("idemp-hash")
	if err != nil {
		t.Fatalf("FindArrFilesByInfoHash: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected exactly 1 ArrFile for infohash, got %d", len(refs))
	}
}

// ── HandleArrImport – multiple episode files ──────────────────────────────────

func TestHandleArrImport_MultipleEpisodeFiles(t *testing.T) {
	mgr := newTestManager(t)

	paths := []string{
		filepath.Join("/media/tv", "MultiEpShow", "s01e01.mkv"),
		filepath.Join("/media/tv", "MultiEpShow", "s01e02.mkv"),
		filepath.Join("/media/tv", "MultiEpShow", "s01e03.mkv"),
	}
	episodeFiles := make([]arr.WebhookFile, len(paths))
	for i, p := range paths {
		episodeFiles[i] = arr.WebhookFile{Path: p}
	}

	payload := &arr.WebhookPayload{
		EventType:    arr.EventTypeDownload,
		DownloadId:   "multi-hash",
		SourceFolder: filepath.Join(testDir, "sonarr-multi"),
		EpisodeFiles: episodeFiles,
	}
	mgr.HandleArrImport("sonarr", payload)

	for _, p := range paths {
		ref, err := mgr.storage.GetArrFile(p)
		if err != nil {
			t.Fatalf("GetArrFile(%q): %v", p, err)
		}
		if ref == nil {
			t.Errorf("expected ArrFile at %q, got nil", p)
		}
	}
}

// ── HandleArrRename – unbalanced (more previous paths than new paths) ─────────

func TestHandleArrRename_UnbalancedMismatch_OldDeletedNewSkipped(t *testing.T) {
	mgr := newTestManager(t)

	oldPath1 := filepath.Join("/media/tv", "UnbalShow", "s01e01.old.mkv")
	oldPath2 := filepath.Join("/media/tv", "UnbalShow", "s01e02.old.mkv")
	newPath1 := filepath.Join("/media/tv", "UnbalShow", "s01e01.new.mkv")

	for _, p := range []string{oldPath1, oldPath2} {
		if err := mgr.storage.UpsertArrFile(&storage.ArrFile{
			ArrName:     "sonarr",
			ManagedPath: p,
			InfoHash:    "unbal-hash",
			FileName:    filepath.Base(p),
		}); err != nil {
			t.Fatalf("UpsertArrFile(%q): %v", p, err)
		}
	}
	t.Cleanup(func() { _, _ = mgr.storage.DeleteArrFile(newPath1) })

	// Second entry has an empty new path, making prevPaths longer than newPaths.
	payload := &arr.WebhookPayload{
		EventType: arr.EventTypeRename,
		RenamedEpisodeFiles: []arr.WebhookRename{
			{PreviousPath: oldPath1, Path: newPath1},
			{PreviousPath: oldPath2, Path: ""},
		},
	}
	mgr.HandleArrRename("sonarr", payload) // must not panic

	// oldPath1 must be gone and its new path created.
	if got, _ := mgr.storage.GetArrFile(oldPath1); got != nil {
		t.Error("oldPath1 should be removed")
	}
	if got, _ := mgr.storage.GetArrFile(newPath1); got == nil {
		t.Error("newPath1 should be created for the balanced pair")
	}

	// oldPath2 must be removed (DeleteArrFile was called before the i>=len check).
	if got, _ := mgr.storage.GetArrFile(oldPath2); got != nil {
		t.Error("oldPath2 should be deleted even in unbalanced case")
	}
}
