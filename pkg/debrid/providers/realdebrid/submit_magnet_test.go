package realdebrid

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestSubmitMagnet_MagnetLink_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/torrents/addMagnet" {
			t.Fatalf("path = %s, want /torrents/addMagnet", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("magnet") == "" {
			t.Fatalf("missing magnet form field")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"rd-42","uri":"http://api/torrents/info/rd-42"}`))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	got, err := rd.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"},
		Files:  make(map[string]types.File),
	})
	if err != nil {
		t.Fatalf("SubmitMagnet error: %v", err)
	}
	if got.Id != "rd-42" {
		t.Errorf("Id = %q, want rd-42", got.Id)
	}
	if got.Debrid != "realdebrid" {
		t.Errorf("Debrid = %q, want realdebrid", got.Debrid)
	}
	if got.Added.IsZero() {
		t.Error("Added time should be set")
	}
}

func TestSubmitMagnet_TorrentFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/torrents/addTorrent" {
			t.Fatalf("path = %s, want /torrents/addTorrent", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-bittorrent" {
			t.Fatalf("content-type = %q, want application/x-bittorrent", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Fatal("empty request body for torrent file upload")
		}
		_, _ = w.Write([]byte(`{"id":"rd-99","uri":"http://api/torrents/info/rd-99"}`))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, true)
	got, err := rd.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("fake torrent content")},
		Files:  make(map[string]types.File),
	})
	if err != nil {
		t.Fatalf("SubmitMagnet error: %v", err)
	}
	if got.Id != "rd-99" {
		t.Errorf("Id = %q, want rd-99", got.Id)
	}
	if got.Debrid != "realdebrid" {
		t.Errorf("Debrid = %q, want realdebrid", got.Debrid)
	}
}

func TestSubmitMagnet_TorrentFile_Non2xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, true)
	_, err := rd.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("data")},
		Files:  make(map[string]types.File),
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected status code: 403") {
		t.Fatalf("error = %v, want unexpected status code 403", err)
	}
}

func TestSubmitMagnet_TorrentFile_Non2xxNon509(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, true)
	_, err := rd.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("data")},
		Files:  make(map[string]types.File),
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected status code: 400") {
		t.Fatalf("error = %v, want unexpected status code 400", err)
	}
}

func TestSubmitMagnet_FallsBackToLinkWhenNoFile(t *testing.T) {
	var hitPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"rd-55","uri":"x"}`))
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, true) // useTorrentFile=true but File is nil
	_, err := rd.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"},
		Files:  make(map[string]types.File),
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if hitPath != "/torrents/addMagnet" {
		t.Errorf("path = %s, want /torrents/addMagnet (link fallback)", hitPath)
	}
}

func TestSubmitMagnet_PathDistinction(t *testing.T) {
	var lastPath string
	var lastMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		lastMethod = r.Method
		_, _ = w.Write([]byte(`{"id":"x","uri":"x"}`))
	}))
	defer srv.Close()

	t.Run("file upload uses PUT to addTorrent", func(t *testing.T) {
		rd := newTestRD(srv.URL, true)
		_, _ = rd.SubmitMagnet(&types.Torrent{
			Magnet: &utils.Magnet{File: []byte("torrent bytes")},
			Files:  make(map[string]types.File),
		})
		if lastPath != "/torrents/addTorrent" {
			t.Errorf("path = %s, want /torrents/addTorrent", lastPath)
		}
		if lastMethod != http.MethodPut {
			t.Errorf("method = %s, want PUT", lastMethod)
		}
	})

	t.Run("magnet link uses POST to addMagnet", func(t *testing.T) {
		rd := newTestRD(srv.URL, false)
		_, _ = rd.SubmitMagnet(&types.Torrent{
			Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:xyz"},
			Files:  make(map[string]types.File),
		})
		if lastPath != "/torrents/addMagnet" {
			t.Errorf("path = %s, want /torrents/addMagnet", lastPath)
		}
		if lastMethod != http.MethodPost {
			t.Errorf("method = %s, want POST", lastMethod)
		}
	})
}
