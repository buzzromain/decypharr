package usenet

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/nntp"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func TestDownload_HappyPathAssemblesOrderedExactContent(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)
	t.Setenv("DECYPHARR_USENET__MAX_CONNECTIONS", "3")

	seg1 := []byte("AAA")
	seg2 := []byte("BBB")
	seg3 := []byte("CCC")

	srv := startTestNNTPServer(t, "user", "pass", map[string][]byte{
		"seg-1@test": yencEncodeForNNTP(seg1, "movie.mkv", 1, 1, int64(len(seg1))),
		"seg-2@test": yencEncodeForNNTP(seg2, "movie.mkv", 2, 1, int64(len(seg2))),
		"seg-3@test": yencEncodeForNNTP(seg3, "movie.mkv", 3, 1, int64(len(seg3))),
	})
	host, port := srv.hostPort()
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__HOST", host)
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PORT", itoa(port))
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__USERNAME", "user")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PASSWORD", "pass")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__MAX_CONNECTIONS", "3")

	cfgJSON := `{
		"download_folder":"/downloads",
		"usenet":{
			"max_connections":3,
			"providers":[{
				"host":"` + host + `",
				"port":` + itoa(port) + `,
				"username":"user",
				"password":"pass",
				"max_connections":3
			}]
		}
	}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	u, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = u.Close() })

	nzb := &storage.NZB{
		ID:   "nzo-download-1",
		Name: "movie",
		Files: []storage.NZBFile{{
			NzbID: "nzo-download-1",
			Name:  "movie.mkv",
			Segments: []storage.NZBSegment{
				{Number: 1, MessageID: "seg-1@test", Bytes: int64(len(seg1))},
				{Number: 2, MessageID: "seg-2@test", Bytes: int64(len(seg2))},
				{Number: 3, MessageID: "seg-3@test", Bytes: int64(len(seg3))},
			},
		}},
	}
	if err := u.nzbStorage.AddNZB(nzb); err != nil {
		t.Fatalf("AddNZB: %v", err)
	}

	// Force out-of-order arrival by holding segment 1 until 2 and 3 have been requested.
	blockFirst := make(chan struct{})
	srv.setBodyBlock("seg-1@test", blockFirst)

	var out bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- u.Download(context.Background(), "nzo-download-1", "movie.mkv", &out, nil)
	}()

	want2 := nntp.FormatMessageID("seg-2@test")
	want3 := nntp.FormatMessageID("seg-3@test")
	seen2 := false
	seen3 := false
	for !seen2 || !seen3 {
		got := <-srv.bodyEvents
		if got == want2 {
			seen2 = true
		}
		if got == want3 {
			seen3 = true
		}
	}
	close(blockFirst)

	if err := <-errCh; err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	want := append(append([]byte{}, seg1...), append(seg2, seg3...)...)
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("assembled output mismatch: got %q want %q", out.Bytes(), want)
	}
}

func TestDownload_MissingSegmentPropagatesError(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)
	t.Setenv("DECYPHARR_USENET__MAX_CONNECTIONS", "3")

	seg2 := []byte("BBB")
	seg3 := []byte("CCC")

	srv := startTestNNTPServer(t, "user", "pass", map[string][]byte{
		"seg-2@test": yencEncodeForNNTP(seg2, "movie.mkv", 2, 1, int64(len(seg2))),
		"seg-3@test": yencEncodeForNNTP(seg3, "movie.mkv", 3, 1, int64(len(seg3))),
	})
	host, port := srv.hostPort()
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__HOST", host)
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PORT", itoa(port))
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__USERNAME", "user")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PASSWORD", "pass")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__MAX_CONNECTIONS", "3")

	cfgJSON := `{
		"download_folder":"/downloads",
		"usenet":{
			"max_connections":3,
			"providers":[{
				"host":"` + host + `",
				"port":` + itoa(port) + `,
				"username":"user",
				"password":"pass",
				"max_connections":3
			}]
		}
	}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	u, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = u.Close() })

	nzb := &storage.NZB{
		ID:   "nzo-download-missing",
		Name: "movie",
		Files: []storage.NZBFile{{
			NzbID: "nzo-download-missing",
			Name:  "movie.mkv",
			Segments: []storage.NZBSegment{
				{Number: 1, MessageID: "seg-1@test", Bytes: 3},
				{Number: 2, MessageID: "seg-2@test", Bytes: int64(len(seg2))},
				{Number: 3, MessageID: "seg-3@test", Bytes: int64(len(seg3))},
			},
		}},
	}
	if err := u.nzbStorage.AddNZB(nzb); err != nil {
		t.Fatalf("AddNZB: %v", err)
	}

	var out bytes.Buffer
	err = u.Download(context.Background(), "nzo-download-missing", "movie.mkv", &out, nil)
	if err == nil {
		t.Fatal("Download() expected missing segment error, got nil")
	}
	if !strings.Contains(err.Error(), "ARTICLE_NOT_FOUND") {
		t.Fatalf("Download() error = %v, want ARTICLE_NOT_FOUND", err)
	}
	if out.Len() != 0 {
		t.Fatalf("Download() wrote %d bytes despite missing first segment, want 0", out.Len())
	}
}

