package utils

import (
	"testing"
)

func TestLoggerStdoutAndUnavailableFile(t *testing.T) {
	for _, path := range []string{"", "-", t.TempDir()} {
		logger, closeLogs, err := NewLoggers(Logs{Level: "info", Path: path, Type: "json"})
		if err != nil {
			t.Fatal(err)
		}
		defer closeLogs()
		if err := logger.Log("msg", "synthetic log check"); err != nil {
			t.Fatalf("logging failed for %q: %v", path, err)
		}
	}
}
