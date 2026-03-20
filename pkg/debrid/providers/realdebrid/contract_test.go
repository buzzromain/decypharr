package realdebrid

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
	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

// --- GetTorrents ---

func TestProvider_GetTorrents_MapsRuntimeState(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/torrents" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		requestCount++
		w.Header().Set("X-Total-Count", "3")
		w.Header().Set("Content-Type", "application/json")
		// Return data only on first page; empty array on subsequent requests
		if requestCount > 1 {
			fmt.Fprint(w, `[]`)
			return
		}
		// Mix of statuses: only "downloaded" should appear in results
		fmt.Fprint(w, `[
			{"id":"rd1","filename":"Movie.2024.mkv","hash":"abc123","bytes":1073741824,"progress":100,"status":"downloaded","added":"2024-06-01T12:00:00Z","links":["link1","link2"]},
			{"id":"rd2","filename":"Show.S01E01.mkv","hash":"def456","bytes":524288000,"progress":45.5,"status":"downloading","added":"2024-06-02T12:00:00Z","links":["link3"]},
			{"id":"rd3","filename":"Album.flac","hash":"ghi789","bytes":314572800,"progress":100,"status":"downloaded","added":"2024-06-03T12:00:00Z","links":["link4"]}
		]`)
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	torrents, err := rd.GetTorrents()
	if err != nil {
		t.Fatalf("GetTorrents returned error: %v", err)
	}

	// Only "downloaded" torrents should be mapped
	if len(torrents) != 2 {
		t.Fatalf("expected 2 downloaded torrents, got %d", len(torrents))
	}

	// Verify first torrent mapping
	tr := torrents[0]
	if tr.Id != "rd1" {
		t.Errorf("Id: got %q, want %q", tr.Id, "rd1")
	}
	if tr.Name != "Movie.2024.mkv" {
		t.Errorf("Name: got %q, want %q", tr.Name, "Movie.2024.mkv")
	}
	if tr.InfoHash != "abc123" {
		t.Errorf("InfoHash: got %q, want %q", tr.InfoHash, "abc123")
	}
	if tr.Bytes != 1073741824 {
		t.Errorf("Bytes: got %d, want %d", tr.Bytes, 1073741824)
	}
	if tr.Status != types.TorrentStatusDownloaded {
		t.Errorf("Status: got %q, want %q", tr.Status, types.TorrentStatusDownloaded)
	}
	if tr.Debrid != "realdebrid" {
		t.Errorf("Debrid: got %q, want %q", tr.Debrid, "realdebrid")
	}
	if len(tr.Links) != 2 {
		t.Errorf("Links count: got %d, want 2", len(tr.Links))
	}

	// Verify second torrent
	tr2 := torrents[1]
	if tr2.Id != "rd3" {
		t.Errorf("second torrent Id: got %q, want %q", tr2.Id, "rd3")
	}
}

// --- RefreshDownloadLinks partial failure ---

func TestProvider_RefreshDownloadLinks_PartialFailure(t *testing.T) {
	var mu sync.Mutex
	callsByToken := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if r.URL.Path == "/downloads" {
			mu.Lock()
			defer mu.Unlock()
			switch token {
			case "Bearer dl-good":
				callsByToken["dl-good"]++
				w.Header().Set("Content-Type", "application/json")
				// Return data on first call, empty on second (pagination termination)
				if callsByToken["dl-good"] > 1 {
					fmt.Fprint(w, `[]`)
				} else {
					fmt.Fprint(w, `[{"id":"d1","filename":"file.mkv","filesize":100,"link":"http://orig","download":"http://dl","generated":"2024-06-01T12:00:00Z"}]`)
				}
			case "Bearer dl-bad":
				callsByToken["dl-bad"]++
				http.Error(w, "server error", http.StatusInternalServerError)
			default:
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}
		}
	}))
	defer srv.Close()

	useTorrentFile := false
	dc := config.Debrid{
		Name:            "realdebrid",
		APIKey:          "rd-key",
		DownloadAPIKeys: []string{"dl-good", "dl-bad"},
		UseTorrentFile:  &useTorrentFile,
	}

	rd := &RealDebrid{
		Host:                  srv.URL,
		APIKey:                dc.APIKey,
		client:                request.New(request.WithMaxRetries(0)),
		repairClient:          request.New(request.WithMaxRetries(0)),
		accountsManager:       account.NewManager(dc, nil, zerolog.Nop()),
		autoExpiresLinksAfter: time.Hour,
		rarSemaphore:          make(chan struct{}, 1),
		config:                dc,
	}

	// Override account HTTP clients to point at the test server with correct tokens
	for _, acc := range rd.accountsManager.All() {
		c := request.New(
			request.WithMaxRetries(0),
			request.WithHeaders(map[string]string{"Authorization": fmt.Sprintf("Bearer %s", acc.Token)}),
		)
		overrideAccountClient(t, acc, c)
	}

	err := rd.RefreshDownloadLinks()

	// Should return error because dl-bad failed
	if err == nil {
		t.Fatal("expected error from partial failure, got nil")
	}

	// Both accounts should have been attempted
	if callsByToken["dl-good"] == 0 {
		t.Error("good account was never called")
	}
	if callsByToken["dl-bad"] == 0 {
		t.Error("bad account was never called")
	}
}

