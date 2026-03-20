package torbox

import (
	"testing"

	"github.com/sirrobot01/decypharr/pkg/debrid/types"
)

func TestGetTorboxStatus(t *testing.T) {
	tb := &Torbox{}

	tests := []struct {
		name     string
		status   string
		finished bool
		want     types.TorrentStatus
	}{
		{name: "finished always downloaded", status: "queuedDL", finished: true, want: types.TorrentStatusDownloaded},
		{name: "downloading alias", status: "downloading", finished: false, want: types.TorrentStatusDownloading},
		{name: "status with suffix", status: "downloading (queued)", finished: false, want: types.TorrentStatusDownloading},
		{name: "completed", status: "completed", finished: false, want: types.TorrentStatusDownloaded},
		{name: "unknown", status: "something-else", finished: false, want: types.TorrentStatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tb.getTorboxStatus(tt.status, tt.finished)
			if got != tt.want {
				t.Fatalf("getTorboxStatus(%q, %v) = %q, want %q", tt.status, tt.finished, got, tt.want)
			}
		})
	}
}
