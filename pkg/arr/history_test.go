package arr

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func makeQueue(status, trackedStatus string, messages []struct {
	Title    string
	Messages []string
}) QueueSchema {
	q := QueueSchema{
		Status:                status,
		TrackedDownloadStatus: trackedStatus,
	}
	for _, m := range messages {
		q.StatusMessages = append(q.StatusMessages, struct {
			Title    string   `json:"title"`
			Messages []string `json:"messages"`
		}{Title: m.Title, Messages: m.Messages})
	}
	return q
}

func TestQueueFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		queue  QueueSchema
		want   QueueAction
	}{
		// Failed status → always blocklist
		{
			name:  "failed status",
			queue: makeQueue("failed", "", nil),
			want:  QueueActionBlocklist,
		},
		{
			name:  "failed with warning",
			queue: makeQueue("failed", "warning", nil),
			want:  QueueActionBlocklist,
		},

		// Normal completed without warning → no action
		{
			name:  "completed ok",
			queue: makeQueue("completed", "ok", nil),
			want:  QueueActionNone,
		},
		{
			name:  "completed no status",
			queue: makeQueue("completed", "", nil),
			want:  QueueActionNone,
		},

		// Downloading / queued → no action
		{
			name:  "downloading",
			queue: makeQueue("downloading", "", nil),
			want:  QueueActionNone,
		},
		{
			name:  "queued",
			queue: makeQueue("queued", "ok", nil),
			want:  QueueActionNone,
		},

		// completed+warning with no messages → no action
		{
			name:  "completed warning no messages",
			queue: makeQueue("completed", "warning", nil),
			want:  QueueActionNone,
		},

		// completed+warning: "no files found are eligible" → blocklist
		{
			name: "no files eligible",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{Title: "Import failed", Messages: []string{"No files found are eligible for import"}},
			}),
			want: QueueActionBlocklist,
		},
		// Case-insensitive match
		{
			name: "no files eligible uppercase",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{Title: "Import failed", Messages: []string{"NO FILES FOUND ARE ELIGIBLE for import"}},
			}),
			want: QueueActionBlocklist,
		},

		// completed+warning: "downloaded file is empty" → blocklist
		{
			name: "empty file",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{Title: "Import failed", Messages: []string{"Downloaded file is empty"}},
			}),
			want: QueueActionBlocklist,
		},

		// completed+warning: episode title match → blocklist
		{
			name: "episodes not imported title",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{
					Title:    "One or more episodes expected in this release were not imported or missing from the release",
					Messages: []string{"some message"},
				},
			}),
			want: QueueActionBlocklist,
		},

		// completed+warning: "found matching series via grab history" → import
		{
			name: "matched via grab history",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{Title: "Import failed", Messages: []string{"Found matching series via grab history, but release was matched to series by id"}},
			}),
			want: QueueActionImport,
		},
		// Case-insensitive
		{
			name: "matched via grab history uppercase",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{Title: "Import failed", Messages: []string{"FOUND MATCHING SERIES VIA GRAB HISTORY, BUT RELEASE WAS MATCHED TO SERIES BY ID"}},
			}),
			want: QueueActionImport,
		},

		// Multiple messages: first matches blocklist
		{
			name: "multiple messages first blocklist",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{Title: "ok", Messages: []string{"No files found are eligible"}},
				{Title: "other", Messages: []string{"some other message"}},
			}),
			want: QueueActionBlocklist,
		},

		// Messages with multiple strings joined: match across slice
		{
			name: "message split across slice",
			queue: makeQueue("completed", "warning", []struct {
				Title    string
				Messages []string
			}{
				{Title: "import", Messages: []string{"no files found", "are eligible for import"}},
			}),
			want: QueueActionBlocklist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queueFilter(tt.queue)
			if got != tt.want {
				t.Errorf("queueFilter() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ── GetHistory ────────────────────────────────────────────────────────────────

func TestGetHistory_200ReturnsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/history" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("downloadId") != "abc123" {
			t.Errorf("missing downloadId query param")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"totalRecords":1,"records":[{"id":42,"downloadId":"abc123","eventType":"grabbed"}]}`)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	got := a.GetHistory("abc123", "grabbed")
	if got == nil {
		t.Fatal("expected non-nil HistorySchema")
	}
	if got.TotalRecords != 1 {
		t.Errorf("TotalRecords = %d, want 1", got.TotalRecords)
	}
	if len(got.Records) != 1 || got.Records[0].DownloadID != "abc123" {
		t.Errorf("unexpected records: %+v", got.Records)
	}
}

func TestGetHistory_Non200ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if got := a.GetHistory("", "grabbed"); got != nil {
		t.Errorf("expected nil for non-200, got %+v", got)
	}
}

// ── GetQueue ──────────────────────────────────────────────────────────────────

func TestGetQueue_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"totalRecords":2,"page":1,"pageSize":200,"records":[{"id":1,"status":"completed"},{"id":2,"status":"downloading"}]}`)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	got := a.GetQueue()
	if len(got) != 2 {
		t.Errorf("expected 2 records, got %d", len(got))
	}
}

