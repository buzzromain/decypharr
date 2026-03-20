package realdebrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestSpeedTest_LatencySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Fatalf("path = %s, want /user", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	result := rd.SpeedTest(context.Background())

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("latency = %d, want >= 0", result.LatencyMs)
	}
	if result.Provider != "realdebrid" {
		t.Fatalf("provider = %q, want realdebrid", result.Provider)
	}
	// Account has no cached links → speed test skipped, only latency
	if result.SpeedMBps != 0 {
		t.Fatalf("speed = %f, want 0 (no cached links)", result.SpeedMBps)
	}
}

func TestSpeedTest_LatencyNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	result := rd.SpeedTest(context.Background())

	if result.Error == "" {
		t.Fatal("expected error for non-2xx latency response")
	}
	if !strings.Contains(result.Error, "400") {
		t.Fatalf("error = %q, want status 400 mention", result.Error)
	}
	if result.LatencyMs != 0 {
		t.Fatalf("latency = %d, want 0 on error", result.LatencyMs)
	}
}

func TestSpeedTest_LatencyTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	rd := newTestRDProviderWithAccount(closedURL)
	result := rd.SpeedTest(context.Background())

	if result.Error == "" {
		t.Fatal("expected error for transport failure")
	}
	if !strings.Contains(result.Error, "latency test failed") {
		t.Fatalf("error = %q, want 'latency test failed' prefix", result.Error)
	}
}

func TestSpeedTest_ContextCancellation_DuringDownload(t *testing.T) {
	latencySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer latencySrv.Close()

	// Use a download server that blocks until context is done
	downloadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer downloadSrv.Close()

	rd := newTestRDProviderWithAccount(latencySrv.URL)

	// Inject a link that points to our blocking download server
	acc := rd.accountsManager.Current()
	if acc == nil {
		t.Fatal("expected current account")
	}
	acc.StoreDownloadLinks(map[string]*types.DownloadLink{
		"test-link": {
			Link:         "http://source/link",
			DownloadLink: downloadSrv.URL + "/download",
			Token:        "dl-token",
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-cancel

	result := rd.SpeedTest(ctx)

	// Should get latency but no speed (download canceled)
	if result.SpeedMBps != 0 {
		t.Fatalf("speed = %f, want 0 after cancellation", result.SpeedMBps)
	}
	// No error field set — SpeedTest silently degrades on download failure
	if result.Provider != "realdebrid" {
		t.Fatalf("provider = %q, want realdebrid", result.Provider)
	}
}
