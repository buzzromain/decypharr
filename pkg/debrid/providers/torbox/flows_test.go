package torbox

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func tbBoolPtr(v bool) *bool { return &v }

func newTestTorbox(host string, useTorrentFile bool) *Torbox {
	return &Torbox{
		Host:   host,
		APIKey: "tb-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{
			Name:           "torbox",
			UseTorrentFile: tbBoolPtr(useTorrentFile),
		},
	}
}

func TestSubmitMagnet_FormFlow_SuccessAndError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/api/torrents/createtorrent" {
				t.Fatalf("path = %s, want /api/torrents/createtorrent", r.URL.Path)
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
				t.Fatalf("content-type = %q, want form-encoded", r.Header.Get("Content-Type"))
			}
			body, _ := io.ReadAll(r.Body)
			vals, _ := url.ParseQuery(string(body))
			if vals.Get("magnet") == "" {
				t.Fatalf("missing magnet form field")
			}
			if vals.Get("add_only_if_cached") != "true" {
				t.Fatalf("add_only_if_cached = %q, want true", vals.Get("add_only_if_cached"))
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"torrent_id":42,"hash":"deadbeef"}}`))
		}))
		defer srv.Close()

		tb := newTestTorbox(srv.URL, false)
		tor := &types.Torrent{Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"}, Files: map[string]types.File{}, DownloadUncached: false}
		got, err := tb.SubmitMagnet(tor)
		if err != nil {
			t.Fatalf("SubmitMagnet error: %v", err)
		}
		if got.Id != "42" {
			t.Fatalf("id = %q, want 42", got.Id)
		}
		if got.Debrid != "torbox" {
			t.Fatalf("debrid = %q, want torbox", got.Debrid)
		}
	})

	t.Run("non 2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "upstream", http.StatusTeapot)
		}))
		defer srv.Close()

		tb := newTestTorbox(srv.URL, false)
		_, err := tb.SubmitMagnet(&types.Torrent{Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"}, Files: map[string]types.File{}})
		if err == nil || !strings.Contains(err.Error(), "Status: 418") {
			t.Fatalf("error = %v, want status 418", err)
		}
	})
}

func TestGetAndUpdateTorrent_StatusAndFiles(t *testing.T) {
	const payload = `{"success":true,"data":{"id":7,"name":"Pack","size":1000,"progress":0.4,"download_state":"downloading (queued)","download_finished":false,"download_speed":111,"seeds":12,"hash":"abc123","created_at":"2024-01-01T00:00:00Z","files":[{"id":9,"name":"root/folder/file.mkv","absolute_path":"/mnt/root/folder/file.mkv","size":900}]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrents/mylist/" {
			t.Fatalf("path = %s, want /api/torrents/mylist/", r.URL.Path)
		}
		if r.URL.Query().Get("id") == "" {
			t.Fatalf("missing id query")
		}
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	got, err := tb.GetTorrent("7")
	if err != nil {
		t.Fatalf("GetTorrent error: %v", err)
	}
	if got.Status != types.TorrentStatusDownloading {
		t.Fatalf("status = %q, want downloading", got.Status)
	}
	if got.Progress != 40 {
		t.Fatalf("progress = %v, want 40", got.Progress)
	}
	if got.OriginalFilename != "root" {
		t.Fatalf("original filename = %q, want root", got.OriginalFilename)
	}

	tor := &types.Torrent{Id: "7", Files: make(map[string]types.File)}
	if err := tb.UpdateTorrent(tor); err != nil {
		t.Fatalf("UpdateTorrent error: %v", err)
	}
	if tor.Status != types.TorrentStatusDownloading {
		t.Fatalf("updated status = %q, want downloading", tor.Status)
	}
	if tor.InfoHash != "abc123" {
		t.Fatalf("updated infohash = %q, want abc123", tor.InfoHash)
	}
	if _, ok := tor.Files["file.mkv"]; !ok {
		b, _ := json.Marshal(tor.Files)
		t.Fatalf("expected file.mkv in files, got %s", string(b))
	}
}

func TestDeleteTorrent_RequestContractAndError(t *testing.T) {
	var call int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/torrents/controltorrent/11" {
			t.Fatalf("path = %s, want /api/torrents/controltorrent/11", r.URL.Path)
		}
		if call == 1 {
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type = %q, want application/json", ct)
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"action":"Delete"`) || !strings.Contains(string(body), `"torrent_id":"11"`) {
				t.Fatalf("unexpected delete payload: %s", string(body))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "nope", http.StatusTeapot)
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	if err := tb.DeleteTorrent("11"); err != nil {
		t.Fatalf("DeleteTorrent first call error: %v", err)
	}
	err := tb.DeleteTorrent("11")
	if err == nil || !strings.Contains(err.Error(), "Status: 418") {
		t.Fatalf("DeleteTorrent second error = %v, want status 418", err)
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

	tb := &Torbox{
		Host: srv.URL,
		client: request.New(
			request.WithMaxRetries(0),
			request.WithTransport(&http.Transport{ResponseHeaderTimeout: 20 * time.Millisecond}),
		),
		config: config.Debrid{Name: "torbox"},
	}
	_, err := tb.GetTorrent("7")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestCheckStatus_NotCachedWhenDownloading(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"name":"Pack","size":1000,"progress":0.4,"download_state":"downloading","download_finished":false,"download_speed":111,"seeds":12,"hash":"abc123","created_at":"2024-01-01T00:00:00Z","files":[]}}`))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, false)
	tor := &types.Torrent{Id: "7", DownloadUncached: false, Files: map[string]types.File{}}
	_, err := tb.CheckStatus(tor)
	if err == nil || !strings.Contains(err.Error(), "not cached") {
		t.Fatalf("error = %v, want not cached", err)
	}
}
