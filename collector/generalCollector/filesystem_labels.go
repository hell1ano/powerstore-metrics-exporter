package generalCollector

import (
	"fmt"
	"github.com/tidwall/gjson"
)

// Resolve all labels from the same inventory snapshot. Names are for display;
// stable filesystem IDs distinguish equal names on different NAS servers.
func filesystemLabels(inventory map[string]map[string]gjson.Result, id string) ([]string, error) {
	name := inventory["filesystem"][id].String()
	nasID := inventory["filesystem_nas"][id].String()
	nasName := inventory["nas"][nasID].String()
	if id == "" || name == "" || nasID == "" || nasName == "" {
		return nil, fmt.Errorf("filesystem NAS ownership unavailable in inventory")
	}
	return []string{name, id, nasID, nasName}, nil
}
