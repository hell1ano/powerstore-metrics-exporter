package utils

import (
	"path/filepath"
	"testing"
)

func TestLoggerStdoutAndUnavailableFile(t *testing.T) {
	for _, path := range []string{"", "-", filepath.Join(t.TempDir(), "missing-parent", "log")} {
		logger := GetLogger("info", path, "json")
		if err := logger.Log("msg", "synthetic log check"); err != nil {
			t.Fatalf("logging failed for %q: %v", path, err)
		}
	}
}
