package torbox

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestSubmitMagnet_TorrentFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrents/createtorrent" {
			t.Fatalf("path = %s, want /api/torrents/createtorrent", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("content-type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm error: %v", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("missing file form field: %v", err)
		}
		file.Close()

		_, _ = w.Write([]byte(`{"success":true,"data":{"torrent_id":101,"hash":"deadbeef"}}`))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, true)
	got, err := tb.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("fake torrent data")},
		Files:  make(map[string]types.File),
	})
	if err != nil {
		t.Fatalf("SubmitMagnet error: %v", err)
	}
	if got.Id != "101" {
		t.Errorf("Id = %q, want 101", got.Id)
	}
	if got.Debrid != "torbox" {
		t.Errorf("Debrid = %q, want torbox", got.Debrid)
	}
	if got.Added.IsZero() {
		t.Error("Added time should be set")
	}
}

func TestSubmitMagnet_TorrentFile_AddOnlyIfCached(t *testing.T) {
	tests := []struct {
		name             string
		downloadUncached bool
		wantCachedField  bool
	}{
		{"cached only", false, true},
		{"allow uncached", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Fatalf("ParseMultipartForm error: %v", err)
				}
				val := r.FormValue("add_only_if_cached")
				if tt.wantCachedField && val != "true" {
					t.Errorf("add_only_if_cached = %q, want true", val)
				}
				if !tt.wantCachedField && val != "" {
					t.Errorf("add_only_if_cached = %q, want empty (not sent)", val)
				}
				_, _ = w.Write([]byte(`{"success":true,"data":{"torrent_id":1,"hash":"h"}}`))
			}))
			defer srv.Close()

			tb := newTestTorbox(srv.URL, true)
			_, err := tb.SubmitMagnet(&types.Torrent{
				Magnet:           &utils.Magnet{File: []byte("data")},
				Files:            make(map[string]types.File),
				DownloadUncached: tt.downloadUncached,
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestSubmitMagnet_TorrentFile_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, true)
	_, err := tb.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("data")},
		Files:  make(map[string]types.File),
	})
	if err == nil || !strings.Contains(err.Error(), "Status: 403") {
		t.Fatalf("error = %v, want status 403", err)
	}
}

func TestSubmitMagnet_TorrentFile_NilDataResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":null}`))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, true)
	_, err := tb.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("data")},
		Files:  make(map[string]types.File),
	})
	if err == nil || !strings.Contains(err.Error(), "error adding torrent") {
		t.Fatalf("error = %v, want error adding torrent", err)
	}
}

func TestSubmitMagnet_FallsBackToFormWhenNoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should be form-encoded, not multipart
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			t.Fatalf("content-type = %q, want form-encoded", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "magnet=") {
			t.Fatalf("missing magnet form field in body: %s", body)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"torrent_id":50,"hash":"h"}}`))
	}))
	defer srv.Close()

	tb := newTestTorbox(srv.URL, true) // useTorrentFile=true but File is nil
	_, err := tb.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"},
		Files:  make(map[string]types.File),
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestSubmitMagnet_PathDistinction(t *testing.T) {
	var lastContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastContentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"success":true,"data":{"torrent_id":1,"hash":"h"}}`))
	}))
	defer srv.Close()

	t.Run("file upload uses multipart", func(t *testing.T) {
		tb := newTestTorbox(srv.URL, true)
		_, _ = tb.SubmitMagnet(&types.Torrent{
			Magnet: &utils.Magnet{File: []byte("torrent bytes")},
			Files:  make(map[string]types.File),
		})
		if !strings.HasPrefix(lastContentType, "multipart/form-data") {
			t.Errorf("content-type = %q, want multipart/form-data", lastContentType)
		}
	})

	t.Run("magnet link uses form-encoded", func(t *testing.T) {
		tb := newTestTorbox(srv.URL, false)
		_, _ = tb.SubmitMagnet(&types.Torrent{
			Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"},
			Files:  make(map[string]types.File),
		})
		if !strings.HasPrefix(lastContentType, "application/x-www-form-urlencoded") {
			t.Errorf("content-type = %q, want form-encoded", lastContentType)
		}
	})
}
