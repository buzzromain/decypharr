package usenet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

func TestParseAndProcess_HappyPathMarksCompletedAndStoresResult(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)

	segmentPayload := []byte("ABCD")
	srv := startTestNNTPServer(t, "user", "pass", map[string][]byte{
		"seg-parse@test": yencEncodeForNNTP(segmentPayload, "movie.mkv", 1, 1, int64(len(segmentPayload))),
	})
	host, port := srv.hostPort()

	cfgJSON := `{
		"download_folder":"/downloads",
		"usenet":{
			"providers":[{
				"host":"` + host + `",
				"port":` + itoa(port) + `,
				"username":"user",
				"password":"pass",
				"max_connections":2
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

	nzbContent := buildSimpleNZB("movie.mkv", "seg-parse@test", len(segmentPayload))
	nzb, groups, err := u.Parse(context.Background(), "movie.nzb", nzbContent, "movies")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if nzb.ID == "" {
		t.Fatal("Parse() produced empty NZB ID")
	}
	if len(groups) == 0 {
		t.Fatal("Parse() returned no groups")
	}
	if nzb.Status != NZBStatusParsing {
		t.Fatalf("nzb status after Parse = %q, want %q", nzb.Status, NZBStatusParsing)
	}

	if _, err := os.Stat(nzb.Path + ".processing"); err != nil {
		t.Fatalf("processing marker missing after Parse: %v", err)
	}

	processed, err := u.Process(context.Background(), nzb, groups)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if processed.Status != NZBStatusCompleted {
		t.Fatalf("processed status = %q, want %q", processed.Status, NZBStatusCompleted)
	}
	if len(processed.Files) == 0 {
		t.Fatal("Process() produced no files")
	}

	if _, err := os.Stat(nzb.Path + ".processed"); err != nil {
		t.Fatalf("processed marker missing after Process: %v", err)
	}
	if _, err := os.Stat(nzb.Path + ".processing"); !os.IsNotExist(err) {
		t.Fatalf("processing marker should be removed after success, got err=%v", err)
	}

	stored, err := u.GetNZB(processed.ID)
	if err != nil {
		t.Fatalf("GetNZB() error = %v", err)
	}
	if stored.Status != NZBStatusCompleted {
		t.Fatalf("stored status = %q, want %q", stored.Status, NZBStatusCompleted)
	}
	if len(stored.Files) == 0 {
		t.Fatal("stored NZB has no files")
	}
}
