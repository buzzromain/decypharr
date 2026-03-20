package providers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "provider-concurrency-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	config.SetConfigPath(tmpDir)

	configFile := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configFile, []byte(`{}`), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write config: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// concurrencyResult holds the outcome of one goroutine's GetDownloadLink call.
type concurrencyResult struct {
	index        int
	downloadLink types.DownloadLink
	err          error
}

// setAccountHTTPClient overrides the unexported httpClient field on an Account
// so requests go to our test server.
func setAccountHTTPClient(t *testing.T, acc *account.Account, c *request.Client) {
	t.Helper()
	v := reflect.ValueOf(acc).Elem().FieldByName("httpClient")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(c))
}

// providerWithFakeServer builds an account.Manager wired to a test server.
// It returns the manager and a cleanup function.
func newTestAccountManager(t *testing.T) *account.Manager {
	t.Helper()

	dc := config.Debrid{
		Name:            "realdebrid",
		APIKey:          "test-api-key",
		DownloadAPIKeys: []string{"dl-token-1"},
	}

	mgr := account.NewManager(dc, nil, zerolog.Nop())

	acc := mgr.Current()
	if acc == nil {
		t.Fatal("expected at least one account")
	}

	client := request.New(request.WithMaxRetries(0))
	setAccountHTTPClient(t, acc, client)

	return mgr
}

// fetchDownloadLinkViaHTTP simulates what a real provider's fetchDownloadLink does:
// POST to the server and parse the unrestrict response.
func makeFetcher(serverURL string) account.LinkFetcher {
	return func(acc *account.Account, id string, file *types.File) (types.DownloadLink, error) {
		formData := map[string]string{"link": file.Link}
		form := ""
		for k, v := range formData {
			if form != "" {
				form += "&"
			}
			form += k + "=" + v
		}

		req, err := http.NewRequest(http.MethodPost, serverURL+"/unrestrict/link/", nil)
		if err != nil {
			return types.DownloadLink{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-File-Link", file.Link)

		resp, err := acc.Client().Do(req)
		if err != nil {
			return types.DownloadLink{}, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return types.DownloadLink{}, fmt.Errorf("server error: status %d", resp.StatusCode)
		}

		downloadURL := resp.Header.Get("X-Download-URL")
		if downloadURL == "" {
			return types.DownloadLink{}, fmt.Errorf("download link not found")
		}

		now := time.Now()
		return types.DownloadLink{
			Debrid:       "realdebrid",
			Token:        acc.Token,
			Filename:     file.Name,
			Link:         file.Link,
			DownloadLink: downloadURL,
			Generated:    now,
			ExpiresAt:    now.Add(time.Hour),
		}, nil
	}
}

// ---------------------------------------------------------------------------
// Test 1: Concurrent GetDownloadLink — all succeed
// ---------------------------------------------------------------------------

func TestConcurrentGetDownloadLink_AllSucceed(t *testing.T) {
	const goroutines = 20

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileLink := r.Header.Get("X-File-Link")
		downloadURL := fmt.Sprintf("https://cdn.test/download/%s", fileLink)
		w.Header().Set("X-Download-URL", downloadURL)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mgr := newTestAccountManager(t)
	fetcher := makeFetcher(srv.URL)

	results := make([]concurrencyResult, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	ready := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			<-ready // synchronize start

			file := &types.File{
				Name: fmt.Sprintf("movie_%d.mkv", idx),
				Link: fmt.Sprintf("https://real-debrid.com/d/file-%02d", idx),
				Size: int64(1000 + idx),
			}

			dl, err := mgr.GetDownloadLink(fmt.Sprintf("torrent-%d", idx), file, fetcher)
			results[idx] = concurrencyResult{index: idx, downloadLink: dl, err: err}
		}(i)
	}

	close(ready) // release all goroutines simultaneously
	wg.Wait()

	// Verify: all succeeded, each got the correct unique link, no duplicates.
	seen := make(map[string]int)
	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d returned error: %v", i, r.err)
			continue
		}
		if r.downloadLink.DownloadLink == "" {
			t.Errorf("goroutine %d returned empty download link", i)
			continue
		}

		expectedLink := fmt.Sprintf("https://cdn.test/download/https://real-debrid.com/d/file-%02d", i)
		if r.downloadLink.DownloadLink != expectedLink {
			t.Errorf("goroutine %d: got link %q, want %q", i, r.downloadLink.DownloadLink, expectedLink)
		}

		if r.downloadLink.Token != "dl-token-1" {
			t.Errorf("goroutine %d: got token %q, want %q", i, r.downloadLink.Token, "dl-token-1")
		}

		if prev, ok := seen[r.downloadLink.DownloadLink]; ok {
			t.Errorf("duplicate link between goroutine %d and %d: %s", prev, i, r.downloadLink.DownloadLink)
		}
		seen[r.downloadLink.DownloadLink] = i
	}

	if len(seen) != goroutines {
		t.Errorf("expected %d unique links, got %d", goroutines, len(seen))
	}
}

