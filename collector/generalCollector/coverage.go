package generalCollector

import (
	"fmt"
	"powerstore-metrics-exporter/collector/bulkClient"

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
	for _, row := range gjson.Parse(data).Array() {
		id := row.Get(idField).String()
		name, known := expected[id]
		if !known || name.String() == "" {
			return fmt.Errorf("sample cannot be matched to named inventory object")
		}
		usable := false
		for _, metric := range metrics {
			if row.Get(metric).Type == gjson.Number {
				usable = true
				break
			}
		}
		if !usable {
			return fmt.Errorf("object has no supported measurements")
		}
		seen[id] = true
	}
	for id := range expected {
		if !seen[id] {
			return fmt.Errorf("known object is missing measurements")
		}
	}
	return nil
}