func overrideAccountClient(t *testing.T, acc *account.Account, c *request.Client) {
	t.Helper()
	v := reflect.ValueOf(acc).Elem().FieldByName("httpClient")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(c))
}

// --- GetAvailableSlots ---

func TestProvider_GetAvailableSlots_ValidMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/torrents/activeCount" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"nb":3,"limit":10}`)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	rd.config.MinimumFreeSlot = 2

	slots, err := rd.GetAvailableSlots()
	if err != nil {
		t.Fatalf("GetAvailableSlots error: %v", err)
	}

	// 10 (limit) - 3 (active) - 2 (minFreeSlot) = 5
	if slots != 5 {
		t.Errorf("slots: got %d, want 5", slots)
	}
}

func TestProvider_GetAvailableSlots_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	_, err := rd.GetAvailableSlots()
	if err == nil {
		t.Fatal("expected error on API error, got nil")
	}
}

// --- Profile malformed payload ---

func TestProvider_AccountProfile_MalformedPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not valid json at all`))
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	rd.Profile = nil // ensure cache is cleared

	_, err := rd.GetProfile()
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestProvider_AccountProfile_ValidMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":42,"username":"testuser","email":"test@example.com","points":500,"type":"premium","premium":9999999999,"expiration":"2025-12-31T23:59:59Z"}`)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	rd := newTestRDProviderWithAccount(srv.URL)
	profile, err := rd.GetProfile()
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}

	if profile.Username != "testuser" {
		t.Errorf("Username: got %q, want %q", profile.Username, "testuser")
	}
	if profile.Email != "test@example.com" {
		t.Errorf("Email: got %q, want %q", profile.Email, "test@example.com")
	}
	if profile.Type != "premium" {
		t.Errorf("Type: got %q, want %q", profile.Type, "premium")
	}
	if profile.Id != 42 {
		t.Errorf("Id: got %d, want 42", profile.Id)
	}
}

// --- AccountSync cancellation / expiration ---

func TestProvider_AccountSync_Cancellation(t *testing.T) {
	expiredAt := time.Now().Add(-24 * time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"id":1,"username":"expired-user","email":"exp@test.com","points":0,"type":"premium","premium":0,"expiration":"%s"}`, expiredAt.Format(time.RFC3339))
		case "/traffic/details":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{}`)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	useTorrentFile := false
	dc := config.Debrid{
		Name:            "realdebrid",
		APIKey:          "rd-key",
		DownloadAPIKeys: []string{"sync-token"},
		UseTorrentFile:  &useTorrentFile,
	}

	rd := &RealDebrid{
		Host:                  srv.URL,
		APIKey:                dc.APIKey,
		client:                request.New(request.WithMaxRetries(0)),
		repairClient:          request.New(request.WithMaxRetries(0)),
		accountsManager:       account.NewManager(dc, nil, zerolog.Nop()),
		autoExpiresLinksAfter: time.Hour,
		rarSemaphore:          make(chan struct{}, 1),
		config:                dc,
	}

	// Override the account's HTTP client to point at our test server
	for _, acc := range rd.accountsManager.All() {
		c := request.New(
			request.WithMaxRetries(0),
			request.WithHeaders(map[string]string{"Authorization": "Bearer sync-token"}),
		)
		overrideAccountClient(t, acc, c)
	}

	rd.SyncAccounts()

	// After sync, the account should be disabled because it's expired
	var disabled bool
	for _, acc := range rd.accountsManager.All() {
		if acc.Token == "sync-token" {
			disabled = acc.Disabled.Load()
			if acc.Username != "expired-user" {
				t.Errorf("Username: got %q, want %q", acc.Username, "expired-user")
			}
		}
	}

	if !disabled {
		t.Error("expected expired account to be disabled after sync")
	}
}
