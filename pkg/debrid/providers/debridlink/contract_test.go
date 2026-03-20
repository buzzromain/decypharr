package debridlink

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/request"
	"github.com/sirrobot01/decypharr/pkg/debrid/account"
)

// --- GetTorrents ---

func TestProvider_GetTorrents_MapsRuntimeState(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/seedbox/list" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		// Return data only on first page; empty on subsequent
		if requestCount > 1 {
			fmt.Fprint(w, `{"success":true,"value":[]}`)
			return
		}
		// status 100 = downloaded, anything else = downloading (filtered out by getTorrents)
		fmt.Fprint(w, `{
			"success": true,
			"value": [
				{
					"id": "dl1", "name": "Movie.2024", "hashString": "abc123",
					"totalSize": 1073741824, "status": 100, "downloadPercent": 100,
					"created": 1717200000,
					"files": [
						{"id": "f1", "name": "movie.mkv", "downloadUrl": "http://dl/movie.mkv", "size": 1073741824, "downloadPercent": 100}
					]
				},
				{
					"id": "dl2", "name": "Incomplete.Item", "hashString": "def456",
					"totalSize": 524288000, "status": 50, "downloadPercent": 50,
					"created": 1717200000,
					"files": [
						{"id": "f2", "name": "file.mkv", "downloadUrl": "http://dl/file.mkv", "size": 524288000, "downloadPercent": 50}
					]
				}
			]
		}`)
	}))
	defer srv.Close()

	dl := newTestDLWithAccount(srv.URL)
	torrents, err := dl.GetTorrents()
	if err != nil {
		t.Fatalf("GetTorrents error: %v", err)
	}

	// Only status=100 should be returned
	if len(torrents) != 1 {
		t.Fatalf("expected 1 downloaded torrent, got %d", len(torrents))
	}

	tr := torrents[0]
	if tr.Id != "dl1" {
		t.Errorf("Id: got %q, want %q", tr.Id, "dl1")
	}
	if tr.Name != "Movie.2024" {
		t.Errorf("Name: got %q, want %q", tr.Name, "Movie.2024")
	}
	if tr.InfoHash != "abc123" {
		t.Errorf("InfoHash: got %q, want %q", tr.InfoHash, "abc123")
	}
	if tr.Bytes != 1073741824 {
		t.Errorf("Bytes: got %d, want %d", tr.Bytes, 1073741824)
	}
	if tr.Status != "downloaded" {
		t.Errorf("Status: got %q, want %q", tr.Status, "downloaded")
	}
	if tr.Debrid != "debridlink" {
		t.Errorf("Debrid: got %q, want %q", tr.Debrid, "debridlink")
	}

	// Files should be mapped with download links
	if len(tr.Files) != 1 {
		t.Fatalf("Files: got %d, want 1", len(tr.Files))
	}
	for _, f := range tr.Files {
		if f.Link != "http://dl/movie.mkv" {
			t.Errorf("File link: got %q, want %q", f.Link, "http://dl/movie.mkv")
		}
		if f.DownloadLink.DownloadLink != "http://dl/movie.mkv" {
			t.Errorf("DownloadLink: got %q, want %q", f.DownloadLink.DownloadLink, "http://dl/movie.mkv")
		}
	}
}

// --- RefreshDownloadLinks partial failure ---

