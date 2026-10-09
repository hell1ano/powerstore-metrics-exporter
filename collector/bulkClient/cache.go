package bulkClient

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const defaultMaxAge = 15 * time.Minute
const maxArchiveBytes int64 = 512 << 20

type moduleStatus struct {
	timestamp time.Time // Oldest source sample: one fresh row must not hide stale rows.
	rows      int
}

// DownloadBulkData preserves the last good cache on every failure. Only complete,
// validated archives are published. No data is extracted onto the filesystem.
func (bc *BulkClient) DownloadBulkData() (err error) {
	bc.downloadMu.Lock()
	defer bc.downloadMu.Unlock()
	defer func() {
		bc.cacheMu.Lock()
		bc.downloadSuccess = err == nil
		bc.cacheMu.Unlock()
	}()
	req, err := http.NewRequest(http.MethodPost, bc.baseUrl+"latest_five_min_metrics/download", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(bc.username, bc.password)
	req.Header.Set("DELL-VISIBILITY", "Partner")
	req.Header.Set("If-None-Match", "start")
	resp, err := bc.http.Do(req)
	if err != nil {
		return fmt.Errorf("download bulk metrics: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download bulk metrics: HTTP %d", resp.StatusCode)
	}
	file, err := os.CreateTemp(bc.outputDir, ".pst-bulk-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	n, err := io.Copy(file, io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return fmt.Errorf("write bulk metrics: %w", err)
	}
	if n > maxArchiveBytes {
		return fmt.Errorf("bulk archive exceeds %d bytes", maxArchiveBytes)
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	modules, err := validateArchive(file)
	if err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	bc.cacheMu.Lock()
	defer bc.cacheMu.Unlock()
	target := filepath.Join(bc.outputDir, fmt.Sprintf("pst_bulk_%s.tar.gz", bc.IP))
	if err = os.Rename(file.Name(), target); err != nil {
		return err
	}
	bc.modules = modules
	bc.lastSuccess = time.Now()
	return nil
}

func validateArchive(reader io.Reader) (map[string]moduleStatus, error) {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("invalid bulk gzip: %w", err)
	}
	defer gz.Close()
	// Bound expanded input too, including ignored archive members.
	limited := &io.LimitedReader{R: gz, N: maxArchiveBytes + 1}
	tr := tar.NewReader(limited)
	modules := make(map[string]moduleStatus)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid bulk tar: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		module := ""
		for key, filename := range moduleToCsvFileMap {
			if filepath.Base(header.Name) == filename {
				module = key
				break
			}
		}
		if module == "" {
			continue
		}
		if _, exists := modules[module]; exists {
			return nil, fmt.Errorf("duplicate bulk module %s", module)
		}
		var csvData bytes.Buffer
		csvReader := csv.NewReader(io.TeeReader(tr, &csvData))
		columns, err := csvReader.Read()
		if err != nil {
			return nil, fmt.Errorf("%s header: %w", module, err)
		}
		timestampColumn := -1
		for i, name := range columns {
			if name == "timestamp" {
				timestampColumn = i
			}
		}
		if timestampColumn < 0 {
			return nil, fmt.Errorf("%s has no timestamp column", module)
		}
		status := moduleStatus{}
		for {
			row, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("%s CSV: %w", module, err)
			}
			timestamp, err := time.Parse(time.RFC3339Nano, row[timestampColumn])
			if err != nil || timestamp.IsZero() || timestamp.After(time.Now().Add(time.Minute)) {
				return nil, fmt.Errorf("%s has invalid or future source timestamp %q", module, row[timestampColumn])
			}
			if status.timestamp.IsZero() || timestamp.Before(status.timestamp) {
				status.timestamp = timestamp
			}
			status.rows++
		}
		if _, err := decodeCSV(moduleToCsvFileMap[module], bytes.NewReader(csvData.Bytes())); err != nil {
			return nil, fmt.Errorf("%s metric values: %w", module, err)
		}
		modules[module] = status
	}
	// Read through the gzip trailer so truncated streams and bad CRCs cannot be published.
	if _, err := io.Copy(io.Discard, limited); err != nil {
		return nil, fmt.Errorf("bulk gzip checksum: %w", err)
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("expanded bulk archive exceeds %d bytes", maxArchiveBytes)
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("bulk archive contains no recognized metric CSVs")
	}
	return modules, nil
}

func (bc *BulkClient) checkFreshness(module string, now time.Time) error {
	maxAge := bc.MaxAge
	if maxAge <= 0 {
		maxAge = defaultMaxAge
	}
	if bc.lastSuccess.IsZero() || now.Sub(bc.lastSuccess) > maxAge {
		return fmt.Errorf("bulk cache has no recent successful download")
	}
	status, ok := bc.modules[module]
	if !ok {
		return fmt.Errorf("bulk cache is missing %s", module)
	}
	if status.rows > 0 && now.Sub(status.timestamp) > maxAge {
		return fmt.Errorf("bulk source samples for %s are older than %s", module, maxAge)
	}
	return nil
}

var bulkEnabled = prometheus.NewDesc("powerstore_bulk_enabled", "Whether bulk collection is enabled.", []string{"IP"}, nil)
var bulkSuccess = prometheus.NewDesc("powerstore_bulk_download_success", "Whether the latest bulk refresh succeeded (zero before first success).", []string{"IP"}, nil)
var bulkLastSuccess = prometheus.NewDesc("powerstore_bulk_last_success_timestamp_seconds", "Unix time of last validated cache replacement, not source sample time.", []string{"IP"}, nil)
var bulkSourceTime = prometheus.NewDesc("powerstore_bulk_source_timestamp_seconds", "Oldest source sample in a bulk module; zero for a header-only CSV.", []string{"IP", "module"}, nil)
var bulkFresh = prometheus.NewDesc("powerstore_bulk_module_fresh", "Whether both download and source sample ages are within bulkMaxAge.", []string{"IP", "module"}, nil)

// Describe and Collect expose cache health without contacting the array.
func (bc *BulkClient) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{bulkEnabled, bulkSuccess, bulkLastSuccess, bulkSourceTime, bulkFresh} {
		ch <- d
	}
}

func (bc *BulkClient) Collect(ch chan<- prometheus.Metric) {
	bc.cacheMu.RLock()
	defer bc.cacheMu.RUnlock()
	enabled, success := 0.0, 0.0
	if bc.IsEnable {
		enabled = 1
	}
	if bc.downloadSuccess {
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(bulkEnabled, prometheus.GaugeValue, enabled, bc.IP)
	ch <- prometheus.MustNewConstMetric(bulkSuccess, prometheus.GaugeValue, success, bc.IP)
	last := 0.0
	if !bc.lastSuccess.IsZero() {
		last = float64(bc.lastSuccess.Unix())
	}
	ch <- prometheus.MustNewConstMetric(bulkLastSuccess, prometheus.GaugeValue, last, bc.IP)
	for module, status := range bc.modules {
		timestamp, fresh := 0.0, 0.0
		if !status.timestamp.IsZero() {
			timestamp = float64(status.timestamp.Unix())
		}
		if bc.checkFreshness(module, time.Now()) == nil {
			fresh = 1
		}
		ch <- prometheus.MustNewConstMetric(bulkSourceTime, prometheus.GaugeValue, timestamp, bc.IP, module)
		ch <- prometheus.MustNewConstMetric(bulkFresh, prometheus.GaugeValue, fresh, bc.IP, module)
	}
}
