package bulkClient

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartupReplacesPreexistingArchive(t *testing.T) {
	fresh := archive(t, sample(time.Now()))
	for _, tc := range []struct {
		name string
		old  []byte
	}{
		{"stale", archive(t, strings.Replace(sample(time.Now().Add(-24*time.Hour)), ",42", ",7", 1))},
		{"corrupt", []byte("leftover invalid archive")},
		{"empty", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bc := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(fresh) })
			target := filepath.Join(bc.outputDir, "pst_bulk_array.tar.gz")
			if err := os.WriteFile(target, tc.old, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := bc.ReadCsvData("PerformanceMetricsByVolume"); err == nil || !strings.Contains(err.Error(), "no recent successful download") {
				t.Fatalf("unexpected startup state: %v", err)
			}
			if err := bc.DownloadBulkData(); err != nil {
				t.Fatal(err)
			}
			disk, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(disk, fresh) {
				t.Fatalf("existing archive was not replaced: %v", err)
			}
			data, err := bc.ReadCsvData("PerformanceMetricsByVolume")
			if err != nil || !strings.Contains(data, `"avg_read_iops":42`) {
				t.Fatalf("new measurements unavailable: %s %v", data, err)
			}
			if !bc.downloadSuccess || bc.lastSuccess.IsZero() || !bc.CacheReady() {
				t.Fatal("successful replacement did not restore readiness")
			}
			files, err := filepath.Glob(filepath.Join(bc.outputDir, ".pst-bulk-*.tmp"))
			if err != nil || len(files) != 0 {
				t.Fatalf("temporary files remain: %v", files)
			}
		})
	}
}
