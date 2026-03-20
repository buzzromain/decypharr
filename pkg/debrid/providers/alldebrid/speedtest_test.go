package alldebrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSpeedTest_LatencySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Fatalf("path = %s, want /user", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer srv.Close()

	ad := newTestADWithAccount(srv.URL)
	result := ad.SpeedTest(context.Background())

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("latency = %d, want >= 0", result.LatencyMs)
	}
	if result.Provider != "alldebrid" {
		t.Fatalf("provider = %q, want alldebrid", result.Provider)
	}
	if result.SpeedMBps != 0 {
		t.Fatalf("speed = %f, want 0 (no cached links)", result.SpeedMBps)
	}
}

func TestSpeedTest_LatencyNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	ad := newTestADWithAccount(srv.URL)
	result := ad.SpeedTest(context.Background())

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

	ad := newTestADWithAccount(closedURL)
	result := ad.SpeedTest(context.Background())

	if result.Error == "" {
		t.Fatal("expected error for transport failure")
	}
	if !strings.Contains(result.Error, "latency test failed") {
		t.Fatalf("error = %q, want 'latency test failed' prefix", result.Error)
	}
}
