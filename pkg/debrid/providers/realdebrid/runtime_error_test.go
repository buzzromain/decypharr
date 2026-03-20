package realdebrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirrobot01/decypharr/internal/customerror"
)

func TestCheckFile_NonNotFoundStatusReturnsNil(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"200 ok", http.StatusOK},
		{"403 forbidden", http.StatusForbidden},
		{"400 bad request", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			rd := newTestRD(srv.URL, false)
			err := rd.CheckFile(context.Background(), "hash", "http://hoster/file")
			if err != nil {
				t.Fatalf("CheckFile with status %d returned error: %v, want nil", tt.status, err)
			}
		})
	}
}

func TestCheckFile_Only404MapsToHosterUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	rd := newTestRD(srv.URL, false)
	err := rd.CheckFile(context.Background(), "hash", "http://hoster/file")
	if err != customerror.HosterUnavailableError {
		t.Fatalf("error = %v, want HosterUnavailableError", err)
	}
}
