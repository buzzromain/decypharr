package notifications

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
)

func TestCallbackNotifier_Send(t *testing.T) {
	t.Parallel()

	t.Run("success posts json payload", func(t *testing.T) {
		t.Parallel()
		var called atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called.Store(true)
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type = %q, want application/json", ct)
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"event":"download_complete"`) {
				t.Fatalf("body missing event field: %s", string(body))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		n := NewCallback(srv.URL)
		err := n.Send(Event{Type: config.EventDownloadComplete, Status: "success", Message: "done"})
		if err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		if !called.Load() {
			t.Fatal("callback endpoint not called")
		}
	})

	t.Run("timeout returns explicit error", func(t *testing.T) {
		t.Parallel()
		n := NewCallback("http://example.com/timeout")
		n.client.Timeout = 10 * time.Millisecond
		n.client.Transport = blockingTransport{}

		err := n.Send(Event{Type: config.EventDownloadFailed, Status: "error"})
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to send callback request") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-2xx is surfaced as error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		n := NewCallback(srv.URL)
		err := n.Send(Event{Type: config.EventRepairFailed, Status: "error"})
		if err == nil || !strings.Contains(err.Error(), "callback returned non-2xx status") {
			t.Fatalf("expected non-2xx error, got: %v", err)
		}
	})

	t.Run("invalid callback URL returns actionable error", func(t *testing.T) {
		t.Parallel()
		n := NewCallback("://bad-url")
		err := n.Send(Event{Type: config.EventRepairPending, Status: "pending"})
		if err == nil || !strings.Contains(err.Error(), "failed to create callback request") {
			t.Fatalf("expected request creation error, got: %v", err)
		}
	})
}

func TestDiscordNotifier_Send(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"embeds"`) {
				t.Fatalf("missing embeds payload: %s", string(body))
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		n := NewDiscord(srv.URL)
		if err := n.Send(Event{Type: config.EventRepairComplete, Status: "success", Message: "ok"}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	})

	t.Run("5xx response includes status", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("discord down"))
		}))
		defer srv.Close()

		n := NewDiscord(srv.URL)
		err := n.Send(Event{Type: config.EventRepairFailed, Status: "error", Message: "boom"})
		if err == nil || !strings.Contains(err.Error(), "discord returned error status code") {
			t.Fatalf("expected status error, got: %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		n := NewDiscord("http://example.com/timeout")
		n.client.Timeout = 10 * time.Millisecond
		n.client.Transport = blockingTransport{}
		err := n.Send(Event{Type: config.EventDownloadComplete, Status: "success", Message: "x"})
		if err == nil || !strings.Contains(err.Error(), "failed to send discord message") {
			t.Fatalf("expected timeout send error, got: %v", err)
		}
	})
}

type blockingTransport struct{}

func (blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, &net.OpError{Op: "roundtrip", Net: "tcp", Err: req.Context().Err()}
}

type blockingNotifier struct {
	name    string
	called  atomic.Bool
	err     error
	started chan struct{}
	release <-chan struct{}
}

func (b *blockingNotifier) Send(Event) error {
	b.called.Store(true)
	if b.started != nil {
		close(b.started)
	}
	if b.release != nil {
		<-b.release
	}
	return b.err
}

func (b *blockingNotifier) Name() string { return b.name }

func TestService_Notify_NonBlockingAndResilient(t *testing.T) {
	t.Parallel()

	cfg := &config.Notifications{Enabled: true}
	svc := New(cfg, zerolog.New(io.Discard))
	fastStarted := make(chan struct{})
	slowStarted := make(chan struct{})
	fastErr := &blockingNotifier{name: "fast-err", err: errors.New("boom"), started: fastStarted}
	slowRelease := make(chan struct{})
	slow := &blockingNotifier{name: "slow", started: slowStarted, release: slowRelease}
	svc.notifiers = []Notifier{fastErr, slow}

	svc.Notify(Event{Type: config.EventDownloadComplete, Status: "success"})

	select {
	case <-fastStarted:
	case <-time.After(time.Second):
		t.Fatal("fast notifier was not invoked")
	}
	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("slow notifier was not invoked")
	}

	close(slowRelease)

	if !fastErr.called.Load() || !slow.called.Load() {
		t.Fatalf("expected both notifiers to be called, fast=%v slow=%v", fastErr.called.Load(), slow.called.Load())
	}
}
