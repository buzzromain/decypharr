package usenet

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/customerror"
	"github.com/sirrobot01/decypharr/internal/nntp"
	"github.com/sirrobot01/decypharr/pkg/storage"
	"github.com/sirrobot01/decypharr/pkg/usenet/types"
)

type testPrefetchReader struct {
	data []byte
}

func (r *testPrefetchReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if off+int64(n) >= int64(len(r.data)) {
		return n, io.EOF
	}
	return n, nil
}

func (r *testPrefetchReader) Prefetch(_ context.Context, _, _ int64) {}

type articleNotFoundReader struct{}

func (r *articleNotFoundReader) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, &nntp.Error{Type: nntp.ErrorTypeArticleNotFound, Message: "missing"}
}

func (r *articleNotFoundReader) Prefetch(_ context.Context, _, _ int64) {}

func TestNew_NoProvidersConfiguredReturnsError(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)

	cfgJSON := `{"download_folder":"/downloads","usenet":{"providers":[]}}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := New(); err == nil {
		t.Fatal("New() expected error when no usenet providers are configured, got nil")
	}
}

func TestNew_UsesFallbacksForInvalidReadAheadAndConnections(t *testing.T) {
	base := t.TempDir()
	config.Reset()
	t.Cleanup(config.Reset)
	config.SetConfigPath(base)

	cfgJSON := `{
		"download_folder":"/downloads",
		"usenet":{
			"max_connections":5,
			"read_ahead":"not-a-size",
			"providers":[{"host":"news.example.com","username":"u","password":"p","max_connections":1}]
		}
	}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("DECYPHARR_USENET__MAX_CONNECTIONS", "-1")

	u, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = u.Close() })

	if u.maxConnections != 10 {
		t.Fatalf("maxConnections = %d, want fallback 10", u.maxConnections)
	}
	if u.prefetchSize != 16*1024*1024 {
		t.Fatalf("prefetchSize = %d, want fallback 16MB", u.prefetchSize)
	}

	if _, err := os.Stat(filepath.Join(base, "usenet", "nzbs")); err != nil {
		t.Fatalf("metadata directory not created: %v", err)
	}
}

func TestDownload_CanceledContextStopsWorkflow(t *testing.T) {
	base := t.TempDir()
	config.SetConfigPath(base)

	nzbStorage, err := NewNZBStorage()
	if err != nil {
		t.Fatalf("NewNZBStorage() error = %v", err)
	}

	nzb := &storage.NZB{
		ID:   "nzo-1",
		Name: "test.nzb",
		Files: []storage.NZBFile{{
			NzbID: "nzo-1",
			Name:  "file.bin",
			Segments: []storage.NZBSegment{
				{Number: 1, MessageID: "<a@x>", Bytes: 100},
				{Number: 2, MessageID: "<b@x>", Bytes: 100},
			},
		}},
	}
	if err := nzbStorage.AddNZB(nzb); err != nil {
		t.Fatalf("AddNZB() error = %v", err)
	}

	u := &Usenet{
		nzbStorage:     nzbStorage,
		maxConnections: 2,
		logger:         zerolog.Nop(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	err = u.Download(ctx, "nzo-1", "file.bin", &out, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Download() error = %v, want context canceled", err)
	}
	if out.Len() != 0 {
		t.Fatalf("download wrote %d bytes after cancellation, want 0", out.Len())
	}
}

func TestStream_UsesCachedEntryAndClampsRange(t *testing.T) {
	u := &Usenet{
		fs:          xsync.NewMap[string, *fsEntry](),
		failedFiles: xsync.NewMap[string, error](),
		logger:      zerolog.Nop(),
	}

	content := []byte("0123456789")
	entry := &fsEntry{
		volumes: []*types.Volume{{Name: "file.bin", Size: int64(len(content))}},
	}

	reader := &testPrefetchReader{data: content}
	entry.readerOnce.Do(func() {
		entry.reader = reader
		entry.readerSize = int64(len(content))
	})

	key := fsKey("nzo-2", "file.bin")
	u.fs.Store(key, entry)

	var out bytes.Buffer
	if err := u.Stream(context.Background(), "nzo-2", "file.bin", 3, 1000, &out); err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	if got, want := out.String(), "3456789"; got != want {
		t.Fatalf("stream output = %q, want %q", got, want)
	}
}

func TestStream_ArticleNotFoundBecomesPermanentAndIsCached(t *testing.T) {
	u := &Usenet{
		fs:          xsync.NewMap[string, *fsEntry](),
		failedFiles: xsync.NewMap[string, error](),
		logger:      zerolog.Nop(),
	}

	entry := &fsEntry{
		volumes: []*types.Volume{{Name: "file.bin", Size: 10}},
	}
	entry.readerOnce.Do(func() {
		entry.reader = &articleNotFoundReader{}
		entry.readerSize = 10
	})

	key := fsKey("nzo-3", "file.bin")
	u.fs.Store(key, entry)

	var out bytes.Buffer
	err := u.Stream(context.Background(), "nzo-3", "file.bin", 0, 9, &out)
	if err == nil {
		t.Fatal("Stream() expected article-not-found error, got nil")
	}

	ce := customerror.FromError(err)
	if !ce.IsPermanent() {
		t.Fatalf("error permanent = false, want true, err=%v", err)
	}
	if _, ok := u.failedFiles.Load(key); !ok {
		t.Fatal("failedFiles cache was not updated for article-not-found error")
	}
}

func TestProcessNewNZBs_SkipsMarkedAndMarksInvalidAsFailed(t *testing.T) {
	base := t.TempDir()
	metaDir := filepath.Join(base, "usenet", "nzbs")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir meta dir: %v", err)
	}

	mustWrite := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(metaDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	mustWrite("new-invalid.nzb", "not xml")
	mustWrite("already-processed.nzb", "<nzb><file/></nzb>")
	mustWrite("already-processed.nzb.processed", "done")
	mustWrite("in-progress.nzb", "<nzb><file/></nzb>")
	mustWrite("in-progress.nzb.processing", "pid")
	mustWrite("already-failed.nzb", "<nzb><file/></nzb>")
	mustWrite("already-failed.nzb.failed", "old fail")

	u := &Usenet{
		metadataDir: metaDir,
		logger:      zerolog.Nop(),
	}

	nzbs, err := u.ProcessNewNZBs(context.Background())
	if err != nil {
		t.Fatalf("ProcessNewNZBs() error = %v", err)
	}
	if len(nzbs) != 0 {
		t.Fatalf("processed nzbs = %d, want 0 for invalid-only input", len(nzbs))
	}

	if _, err := os.Stat(filepath.Join(metaDir, "new-invalid.nzb.failed")); err != nil {
		t.Fatalf("new-invalid.nzb.failed not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(metaDir, "already-processed.nzb.failed")); !os.IsNotExist(err) {
		t.Fatalf("already-processed should not be reprocessed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(metaDir, "in-progress.nzb.failed")); !os.IsNotExist(err) {
		t.Fatalf("in-progress should be skipped, got err=%v", err)
	}
}
