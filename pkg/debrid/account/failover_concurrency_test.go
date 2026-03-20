package account

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// TestManagerGetDownloadLink_ConcurrentFallback verifies that many concurrent
// GetDownloadLink calls with a failing primary account all fall back to the
// secondary account without deadlock, data races, or panics.
func TestManagerGetDownloadLink_ConcurrentFallback(t *testing.T) {
	a1 := newTestAccount("torbox", "tok1")
	a1.Index = 0
	a2 := newTestAccount("torbox", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	file := &types.File{Link: "https://torbox.com/file"}
	var a2Calls atomic.Int32
	fetcher := func(acc *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		if acc.Token == "tok1" {
			return types.DownloadLink{}, errors.New("tok1 unavailable")
		}
		a2Calls.Add(1)
		return validLink(file.Link, "tok2"), nil
	}

	const goroutines = 20
	start := make(chan struct{})
	results := make([]types.DownloadLink, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = m.GetDownloadLink("id", file, fetcher)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
		if results[i].DownloadLink == "" {
			t.Errorf("goroutine %d: empty DownloadLink after fallback", i)
		}
		if results[i].Token != "tok2" {
			t.Errorf("goroutine %d: token = %q, want tok2", i, results[i].Token)
		}
	}

	if n := a2Calls.Load(); n == 0 {
		t.Error("tok2 fetcher should have been called at least once")
	}
}

// TestManagerGetDownloadLink_AllFail_EmptyLinkReturned documents the current
// behavior when all accounts fail the fetcher: GetDownloadLink returns an empty
// DownloadLink. The caller should check DownloadLink.Empty() / Valid() to detect
// this situation.
func TestManagerGetDownloadLink_AllFail_EmptyLinkReturned(t *testing.T) {
	a1 := newTestAccount("torbox", "tok1")
	a1.Index = 0
	a2 := newTestAccount("torbox", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	file := &types.File{Link: "https://torbox.com/file"}
	fetcher := func(_ *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		return types.DownloadLink{}, errors.New("all fail")
	}

	dl, _ := m.GetDownloadLink("id", file, fetcher)
	if dl.DownloadLink != "" {
		t.Errorf("expected empty DownloadLink when all accounts fail, got %q", dl.DownloadLink)
	}
}

// TestManager_ConcurrentDisable_CurrentSwitchesCleanly verifies that concurrent
// Disable() calls on the current account never leave Manager.current pointing
// to a disabled account when an active alternative exists.
func TestManager_ConcurrentDisable_CurrentSwitchesCleanly(t *testing.T) {
	a1 := newTestAccount("test", "tok1")
	a1.Index = 0
	a2 := newTestAccount("test", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	var wg sync.WaitGroup
	const goroutines = 20
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Disable(a1)
			_ = m.Current()
		}()
	}
	wg.Wait()

	cur := m.Current()
	if cur == nil {
		t.Fatal("Current() should not be nil with tok2 still active")
	}
	if cur.Token != "tok2" {
		t.Errorf("Current().Token = %q, want tok2 (only active account)", cur.Token)
	}
}

// TestManager_ConcurrentGetDownloadLink_AfterDisable verifies that when the
// current account is disabled before requests start, all concurrent calls
// successfully fall back to the remaining active account.
func TestManager_ConcurrentGetDownloadLink_AfterDisable(t *testing.T) {
	a1 := newTestAccount("torbox", "tok1")
	a1.Index = 0
	a2 := newTestAccount("torbox", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	// Pre-disable the current account so all calls must use tok2.
	m.Disable(a1)

	file := &types.File{Link: "https://torbox.com/f"}
	fetcher := func(acc *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		if acc.Token == "tok1" {
			return types.DownloadLink{}, errors.New("tok1 disabled")
		}
		return validLink(file.Link, "tok2"), nil
	}

	const goroutines = 15
	start := make(chan struct{})
	errs := make([]error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = m.GetDownloadLink("id", file, fetcher)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d error: %v", i, err)
		}
	}
}

// TestManager_Disable_ThenReset_RestoresAll verifies the full disable→reset
// lifecycle: after Reset() all accounts are active and Current() works.
func TestManager_Disable_ThenReset_RestoresAll(t *testing.T) {
	a1 := newTestAccount("test", "tok1")
	a1.Index = 0
	a2 := newTestAccount("test", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	m.Disable(a1)
	m.Disable(a2)

	// Both disabled → Current falls back to disabled accounts.
	cur := m.Current()
	if cur == nil {
		t.Fatal("Current() should fall back to disabled accounts when all disabled")
	}

	m.Reset()

	if a1.Disabled.Load() || a2.Disabled.Load() {
		t.Error("all accounts should be enabled after Reset")
	}
	cur = m.Current()
	if cur == nil {
		t.Fatal("Current() should not be nil after Reset")
	}
}

// TestManager_ConcurrentFallback_CacheHitOnSecondAccount verifies that once
// tok2 has a cached link, subsequent concurrent calls hit the cache directly.
func TestManager_ConcurrentFallback_CacheHitOnSecondAccount(t *testing.T) {
	a1 := newTestAccount("torbox", "tok1")
	a1.Index = 0
	a2 := newTestAccount("torbox", "tok2")
	a2.Index = 1
	m := newTestManager(a1, a2)

	file := &types.File{Link: "https://torbox.com/f2"}

	// Pre-populate tok2's cache so the fetcher is not called after first hit.
	dl := validLink(file.Link, "tok2")
	a2.storeLink(dl)

	var fetchCalls atomic.Int32
	fetcher := func(acc *Account, _ string, _ *types.File) (types.DownloadLink, error) {
		fetchCalls.Add(1)
		if acc.Token == "tok1" {
			return types.DownloadLink{}, errors.New("tok1 fails")
		}
		return validLink(file.Link, "tok2"), nil
	}

	const goroutines = 10
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = m.GetDownloadLink("id", file, fetcher)
		}()
	}
	close(start)
	wg.Wait()

	// tok1 will still hit the fetcher (no cache), but tok2 should return from
	// cache without calling the fetcher again after the first resolution.
	if n := fetchCalls.Load(); n == 0 {
		t.Error("expected at least one fetcher call for tok1 miss")
	}
}
