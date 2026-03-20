package arr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterWebhook_CreatesNew(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/notification":
			if r.Method == http.MethodGet {
				fmt.Fprint(w, `[]`) // no existing notifications
			} else {
				method = r.Method
				path = r.URL.Path
				w.WriteHeader(http.StatusCreated)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if err := a.RegisterWebhook("http://decypharr/webhook"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("expected POST for new webhook, got %s", method)
	}
	if path != "/api/v3/notification" {
		t.Errorf("path = %q, want /api/v3/notification", path)
	}
}

func TestRegisterWebhook_UpdatesExisting(t *testing.T) {
	var putPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v3/notification" && r.Method == http.MethodGet:
			// Return existing notification with name "Decypharr"
			fmt.Fprintf(w, `[{"id":42,"name":%q}]`, webhookNotificationName)
		case r.Method == http.MethodPut:
			putPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if err := a.RegisterWebhook("http://decypharr/webhook"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if putPath != "/api/v3/notification/42" {
		t.Errorf("expected PUT to /api/v3/notification/42, got %q", putPath)
	}
}

func TestRegisterWebhook_ListError(t *testing.T) {
	a := &Arr{Host: "http://x"} // no token → fails immediately
	if err := a.RegisterWebhook("http://decypharr/webhook"); err == nil {
		t.Error("expected error when GET notifications fails")
	}
}

func TestRegisterWebhook_ListNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if err := a.RegisterWebhook("http://decypharr/webhook"); err == nil {
		t.Error("expected error for non-200 list response")
	}
}

func TestRegisterWebhook_PostNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `[]`)
		} else {
			w.WriteHeader(http.StatusBadRequest) // 400 → error
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if err := a.RegisterWebhook("http://decypharr/webhook"); err == nil {
		t.Error("expected error for 400 POST response")
	}
}
