package usenet

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func TestStream_E2E_ThreeSegments_ExactBytesAssembled(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)

	seg1 := []byte("AAA")
	seg2 := []byte("BBB")
	seg3 := []byte("CCC")

	srv := startTestNNTPServer(t, "user", "pass", map[string][]byte{
		"seg-1@stream": yencEncodeForNNTP(seg1, "movie.mkv", 1, 1, int64(len(seg1))),
		"seg-2@stream": yencEncodeForNNTP(seg2, "movie.mkv", 2, 4, 6),
		"seg-3@stream": yencEncodeForNNTP(seg3, "movie.mkv", 3, 7, 9),
	})
	host, port := srv.hostPort()

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

	const totalSize = 9
	nzb := &storage.NZB{
		ID:   "nzo-stream-e2e-1",
		Name: "movie",
		Files: []storage.NZBFile{{
			NzbID: "nzo-stream-e2e-1",
			Name:  "movie.mkv",
			Size:  totalSize,
			Segments: []storage.NZBSegment{
				{Number: 1, MessageID: "seg-1@stream", Bytes: int64(len(seg1))},
				{Number: 2, MessageID: "seg-2@stream", Bytes: int64(len(seg2))},
				{Number: 3, MessageID: "seg-3@stream", Bytes: int64(len(seg3))},
			},
		}},
	}
	if err := u.nzbStorage.AddNZB(nzb); err != nil {
		t.Fatalf("AddNZB: %v", err)
	}

	var out bytes.Buffer
	if err := u.Stream(context.Background(), "nzo-stream-e2e-1", "movie.mkv", 0, totalSize-1, &out); err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	want := append(append([]byte{}, seg1...), append(seg2, seg3...)...)
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("Stream() output mismatch: got %q, want %q", out.Bytes(), want)
	}
}

func TestStream_E2E_MissingSegment_ReturnsError(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)

	// Only seg-2 and seg-3 exist; seg-1 is missing → 430 from server.
	seg2 := []byte("BBB")
	seg3 := []byte("CCC")

	srv := startTestNNTPServer(t, "user", "pass", map[string][]byte{
		"seg-2@stream-missing": yencEncodeForNNTP(seg2, "movie.mkv", 2, 4, 6),
		"seg-3@stream-missing": yencEncodeForNNTP(seg3, "movie.mkv", 3, 7, 9),
	})
	host, port := srv.hostPort()

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

	const totalSize = 9
	nzb := &storage.NZB{
		ID:   "nzo-stream-missing",
		Name: "movie",
		Files: []storage.NZBFile{{
			NzbID: "nzo-stream-missing",
			Name:  "movie.mkv",
			Size:  totalSize,
			Segments: []storage.NZBSegment{
				{Number: 1, MessageID: "seg-1@stream-missing", Bytes: 3},
				{Number: 2, MessageID: "seg-2@stream-missing", Bytes: int64(len(seg2))},
				{Number: 3, MessageID: "seg-3@stream-missing", Bytes: int64(len(seg3))},
			},
		}},
	}
	if err := u.nzbStorage.AddNZB(nzb); err != nil {
		t.Fatalf("AddNZB: %v", err)
	}

	var out bytes.Buffer
	err = u.Stream(context.Background(), "nzo-stream-missing", "movie.mkv", 0, totalSize-1, &out)
	if err == nil {
		t.Fatal("Stream() expected missing segment error, got nil")
	}
	if !strings.Contains(err.Error(), "ARTICLE_NOT_FOUND") && !strings.Contains(err.Error(), "430") &&
		!strings.Contains(err.Error(), "no such article") && !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Stream() error = %q, want article-not-found indication", err.Error())
	}
}
