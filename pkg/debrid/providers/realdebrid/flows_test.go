package realdebrid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/customerror"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "realdebrid-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	config.SetConfigPath(dir)
	os.Exit(m.Run())
}

func newTestRD(host string, useTorrentFile bool) *RealDebrid {
	return &RealDebrid{
		Host:         host,
		APIKey:       "rd-key",
		client:       request.New(request.WithMaxRetries(0), request.WithRetryableStatus(http.StatusTooManyRequests)),
		repairClient: request.New(request.WithMaxRetries(0), request.WithRetryableStatus(http.StatusTooManyRequests)),
		rarSemaphore: make(chan struct{}, 1),
		config: config.Debrid{
			Name:           "realdebrid",
			UseTorrentFile: &useTorrentFile,
		},
	}
}

func TestSubmitMagnet_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/torrents/addMagnet" {
			t.Fatalf("path = %s, want /torrents/addMagnet", r.URL.Path)
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	_, err := rd.SubmitMagnet(&types.Torrent{Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"}})
	if err == nil || !strings.Contains(err.Error(), "Status: 400") {
		t.Fatalf("error = %v, want status 400", err)
	}
}

func TestGetTorrent_NotFoundAndUpdateMapping(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		rd := newTestRD(srv.URL, false)
		_, err := rd.GetTorrent("404")
		if !errors.Is(err, customerror.TorrentNotFoundError) {
			t.Fatalf("error = %v, want TorrentNotFoundError", err)
		}
	})

	t.Run("update maps waiting_files_selection", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":"x1","filename":"Show","original_filename":"Show","bytes":1000,"progress":10,"status":"waiting_files_selection","speed":55,"seeders":9,"links":[],"files":[{"id":1,"path":"Show/ep1.mkv","bytes":1000,"selected":0}]}`))
		}))
		defer srv.Close()

		rd := newTestRD(srv.URL, false)
		tor := &types.Torrent{Id: "x1", Files: make(map[string]types.File)}
		if err := rd.UpdateTorrent(tor); err != nil {
			t.Fatalf("UpdateTorrent error: %v", err)
		}
		if tor.Status != types.TorrentStatusDownloading {
			t.Fatalf("status = %q, want downloading", tor.Status)
		}
	})
}

func TestCheckStatus_WaitingSelectionThenDownloaded(t *testing.T) {
	var infoCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/torrents/info/"):
			infoCalls++
			if infoCalls == 1 {
				_, _ = w.Write([]byte(`{"id":"z1","filename":"Pack","original_filename":"Pack","hash":"abcdef","bytes":200,"progress":20,"status":"waiting_files_selection","links":[],"files":[{"id":10,"path":"Pack/ep1.mkv","bytes":100,"selected":1},{"id":11,"path":"Pack/ep2.mkv","bytes":100,"selected":1}],"added":"2024-01-01T00:00:00Z"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"z1","filename":"Pack","original_filename":"Pack","hash":"abcdef","bytes":200,"progress":100,"status":"downloaded","links":["http://dl/1","http://dl/2"],"files":[{"id":10,"path":"Pack/ep1.mkv","bytes":100,"selected":1},{"id":11,"path":"Pack/ep2.mkv","bytes":100,"selected":1}],"added":"2024-01-01T00:00:00Z"}`))
		case strings.HasPrefix(r.URL.Path, "/torrents/selectFiles/"):
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	tor := &types.Torrent{Id: "z1", Files: make(map[string]types.File), DownloadUncached: true}
	got, err := rd.CheckStatus(tor)
	if err != nil {
		t.Fatalf("CheckStatus error: %v", err)
	}
	if got.Status != types.TorrentStatusDownloaded {
		t.Fatalf("status = %q, want downloaded", got.Status)
	}
	if got.InfoHash != "abcdef" {
		t.Fatalf("infohash = %q, want abcdef", got.InfoHash)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files len = %d, want 2", len(got.Files))
	}
	if got.Files["ep1.mkv"].Link == "" || got.Files["ep2.mkv"].Link == "" {
		t.Fatalf("expected selected files with links, got %#v", got.Files)
	}
}

func TestIsAvailable_AtypicalFormats(t *testing.T) {
	h1 := strings.Repeat("a", 40)
	h2 := strings.Repeat("b", 40)
	h3 := strings.Repeat("c", 40)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf("/torrents/instantAvailability/%s/%s/%s", strings.ToUpper(h1), h2, h3)
		if r.URL.Path != want {
			t.Fatalf("path = %s, want %s", r.URL.Path, want)
		}
		// h1 has rd entries => available
		// h2 uses empty array hoster payload => unavailable (atypical but valid)
		// h3 has rd empty => unavailable
		_, _ = w.Write([]byte(`{"` + h1 + `":{"rd":[{"1":{"filename":"ok","filesize":1}}]},"` + h2 + `":[],"` + h3 + `":{"rd":[]}}`))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	got := rd.IsAvailable([]string{strings.ToUpper(h1), h2, h3})
	if !got[strings.ToUpper(h1)] {
		t.Fatalf("expected first hash to be available, got %#v", got)
	}
	if got[h2] || got[h3] {
		t.Fatalf("unexpected availability for atypical empty hosters: %#v", got)
	}
}

func TestDeleteTorrent_StatusHandling(t *testing.T) {
	var call int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/torrents/delete/id-1" {
			t.Fatalf("path = %s, want /torrents/delete/id-1", r.URL.Path)
		}
		if call == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "upstream", http.StatusTeapot)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	if err := rd.DeleteTorrent("id-1"); err != nil {
		t.Fatalf("DeleteTorrent first call error: %v", err)
	}
	err := rd.DeleteTorrent("id-1")
	if err == nil || !strings.Contains(err.Error(), "Status: 418") {
		t.Fatalf("DeleteTorrent second error = %v, want status 418", err)
	}
}

