package bulkClient

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

func archive(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tr := tar.NewWriter(gz)
	if err := tr.WriteHeader(&tar.Header{Name: "metrics/performance_metrics_by_volume.csv", Mode: 0600, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sample(timestamp time.Time) string {
	return "volume_id,appliance_id,timestamp,avg_read_iops\nvolume-1,appliance-1," + timestamp.UTC().Format(time.RFC3339Nano) + ",42\n"
}

func testClient(t *testing.T, handler http.HandlerFunc) *BulkClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &BulkClient{IP: "array", IsEnable: true, baseUrl: server.URL + "/", http: server.Client(), outputDir: t.TempDir(), logger: log.NewNopLogger()}
}

func TestFailedRefreshPreservesGoodCache(t *testing.T) {
	valid := archive(t, sample(time.Now()))
	badCRC := append([]byte(nil), valid...)
	badCRC[len(badCRC)-8] ^= 0xff
	cases := []struct {
		name string
		code int
		body []byte
	}{
		{"HTTP failure", 500, []byte("unavailable")},
		{"not modified without a new archive", 304, nil},
		{"not gzip", 200, []byte("not gzip")},
		{"truncated", 200, valid[:len(valid)-12]},
		{"checksum", 200, badCRC},
		{"bad CSV", 200, archive(t, "timestamp,volume_id\n\"unterminated")},
		{"invalid number", 200, archive(t, strings.Replace(sample(time.Now()), ",42", ",bad-number", 1))},
		{"no timestamp", 200, archive(t, "volume_id\nv1\n")},
		{"invalid timestamp", 200, archive(t, "timestamp\nyesterday\n")},
		{"future timestamp", 200, archive(t, sample(time.Now().Add(time.Hour)))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, code := valid, 200
			var responseMu sync.RWMutex
			bc := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				responseMu.RLock()
				defer responseMu.RUnlock()
				w.WriteHeader(code)
				_, _ = w.Write(body)
			})
			if err := bc.DownloadBulkData(); err != nil {
				t.Fatal(err)
			}
			last := bc.lastSuccess
			responseMu.Lock()
			body, code = tc.body, tc.code
			responseMu.Unlock()
			if err := bc.DownloadBulkData(); err == nil {
				t.Fatal("expected refresh failure")
			}
			if bc.downloadSuccess || !bc.lastSuccess.Equal(last) {
				t.Fatal("failed refresh changed success time")
			}
			disk, err := os.ReadFile(filepath.Join(bc.outputDir, "pst_bulk_array.tar.gz"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(disk, valid) {
				t.Fatal("good cache replaced by invalid download")
			}
			if _, err := bc.ReadCsvData("PerformanceMetricsByVolume"); err != nil {
				t.Fatal(err)
			}
			files, _ := filepath.Glob(filepath.Join(bc.outputDir, ".pst-bulk-*.tmp"))
			if len(files) != 0 {
				t.Fatal("temporary download leaked")
			}
			responseMu.Lock()
			body, code = valid, 200
			responseMu.Unlock()
			if err := bc.DownloadBulkData(); err != nil || !bc.downloadSuccess {
				t.Fatalf("did not recover: %v", err)
			}
		})
	}
}

func TestFreshnessAndMissingModules(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		stale         bool
	}{
		{"fresh", sample(time.Now()), false},
		{"stale source", sample(time.Now().Add(-time.Hour)), true},
		{"empty optional module", "volume_id,appliance_id,timestamp,avg_read_iops\n", false},
		{"mixed old and new", sample(time.Now()) + "v2,a1," + time.Now().Add(-time.Hour).UTC().Format(time.RFC3339) + ",1\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := archive(t, tc.content)
			bc := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
			if _, err := bc.ReadCsvData("PerformanceMetricsByVolume"); err == nil {
				t.Fatal("uninitialized cache accepted")
			}
			if err := bc.DownloadBulkData(); err != nil {
				t.Fatal(err)
			}
			_, err := bc.ReadCsvData("PerformanceMetricsByVolume")
			if (err != nil) != tc.stale {
				t.Fatalf("staleness = %v, error %v", tc.stale, err)
			}
			if _, err := bc.ReadCsvData("PerformanceMetricsByAppliance"); err == nil {
				t.Fatal("missing module accepted")
			}
			bc.lastSuccess = time.Now().Add(-time.Hour)
			if _, err := bc.ReadCsvData("PerformanceMetricsByVolume"); err == nil {
				t.Fatal("old download accepted")
			}
		})
	}
}

func TestConcurrentReadAndRefresh(t *testing.T) {
	body := archive(t, sample(time.Now()))
	bc := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
	if err := bc.DownloadBulkData(); err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(bc)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if err := bc.DownloadBulkData(); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				data, err := bc.ReadCsvData("PerformanceMetricsByVolume")
				if err != nil || data == "" {
					t.Error(fmt.Sprintf("partial cache read: %v", err))
				}
				if _, err := registry.Gather(); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}