func TestDownload_ConnectionDropMidDownloadReturnsError(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)
	t.Setenv("DECYPHARR_USENET__MAX_CONNECTIONS", "1")

	seg1 := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	encoded := yencEncodeForNNTP(seg1, "movie.mkv", 1, 1, int64(len(seg1)))

	srv := startTestNNTPServer(t, "user", "pass", map[string][]byte{
		"seg-1@test": encoded,
	})
	srv.setBodyDrop("seg-1@test", len(encoded)/2)
	host, port := srv.hostPort()
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__HOST", host)
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PORT", itoa(port))
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__USERNAME", "user")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PASSWORD", "pass")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__MAX_CONNECTIONS", "1")

	cfgJSON := `{
		"download_folder":"/downloads",
		"usenet":{
			"max_connections":1,
			"providers":[{
				"host":"` + host + `",
				"port":` + itoa(port) + `,
				"username":"user",
				"password":"pass",
				"max_connections":1
			}]
		}
	}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	u, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = u.Close() })

	nzb := &storage.NZB{
		ID:   "nzo-download-drop",
		Name: "movie",
		Files: []storage.NZBFile{{
			NzbID: "nzo-download-drop",
			Name:  "movie.mkv",
			Segments: []storage.NZBSegment{
				{Number: 1, MessageID: "seg-1@test", Bytes: int64(len(seg1))},
			},
		}},
	}
	if err := u.nzbStorage.AddNZB(nzb); err != nil {
		t.Fatalf("AddNZB: %v", err)
	}

	var out bytes.Buffer
	err = u.Download(context.Background(), "nzo-download-drop", "movie.mkv", &out, nil)
	if err == nil {
		t.Fatal("Download() expected connection drop error, got nil")
	}
	if !strings.Contains(err.Error(), "streaming yenc decode failed") {
		t.Fatalf("Download() error = %v, want streaming decode failure", err)
	}
	if out.Len() != 0 {
		t.Fatalf("Download() wrote %d bytes despite dropped connection, want 0", out.Len())
	}
}

func TestDownload_ConcurrentSegmentDownloadMaintainsIntegrity(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)
	t.Setenv("DECYPHARR_USENET__MAX_CONNECTIONS", "6")

	type segDef struct {
		id   string
		data []byte
	}
	segments := []segDef{
		{id: "seg-1@test", data: []byte("AA")},
		{id: "seg-2@test", data: []byte("BB")},
		{id: "seg-3@test", data: []byte("CC")},
		{id: "seg-4@test", data: []byte("DD")},
		{id: "seg-5@test", data: []byte("EE")},
		{id: "seg-6@test", data: []byte("FF")},
	}

	articles := make(map[string][]byte, len(segments))
	for i, seg := range segments {
		articles[seg.id] = yencEncodeForNNTP(seg.data, "movie.mkv", i+1, 1, int64(len(seg.data)))
	}
	srv := startTestNNTPServer(t, "user", "pass", articles)
	host, port := srv.hostPort()
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__HOST", host)
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PORT", itoa(port))
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__USERNAME", "user")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PASSWORD", "pass")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__MAX_CONNECTIONS", "6")

	cfgJSON := `{
		"download_folder":"/downloads",
		"usenet":{
			"max_connections":6,
			"providers":[{
				"host":"` + host + `",
				"port":` + itoa(port) + `,
				"username":"user",
				"password":"pass",
				"max_connections":6
			}]
		}
	}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	u, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = u.Close() })

	nzbSegments := make([]storage.NZBSegment, 0, len(segments))
	for i, seg := range segments {
		nzbSegments = append(nzbSegments, storage.NZBSegment{
			Number:    i + 1,
			MessageID: seg.id,
			Bytes:     int64(len(seg.data)),
		})
	}
	nzb := &storage.NZB{
		ID:   "nzo-download-concurrent",
		Name: "movie",
		Files: []storage.NZBFile{{
			NzbID:    "nzo-download-concurrent",
			Name:     "movie.mkv",
			Segments: nzbSegments,
		}},
	}
	if err := u.nzbStorage.AddNZB(nzb); err != nil {
		t.Fatalf("AddNZB: %v", err)
	}

	var out bytes.Buffer
	if err := u.Download(context.Background(), "nzo-download-concurrent", "movie.mkv", &out, nil); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	var want []byte
	for _, seg := range segments {
		want = append(want, seg.data...)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("concurrent assembled output mismatch: got %q want %q", out.Bytes(), want)
	}

	requested := make(map[string]struct{}, len(segments))
	for {
		select {
		case msgID := <-srv.bodyEvents:
			requested[msgID] = struct{}{}
		default:
			if len(requested) != len(segments) {
				t.Fatalf("requested segments = %d, want %d", len(requested), len(segments))
			}
			return
		}
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
