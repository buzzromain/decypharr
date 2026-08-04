package manager

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	debridCommon "github.com/sirrobot01/decypharr/pkg/debrid/common"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// This file covers the managed-only auto-reinsertion loop and its interaction
// with the AllDebrid remove_after_add slot strategy. Neither feature depends
// on the other in production, but they share one function — Fixer.MoveTorrent
// — so a change to either can silently break their combination. That is
// exactly what the fix in this same commit closed: before it, a torrent
// managed-only reinserted after an unexpected removal would never have its
// slot freed again, contradicting remove_after_add's own promise.

// withManagedOnly loads a fresh, isolated global config with managed_only
// true. Needed because reinsertDeletedTorrents reads config.Get() directly
// (not m.config), and config.Get() is a sync.Once singleton — any earlier
// test in this package (several call storage.NewStorage, which logs, which
// already triggers config.Get()) may have cached a different instance first.
// config.Reset() forces a fresh load from this test's own directory.
func withManagedOnly(t *testing.T) {
	t.Helper()
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()
	config.SetConfigPath(dir)
	cfgFile := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgFile, []byte(`{"managed_only": true}`), 0644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}

	if !config.Get().ManagedOnly {
		t.Fatal("setup: expected ManagedOnly to be true after loading the seeded config")
	}
}

// Entries the slot strategy intentionally removed (RemovedAt set) must never
// be handed to the reinsertion loop — otherwise managed-only would fight
// remove_after_add forever, reinserting and re-freeing on every sync pass.
func TestDetectTorrentChanges_SkipsSlotStrategyRemovals(t *testing.T) {
	withManagedOnly(t)

	strg, err := storage.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer strg.Close()

	m := &Manager{storage: strg, logger: zerolog.Nop(), queue: newQueue(strg, "")}

	freed := &storage.Entry{InfoHash: "aaa1", Name: "freed by remove_after_add", Protocol: config.ProtocolTorrent}
	freed.AddTorrentProvider(&types.Torrent{Id: "1", Debrid: "alldebrid"})
	removedAt := time.Now()
	freed.Providers["alldebrid"].RemovedAt = &removedAt
	if err := strg.AddOrUpdate(freed); err != nil {
		t.Fatalf("AddOrUpdate(freed): %v", err)
	}

	missing := &storage.Entry{InfoHash: "bbb2", Name: "unexpectedly missing", Protocol: config.ProtocolTorrent}
	missing.AddTorrentProvider(&types.Torrent{Id: "2", Debrid: "alldebrid"})
	if err := strg.AddOrUpdate(missing); err != nil {
		t.Fatalf("AddOrUpdate(missing): %v", err)
	}

	// Neither torrent is on the remote list — both are absent from AllDebrid.
	_, _, toDelete, err := m.detectTorrentChanges("alldebrid", map[string]*types.Torrent{})
	if err != nil {
		t.Fatalf("detectTorrentChanges: %v", err)
	}

	got := make(map[string]bool, len(toDelete))
	for _, h := range toDelete {
		got[h] = true
	}
	if got["aaa1"] {
		t.Error("entry freed by remove_after_add was queued for reinsertion — RemovedAt was not respected")
	}
	if !got["bbb2"] {
		t.Error("entry unexpectedly missing was not queued for reinsertion")
	}
}

// End-to-end: managed-only reinserts a torrent that vanished from AllDebrid
// for a reason unrelated to remove_after_add, and remove_after_add still
// frees the slot again right after — the two features compose instead of
// fighting each other.
func TestReinsertDeletedTorrents_ReappliesRemoveAfterAdd(t *testing.T) {
	withManagedOnly(t)

	strg, err := storage.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer strg.Close()

	client := &fakeMoveClient{cfg: config.Debrid{
		Name: "alldebrid", Provider: "alldebrid", SlotStrategy: "remove_after_add",
	}}
	m := &Manager{
		storage:          strg,
		logger:           zerolog.Nop(),
		clients:          xsync.NewMap[string, debridCommon.Client](),
		reinsertAttempts: xsync.NewMap[string, *reinsertAttempt](),
		config:           &config.Config{},
	}
	m.clients.Store("alldebrid", client)
	m.fixer = &Fixer{manager: m}

	entry := &storage.Entry{
		InfoHash: "ccc3",
		Name:     "Test Movie",
		Protocol: config.ProtocolTorrent,
		Magnet:   "magnet:?xt=urn:btih:ccc3&dn=Test+Movie",
	}
	if err := strg.AddOrUpdate(entry); err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}

	remaining := m.reinsertDeletedTorrents("alldebrid", []string{"ccc3"})
	if len(remaining) != 0 {
		t.Fatalf("remaining = %v, want none — reinsertion should have succeeded", remaining)
	}
	if len(client.submitted) != 1 {
		t.Fatalf("submitted = %v, want exactly one submission", client.submitted)
	}
	if len(client.deleted) != 1 || client.deleted[0] != client.submitted[0] {
		t.Fatalf("deleted = %v, want [%s] — remove_after_add should free the slot right after reinsertion", client.deleted, client.submitted[0])
	}

	saved, err := strg.Get("ccc3")
	if err != nil {
		t.Fatalf("storage.Get: %v", err)
	}
	pe := saved.Providers["alldebrid"]
	if pe == nil || pe.RemovedAt == nil {
		t.Fatal("RemovedAt not persisted after managed-only's own reinsertion — the two features are not composing correctly")
	}

	// The retry counter is cleared on success — confirms the reinsertion was
	// recorded as a success, not silently dropped or left mid-retry.
	if _, ok := m.reinsertAttempts.Load("ccc3"); ok {
		t.Error("reinsertAttempts still holds an entry after a successful reinsertion")
	}
}
