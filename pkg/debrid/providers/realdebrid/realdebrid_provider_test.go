package realdebrid

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/customerror"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func newTestRDProviderWithAccount(host string) *RealDebrid {
	useTorrentFile := false
	dc := config.Debrid{
		Name:            "realdebrid",
		APIKey:          "rd-key",
		DownloadAPIKeys: []string{"dl-token"},
		UseTorrentFile:  &useTorrentFile,
	}

	return &RealDebrid{
		Host:                  host,
		APIKey:                dc.APIKey,
		client:                request.New(request.WithMaxRetries(0), request.WithRetryableStatus(http.StatusTooManyRequests)),
		repairClient:          request.New(request.WithMaxRetries(0), request.WithRetryableStatus(http.StatusTooManyRequests)),
		accountsManager:       account.NewManager(dc, nil, zerolog.Nop()),
		autoExpiresLinksAfter: time.Hour,
		rarSemaphore:          make(chan struct{}, 1),
		config:                dc,
	}
}

func retryableClient(t *testing.T, c *request.Client) *retryablehttp.Client {
	t.Helper()
	v := reflect.ValueOf(c).Elem().FieldByName("client")
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Interface().(*retryablehttp.Client)
}

func setCheckRetry(t *testing.T, c *request.Client, maxRetries int, fn retryablehttp.CheckRetry) {
	t.Helper()
	rc := retryableClient(t, c)
	rc.RetryMax = maxRetries
	rc.RetryWaitMin = 1 * time.Millisecond
	rc.RetryWaitMax = 1 * time.Millisecond
	rc.CheckRetry = fn
}

func setAccountClient(t *testing.T, acc *account.Account, c *request.Client) {
	t.Helper()
	v := reflect.ValueOf(acc).Elem().FieldByName("httpClient")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(c))
}

func assertNoGoroutineLeak(t *testing.T, before int) {
	t.Helper()
	for i := 0; i < 50; i++ {
		runtime.GC()
		runtime.Gosched()
		if runtime.NumGoroutine() <= before+2 {
			return
		}
	}
	t.Fatalf("possible goroutine leak: before=%d after=%d", before, runtime.NumGoroutine())
}

func TestRealDebrid_509MapsToTooManyActiveDownloads(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(509)
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	rd.client = request.New(request.WithMaxRetries(3), request.WithRetryableStatus(http.StatusTooManyRequests))
	setCheckRetry(t, rd.client, 3, func(_ context.Context, resp *http.Response, err error) (bool, error) {
		if err != nil {
			return false, err
		}
		if resp != nil && resp.StatusCode == 509 {
			// Retry once, then return final 509 response to provider for mapping.
			return calls.Load() < 2, nil
		}
		return false, nil
	})

	got, err := rd.SubmitMagnet(&types.Torrent{Magnet: &utils.Magnet{Link: "magnet:?xt=urn:btih:abc"}})
	if got != nil {
		t.Fatalf("torrent = %#v, want nil on 509", got)
	}
	if !errors.Is(err, customerror.TooManyActiveDownloadsError) {
		t.Fatalf("error = %v, want TooManyActiveDownloadsError", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least one retry before mapping, calls=%d", calls.Load())
	}
}

func TestRealDebrid_GetDownloadLink_Table(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    error
		wantSubstr string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"id":"1","filename":"movie.mkv","filesize":123,"link":"http://source/link","download":"http://cdn/download"}`,
		},
		{
			name:       "missing download link",
			statusCode: http.StatusOK,
			body:       `{"id":"1","filename":"movie.mkv","filesize":123,"link":"http://source/link","download":""}`,
			wantSubstr: "download link not found",
		},
		{
			name:       "hoster unavailable mapping",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":"hoster unavailable","error_code":19}`,
			wantErr:    customerror.HosterUnavailableError,
		},
		{
			name:       "traffic exceeded mapping",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":"traffic exceeded","error_code":23}`,
			wantErr:    customerror.TrafficExceededError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/unrestrict/link/" {
					t.Fatalf("path = %s, want /unrestrict/link/", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			rd := newTestRDProviderWithAccount(srv.URL)
			acc := rd.accountsManager.Current()
			if acc == nil {
				t.Fatal("expected current account")
			}
			accountClient := request.New(request.WithMaxRetries(0), request.WithRetryableStatus(http.StatusTooManyRequests))
			setCheckRetry(t, accountClient, 0, func(_ context.Context, _ *http.Response, err error) (bool, error) {
				return false, err
			})
			setAccountClient(t, acc, accountClient)

			file := &types.File{Name: "movie.mkv", Link: "http://source/link", Size: 123}
			got, err := rd.fetchDownloadLink(acc, "torrent-id", file)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if tt.wantSubstr != "" {
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantSubstr)) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetDownloadLink error: %v", err)
			}
			if got.DownloadLink != "http://cdn/download" {
				t.Fatalf("download link = %q, want http://cdn/download", got.DownloadLink)
			}
			if got.Token != "dl-token" {
				t.Fatalf("token = %q, want dl-token", got.Token)
			}
		})
	}
}

