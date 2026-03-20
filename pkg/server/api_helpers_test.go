package server

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"
	"time"

	json "github.com/bytedance/sonic"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/logger"
	"github.com/sirrobot01/decypharr/pkg/manager"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()

	origPath := config.GetMainPath()
	config.SetConfigPath(dir)
	t.Cleanup(func() { config.SetConfigPath(origPath) })

	config.Get().UseAuth = false
	config.Get().DownloadFolder = dir

	mgr := manager.New()
	t.Cleanup(func() { _ = mgr.Stop() })
	return &Server{
		logger:  logger.New("api-test"),
		manager: mgr,
	}
}

func seedEntry(t *testing.T, s *Server, infohash, name, category string, size int64) {
	t.Helper()
	entry := &storage.Entry{
		Protocol:       config.ProtocolTorrent,
		InfoHash:       infohash,
		Name:           name,
		Size:           size,
		Category:       category,
		State:          storage.EntryStatePausedUP,
		AddedOn:        time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Files:          make(map[string]*storage.File),
		Providers:      make(map[string]*storage.ProviderEntry),
		ActiveProvider: "",
	}
	entry.Files["test.mkv"] = &storage.File{
		Name:     "test.mkv",
		Size:     size,
		InfoHash: infohash,
		AddedOn:  time.Now(),
	}
	if err := s.manager.AddOrUpdate(entry, nil); err != nil {
		t.Fatalf("seedEntry: %v", err)
	}
}

func buildMultipartForm(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("WriteField(%s): %v", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart.Close: %v", err)
	}
	return body, writer.FormDataContentType()
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("JSON decode: %v\nbody: %s", err, rec.Body.String())
	}
}
