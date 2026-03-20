package torbox

import (
	"os"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "torbox-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	config.SetConfigPath(dir)
	os.Exit(m.Run())
}
