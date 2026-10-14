package utils

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func boolPtr(v bool) *bool { return &v }
func TestLogStreams(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "powerstore.log")
	l, closeLogs, err := NewLoggers(Logs{Path: path, Stdout: boolPtr(false)})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = level.Info(log.With(l, "stream", "client_request")).Log("msg", "incoming-only")
			_ = level.Warn(l).Log("msg", "collection-only")
			_ = level.Info(l).Log("stream", "powerstore_api", "msg", "outgoing-only")
		}()
	}
	wg.Wait()
	closeLogs()
	a, _ := os.ReadFile(path)
	b, _ := os.ReadFile(filepath.Join(dir, "nested", "powerstore.access.log"))
	if strings.Contains(string(a), "incoming-only") || strings.Count(string(a), "outgoing-only") != 20 || strings.Count(string(a), "collection-only") != 20 {
		t.Fatal("incorrect activity routing")
	}
	if strings.Contains(string(b), "outgoing-only") || strings.Contains(string(b), "collection-only") || strings.Count(string(b), "incoming-only") != 20 {
		t.Fatal("incorrect access routing")
	}
}
func TestLogRotationAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "powerstore.log")
	// Existing oversized log must rotate on first write, without manual deletion.
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 1100000), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := normalizeLogs(Logs{Stdout: boolPtr(false), Rotation: Rotation{MaxSizeMB: 1, MaxBackups: 2, MaxAgeDays: 1, Compress: boolPtr(false)}})
	if err != nil {
		t.Fatal(err)
	}
	w, closer := logWriter(path, c)
	defer closer.Close()
	for i := 0; i < 5; i++ {
		if _, err := w.Write(bytes.Repeat([]byte("y"), 600000)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "powerstore-*.log"))
		if len(files) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("retention left %d backups", len(files))
		}
		time.Sleep(10 * time.Millisecond)
	}
	stat, err := os.Stat(path)
	if err != nil || stat.Size() > 1024*1024 {
		t.Fatalf("active file not bounded: %v", err)
	}
}
func TestLogSettings(t *testing.T) {
	c, err := normalizeLogs(Logs{Path: "legacy.log"})
	if err != nil || c.AccessPath != "legacy.access.log" || c.Rotation.MaxSizeMB != 50 || c.Rotation.MaxBackups != 5 || c.Rotation.MaxAgeDays != 7 {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	for _, c := range []Logs{{Path: "same.log", AccessPath: "./same.log"}, {Rotation: Rotation{MaxSizeMB: -1}}, {Rotation: Rotation{MaxBackups: -1}}, {Rotation: Rotation{MaxAgeDays: -1}}} {
		if _, err := normalizeLogs(c); err == nil {
			t.Fatal("invalid settings accepted")
		}
	}
}

func TestLogCompressionAndAgeCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.log")
	old := filepath.Join(dir, "events-2000-01-01T00-00-00.000.log")
	if err := os.WriteFile(old, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	c, _ := normalizeLogs(Logs{Stdout: boolPtr(false), Rotation: Rotation{MaxSizeMB: 1, MaxBackups: 2, MaxAgeDays: 1}})
	w, closer := logWriter(path, c)
	defer closer.Close()
	for i := 0; i < 2; i++ {
		if _, err := w.Write(bytes.Repeat([]byte("x"), 600000)); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		files, _ := filepath.Glob(filepath.Join(dir, "events-*.log.gz"))
		_, oldErr := os.Stat(old)
		if len(files) == 1 && os.IsNotExist(oldErr) {
			f, err := os.Open(files[0])
			if err != nil {
				t.Fatal(err)
			}
			z, err := gzip.NewReader(f)
			if err == nil {
				b, e := io.ReadAll(z)
				_ = z.Close()
				_ = f.Close()
				if e == nil && len(b) == 600000 {
					return
				}
			} else {
				_ = f.Close()
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("compression/age cleanup did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
