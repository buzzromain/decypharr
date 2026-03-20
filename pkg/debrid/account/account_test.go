package account

import (
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func newTestAccount(debrid, token string) *Account {
	return &Account{
		Debrid: debrid,
		Token:  token,
		links:  xsync.NewMap[string, types.DownloadLink](),
	}
}

func newTestManager(accounts ...*Account) *Manager {
	m := &Manager{
		debrid:   "test",
		accounts: xsync.NewMap[string, *Account](),
		logger:   zerolog.Nop(),
	}
	for _, acc := range accounts {
		m.accounts.Store(acc.Token, acc)
	}
	if len(accounts) > 0 {
		m.current.Store(accounts[0])
	}
	return m
}

// ── Account.Equals ────────────────────────────────────────────────────────────

func TestAccountEquals(t *testing.T) {
	a := newTestAccount("realdebrid", "tok1")

	if a.Equals(nil) {
		t.Error("Equals(nil) should return false")
	}

	same := newTestAccount("realdebrid", "tok1")
	if !a.Equals(same) {
		t.Error("Equals should return true for same debrid and token")
	}

	diffToken := newTestAccount("realdebrid", "tok2")
	if a.Equals(diffToken) {
		t.Error("Equals should return false for different token")
	}

	diffDebrid := newTestAccount("torbox", "tok1")
	if a.Equals(diffDebrid) {
		t.Error("Equals should return false for different debrid")
	}
}

// ── Account.sliceFileLink ─────────────────────────────────────────────────────

func TestSliceFileLink(t *testing.T) {
	// link39 is exactly 39 characters — passes through unchanged for realdebrid
	const link39 = "https://real-debrid.com/d/AAAABBBBCCCC1"
	// longLink is longer than 39 — gets truncated to link39 for realdebrid
	const longLink = link39 + "2345/extra-path-data"

	t.Run("realdebrid truncates at 39", func(t *testing.T) {
		a := newTestAccount("realdebrid", "tok")
		got := a.sliceFileLink(longLink)
		if got != link39 {
			t.Errorf("sliceFileLink = %q, want %q", got, link39)
		}
	})

	t.Run("realdebrid short link passes through", func(t *testing.T) {
		a := newTestAccount("realdebrid", "tok")
		short := "https://short.link"
		got := a.sliceFileLink(short)
		if got != short {
			t.Errorf("sliceFileLink = %q, want %q", got, short)
		}
	})

	t.Run("realdebrid exactly 39 chars unchanged", func(t *testing.T) {
		a := newTestAccount("realdebrid", "tok")
		got := a.sliceFileLink(link39)
		if got != link39 {
			t.Errorf("sliceFileLink = %q, want %q", got, link39)
		}
	})

	t.Run("non-realdebrid passes long link through", func(t *testing.T) {
		a := newTestAccount("torbox", "tok")
		got := a.sliceFileLink(longLink)
		if got != longLink {
			t.Errorf("sliceFileLink = %q, want %q", got, longLink)
		}
	})
}

// ── Account.GetRandomLink ─────────────────────────────────────────────────────

func TestGetRandomLink(t *testing.T) {
	t.Run("empty cache returns not found", func(t *testing.T) {
		a := newTestAccount("realdebrid", "tok")
		_, found := a.GetRandomLink()
		if found {
			t.Error("expected found=false for empty link cache")
		}
	})

	t.Run("non-empty cache returns a link", func(t *testing.T) {
		a := newTestAccount("realdebrid", "tok")
		dl := types.DownloadLink{
			Link:         "https://example.com/file",
			DownloadLink: "https://cdn.example.com/dl/file",
		}
		a.storeLink(dl)

		got, found := a.GetRandomLink()
		if !found {
			t.Fatal("expected found=true after storing a link")
		}
		if got.DownloadLink != dl.DownloadLink {
			t.Errorf("DownloadLink = %q, want %q", got.DownloadLink, dl.DownloadLink)
		}
	})

	t.Run("empty DownloadLink is skipped", func(t *testing.T) {
		a := newTestAccount("torbox", "tok")
		// Store a link with empty DownloadLink (Empty() == true)
		a.links.Store("key", types.DownloadLink{Link: "https://example.com/file"})
		_, found := a.GetRandomLink()
		if found {
			t.Error("expected found=false when only empty download links are cached")
		}
	})
}

// ── Account.MarkDisabled / Reset ──────────────────────────────────────────────

func TestMarkDisabledAndReset(t *testing.T) {
	a := newTestAccount("realdebrid", "tok")

	if a.Disabled.Load() {
		t.Error("account should not be disabled initially")
	}
	if a.DisableCount.Load() != 0 {
		t.Error("disable count should be 0 initially")
	}

	a.MarkDisabled()
	if !a.Disabled.Load() {
		t.Error("account should be disabled after MarkDisabled")
	}
	if a.DisableCount.Load() != 1 {
		t.Errorf("disable count = %d, want 1", a.DisableCount.Load())
	}

	a.MarkDisabled()
	if a.DisableCount.Load() != 2 {
		t.Errorf("disable count = %d, want 2 after second MarkDisabled", a.DisableCount.Load())
	}

	a.Reset()
	if a.Disabled.Load() {
		t.Error("account should not be disabled after Reset")
	}
	if a.DisableCount.Load() != 0 {
		t.Errorf("disable count = %d, want 0 after Reset", a.DisableCount.Load())
	}
}

// ── Manager.Active / All / Current / Disable ──────────────────────────────────

func TestManagerActive(t *testing.T) {
	a1 := newTestAccount("test", "tok1")
	a1.Index = 0
	a2 := newTestAccount("test", "tok2")
	a2.Index = 1
	a2.MarkDisabled()

	m := newTestManager(a1, a2)

	active := m.Active()
	if len(active) != 1 {
		t.Fatalf("Active() len = %d, want 1", len(active))
	}
	if active[0].Token != "tok1" {
		t.Errorf("Active()[0].Token = %q, want %q", active[0].Token, "tok1")
	}
}

func TestManagerAll(t *testing.T) {
	a1 := newTestAccount("test", "tok1")
	a1.Index = 0
	a2 := newTestAccount("test", "tok2")
	a2.Index = 1
	a2.MarkDisabled()

	m := newTestManager(a1, a2)

	all := m.All()
	if len(all) != 2 {
		t.Fatalf("All() len = %d, want 2", len(all))
	}
}

func TestManagerCurrent(t *testing.T) {
	t.Run("returns active account", func(t *testing.T) {
		a := newTestAccount("test", "tok1")
		a.Index = 0
		m := newTestManager(a)

		got := m.Current()
		if got == nil {
			t.Fatal("Current() should not be nil")
		}
		if got.Token != "tok1" {
			t.Errorf("Current().Token = %q, want %q", got.Token, "tok1")
		}
	})

	t.Run("falls back when current is disabled", func(t *testing.T) {
		a1 := newTestAccount("test", "tok1")
		a1.Index = 0
		a1.MarkDisabled()
		a2 := newTestAccount("test", "tok2")
		a2.Index = 1

		m := newTestManager(a1, a2)
		m.current.Store(a1) // force disabled account as current

		got := m.Current()
		if got == nil {
			t.Fatal("Current() should find a2 as fallback")
		}
		if got.Token != "tok2" {
			t.Errorf("Current().Token = %q, want %q", got.Token, "tok2")
		}
	})

	t.Run("returns nil when no accounts", func(t *testing.T) {
		m := newTestManager()
		if got := m.Current(); got != nil {
			t.Errorf("Current() = %+v, want nil for empty manager", got)
		}
	})
}

func TestManagerDisable(t *testing.T) {
	a1 := newTestAccount("test", "tok1")
	a1.Index = 0
	a2 := newTestAccount("test", "tok2")
	a2.Index = 1

	m := newTestManager(a1, a2)

	m.Disable(a1)

	if !a1.Disabled.Load() {
		t.Error("a1 should be disabled after Disable(a1)")
	}

	// current should have switched to a2
	cur := m.current.Load()
	if cur == nil || cur.Token != "tok2" {
		t.Errorf("current after Disable(a1) = %v, want tok2", cur)
	}
}

func TestManagerReset(t *testing.T) {
	a1 := newTestAccount("test", "tok1")
	a1.Index = 0
	a1.MarkDisabled()
	a2 := newTestAccount("test", "tok2")
	a2.Index = 1
	a2.MarkDisabled()

	m := newTestManager(a1, a2)

	m.Reset()

	if a1.Disabled.Load() || a2.Disabled.Load() {
		t.Error("all accounts should be enabled after Reset")
	}
	if a1.DisableCount.Load() != 0 || a2.DisableCount.Load() != 0 {
		t.Error("disable counts should be 0 after Reset")
	}

	cur := m.current.Load()
	if cur == nil {
		t.Error("current should be set after Reset")
	}
}