func TestGetTorrent_ZeroAddedFallsBackToNow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"g1","filename":"Movie","original_filename":"Movie","bytes":1,"progress":1,"status":"downloaded","files":[],"links":[]}`))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	before := time.Now().Add(-2 * time.Second)
	got, err := rd.GetTorrent("g1")
	if err != nil {
		t.Fatalf("GetTorrent error: %v", err)
	}
	if got.Added.Before(before) {
		t.Fatalf("expected Added fallback to now, got %s", got.Added)
	}
}

func TestGetTorrent_TimeoutPropagation(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer func() {
		close(release)
		srv.Close()
	}()

	rd := &RealDebrid{
		Host: srv.URL,
		client: request.New(
			request.WithMaxRetries(0),
			request.WithTransport(&http.Transport{ResponseHeaderTimeout: 20 * time.Millisecond}),
		),
		repairClient: request.New(request.WithMaxRetries(0)),
		rarSemaphore: make(chan struct{}, 1),
		config:       config.Debrid{Name: "realdebrid"},
	}

	_, err := rd.GetTorrent("g1")
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "timeout") && !strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")) {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestGetTorrent_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"g1","filename":`))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	_, err := rd.GetTorrent("g1")
	if err == nil {
		t.Fatal("expected decode error for malformed payload, got nil")
	}
}

func TestIsAvailable_BatchedPartialFailures(t *testing.T) {
	hashes := make([]string, 0, 401)
	for i := 0; i < 401; i++ {
		hashes = append(hashes, fmt.Sprintf("hash-%03d", i))
	}
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if !strings.HasPrefix(r.URL.Path, "/torrents/instantAvailability/") {
			http.NotFound(w, r)
			return
		}
		if n == 1 {
			_, _ = w.Write([]byte(`{"hash-000":{"rd":[{"1":{"filename":"ok","filesize":1}}]}}`))
			return
		}
		http.Error(w, "upstream error", http.StatusBadGateway)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	got := rd.IsAvailable(hashes)
	if len(got) != 1 || !got["hash-000"] {
		t.Fatalf("availability = %#v, want only hash-000", got)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3 batches", calls.Load())
	}
}

func TestCheckStatus_UnknownStateReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"z2","filename":"Pack","original_filename":"Pack","bytes":200,"progress":50,"status":"state_from_future","links":[],"files":[],"added":"2024-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	tor := &types.Torrent{Id: "z2", Files: map[string]types.File{}, DownloadUncached: true}
	_, err := rd.CheckStatus(tor)
	if err == nil || !strings.Contains(err.Error(), "has error") {
		t.Fatalf("error = %v, want has error", err)
	}
}

func TestCheckFile_HosterUnavailableMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	err := rd.CheckFile(context.Background(), "hash", "http://hoster/file")
	if !errors.Is(err, customerror.HosterUnavailableError) {
		t.Fatalf("error = %v, want HosterUnavailableError", err)
	}
}
