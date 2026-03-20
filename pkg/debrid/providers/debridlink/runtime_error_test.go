package debridlink

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestDeleteLink_APINon2xxPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloader/d1/remove":
			if r.Method != http.MethodDelete {
				t.Fatalf("method = %s, want DELETE", r.Method)
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dl := newTestDLWithAccount(srv.URL)
	err := dl.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"})
	if err == nil {
		t.Fatal("expected error from DeleteLink on 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v, want status 403 mention", err)
	}
}

func TestDeleteLink_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloader/d1/remove":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dl := newTestDLWithAccount(srv.URL)
	if err := dl.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"}); err != nil {
		t.Fatalf("DeleteLink error: %v", err)
	}
}

func TestDeleteTorrent_TransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	dl := newTestDL(closedURL, false)
	err := dl.DeleteTorrent("id-1")
	if err == nil {
		t.Fatal("expected error for transport failure")
	}
}