// ---------------------------------------------------------------------------
// Test 2: Concurrent GetDownloadLink — partial HTTP 500 failures
// ---------------------------------------------------------------------------

func TestConcurrentGetDownloadLink_PartialFailures(t *testing.T) {
	const goroutines = 20

	// Deterministic failure: every 3rd goroutine's file triggers HTTP 500.
	failIndices := make(map[string]bool)
	for i := 0; i < goroutines; i++ {
		if i%3 == 0 {
			failIndices[fmt.Sprintf("https://real-debrid.com/d/file-%02d", i)] = true
		}
	}
	expectedFailures := len(failIndices)
	expectedSuccesses := goroutines - expectedFailures

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileLink := r.Header.Get("X-File-Link")

		if failIndices[fileLink] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		downloadURL := fmt.Sprintf("https://cdn.test/download/%s", fileLink)
		w.Header().Set("X-Download-URL", downloadURL)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Test at the Account level where errors ARE propagated.
	// (Manager.GetDownloadLink swallows errors; Account.GetDownloadLink does not.)
	dc := config.Debrid{
		Name:            "realdebrid",
		APIKey:          "test-api-key",
		DownloadAPIKeys: []string{"dl-token-1"},
	}
	mgr := account.NewManager(dc, nil, zerolog.Nop())
	acc := mgr.Current()
	if acc == nil {
		t.Fatal("expected at least one account")
	}
	setAccountHTTPClient(t, acc, request.New(request.WithMaxRetries(0)))

	fetcher := makeFetcher(srv.URL)

	results := make([]concurrencyResult, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	ready := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			<-ready

			file := &types.File{
				Name: fmt.Sprintf("movie_%d.mkv", idx),
				Link: fmt.Sprintf("https://real-debrid.com/d/file-%02d", idx),
				Size: int64(1000 + idx),
			}

			dl, err := acc.GetDownloadLink(fmt.Sprintf("torrent-%d", idx), file, fetcher)
			results[idx] = concurrencyResult{index: idx, downloadLink: dl, err: err}
		}(i)
	}

	close(ready)
	wg.Wait()

	var successes, failures int
	successLinks := make(map[string]int)

	for i, r := range results {
		expectedFail := failIndices[fmt.Sprintf("https://real-debrid.com/d/file-%02d", i)]

		if expectedFail {
			if r.err == nil {
				t.Errorf("goroutine %d: expected error for failing request, got link %q", i, r.downloadLink.DownloadLink)
			} else {
				failures++
			}
		} else {
			if r.err != nil {
				t.Errorf("goroutine %d: expected success, got error: %v", i, r.err)
			} else {
				successes++
				expectedLink := fmt.Sprintf("https://cdn.test/download/https://real-debrid.com/d/file-%02d", i)
				if r.downloadLink.DownloadLink != expectedLink {
					t.Errorf("goroutine %d: got link %q, want %q", i, r.downloadLink.DownloadLink, expectedLink)
				}

				if prev, ok := successLinks[r.downloadLink.DownloadLink]; ok {
					t.Errorf("duplicate link between goroutine %d and %d: %s", prev, i, r.downloadLink.DownloadLink)
				}
				successLinks[r.downloadLink.DownloadLink] = i
			}
		}
	}

	if successes != expectedSuccesses {
		t.Errorf("successes: got %d, want %d", successes, expectedSuccesses)
	}
	if failures != expectedFailures {
		t.Errorf("failures: got %d, want %d", failures, expectedFailures)
	}

	// Verify failures did not corrupt successful links.
	for link := range successLinks {
		if link == "" {
			t.Error("found empty link among successes")
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3: Concurrent GetDownloadLink — no panics under stress
// ---------------------------------------------------------------------------

func TestConcurrentGetDownloadLink_NoPanics(t *testing.T) {
	const goroutines = 20

	var served atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		fileLink := r.Header.Get("X-File-Link")
		downloadURL := fmt.Sprintf("https://cdn.test/stress/%s", fileLink)
		w.Header().Set("X-Download-URL", downloadURL)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mgr := newTestAccountManager(t)
	fetcher := makeFetcher(srv.URL)

	var panicked atomic.Int32
	var wg sync.WaitGroup
	wg.Add(goroutines)

	ready := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicked.Add(1)
					t.Errorf("goroutine %d panicked: %v", idx, r)
				}
			}()

			<-ready

			file := &types.File{
				Name: fmt.Sprintf("stress_%d.mkv", idx),
				Link: fmt.Sprintf("https://real-debrid.com/d/stress-%02d", idx),
				Size: int64(2000 + idx),
			}

			_, _ = mgr.GetDownloadLink(fmt.Sprintf("torrent-stress-%d", idx), file, fetcher)
		}(i)
	}

	close(ready)
	wg.Wait()

	if panicked.Load() > 0 {
		t.Fatalf("%d goroutines panicked", panicked.Load())
	}
	if served.Load() == 0 {
		t.Error("test server received no requests")
	}
}

