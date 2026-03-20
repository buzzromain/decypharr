package alldebrid

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "alldebrid-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	config.SetConfigPath(dir)
	os.Exit(m.Run())
}

func newTestAD(host string, useTorrentFile bool) *AllDebrid {
	return &AllDebrid{
		Host:   host,
		APIKey: "ad-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{
			Name:           "alldebrid",
			UseTorrentFile: &useTorrentFile,
		},
	}
}

func TestSubmitMagnet_LinkFlow_SuccessAndEmptyMagnets(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/magnet/upload" {
				t.Fatalf("path = %s, want /magnet/upload", r.URL.Path)
			}
			if r.URL.Query().Get("magnets[]") == "" {
				t.Fatalf("missing magnets[] query")
			}
			_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":[{"id":12}]}}`))
		}))
		defer srv.Close()

		ad := newTestAD(srv.URL, false)
		got, err := ad.SubmitMagnet(&types.Torrent{Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"}})
		if err != nil {
			t.Fatalf("SubmitMagnet error: %v", err)
		}
		if got.Id != "12" {
			t.Fatalf("id = %q, want 12", got.Id)
		}
		if got.Added.IsZero() {
			t.Fatal("added time should be set")
		}
	})

	t.Run("empty magnets", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":[]}}`))
		}))
		defer srv.Close()

		ad := newTestAD(srv.URL, false)
		_, err := ad.SubmitMagnet(&types.Torrent{Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"}})
		if err == nil || !strings.Contains(err.Error(), "No magnets returned") {
			t.Fatalf("error = %v, want no magnets returned", err)
		}
	})
}

func TestGetAndUpdateTorrent_CycleAndPartialResponse(t *testing.T) {
	var call int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/magnet/status" {
			t.Fatalf("path = %s, want /magnet/status", r.URL.Path)
		}
		call++
		if call == 1 {
			_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":{"id":1,"filename":"Show","size":100,"hash":"abc","statusCode":4,"completionDate":1700000000,"seeders":5,"files":[{"n":"season1","e":[{"n":"ep1.mkv","s":100,"l":"http://link"}]}]}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":{"id":1,"filename":"Show","size":100,"hash":"abc","statusCode":2,"downloaded":25,"downloadSpeed":77,"completionDate":1700000000,"seeders":2}}}`))
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, false)
	got, err := ad.GetTorrent("1")
	if err != nil {
		t.Fatalf("GetTorrent error: %v", err)
	}
	if got.Status != types.TorrentStatusDownloaded {
		t.Fatalf("status = %q, want downloaded", got.Status)
	}
	if got.Progress != 100 {
		t.Fatalf("progress = %v, want 100", got.Progress)
	}
	if _, ok := got.Files["ep1.mkv"]; !ok {
		t.Fatalf("expected flattened file ep1.mkv, got %#v", got.Files)
	}

	tor := &types.Torrent{Id: "1", Files: make(map[string]types.File)}
	if err := ad.UpdateTorrent(tor); err != nil {
		t.Fatalf("UpdateTorrent error: %v", err)
	}
	if tor.Status != types.TorrentStatusDownloading {
		t.Fatalf("updated status = %q, want downloading", tor.Status)
	}
	if tor.Progress != 25 {
		t.Fatalf("updated progress = %v, want 25", tor.Progress)
	}
}

func TestCheckStatusAndDeleteTorrent_Errors(t *testing.T) {
	t.Run("not cached", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":{"id":2,"filename":"NotCached","size":100,"statusCode":2,"downloaded":10,"downloadSpeed":1}}}`))
		}))
		defer srv.Close()

		ad := newTestAD(srv.URL, false)
		tor := &types.Torrent{Id: "2", DownloadUncached: false, Files: make(map[string]types.File)}
		_, err := ad.CheckStatus(tor)
		if err == nil || !strings.Contains(err.Error(), "not cached") {
			t.Fatalf("error = %v, want not cached", err)
		}
	})

	t.Run("delete non-2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/magnet/delete" || r.URL.Query().Get("id") != "99" {
				t.Fatalf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
			}
			http.Error(w, "upstream", http.StatusTeapot)
		}))
		defer srv.Close()

		ad := newTestAD(srv.URL, false)
		err := ad.DeleteTorrent("99")
		if err == nil || !strings.Contains(err.Error(), "Status: 418") {
			t.Fatalf("error = %v, want status 418", err)
		}
	})
}

func TestGetTorrents_ObjectMagnetsPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "ready" {
			t.Fatalf("status query = %q, want ready", r.URL.Query().Get("status"))
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":{"first":{"id":3,"filename":"A","size":10,"statusCode":4,"completionDate":1700000000,"files":[]},"second":{"id":4,"filename":"B","size":20,"statusCode":1,"completionDate":1700000001,"files":[]}}}}`))
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, false)
	got, err := ad.GetTorrents()
	if err != nil {
		t.Fatalf("GetTorrents error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(torrents) = %d, want 2", len(got))
	}
	if got[0].Added.Equal(time.Time{}) || got[1].Added.Equal(time.Time{}) {
		t.Fatal("expected completionDate mapping to Added for all torrents")
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

	ad := &AllDebrid{
		Host: srv.URL,
		client: request.New(
			request.WithMaxRetries(0),
			request.WithTransport(&http.Transport{ResponseHeaderTimeout: 20 * time.Millisecond}),
		),
		config: config.Debrid{Name: "alldebrid"},
	}

	_, err := ad.GetTorrent("1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestGetTorrent_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":`))
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, false)
	_, err := ad.GetTorrent("1")
	if err == nil {
		t.Fatal("expected decode error for malformed payload, got nil")
	}
}
