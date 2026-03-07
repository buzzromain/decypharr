package arr

import (
	"testing"
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
