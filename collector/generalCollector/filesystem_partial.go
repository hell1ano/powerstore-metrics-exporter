package generalCollector

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/bulkClient"
	"strings"
)

func filesystemDestination(inventory map[string]map[string]gjson.Result, id string) bool {
	owner := inventory["filesystem_nas"][id].String()
	policy := inventory["nas_policy"][owner]
	// NAS async and synchronous destinations use the same monitoring rule.
	// A retained destination flag without a policy is not enough to exclude data.
	return inventory["nas_destination"][owner].Type == gjson.True && policy.Type == gjson.String && strings.TrimSpace(policy.String()) != ""
}

// A bad object must not hide usable samples from other filesystems. Archive
// integrity and freshness remain enforced before any rows reach this function.
func readFilesystemBulk(bc *bulkClient.BulkClient, module string, inventory map[string]map[string]gjson.Result, fields []string, report func(string, error)) (string, error) {
	data, err := bc.ReadCsvData(module)
	if err != nil {
		return "", err
	}
	if inventory["filesystem"] == nil {
		report("", fmt.Errorf("filesystem inventory unavailable; displaying available samples by ID"))
	}
	rows := make([]json.RawMessage, 0)
	seen := map[string]bool{}
	for _, row := range gjson.Parse(data).Array() {
		id := row.Get("file_system_id").String()
		if id == "" {
			report("", fmt.Errorf("sample has no file_system_id; sample omitted"))
			continue
		}
		if filesystemDestination(inventory, id) {
			continue
		}
		if seen[id] {
			report(id, fmt.Errorf("duplicate filesystem sample; duplicate omitted"))
			continue
		}
		seen[id] = true
		usable := false
		for _, field := range fields {
			if row.Get(field).Type == gjson.Number {
				usable = true
				break
			}
		}
		if !usable {
			report(id, fmt.Errorf("object has no supported numeric measurements"))
			continue
		}
		rows = append(rows, json.RawMessage(row.Raw))
	}
	for id := range inventory["filesystem"] {
		if !seen[id] && !filesystemDestination(inventory, id) {
			report(id, fmt.Errorf("known filesystem is missing measurements in %s", module))
		}
	}
	result, err := json.Marshal(rows)
	return string(result), err
}
