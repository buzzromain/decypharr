package manager

import (
	"path/filepath"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/storage"
)

func TestDeleteOrphanedEntry_EntryMissing_NoOp(t *testing.T) {
	mgr := newTestManager(t)

	const (
		missingInfohash   = "missing-entry-hash"
		unrelatedInfohash = "keep-entry-hash"
	)
	unrelatedPath := filepath.Join("/media/tv", "GuardShow", "s01e01.mkv")

	seedEntry(t, mgr, unrelatedInfohash)
	if err := mgr.storage.UpsertArrMedia(&storage.ArrMedia{
		ArrName:     "sonarr",
		ManagedPath: unrelatedPath,
		InfoHash:    unrelatedInfohash,
		FileName:    "s01e01.mkv",
	}); err != nil {
		t.Fatalf("UpsertArrMedia: %v", err)
	}

	entriesBefore, err := mgr.storage.Count()
	if err != nil {
		t.Fatalf("Count before: %v", err)
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("deleteOrphanedEntry panicked: %v", r)
			}
		}()
		mgr.deleteOrphanedEntry(missingInfohash)
	}()

	entriesAfter, err := mgr.storage.Count()
	if err != nil {
		t.Fatalf("Count after: %v", err)
	}
	if entriesAfter != entriesBefore {
		t.Fatalf("entry count changed: before=%d after=%d", entriesBefore, entriesAfter)
	}

	if _, err := mgr.GetEntry(missingInfohash); err == nil {
		t.Fatal("missing entry was unexpectedly created")
	}

	missingRefs, err := mgr.storage.FindArrMediaByInfoHash(missingInfohash)
	if err != nil {
		t.Fatalf("FindArrMediaByInfoHash(missing): %v", err)
	}
	if len(missingRefs) != 0 {
		t.Fatalf("unexpected arr files for missing infohash: %d", len(missingRefs))
	}

	keptRef, err := mgr.storage.GetArrMedia(unrelatedPath)
	if err != nil {
		t.Fatalf("GetArrMedia(unrelated): %v", err)
	}
	if keptRef == nil {
		t.Fatal("unrelated arr file was deleted")
	}

	if _, err := mgr.GetEntry(unrelatedInfohash); err != nil {
		t.Fatalf("unrelated entry was deleted: %v", err)
	}
}
