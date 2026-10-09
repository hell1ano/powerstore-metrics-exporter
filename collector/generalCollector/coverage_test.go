package generalCollector

import (
	"github.com/tidwall/gjson"
	"testing"
)

func TestObjectCoverage(t *testing.T) {
	known := map[string]gjson.Result{"v1": gjson.Parse(`"one"`), "v2": gjson.Parse(`"two"`)}
	for _, tc := range []struct {
		name, data string
		inventory  map[string]gjson.Result
		fail       bool
	}{
		{"complete", `[{"volume_id":"v1","avg_read_iops":0},{"volume_id":"v2","avg_read_iops":2}]`, known, false},
		{"partial", `[{"volume_id":"v1","avg_read_iops":0}]`, known, true},
		{"empty known", `[]`, known, true},
		{"empty class", `[]`, map[string]gjson.Result{}, false},
		{"unavailable inventory", `[]`, nil, true},
		{"no usable values", `[{"volume_id":"v1","avg_read_iops":null},{"volume_id":"v2","other":1}]`, known, true},
		{"unknown identity", `[{"volume_id":"v3","avg_read_iops":1}]`, known, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkObjectCoverage(tc.data, "volume_id", tc.inventory, []string{"avg_read_iops"})
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected coverage: %v", err)
			}
		})
	}
}
