package generalCollector

import (
	"encoding/json"
	"errors"
	"fmt"
	"powerstore-metrics-exporter/collector/bulkClient"
	"sort"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

// The cache checks every row's source age. Coverage additionally checks every
// known object, so a fresh subset cannot hide missing objects. A missing inventory
// is different from a successfully loaded empty resource class.
func readBulkForObjects(bc *bulkClient.BulkClient, module, idField string, expected map[string]gjson.Result, metrics []string) (string, error) {
	data, err := bc.ReadCsvData(module)
	if err != nil {
		return "", err
	}
	if err := checkObjectCoverage(data, idField, expected, metrics); err != nil {
		return "", fmt.Errorf("%s: %w", module, err)
	}
	return data, nil
}

func checkObjectCoverage(data, idField string, expected map[string]gjson.Result, metrics []string) error {
	if expected == nil {
		return fmt.Errorf("inventory unavailable")
	}
	seen := map[string]bool{}
	var failures []error
	for _, row := range gjson.Parse(data).Array() {
		id := row.Get(idField).String()
		name, known := expected[id]
		seen[id] = true
		if !known {
			failures = append(failures, fmt.Errorf("%s=%q: sample ID absent from inventory", idField, id))
			continue
		}
		if name.String() == "" {
			failures = append(failures, fmt.Errorf("%s=%q: inventory name is empty", idField, id))
			continue
		}
		usable := false
		for _, metric := range metrics {
			if row.Get(metric).Type == gjson.Number {
				usable = true
				break
			}
		}
		if !usable {
			failures = append(failures, fmt.Errorf("%s=%q: object has no supported measurements", idField, id))
		}
	}
	for id := range expected {
		if !seen[id] {
			failures = append(failures, fmt.Errorf("%s=%q: known object is missing measurements", idField, id))
		}
	}
	return summarizeCoverageFailures(failures)
}

// Optional fields may be absent, but a known object must have a numeric sample.
func requireMeasurements(ch chan<- prometheus.Metric, logger log.Logger, ip, id string, row gjson.Result, fields []string) bool {
	for _, field := range fields {
		if row.Get(field).Type == gjson.Number {
			return true
		}
	}
	err := fmt.Errorf("object %q has no supported numeric measurements", id)
	reportCollectionError(ch, err)
	level.Warn(logger).Log("msg", "incomplete API sample", "ip", ip, "object_id", id, "err", err)
	return false
}

// Health endpoints must return every known object and its required string fields.
// New objects are accepted; they need not wait for a scheduled inventory refresh.
func checkHealthCoverage(data string, expected map[string]gjson.Result, fields []string) error {
	if expected == nil {
		return fmt.Errorf("inventory unavailable")
	}
	seen := map[string]bool{}
	var failures []error
	for _, row := range gjson.Parse(data).Array() {
		id := row.Get("id").String()
		seen[id] = true
		if err := checkStringFields(row, fields); err != nil {
			failures = append(failures, err)
		}
	}
	for id := range expected {
		if !seen[id] {
			failures = append(failures, fmt.Errorf("id=%q: known object missing from health response", id))
		}
	}
	return errors.Join(failures...)
}

func checkStringFields(row gjson.Result, fields []string) error {
	var failures []error
	for _, field := range fields {
		value := row.Get(field)
		if value.Type != gjson.String || value.String() == "" {
			failures = append(failures, fmt.Errorf("id=%q: missing or invalid %s", row.Get("id").String(), field))
		}
	}
	return errors.Join(failures...)
}

// Keep coverage failures deterministic and bounded even for thousands of IDs.
func summarizeCoverageFailures(failures []error) error {
	if len(failures) == 0 {
		return nil
	}
	messages := make([]string, len(failures))
	for i, err := range failures {
		messages[i] = err.Error()
	}
	sort.Strings(messages)
	const limit = 10
	shown := len(messages)
	if shown > limit {
		shown = limit
	}
	for i := 0; i < shown; i++ {
		if len(messages[i]) > 300 {
			messages[i] = messages[i][:300] + "..."
		}
	}
	return fmt.Errorf("%d coverage failures (showing %d): %s", len(messages), shown, strings.Join(messages[:shown], "; "))
}

// Source-only inventory is an explicit local allowlist. Bulk archives can also
// contain excluded destinations, snapshots or IDs not present on this array.
// Never remap those IDs to a source on another array.
func selectMonitoredRows(data, idField string, expected map[string]gjson.Result) (string, error) {
	if expected == nil {
		return "", fmt.Errorf("inventory unavailable")
	}
	rows := make([]json.RawMessage, 0)
	for _, row := range gjson.Parse(data).Array() {
		id := row.Get(idField).String()
		if id == "" {
			return "", fmt.Errorf("sample has no %s", idField)
		}
		if _, ok := expected[id]; ok {
			rows = append(rows, json.RawMessage(row.Raw))
		}
	}
	selected, err := json.Marshal(rows)
	return string(selected), err
}

func readBulkForMonitoredObjects(bc *bulkClient.BulkClient, module, idField string, expected map[string]gjson.Result, metrics []string) (string, error) {
	if expected == nil {
		return "", fmt.Errorf("inventory unavailable")
	}
	if len(expected) == 0 {
		return "[]", nil
	}
	data, err := bc.ReadCsvData(module)
	if err != nil {
		return "", err
	}
	data, err = selectMonitoredRows(data, idField, expected)
	if err != nil {
		return "", fmt.Errorf("%s: %w", module, err)
	}
	if err = checkObjectCoverage(data, idField, expected, metrics); err != nil {
		return "", fmt.Errorf("%s: %w", module, err)
	}
	return data, nil
}