// ---------------------------------------------------------------------------
// Test 4: Concurrent GetDownloadLink — cached links are consistent
// ---------------------------------------------------------------------------

func TestConcurrentGetDownloadLink_CacheConsistency(t *testing.T) {
	const goroutines = 20
	// All goroutines request the SAME file — only one should hit the server,
	// the rest should get the cached result.
	var serverHits atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits.Add(1)
		w.Header().Set("X-Download-URL", "https://cdn.test/cached/movie.mkv")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mgr := newTestAccountManager(t)
	fetcher := makeFetcher(srv.URL)

	results := make([]concurrencyResult, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	ready := make(chan struct{})

	sharedFile := &types.File{
		Name: "movie.mkv",
		Link: "https://real-debrid.com/d/shared-file-link",
		Size: 5000,
	}

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			<-ready

			dl, err := mgr.GetDownloadLink("torrent-shared", sharedFile, fetcher)
			results[idx] = concurrencyResult{index: idx, downloadLink: dl, err: err}
		}(i)
	}

	close(ready)
	wg.Wait()

	// All goroutines should get the same cached link.
	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d returned error: %v", i, r.err)
			continue
		}
		if r.downloadLink.DownloadLink != "https://cdn.test/cached/movie.mkv" {
			t.Errorf("goroutine %d: got link %q, want cached link", i, r.downloadLink.DownloadLink)
		}
	}

	// xsync.Map is thread-safe, so the fetcher may be called multiple times
	// in a thundering-herd scenario (no single-flight). But results must be consistent.
	if serverHits.Load() == 0 {
		t.Error("expected at least one server hit")
	}
	t.Logf("server was hit %d times for %d concurrent requests (caching effect)", serverHits.Load(), goroutines)
}
