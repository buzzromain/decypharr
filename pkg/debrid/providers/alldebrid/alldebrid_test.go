package alldebrid

import (
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

// torrentLimit caps what a configured `limit` can request; it never lets a
// user push AllDebrid's own ~5000 ceiling higher, and still lets them set a
// lower, self-imposed one.
func TestTorrentLimit(t *testing.T) {
	cases := []struct {
		name  string
		limit int
		want  int
	}{
		{"unset defaults to the cap", 0, maxTorrentLimit},
		{"negative defaults to the cap", -1, maxTorrentLimit},
		{"below the cap is honoured", 1000, 1000},
		{"exactly at the cap is honoured", maxTorrentLimit, maxTorrentLimit},
		{"above the cap is clamped down", 9000, maxTorrentLimit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ad := &AllDebrid{config: config.Debrid{Limit: c.limit}}
			if got := ad.torrentLimit(); got != c.want {
				t.Errorf("torrentLimit() with configured limit %d = %d, want %d", c.limit, got, c.want)
			}
		})
	}
}
