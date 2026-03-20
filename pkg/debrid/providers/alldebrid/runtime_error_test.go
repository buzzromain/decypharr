package alldebrid

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func newTestADWithAccount(host string) *AllDebrid {
	useTorrentFile := false
	dc := config.Debrid{
		Name:            "alldebrid",
		APIKey:          "ad-key",
		DownloadAPIKeys: []string{"dl-token"},
		UseTorrentFile:  &useTorrentFile,
	}
	return &AllDebrid{
		Host:                  host,
		APIKey:                dc.APIKey,
		client:                request.New(request.WithMaxRetries(0), request.WithRetryableStatus(http.StatusTooManyRequests)),
		accountsManager:       account.NewManager(dc, nil, zerolog.Nop()),
		autoExpiresLinksAfter: time.Hour,
		config:                dc,
	}
}

func TestDeleteLink_APINon2xxPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/links/delete" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ad := newTestADWithAccount(srv.URL)
	err := ad.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"})
	if err == nil {
		t.Fatal("expected error from DeleteLink on 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v, want status 403 mention", err)
	}
}

func TestDeleteLink_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/links/delete" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ad := newTestADWithAccount(srv.URL)
	if err := ad.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"}); err != nil {
		t.Fatalf("DeleteLink error: %v", err)
	}
}

func TestDeleteTorrent_TransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	ad := newTestAD(closedURL, false)
	err := ad.DeleteTorrent("99")
	if err == nil {
		t.Fatal("expected error for transport failure")
	}
}
