package torbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
)

func newTestTorboxWithAccount(host string) *Torbox {
	useTorrentFile := false
	dc := config.Debrid{
		Name:            "torbox",
		APIKey:          "tb-key",
		DownloadAPIKeys: []string{"dl-token"},
		UseTorrentFile:  &useTorrentFile,
	}
	return &Torbox{
		Host:                  host,
		APIKey:                dc.APIKey,
		client:                request.New(request.WithMaxRetries(0), request.WithRetryableStatus(http.StatusTooManyRequests)),
		accountsManager:       account.NewManager(dc, nil, zerolog.Nop()),
		autoExpiresLinksAfter: time.Hour,
		config:                dc,
	}
}

func TestSpeedTest_LatencySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/me" {
			t.Fatalf("path = %s, want /api/user/me", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	tb := newTestTorboxWithAccount(srv.URL)
	result := tb.SpeedTest(context.Background())

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("latency = %d, want >= 0", result.LatencyMs)
	}
	if result.Provider != "torbox" {
		t.Fatalf("provider = %q, want torbox", result.Provider)
	}
	if result.SpeedMBps != 0 {
		t.Fatalf("speed = %f, want 0 (no account)", result.SpeedMBps)
	}
}

func TestSpeedTest_LatencyNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	tb := newTestTorboxWithAccount(srv.URL)
	result := tb.SpeedTest(context.Background())

	if result.Error == "" {
		t.Fatal("expected error for non-2xx latency response")
	}
	if !strings.Contains(result.Error, "400") {
		t.Fatalf("error = %q, want 400 mention", result.Error)
	}
}

func TestSpeedTest_LatencyTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	tb := newTestTorboxWithAccount(closedURL)
	result := tb.SpeedTest(context.Background())

	if result.Error == "" {
		t.Fatal("expected error for transport failure")
	}
	if !strings.Contains(result.Error, "latency test failed") {
		t.Fatalf("error = %q, want 'latency test failed' prefix", result.Error)
	}
}
