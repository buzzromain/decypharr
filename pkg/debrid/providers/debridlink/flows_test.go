package debridlink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func boolPtr(v bool) *bool { return &v }

func newTestDL(host string, useTorrentFile bool) *DebridLink {
	return &DebridLink{
		Host:                  host,
		APIKey:                "api-key",
		client:                request.New(request.WithMaxRetries(0)),
		autoExpiresLinksAfter: time.Hour,
		config: config.Debrid{
			Name:           "debridlink",
			UseTorrentFile: boolPtr(useTorrentFile),
		},
	}
}

func TestSubmitMagnet_TorrentFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/seedbox/add" {
			t.Fatalf("path = %s, want /seedbox/add", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("content-type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(`{"success":true,"value":{"id":"dl-1","name":"Movie: 2026","totalSize":1234,"created":1700000000}}`))
	}))
	defer srv.Close()

	dl := newTestDL(srv.URL, true)
	in := &types.Torrent{
		Magnet: &utils.Magnet{File: []byte("dummy torrent bytes")},
		Files:  make(map[string]types.File),
	}

	got, err := dl.SubmitMagnet(in)
	if err != nil {
		t.Fatalf("SubmitMagnet error: %v", err)
	}
	if got.Id != "dl-1" {
		t.Fatalf("id = %q, want dl-1", got.Id)
	}
	if got.Status != types.TorrentStatusDownloading {
		t.Fatalf("status = %q, want %q", got.Status, types.TorrentStatusDownloading)
	}
	if got.Debrid != "debridlink" {
		t.Fatalf("debrid = %q, want debridlink", got.Debrid)
	}
}

func TestSubmitMagnet_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusTeapot)
	}))
	defer srv.Close()

	dl := newTestDL(srv.URL, true)
	_, err := dl.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("dummy")},
		Files:  make(map[string]types.File),
	})
	if err == nil || !strings.Contains(err.Error(), "Status: 418") {
		t.Fatalf("error = %v, want API status 418", err)
	}
}

func TestGetTorrent_SuccessAndError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/seedbox/t-1" {
				t.Fatalf("path = %s, want /seedbox/t-1", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"success":true,"value":[{"id":"t-1","name":"My:Movie","totalSize":99,"created":1700000000,"files":[]}]}`))
		}))
		defer srv.Close()

		dl := newTestDL(srv.URL, true)
		got, err := dl.GetTorrent("t-1")
		if err != nil {
			t.Fatalf("GetTorrent error: %v", err)
		}
		if got.Id != "t-1" || got.Bytes != 99 {
			t.Fatalf("unexpected torrent: %+v", got)
		}
	})

	t.Run("upstream status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "err", http.StatusUnauthorized)
		}))
		defer srv.Close()

		dl := newTestDL(srv.URL, true)
		_, err := dl.GetTorrent("t-1")
		if err == nil || !strings.Contains(err.Error(), "Status: 401") {
			t.Fatalf("error = %v, want status 401", err)
		}
	})
}

func TestUpdateTorrent_And_DeleteTorrent(t *testing.T) {
	t.Run("update success maps fields", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/seedbox/list" {
				t.Fatalf("path = %s, want /seedbox/list", r.URL.Path)
			}
			if r.URL.Query().Get("ids") != "abc" {
				t.Fatalf("ids = %q, want abc", r.URL.Query().Get("ids"))
			}
			_, _ = w.Write([]byte(`{"success":true,"value":[{"id":"abc","name":"Pack:One","hashString":"deadbeef","totalSize":777,"status":100,"downloadPercent":42.5,"downloadSpeed":123,"peersConnected":9,"created":1700000000,"files":[]}]}`))
		}))
		defer srv.Close()

		dl := newTestDL(srv.URL, true)
		tor := &types.Torrent{Id: "abc", Files: make(map[string]types.File)}
		if err := dl.UpdateTorrent(tor); err != nil {
			t.Fatalf("UpdateTorrent error: %v", err)
		}
		if tor.Status != types.TorrentStatusDownloaded {
			t.Fatalf("status = %q, want downloaded", tor.Status)
		}
		if tor.InfoHash != "deadbeef" {
			t.Fatalf("infohash = %q, want deadbeef", tor.InfoHash)
		}
	})

	t.Run("delete status handling", func(t *testing.T) {
		var hit int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hit++
			if r.Method != http.MethodDelete {
				t.Fatalf("method = %s, want DELETE", r.Method)
			}
			if r.URL.Path != "/seedbox/x/remove" {
				t.Fatalf("path = %s, want /seedbox/x/remove", r.URL.Path)
			}
			if hit == 1 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.Error(w, "fail", http.StatusTeapot)
		}))
		defer srv.Close()

		dl := newTestDL(srv.URL, true)
		if err := dl.DeleteTorrent("x"); err != nil {
			t.Fatalf("DeleteTorrent first call error: %v", err)
		}
		err := dl.DeleteTorrent("x")
		if err == nil || !strings.Contains(err.Error(), "Status: 418") {
			t.Fatalf("second delete error = %v, want status 418", err)
		}
	})
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

	dl := &DebridLink{
		Host: srv.URL,
		client: request.New(
			request.WithMaxRetries(0),
			request.WithTransport(&http.Transport{ResponseHeaderTimeout: 20 * time.Millisecond}),
		),
		config: config.Debrid{Name: "debridlink"},
	}
	_, err := dl.GetTorrent("t-1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestUpdateTorrent_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"value":`))
	}))
	defer srv.Close()

	dl := newTestDL(srv.URL, true)
	err := dl.UpdateTorrent(&types.Torrent{Id: "x", Files: map[string]types.File{}})
	if err == nil {
		t.Fatal("expected decode error for malformed payload, got nil")
	}
}

func TestCheckStatus_NotCachedWhenDownloading(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"value":[{"id":"abc","name":"Pack","totalSize":777,"status":10,"downloadPercent":42.5,"downloadSpeed":123,"peersConnected":9,"created":1700000000,"files":[]}]}`))
	}))
	defer srv.Close()

	dl := newTestDL(srv.URL, true)
	tor := &types.Torrent{Id: "abc", DownloadUncached: false, Files: map[string]types.File{}}
	_, err := dl.CheckStatus(tor)
	if err == nil || !strings.Contains(err.Error(), "not cached") {
		t.Fatalf("error = %v, want not cached", err)
	}
}