func TestGetQueue_Pagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")
		if page == 1 {
			// First page: 1 of 2 total records
			fmt.Fprint(w, `{"totalRecords":2,"page":1,"pageSize":1,"records":[{"id":1,"status":"completed"}]}`)
		} else {
			// Second page: completes
			fmt.Fprint(w, `{"totalRecords":2,"page":2,"pageSize":1,"records":[{"id":2,"status":"failed"}]}`)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	got := a.GetQueue()
	if len(got) != 2 {
		t.Errorf("expected 2 records across pages, got %d", len(got))
	}
	if got[0].Id != 1 || got[1].Id != 2 {
		t.Errorf("unexpected IDs: %d %d", got[0].Id, got[1].Id)
	}
}

func TestGetQueue_Non200ReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if got := a.GetQueue(); len(got) != 0 {
		t.Errorf("expected empty slice for non-200, got %d items", len(got))
	}
}

// ── GetImportHistorySince ─────────────────────────────────────────────────────

func TestGetImportHistorySince_FiltersEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/history/since" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// Mix of valid and invalid records
		fmt.Fprint(w, `[
			{"downloadId":"dl1","eventType":"downloadFolderImported","date":"2024-01-01T00:00:00Z","data":{"importedPath":"/movies/foo.mkv"}},
			{"downloadId":"","eventType":"downloadFolderImported","date":"2024-01-01T00:00:00Z","data":{"importedPath":"/movies/bar.mkv"}},
			{"downloadId":"dl3","eventType":"grabbed","date":"2024-01-01T00:00:00Z","data":{"importedPath":"/movies/baz.mkv"}},
			{"downloadId":"dl4","eventType":"downloadFolderImported","date":"2024-01-01T00:00:00Z","data":{}}
		]`)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	got := a.GetImportHistorySince(time.Time{})

	// Only the first record passes all filters:
	// - eventType == "downloadFolderImported" ✓
	// - DownloadID != "" ✓
	// - importedPath != "" ✓
	if len(got) != 1 {
		t.Errorf("expected 1 filtered record, got %d: %+v", len(got), got)
	}
	if got[0].DownloadID != "dl1" {
		t.Errorf("expected DownloadID dl1, got %q", got[0].DownloadID)
	}
}

func TestGetImportHistorySince_ZeroTimeSendsEpoch(t *testing.T) {
	var gotDate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDate = r.URL.Query().Get("date")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	a.GetImportHistorySince(time.Time{})

	if gotDate != "1970-01-01T00:00:00Z" {
		t.Errorf("expected epoch date, got %q", gotDate)
	}
}

func TestGetImportHistorySince_Non200ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if got := a.GetImportHistorySince(time.Now()); got != nil {
		t.Errorf("expected nil for non-200, got %+v", got)
	}
}

// ── BlackListAndResearchItems ─────────────────────────────────────────────────

func TestBlackListAndResearchItems_SendsDeleteWithIDs(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	err := a.BlackListAndResearchItems(map[int]bool{42: true, 99: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v3/queue/bulk" {
		t.Errorf("path = %q, want /api/v3/queue/bulk", gotPath)
	}
	if gotQuery == "" {
		t.Error("expected query params (removeFromClient, blocklist, etc.)")
	}
	// Body should contain both IDs
	body := string(gotBody)
	if body == "" {
		t.Error("expected non-empty JSON body with ids")
	}
}

func TestBlackListAndResearchItems_NotConfigured(t *testing.T) {
	// Empty token → Request returns "arr not configured" before any HTTP call
	a := &Arr{Host: "http://sonarr:8989"}
	if err := a.BlackListAndResearchItems(map[int]bool{1: true}); err == nil {
		t.Error("expected error for unconfigured arr")
	}
}

// ── CleanupQueue ──────────────────────────────────────────────────────────────

func TestCleanupQueue_BlocklistsFailedItems(t *testing.T) {
	deleteCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/queue":
			// Return one failed item
			fmt.Fprint(w, `{"totalRecords":1,"page":1,"pageSize":200,"records":[{"id":7,"status":"failed","downloadId":"dl7"}]}`)
		case "/api/v3/queue/bulk":
			deleteCallCount++
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	if err := a.CleanupQueue(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCallCount != 1 {
		t.Errorf("blocklist DELETE called %d times, want 1", deleteCallCount)
	}
}

func TestCleanupQueue_NoActionForOKItems(t *testing.T) {
	deleteCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/queue":
			fmt.Fprint(w, `{"totalRecords":1,"page":1,"pageSize":200,"records":[{"id":5,"status":"completed","trackedDownloadStatus":"ok"}]}`)
		case "/api/v3/queue/bulk":
			deleteCallCount++
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	a := &Arr{Host: srv.URL, Token: "token"}
	_ = a.CleanupQueue()
	if deleteCallCount != 0 {
		t.Errorf("no blocklist expected for OK item, but DELETE called %d times", deleteCallCount)
	}
}
