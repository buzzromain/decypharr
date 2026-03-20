package torbox

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirrobot01/decypharr/internal/request"
)

func TestIsAvailable_UppercasesAndFiltersZeroSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrents/checkcached" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("hash") == "" {
			t.Fatalf("expected hash query parameter")
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"abc":{"size":10},"def":{"size":0}}}`))
	}))
	defer srv.Close()

	tb := &Torbox{
		Host:   srv.URL,
		client: request.New(request.WithMaxRetries(0)),
	}

	got := tb.IsAvailable([]string{"abc", "def"})
	if len(got) != 1 {
		t.Fatalf("availability size = %d, want 1", len(got))
	}
	if !got["ABC"] {
		t.Fatalf("expected uppercase key ABC to be cached, got: %#v", got)
	}
	if got["DEF"] {
		t.Fatalf("DEF should not be marked cached when size=0")
	}
}

func TestIsAvailable_ServerFailureReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failure", http.StatusGatewayTimeout)
	}))
	defer srv.Close()

	tb := &Torbox{
		Host:   srv.URL,
		client: request.New(request.WithMaxRetries(0)),
	}
	got := tb.IsAvailable([]string{"abc"})
	if len(got) != 0 {
		t.Fatalf("availability size = %d, want 0 on upstream error", len(got))
	}
}