func TestProvider_RefreshDownloadLinks_PartialFailure(t *testing.T) {
	callsByToken := make(map[string]int)
	var callsMu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if r.URL.Path == "/downloader/list" {
			switch token {
			case "Bearer dl-good":
				callsMu.Lock()
				callsByToken["dl-good"]++
				goodCalls := callsByToken["dl-good"]
				callsMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				// Return data on first call, empty on second (pagination termination)
				if goodCalls > 1 {
					fmt.Fprint(w, `{"success":true,"value":[]}`)
				} else {
					fmt.Fprint(w, `{"success":true,"value":[{"id":"link1","name":"file.mkv","url":"http://orig","downloadUrl":"http://dl","size":100,"created":1717200000}]}`)
				}
			case "Bearer dl-bad":
				callsMu.Lock()
				callsByToken["dl-bad"]++
				callsMu.Unlock()
				http.Error(w, "server error", http.StatusInternalServerError)
			default:
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}
		}
	}))
	defer srv.Close()

	useTorrentFile := false
	dc := config.Debrid{
		Name:            "debridlink",
		APIKey:          "dl-key",
		DownloadAPIKeys: []string{"dl-good", "dl-bad"},
		UseTorrentFile:  &useTorrentFile,
	}

	dl := &DebridLink{
		Host:                  srv.URL,
		APIKey:                dc.APIKey,
		client:                request.New(request.WithMaxRetries(0)),
		accountsManager:       account.NewManager(dc, nil, zerolog.Nop()),
		autoExpiresLinksAfter: time.Hour,
		config:                dc,
	}

	// Override account HTTP clients
	for _, acc := range dl.accountsManager.All() {
		c := request.New(
			request.WithMaxRetries(0),
			request.WithHeaders(map[string]string{"Authorization": fmt.Sprintf("Bearer %s", acc.Token)}),
		)
		overrideDLAccountClient(t, acc, c)
	}

	err := dl.RefreshDownloadLinks()

	if err == nil {
		t.Fatal("expected error from partial failure, got nil")
	}

	callsMu.Lock()
	defer callsMu.Unlock()
	if callsByToken["dl-good"] == 0 {
		t.Error("good account was never called")
	}
	if callsByToken["dl-bad"] == 0 {
		t.Error("bad account was never called")
	}
}

func overrideDLAccountClient(t *testing.T, acc *account.Account, c *request.Client) {
	t.Helper()
	v := reflect.ValueOf(acc).Elem().FieldByName("httpClient")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(c))
}

// --- GetAvailableSlots ---

func TestProvider_GetAvailableSlots_ValidMapping(t *testing.T) {
	dl := newTestDLWithAccount("http://unused")

	slots, err := dl.GetAvailableSlots()
	if err != nil {
		t.Fatalf("GetAvailableSlots error: %v", err)
	}
	if slots != config.DefaultAvailableSlots {
		t.Errorf("slots: got %d, want %d", slots, config.DefaultAvailableSlots)
	}
}

// --- Profile malformed payload ---

func TestProvider_AccountProfile_MalformedPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{totally broken json`))
	}))
	defer srv.Close()

	dl := &DebridLink{
		Host:   srv.URL,
		APIKey: "dl-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "debridlink"},
	}

	_, err := dl.GetProfile()
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestProvider_AccountProfile_ValidMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/account/infos" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"success":true,"value":{"username":"dluser","email":"dl@example.com","premiumLeft":1767225599,"pts":100}}`)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	dl := &DebridLink{
		Host:   srv.URL,
		APIKey: "dl-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "debridlink"},
	}

	profile, err := dl.GetProfile()
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}
	if profile.Username != "dluser" {
		t.Errorf("Username: got %q, want %q", profile.Username, "dluser")
	}
	if profile.Email != "dl@example.com" {
		t.Errorf("Email: got %q, want %q", profile.Email, "dl@example.com")
	}
	if profile.Type != "premium" {
		t.Errorf("Type: got %q, want %q", profile.Type, "premium")
	}
	if profile.Points != 100 {
		t.Errorf("Points: got %d, want 100", profile.Points)
	}
}

func TestProvider_AccountProfile_NotSuccessful(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":false,"value":null}`)
	}))
	defer srv.Close()

	dl := &DebridLink{
		Host:   srv.URL,
		APIKey: "dl-key",
		client: request.New(request.WithMaxRetries(0)),
		config: config.Debrid{Name: "debridlink"},
	}

	_, err := dl.GetProfile()
	if err == nil {
		t.Fatal("expected error on success=false, got nil")
	}
}

// --- AccountSync is a no-op for DebridLink ---

func TestProvider_AccountSync_NoOp(t *testing.T) {
	dl := &DebridLink{}
	err := dl.syncAccount(nil)
	if err != nil {
		t.Fatalf("syncAccount should be no-op, got error: %v", err)
	}
}

// --- Helpers ---

func newTestDLWithAccount(host string) *DebridLink {
	useTorrentFile := false
	dc := config.Debrid{
		Name:            "debridlink",
		APIKey:          "dl-key",
		DownloadAPIKeys: []string{"dl-token"},
		UseTorrentFile:  &useTorrentFile,
	}
	return &DebridLink{
		Host:                  host,
		APIKey:                dc.APIKey,
		client:                request.New(request.WithMaxRetries(0)),
		accountsManager:       account.NewManager(dc, nil, zerolog.Nop()),
		autoExpiresLinksAfter: time.Hour,
		config:                dc,
	}
}
