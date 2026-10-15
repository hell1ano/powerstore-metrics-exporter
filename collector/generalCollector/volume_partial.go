package generalCollector

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/bulkClient"
	"sort"
)

// Keep source selection and archive integrity/freshness checks. Only missing or
// unusable selected objects become partial results. Missing rows are zero-filled
// by monitoring policy, with failed health to distinguish them from measured zeros.
func readVolumePerformanceBulk(bc *bulkClient.BulkClient, expected map[string]gjson.Result, appliances map[string]gjson.Result, report func(string, error)) (string, error) {
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
	rows := []json.RawMessage{}
	seen := map[string]bool{}
	for _, row := range gjson.Parse(data).Array() {
		id := row.Get("volume_id").String()
		if id == "" {
			report("", fmt.Errorf("volume sample has no ID; sample omitted"))
			continue
		}
		if _, selected := expected[id]; !selected {
			continue
		}
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
		appliance := appliances[id].String()
		if appliance == "" || expected[id].String() == "" {
			report(id, fmt.Errorf("known volume is missing measurements; cannot zero-fill without inventory name and appliance ID"))
			continue
		}
		zero := map[string]interface{}{"volume_id": id, "appliance_id": appliance}
		for _, field := range metricVolumeCollectorMetric {
			zero[field] = 0
		}
		encoded, err := json.Marshal(zero)
		if err != nil {
			return "", err
		}
		rows = append(rows, encoded)
		report(id, fmt.Errorf("volume performance row absent from valid bulk data; displaying zero by monitoring policy"))
	}
	result, err := json.Marshal(rows)
	return string(result), err
}
