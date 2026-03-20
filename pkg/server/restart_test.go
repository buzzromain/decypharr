package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/logger"
	"github.com/sirrobot01/decypharr/pkg/manager"
)

// newTestServerWithRestart creates a minimal *Server wired to a restartCh,
// returning the server, the channel, and a chi router with the config
// endpoint mounted — same handler used in production.
func newTestServerWithRestart(t *testing.T) (*Server, chan struct{}, *chi.Mux) {
	t.Helper()

	dir := t.TempDir()
	origPath := config.GetMainPath()
	config.SetConfigPath(dir)
	config.Reset()
	t.Cleanup(func() {
		config.SetConfigPath(origPath)
		config.Reset()
	})

	cfg := config.Get()
	cfg.UseAuth = false
	cfg.DownloadFolder = dir

	mgr := manager.New()
	t.Cleanup(func() { _ = mgr.Stop() })

	srv := &Server{
		logger:  logger.New("restart-test"),
		manager: mgr,
	}

	restartCh := make(chan struct{}, 1)
	srv.SetRestartFunc(func() {
		select {
		case restartCh <- struct{}{}:
		default:
		}
	})

	r := chi.NewRouter()
	r.Post("/api/config", srv.handleUpdateConfig)
	return srv, restartCh, r
}

func TestRestartEndpoint_TriggersRestartChannel(t *testing.T) {
	_, restartCh, router := newTestServerWithRestart(t)

	ts := httptest.NewServer(router)
	defer ts.Close()

	body := `{"download_folder":"/tmp/test","port":"9999"}`
	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /api/config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	// The handler fires `go s.Restart()` which has a 200ms production delay
	// before calling restartFunc. Use a context deadline instead of time.Sleep.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-restartCh:
		// restartFunc was invoked — restart channel received the signal.
	case <-ctx.Done():
		t.Fatal("restartCh did not receive signal within timeout")
	}
}

func TestRestartEndpoint_ReturnsSuccessJSON(t *testing.T) {
	_, _, router := newTestServerWithRestart(t)

	ts := httptest.NewServer(router)
	defer ts.Close()

	body := `{"download_folder":"/tmp/test","port":"9999"}`
	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /api/config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	b, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(b, []byte(`"status"`)) || !bytes.Contains(b, []byte(`"success"`)) {
		t.Errorf("unexpected body: %s", b)
	}
}

func TestRestartEndpoint_ExactlyOneSignal(t *testing.T) {
	_, restartCh, router := newTestServerWithRestart(t)

	ts := httptest.NewServer(router)
	defer ts.Close()

	body := `{"download_folder":"/tmp/test","port":"9999"}`
	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /api/config: %v", err)
	}
	resp.Body.Close()

	// Wait for the one expected signal.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-restartCh:
		// Good — first signal received.
	case <-ctx.Done():
		t.Fatal("restartCh did not receive signal within timeout")
	}

	// Verify no second signal was sent (channel should be empty).
	select {
	case <-restartCh:
		t.Fatal("restartCh received a second unexpected signal")
	default:
		// Channel empty — correct.
	}
}

func TestRestartEndpoint_InvalidBodyReturns400(t *testing.T) {
	_, _, router := newTestServerWithRestart(t)

	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewBufferString("not-json"))
	if err != nil {
		t.Fatalf("POST /api/config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRestartEndpoint_NoRestartOnBadRequest(t *testing.T) {
	_, restartCh, router := newTestServerWithRestart(t)

	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewBufferString("bad"))
	if err != nil {
		t.Fatalf("POST /api/config: %v", err)
	}
	resp.Body.Close()

	// Give a short window for any erroneous restart signal.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	select {
	case <-restartCh:
		t.Fatal("restartCh received signal on bad request — restart should not fire")
	case <-ctx.Done():
		// Correct: no signal sent.
	}
}
