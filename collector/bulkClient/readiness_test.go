package bulkClient

import (
	"testing"
	"time"
)

func TestCacheReadiness(t *testing.T) {
	bc := &BulkClient{}
	if bc.CacheReady() {
		t.Fatal("empty cache ready")
	}
	bc.downloadSuccess = true
	bc.lastSuccess = time.Now()
	bc.modules = map[string]moduleStatus{"test": {rows: 1, timestamp: time.Now()}}
	if !bc.CacheReady() {
		t.Fatal("fresh cache not ready")
	}
	bc.downloadSuccess = false
	if bc.CacheReady() {
		t.Fatal("failed refresh ready")
	}
	bc.downloadSuccess = true
	bc.modules["test"] = moduleStatus{rows: 1, timestamp: time.Now().Add(-time.Hour)}
	if bc.CacheReady() {
		t.Fatal("stale source ready")
	}
}
