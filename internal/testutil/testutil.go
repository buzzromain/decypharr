package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

// GetTestDataPath returns the path to the testdata directory in the project root
func GetTestDataPath() string {
	return filepath.Join("..", "..", "testdata")
}

// GetTestDataFilePath returns the path to a specific file in the testdata directory
func GetTestDataFilePath(filename string) string {
	return filepath.Join(GetTestDataPath(), filename)
}

// GetTestTorrentPath returns the path to the Ubuntu test torrent file
func GetTestTorrentPath() string {
	return GetTestDataFilePath("ubuntu-25.04-desktop-amd64.iso.torrent")
}

// GetTestMagnetPath returns the path to the Ubuntu test magnet file
func GetTestMagnetPath() string {
	return GetTestDataFilePath("ubuntu-25.04-desktop-amd64.iso.magnet")
}

// GetTestDataBytes reads and returns the raw bytes of a test data file
func GetTestDataBytes(filename string) ([]byte, error) {
	filePath := GetTestDataFilePath(filename)
	return os.ReadFile(filePath)
}

// GetTestDataContent reads and returns the content of a test data file
func GetTestDataContent(filename string) (string, error) {
	content, err := GetTestDataBytes(filename)
	return strings.TrimSpace(string(content)), err
}

// GetTestMagnetContent reads and returns the content of the Ubuntu test magnet file
func GetTestMagnetContent() (string, error) {
	return GetTestDataContent("ubuntu-25.04-desktop-amd64.iso.magnet")
}

// SetupTestConfig creates a minimal config.json in a temp directory so that
// config.Get() succeeds during tests. It registers cleanup via t.Cleanup.
func SetupTestConfig(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	configJSON := `{
		"debrids": [{"name":"test","provider":"realdebrid","api_key":"testkey","folder":"/tmp/test/realdebrid/__all__"}],
		"download_folder": "` + filepath.ToSlash(filepath.Join(tmpDir, "downloads")) + `"
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	config.SetConfigPath(tmpDir)
	config.Reset()
	t.Cleanup(func() {
		config.Reset()
	})
}

// SetupTestConfigDir is like SetupTestConfig but works without *testing.T.
// It returns the temp dir and a cleanup function.
func SetupTestConfigDir() (string, func()) {
	tmpDir, err := os.MkdirTemp("", "decypharr-test-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	configJSON := `{
		"debrids": [{"name":"test","provider":"realdebrid","api_key":"testkey","folder":"` + filepath.ToSlash(filepath.Join(tmpDir, "test/realdebrid/__all__")) + `"}],
		"download_folder": "` + filepath.ToSlash(filepath.Join(tmpDir, "downloads")) + `"
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(configJSON), 0644); err != nil {
		panic("failed to write test config: " + err.Error())
	}
	config.SetConfigPath(tmpDir)
	config.Reset()
	return tmpDir, func() {
		config.Reset()
		os.RemoveAll(tmpDir)
	}
}
