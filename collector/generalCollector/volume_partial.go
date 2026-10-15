package generalCollector

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/bulkClient"
	"sort"
)

// Keep source selection and archive integrity/freshness checks. Only missing or
// unusable selected objects become partial results; never invent zero samples.
func readVolumePerformanceBulk(bc *bulkClient.BulkClient, expected map[string]gjson.Result, report func(string, error)) (string, error) {
	if expected == nil {
		return "", fmt.Errorf("inventory unavailable")
	}
	if len(expected) == 0 {
		return "[]", nil
	}
	data, err := bc.ReadCsvData("PerformanceMetricsByVolume")
	if err != nil {
		return "", err
	}
	data, err = selectMonitoredRows(data, "volume_id", expected)
	if err != nil {
		return "", err
	}
	rows := []json.RawMessage{}
	seen := map[string]bool{}
	for _, row := range gjson.Parse(data).Array() {
		id := row.Get("volume_id").String()
		if seen[id] {
			report(id, fmt.Errorf("duplicate volume sample; duplicate omitted"))
			continue
		}
		seen[id] = true
		if expected[id].String() == "" {
			report(id, fmt.Errorf("inventory name is empty; sample omitted"))
			continue
		}
		usable := false
		for _, field := range metricVolumeCollectorMetric {
			if row.Get(field).Type == gjson.Number {
				usable = true
				break
			}
		}
		if !usable {
			report(id, fmt.Errorf("volume has no supported numeric measurements"))
			continue
		}
		rows = append(rows, json.RawMessage(row.Raw))
	}
	missing := []string{}
	for id := range expected {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		report(id, fmt.Errorf("known volume is missing measurements in PerformanceMetricsByVolume"))
	}
	result, err := json.Marshal(rows)
	return string(result), err
}
