package debridlink

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "debridlink-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	config.SetConfigPath(dir)
	os.Exit(m.Run())
}

func makeHashes(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("hash-%03d", i))
	}
	return out
}

func TestIsAvailable_BatchedAndPartialFailures(t *testing.T) {
	hashes := makeHashes(101) // force two batches
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if !strings.HasPrefix(r.URL.Path, "/seedbox/cached/") {
			http.NotFound(w, r)
			return
		}

		if n == 1 {
			// First batch returns success for a subset.
			_, _ = w.Write([]byte(`{"success":true,"value":{"hash-000":{"a":{"name":"f1"}},"hash-050":{"b":{"name":"f2"}}}}`))
			return
		}

		// Second batch fails: method must skip this batch and keep prior results.
		http.Error(w, "upstream error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dl := &DebridLink{
		Host:   srv.URL,
		client: request.New(request.WithMaxRetries(0)),
	}

	got := dl.IsAvailable(hashes)
	if len(got) != 2 {
		t.Fatalf("availability size = %d, want 2", len(got))
	}
	if !got["hash-000"] || !got["hash-050"] {
		t.Fatalf("expected hash-000 and hash-050 to be cached, got: %#v", got)
	}
	if calls.Load() != 2 {
		t.Fatalf("request calls = %d, want 2 (batched requests)", calls.Load())
	}
}

func TestIsAvailable_NilValueStopsEarly(t *testing.T) {
	hashes := makeHashes(150) // would be 2 batches if not stopped
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"success":true,"value":null}`))
	}))
	defer srv.Close()

	dl := &DebridLink{
		Host:   srv.URL,
		client: request.New(request.WithMaxRetries(0)),
	}
	got := dl.IsAvailable(hashes)
	if len(got) != 0 {
		t.Fatalf("availability size = %d, want 0 for nil payload value", len(got))
	}
	if calls.Load() != 1 {
		t.Fatalf("request calls = %d, want 1 (nil value should stop processing)", calls.Load())
	}
}

func TestIsAvailable_MalformedPayloadIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"value":`)) // invalid JSON
	}))
	defer srv.Close()

	dl := &DebridLink{
		Host:   srv.URL,
		client: request.New(request.WithMaxRetries(0)),
	}
	got := dl.IsAvailable([]string{"hash-a"})
	if len(got) != 0 {
		t.Fatalf("availability size = %d, want 0 for malformed response", len(got))
	}
}
