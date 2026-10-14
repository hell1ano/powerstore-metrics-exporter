package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// splitLogger keeps incoming polls separate from exporter and array activity.
type splitLogger struct{ activity, access log.Logger }

func (l splitLogger) Log(kv ...interface{}) error {
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i] == "stream" && kv[i+1] == "client_request" {
			return l.access.Log(kv...)
		}
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i] == "stream" {
			return l.activity.Log(kv...)
		}
	}
	return l.activity.Log(append([]interface{}{"stream", "exporter"}, kv...)...)
}

func normalizeLogs(c Logs) (Logs, error) {
	r := &c.Rotation
	if r.MaxSizeMB < 0 || r.MaxBackups < 0 || r.MaxAgeDays < 0 {
		return c, fmt.Errorf("log rotation limits cannot be negative")
	}
	if r.MaxSizeMB == 0 {
		r.MaxSizeMB = 50
	}
	if r.MaxBackups == 0 {
		r.MaxBackups = 5
	}
	if r.MaxAgeDays == 0 {
		r.MaxAgeDays = 7
	}
	if c.AccessPath == "" {
		c.AccessPath = c.Path
		if c.Path != "" && c.Path != "-" {
			ext := filepath.Ext(c.Path)
			c.AccessPath = strings.TrimSuffix(c.Path, ext) + ".access" + ext
		}
	}
	if c.Path != "" && c.Path != "-" && c.AccessPath != "-" {
		a, err := filepath.Abs(c.Path)
		if err != nil {
			return c, err
		}
		b, err := filepath.Abs(c.AccessPath)
		if err != nil {
			return c, err
		}
		if strings.EqualFold(a, b) {
			return c, fmt.Errorf("log.path and log.accessPath must be different files")
		}
		// Also reject existing hard-link or symlink aliases.
		ai, ae := os.Stat(a)
		bi, be := os.Stat(b)
		if ae == nil && be == nil && os.SameFile(ai, bi) {
			return c, fmt.Errorf("log files refer to the same file")
		}
	}
	return c, nil
}

// File errors must remain visible even when callers ignore logger.Log errors.
// Report once per failure episode and preserve each failed entry on stdout.
type fallbackWriter struct {
	file   io.Writer
	mirror bool
	mu     sync.Mutex
	failed bool
}

func (w *fallbackWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.file.Write(p)
	if err != nil {
		if !w.failed {
			fmt.Fprintf(os.Stderr, "log file write failed; using stdout: %v\n", err)
		}
		w.failed = true
	} else {
		w.failed = false
	}
	if w.mirror || err != nil {
		return os.Stdout.Write(p)
	}
	return len(p), nil
}

func logWriter(path string, c Logs) (io.Writer, io.Closer) {
	if path == "" || path == "-" {
		return os.Stdout, nil
	}
	// Create private parent directories; bind mounts still require host permissions.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create log directory; using stdout: %v\n", err)
		return os.Stdout, nil
	}
	r := c.Rotation
	rolling := &lumberjack.Logger{Filename: path, MaxSize: r.MaxSizeMB, MaxBackups: r.MaxBackups, MaxAge: r.MaxAgeDays, Compress: r.Compress == nil || *r.Compress}
	// Precreate with private permissions, including when stdout duplication is off.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open log file; using stdout: %v\n", err)
		return os.Stdout, nil
	}
	_ = f.Close()
	return &fallbackWriter{file: rolling, mirror: c.Stdout == nil || *c.Stdout}, rolling
}

func NewLoggers(c Logs) (log.Logger, func(), error) {
	c, err := normalizeLogs(c)
	if err != nil {
		return nil, nil, err
	}
	activity, ac := logWriter(c.Path, c)
	access, rc := logWriter(c.AccessPath, c)
	closeLogs := func() {
		if ac != nil {
			_ = ac.Close()
		}
		if rc != nil {
			_ = rc.Close()
		}
	}
	return splitLogger{formatLogger(c.Level, c.Type, activity), formatLogger(c.Level, c.Type, access)}, closeLogs, nil
}
