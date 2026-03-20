package vfs

import (
	"os"
	"testing"

	"github.com/sirrobot01/decypharr/internal/testutil"
)

func TestMain(m *testing.M) {
	_, cleanup := testutil.SetupTestConfigDir()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