func TestRealDebrid_RefreshDownloadLinksThenGetDownloadLink_UsesRefreshedLink(t *testing.T) {
	var downloadsCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/downloads":
			call := downloadsCalls.Add(1)
			if call == 1 {
				_, _ = w.Write([]byte(`[{"id":"d1","filename":"movie.mkv","filesize":123,"link":"http://source/link","download":"http://cdn/refreshed","generated":"2026-01-01T00:00:00Z"}]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		case "/unrestrict/link/":
			// Should not be needed after refresh populated cache.
			_, _ = w.Write([]byte(`{"id":"1","filename":"movie.mkv","filesize":123,"link":"http://source/link","download":"http://cdn/fetched"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	if err := rd.RefreshDownloadLinks(); err != nil {
		t.Fatalf("RefreshDownloadLinks error: %v", err)
	}

	got, err := rd.GetDownloadLink("torrent-id", &types.File{Name: "movie.mkv", Link: "http://source/link", Size: 123})
	if err != nil {
		t.Fatalf("GetDownloadLink error: %v", err)
	}
	if got.DownloadLink != "http://cdn/refreshed" {
		t.Fatalf("download link = %q, want refreshed link", got.DownloadLink)
	}
	if downloadsCalls.Load() < 2 {
		t.Fatalf("expected paginated refresh calls, got %d", downloadsCalls.Load())
	}
}

func TestRealDebrid_DeleteLink_Behavior(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Fatalf("method = %s, want DELETE", r.Method)
			}
			if r.URL.Path != "/downloads/delete/d1" {
				t.Fatalf("path = %s, want /downloads/delete/d1", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		rd := newTestRDProviderWithAccount(srv.URL)
		if err := rd.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"}); err != nil {
			t.Fatalf("DeleteLink error: %v", err)
		}
	})

	t.Run("propagates transport error", func(t *testing.T) {
		rd := newTestRDProviderWithAccount("http://rd.test")
		acc := rd.accountsManager.Current()
		if acc == nil {
			t.Fatal("expected current account")
		}

		c := request.New(request.WithMaxRetries(0), request.WithTransport(&http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return nil, context.Canceled
			},
		}))
		setCheckRetry(t, c, 0, func(_ context.Context, _ *http.Response, err error) (bool, error) {
			return false, err
		})
		setAccountClient(t, acc, c)

		err := rd.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})
}

func TestRealDebrid_TimeoutPropagation_LinkFlows(t *testing.T) {
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		switch r.URL.Path {
		case "/unrestrict/link/":
			_, _ = w.Write([]byte(`{"id":"1","filename":"movie.mkv","filesize":123,"link":"http://source/link","download":"http://cdn/download"}`))
		case "/downloads":
			_, _ = w.Write([]byte(`[]`))
		case "/downloads/delete/d1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer func() {
		close(release)
		srv.Close()
	}()

	rd := newTestRDProviderWithAccount(srv.URL)
	acc := rd.accountsManager.Current()
	if acc == nil {
		t.Fatal("expected current account")
	}

	accountClient := request.New(
		request.WithMaxRetries(0),
		request.WithTransport(&http.Transport{ResponseHeaderTimeout: 20 * time.Millisecond}),
	)
	setCheckRetry(t, accountClient, 0, func(_ context.Context, _ *http.Response, err error) (bool, error) {
		return false, err
	})
	setAccountClient(t, acc, accountClient)

	_, err := rd.fetchDownloadLink(acc, "torrent-id", &types.File{Name: "movie.mkv", Link: "http://source/link", Size: 123})
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "timeout") && !strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")) {
		t.Fatalf("fetchDownloadLink error = %v, want timeout", err)
	}

	err = rd.RefreshDownloadLinks()
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "timeout") && !strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")) {
		t.Fatalf("RefreshDownloadLinks error = %v, want timeout", err)
	}

	err = rd.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"})
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "timeout") && !strings.Contains(strings.ToLower(err.Error()), "deadline exceeded")) {
		t.Fatalf("DeleteLink error = %v, want timeout", err)
	}

}

func TestRealDebrid_ContextCancellation_LinkFlows(t *testing.T) {
	before := runtime.NumGoroutine()

	rd := newTestRDProviderWithAccount("http://rd.test")
	acc := rd.accountsManager.Current()
	if acc == nil {
		t.Fatal("expected current account")
	}

	cancelClient := request.New(
		request.WithMaxRetries(0),
		request.WithTransport(&http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return nil, context.Canceled
			},
		}),
	)
	setCheckRetry(t, cancelClient, 0, func(_ context.Context, _ *http.Response, err error) (bool, error) {
		return false, err
	})
	setAccountClient(t, acc, cancelClient)

	_, err := rd.fetchDownloadLink(acc, "torrent-id", &types.File{Name: "movie.mkv", Link: "http://source/link", Size: 123})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("fetchDownloadLink error = %v, want context.Canceled", err)
	}

	err = rd.RefreshDownloadLinks()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RefreshDownloadLinks error = %v, want context.Canceled", err)
	}

	err = rd.DeleteLink(types.DownloadLink{Id: "d1", Link: "http://source/link", Token: "dl-token"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DeleteLink error = %v, want context.Canceled", err)
	}

	assertNoGoroutineLeak(t, before)
}
