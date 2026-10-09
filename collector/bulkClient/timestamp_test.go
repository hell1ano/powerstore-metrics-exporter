package bulkClient

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSourceTimestampFormats(t *testing.T) {
	want := time.Date(2026, 1, 2, 3, 4, 5, 123000000, time.UTC)
	for _, value := range []string{
		"2026-01-02T03:04:05.123Z", "2026-01-02 03:04:05.123+00",
		"2026-01-02 05:04:05.123+02", "2026-01-02 00:04:05.123-03",
		"2026-01-02 08:34:05.123+05:30",
	} {
		got, err := parseSourceTimestamp(value)
		if err != nil || !got.Equal(want) {
			t.Errorf("parse %q: %v %v", value, got, err)
		}
	}
	for _, value := range []string{"2026-01-02 03:04:05", "yesterday", "2026-13-01 00:00:00+00"} {
		if _, err := parseSourceTimestamp(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestBulkTimestampRetainsSourceAge(t *testing.T) {
	for _, age := range []time.Duration{0, time.Hour} {
		now := time.Now().UTC().Truncate(time.Second)
		source := now.Add(-age)
		csv := strings.Replace(sample(source), source.Format(time.RFC3339Nano), source.Format("2006-01-02 15:04:05-07"), 1)
		modules, err := validateArchive(bytes.NewReader(archive(t, csv)))
		if err != nil {
			t.Fatal(err)
		}
		bc := &BulkClient{lastSuccess: now, modules: modules}
		if stale := bc.checkFreshness("PerformanceMetricsByVolume", now) != nil; stale != (age > defaultMaxAge) {
			t.Fatalf("incorrect freshness for age %v", age)
		}
	}
}
