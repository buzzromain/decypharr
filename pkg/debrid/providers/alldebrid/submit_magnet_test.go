package alldebrid

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestSubmitMagnet_TorrentFileUpload_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/magnet/upload/file" {
			t.Fatalf("path = %s, want /magnet/upload/file", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("content-type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		}
		// Verify file data was sent
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm error: %v", err)
		}
		file, _, err := r.FormFile("files[]")
		if err != nil {
			t.Fatalf("missing files[] form file: %v", err)
		}
		file.Close()

		_, _ = w.Write([]byte(`{"status":"success","data":{"files":[{"id":77}]}}`))
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, true)
	tor := &types.Torrent{
		Magnet: &utils.Magnet{File: []byte("fake torrent data")},
		Files:  make(map[string]types.File),
	}
	got, err := ad.SubmitMagnet(tor)
	if err != nil {
		t.Fatalf("SubmitMagnet error: %v", err)
	}
	if got.Id != "77" {
		t.Errorf("Id = %q, want 77", got.Id)
	}
	if got.Added.IsZero() {
		t.Error("Added time should be set")
	}
}

func TestSubmitMagnet_TorrentFileUpload_EmptyFilesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"files":[]}}`))
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, true)
	_, err := ad.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("data")},
		Files:  make(map[string]types.File),
	})
	if err == nil || !strings.Contains(err.Error(), "No files returned") {
		t.Fatalf("error = %v, want no files returned", err)
	}
}

func TestSubmitMagnet_TorrentFileUpload_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, true)
	_, err := ad.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{File: []byte("data")},
		Files:  make(map[string]types.File),
	})
	if err == nil || !strings.Contains(err.Error(), "Status: 403") {
		t.Fatalf("error = %v, want status 403", err)
	}
}

func TestSubmitMagnet_FallsBackToLinkWhenNoTorrentFile(t *testing.T) {
	// Even with useTorrentFile=true, if Magnet.File is nil, should use link path
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/magnet/upload" {
			t.Fatalf("path = %s, want /magnet/upload (link path)", r.URL.Path)
		}
		if r.URL.Query().Get("magnets[]") == "" {
			t.Fatalf("missing magnets[] query param")
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":[{"id":88}]}}`))
	}))
	defer srv.Close()

	ad := newTestAD(srv.URL, true) // useTorrentFile=true but File is nil
	got, err := ad.SubmitMagnet(&types.Torrent{
		Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"},
		Files:  make(map[string]types.File),
	})
	if err != nil {
		t.Fatalf("SubmitMagnet error: %v", err)
	}
	if got.Id != "88" {
		t.Errorf("Id = %q, want 88", got.Id)
	}
}

func TestSubmitMagnet_PathDistinction(t *testing.T) {
	// Verify that torrent file path hits /magnet/upload/file
	// and link path hits /magnet/upload
	var hitPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		switch r.URL.Path {
		case "/magnet/upload/file":
			_, _ = w.Write([]byte(`{"status":"success","data":{"files":[{"id":1}]}}`))
		case "/magnet/upload":
			_, _ = w.Write([]byte(`{"status":"success","data":{"magnets":[{"id":2}]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Run("file upload path", func(t *testing.T) {
		ad := newTestAD(srv.URL, true)
		_, err := ad.SubmitMagnet(&types.Torrent{
			Magnet: &utils.Magnet{File: []byte("torrent bytes")},
			Files:  make(map[string]types.File),
		})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if hitPath != "/magnet/upload/file" {
			t.Errorf("path = %s, want /magnet/upload/file", hitPath)
		}
	})

	t.Run("magnet link path", func(t *testing.T) {
		ad := newTestAD(srv.URL, false)
		_, err := ad.SubmitMagnet(&types.Torrent{
			Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:xyz"},
			Files:  make(map[string]types.File),
		})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if hitPath != "/magnet/upload" {
			t.Errorf("path = %s, want /magnet/upload", hitPath)
		}
	})
}
