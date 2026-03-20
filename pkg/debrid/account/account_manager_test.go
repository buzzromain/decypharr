package account

import (
	"errors"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// validLink builds a DownloadLink that passes Valid().
func validLink(fileLink, token string) types.DownloadLink {
	return types.DownloadLink{
		Link:         fileLink,
		DownloadLink: "https://cdn.example.com/dl/file.mkv",
		Token:        token,
	}
}

// ── Account.Client ────────────────────────────────────────────────────────────

func TestAccountClient_ReturnsHTTPClient(t *testing.T) {
	a := newTestAccount("realdebrid", "tok")
	// httpClient is nil in test accounts — just verify it doesn't panic
	_ = a.Client()
}

// ── Account.DownloadLinksCount / ClearDownloadLinks ───────────────────────────

func TestDownloadLinksCount(t *testing.T) {
	a := newTestAccount("realdebrid", "tok")
	if n := a.DownloadLinksCount(); n != 0 {
		t.Errorf("DownloadLinksCount() = %d, want 0 for new account", n)
	}

	a.storeLink(validLink("https://real-debrid.com/d/AAAABBBBCCCC1", "tok"))
	if n := a.DownloadLinksCount(); n != 1 {
		t.Errorf("DownloadLinksCount() = %d, want 1 after store", n)
	}
}

func TestClearDownloadLinks(t *testing.T) {
	a := newTestAccount("realdebrid", "tok")
	a.storeLink(validLink("https://real-debrid.com/d/AAAABBBBCCCC1", "tok"))
	a.storeLink(validLink("https://real-debrid.com/d/AAAABBBBCCCC2", "tok"))

	a.ClearDownloadLinks()
	if n := a.DownloadLinksCount(); n != 0 {
		t.Errorf("DownloadLinksCount() = %d after Clear, want 0", n)
	}
}

// ── Account.StoreDownloadLinks ────────────────────────────────────────────────

func TestStoreDownloadLinks(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	dls := map[string]*types.DownloadLink{
		"k1": {Link: "https://a.com/1", DownloadLink: "https://cdn.a.com/1", Token: "tok"},
		"k2": {Link: "https://a.com/2", DownloadLink: "https://cdn.a.com/2", Token: "tok"},
	}
	a.StoreDownloadLinks(dls)
	if n := a.DownloadLinksCount(); n != 2 {
		t.Errorf("DownloadLinksCount() = %d, want 2", n)
	}
}

// ── Account.DeleteLink ────────────────────────────────────────────────────────

func TestDeleteLink_NilDeleter(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	dl := validLink("https://torbox.com/file", "tok")
	a.storeLink(dl)

	if err := a.DeleteLink(dl, nil); err != nil {
		t.Errorf("DeleteLink(nil deleter) returned error: %v", err)
	}
	if n := a.DownloadLinksCount(); n != 0 {
		t.Errorf("link should be removed from cache, got count %d", n)
	}
}

func TestDeleteLink_WithDeleter(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	dl := validLink("https://torbox.com/file", "tok")
	a.storeLink(dl)

	called := false
	deleter := func(_ *Account, _ types.DownloadLink) error {
		called = true
		return nil
	}

	if err := a.DeleteLink(dl, deleter); err != nil {
		t.Errorf("DeleteLink() returned error: %v", err)
	}
	if !called {
		t.Error("deleter function should have been called")
	}
}

func TestDeleteLink_DeleterError(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	dl := validLink("https://torbox.com/file", "tok")
	a.storeLink(dl)

	errDeleter := errors.New("remote delete failed")
	err := a.DeleteLink(dl, func(_ *Account, _ types.DownloadLink) error {
		return errDeleter
	})
	if err != errDeleter {
		t.Errorf("DeleteLink() error = %v, want %v", err, errDeleter)
	}
}

// ── Account.GetDownloadLink ───────────────────────────────────────────────────

func TestAccountGetDownloadLink_CacheHit(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	dl := validLink("https://torbox.com/file", "tok")
	a.storeLink(dl)

	file := &types.File{Link: "https://torbox.com/file"}
	fetcher := func(_ *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		t.Error("fetcher should not be called on cache hit")
		return types.DownloadLink{}, nil
	}

	got, err := a.GetDownloadLink("id1", file, fetcher)
	if err != nil {
		t.Fatalf("GetDownloadLink() error: %v", err)
	}
	if got.DownloadLink != dl.DownloadLink {
		t.Errorf("DownloadLink = %q, want %q", got.DownloadLink, dl.DownloadLink)
	}
}

func TestAccountGetDownloadLink_CacheMissSuccess(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	file := &types.File{Link: "https://torbox.com/new-file"}
	fetched := validLink("https://torbox.com/new-file", "tok")

	fetcher := func(_ *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		return fetched, nil
	}

	got, err := a.GetDownloadLink("id1", file, fetcher)
	if err != nil {
		t.Fatalf("GetDownloadLink() error: %v", err)
	}
	if got.DownloadLink != fetched.DownloadLink {
		t.Errorf("DownloadLink = %q, want %q", got.DownloadLink, fetched.DownloadLink)
	}
	// Should now be in cache
	if a.DownloadLinksCount() != 1 {
		t.Error("link should be cached after fetch")
	}
}

func TestAccountGetDownloadLink_CacheMissFetcherError(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	file := &types.File{Link: "https://torbox.com/file"}
	fetchErr := errors.New("provider error")

	fetcher := func(_ *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		return types.DownloadLink{}, fetchErr
	}

	_, err := a.GetDownloadLink("id1", file, fetcher)
	if err != fetchErr {
		t.Errorf("GetDownloadLink() error = %v, want %v", err, fetchErr)
	}
}

// ── Manager.Disable(nil) ──────────────────────────────────────────────────────

func TestManagerDisable_Nil(t *testing.T) {
	m := newTestManager()
	// Should not panic
	m.Disable(nil)
}

// ── Manager.Current — all-disabled fallback ───────────────────────────────────

func TestManagerCurrent_AllDisabledFallback(t *testing.T) {
	a := newTestAccount("test", "tok1")
	a.Index = 0
	a.MarkDisabled()

	m := newTestManager(a)
	m.current.Store(nil) // force no current

	// All accounts disabled → falls back to first disabled account
	got := m.Current()
	if got == nil {
		t.Fatal("Current() should fall back to disabled account when no active ones exist")
	}
	if got.Token != "tok1" {
		t.Errorf("Current().Token = %q, want tok1", got.Token)
	}
}

// ── Manager.GetAccount ────────────────────────────────────────────────────────

func TestManagerGetAccount(t *testing.T) {
	a := newTestAccount("test", "tok1")
	m := newTestManager(a)

	t.Run("found", func(t *testing.T) {
		got, err := m.GetAccount("tok1")
		if err != nil {
			t.Fatalf("GetAccount(existing) error: %v", err)
		}
		if got.Token != "tok1" {
			t.Errorf("token = %q, want tok1", got.Token)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := m.GetAccount("unknown")
		if err == nil {
			t.Error("GetAccount(unknown) should return error")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := m.GetAccount("")
		if err == nil {
			t.Error("GetAccount('') should return error")
		}
	})
}

// ── Manager.StoreDownloadLink ─────────────────────────────────────────────────

func TestManagerStoreDownloadLink(t *testing.T) {
	a := newTestAccount("test", "tok1")
	m := newTestManager(a)

	t.Run("valid link stored", func(t *testing.T) {
		dl := validLink("https://test.com/file", "tok1")
		m.StoreDownloadLink(dl)
		if a.DownloadLinksCount() != 1 {
			t.Error("link should be stored in account")
		}
	})

	t.Run("empty link ignored", func(t *testing.T) {
		a.ClearDownloadLinks()
		m.StoreDownloadLink(types.DownloadLink{Token: "tok1"}) // empty Link
		if a.DownloadLinksCount() != 0 {
			t.Error("empty link should be ignored")
		}
	})

	t.Run("empty token ignored", func(t *testing.T) {
		a.ClearDownloadLinks()
		m.StoreDownloadLink(types.DownloadLink{Link: "https://test.com/file"}) // empty Token
		if a.DownloadLinksCount() != 0 {
			t.Error("link with empty token should be ignored")
		}
	})

	t.Run("unknown token ignored", func(t *testing.T) {
		m.StoreDownloadLink(validLink("https://test.com/file", "unknown"))
		// no panic, no side effect
	})
}

// ── Manager.DeleteDownloadLink ────────────────────────────────────────────────

func TestManagerDeleteDownloadLink(t *testing.T) {
	a := newTestAccount("test", "tok1")
	m := newTestManager(a)

	t.Run("valid delete", func(t *testing.T) {
		dl := validLink("https://test.com/file", "tok1")
		a.storeLink(dl)

		err := m.DeleteDownloadLink(dl, nil)
		if err != nil {
			t.Errorf("DeleteDownloadLink() error: %v", err)
		}
	})

	t.Run("empty link returns error", func(t *testing.T) {
		err := m.DeleteDownloadLink(types.DownloadLink{Token: "tok1"}, nil)
		if err == nil {
			t.Error("expected error for empty link")
		}
	})

	t.Run("empty token returns error", func(t *testing.T) {
		err := m.DeleteDownloadLink(types.DownloadLink{Link: "https://test.com/file"}, nil)
		if err == nil {
			t.Error("expected error for empty token")
		}
	})

	t.Run("unknown token returns error", func(t *testing.T) {
		err := m.DeleteDownloadLink(validLink("https://test.com/file", "unknown"), nil)
		if err == nil {
			t.Error("expected error for unknown token")
		}
	})
}

// ── Manager.UpdateAccount ─────────────────────────────────────────────────────

func TestManagerUpdateAccount(t *testing.T) {
	a := newTestAccount("test", "tok1")
	m := newTestManager(a)

	t.Run("nil ignored", func(t *testing.T) {
		m.UpdateAccount(nil) // should not panic
	})

	t.Run("empty token ignored", func(t *testing.T) {
		m.UpdateAccount(&Account{}) // should not panic
	})

	t.Run("valid update stored", func(t *testing.T) {
		updated := newTestAccount("test", "tok1")
		updated.Username = "user123"
		m.UpdateAccount(updated)

		got, err := m.GetAccount("tok1")
		if err != nil {
			t.Fatalf("GetAccount after update: %v", err)
		}
		if got.Username != "user123" {
			t.Errorf("Username = %q, want user123", got.Username)
		}
	})
}

// ── Manager.Stats ─────────────────────────────────────────────────────────────

func TestManagerStats(t *testing.T) {
	a := newTestAccount("test", "tok1")
	a.Username = "alice"
	m := newTestManager(a)

	stats := m.Stats()
	if len(stats) != 1 {
		t.Fatalf("Stats() len = %d, want 1", len(stats))
	}
	s := stats[0]
	if s["username"] != "alice" {
		t.Errorf("username = %v, want alice", s["username"])
	}
	if _, ok := s["token_masked"]; !ok {
		t.Error("stats should contain token_masked")
	}
	if _, ok := s["in_use"]; !ok {
		t.Error("stats should contain in_use")
	}
	if _, ok := s["disabled"]; !ok {
		t.Error("stats should contain disabled")
	}
}

// ── Manager.GetDownloadLink ───────────────────────────────────────────────────

func TestManagerGetDownloadLink_CurrentSuccess(t *testing.T) {
	a := newTestAccount("torbox", "tok1")
	m := newTestManager(a)

	file := &types.File{Link: "https://torbox.com/file"}
	fetcher := func(_ *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		return validLink(file.Link, "tok1"), nil
	}

	dl, err := m.GetDownloadLink("id1", file, fetcher)
	if err != nil {
		t.Fatalf("GetDownloadLink() error: %v", err)
	}
	if dl.DownloadLink == "" {
		t.Error("expected non-empty DownloadLink")
	}
}

func TestManagerGetDownloadLink_NoAccounts(t *testing.T) {
	m := newTestManager() // no accounts

	file := &types.File{Link: "https://torbox.com/file"}
	_, err := m.GetDownloadLink("id1", file, nil)
	if err == nil {
		t.Error("expected error when no active account")
	}
}

func TestManagerGetDownloadLink_FallbackToSecondAccount(t *testing.T) {
	a1 := newTestAccount("torbox", "tok1")
	a1.Index = 0
	a2 := newTestAccount("torbox", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	file := &types.File{Link: "https://torbox.com/file"}
	fetcher := func(acc *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		if acc.Token == "tok1" {
			return types.DownloadLink{}, errors.New("tok1 failed")
		}
		return validLink(file.Link, "tok2"), nil
	}

	dl, err := m.GetDownloadLink("id1", file, fetcher)
	if err != nil {
		t.Fatalf("GetDownloadLink() error: %v", err)
	}
	if dl.Token != "tok2" {
		t.Errorf("expected fallback to tok2, got token %q", dl.Token)
	}
}

// ── Manager.RefreshLinks ──────────────────────────────────────────────────────

func TestManagerRefreshLinks(t *testing.T) {
	a := newTestAccount("torbox", "tok1")
	m := newTestManager(a)

	fetched := []types.DownloadLink{
		validLink("https://torbox.com/f1", "tok1"),
		validLink("https://torbox.com/f2", "tok1"),
	}
	fetcher := func(_ *Account) ([]types.DownloadLink, error) {
		return fetched, nil
	}

	if err := m.RefreshLinks(fetcher); err != nil {
		t.Fatalf("RefreshLinks() error: %v", err)
	}
	if a.DownloadLinksCount() != 2 {
		t.Errorf("expected 2 links after refresh, got %d", a.DownloadLinksCount())
	}
}

func TestManagerRefreshLinks_FetcherError(t *testing.T) {
	a := newTestAccount("torbox", "tok1")
	m := newTestManager(a)

	fetchErr := errors.New("network error")
	fetcher := func(_ *Account) ([]types.DownloadLink, error) {
		return nil, fetchErr
	}

	if err := m.RefreshLinks(fetcher); err == nil {
		t.Error("RefreshLinks() should propagate fetcher errors")
	}
}

// ── Manager.Disable — no active fallback ─────────────────────────────────────

func TestManagerDisable_NoActiveFallback(t *testing.T) {
	a := newTestAccount("test", "tok1")
	a.Index = 0
	m := newTestManager(a)

	m.Disable(a)

	if cur := m.current.Load(); cur == nil {
		t.Fatal("current should fall back to the disabled account when no active accounts remain")
	} else if cur.Token != "tok1" {
		t.Errorf("fallback token = %q, want tok1", cur.Token)
	}
}

// ── Manager.Reset — no accounts ───────────────────────────────────────────────

func TestManagerReset_NoAccounts(t *testing.T) {
	m := newTestManager() // no accounts
	m.Reset()             // should not panic, current stays nil
	if cur := m.current.Load(); cur != nil {
		t.Errorf("current should be nil after Reset with no accounts, got %v", cur)
	}
}

// ── Manager.GetDownloadLink — all accounts fail ───────────────────────────────

func TestManagerGetDownloadLink_AllAccountsFail(t *testing.T) {
	a1 := newTestAccount("torbox", "tok1")
	a1.Index = 0
	a2 := newTestAccount("torbox", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	file := &types.File{Link: "https://torbox.com/file"}
	fetcher := func(_ *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		return types.DownloadLink{}, errors.New("always fails")
	}

	// When all accounts fail the fetcher, GetDownloadLink returns the (empty) result
	// from the first attempt — no panic expected.
	_, _ = m.GetDownloadLink("id1", file, fetcher)
}

// ── Account.GetDownloadLink — cached but invalid ──────────────────────────────

func TestAccountGetDownloadLink_CachedInvalidLink(t *testing.T) {
	a := newTestAccount("torbox", "tok")
	// Store a link whose DownloadLink is empty → Valid() will fail
	invalid := types.DownloadLink{Link: "https://torbox.com/file", DownloadLink: ""}
	a.links.Store("https://torbox.com/file", invalid)

	file := &types.File{Link: "https://torbox.com/file"}
	fetcher := func(_ *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		t.Error("fetcher should not be called on cache hit")
		return types.DownloadLink{}, nil
	}

	_, err := a.GetDownloadLink("id1", file, fetcher)
	if err == nil {
		t.Error("GetDownloadLink should return error for cached invalid link")
	}
}

// ── Manager.Sync ─────────────────────────────────────────────────────────────

func TestManagerSync(t *testing.T) {
	a := newTestAccount("test", "tok1")
	m := newTestManager(a)

	called := false
	syncer := func(acc *Account) error {
		called = true
		acc.Username = "synced"
		return nil
	}

	m.Sync(syncer)

	if !called {
		t.Error("syncer should have been called")
	}
	if a.Username != "synced" {
		t.Errorf("Username = %q, want synced", a.Username)
	}
}

func TestManagerSync_SyncerError(t *testing.T) {
	a := newTestAccount("test", "tok1")
	m := newTestManager(a)

	syncErr := errors.New("sync failed")
	syncer := func(_ *Account) error { return syncErr }

	// Error should be logged but not panic
	m.Sync(syncer)
}

func TestManagerSync_ExpiredAccount(t *testing.T) {
	a := newTestAccount("test", "tok1")
	a.Index = 0
	// Set expiration in the past
	a.Expiration = pastTime()
	m := newTestManager(a)

	syncer := func(acc *Account) error { return nil }
	m.Sync(syncer)

	// Account should have been disabled because it expired
	if !a.Disabled.Load() {
		t.Error("expired account should be disabled after Sync")
	}
}

// pastTime returns a time clearly in the past for expiration tests.
func pastTime() time.Time {
	return time.Now().Add(-24 * time.Hour)
}
