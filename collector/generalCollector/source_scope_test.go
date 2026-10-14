package generalCollector

import (
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
	"strings"
	"testing"
	"time"
)

func TestSourceOnlyBulkCoverage(t *testing.T) {
	expected := map[string]gjson.Result{"source": gjson.Parse(`"source"`)}
	for _, tc := range []struct {
		name, data string
		fail       bool
	}{
		{"source plus excluded", `[{"volume_id":"source","iops":0},{"volume_id":"destination","iops":3},{"volume_id":"snapshot","iops":2},{"volume_id":"unknown","iops":4}]`, false},
		{"missing source", `[{"volume_id":"destination","iops":3}]`, true},
		{"source no numeric data", `[{"volume_id":"source","iops":null}]`, true},
		{"malformed identity", `[{"iops":1}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := selectMonitoredRows(tc.data, "volume_id", expected)
			if err == nil {
				err = checkObjectCoverage(data, "volume_id", expected, []string{"iops"})
			}
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected coverage result: %v", err)
			}
			if !tc.fail && len(gjson.Parse(data).Array()) != 1 {
				t.Fatal("excluded objects retained")
			}
		})
	}
}
func TestSourceOnlyVolumeAndGroupMetrics(t *testing.T) {
	for _, group := range []bool{false, true} {
		filename, idField, moduleKey := "performance_metrics_by_volume.csv", "volume_id", "volume"
		if group {
			filename, idField, moduleKey = "performance_metrics_by_vg.csv", "vg_id", "volumegroup"
		}
		csv := idField + ",appliance_id,timestamp,avg_read_iops\n"
		for _, id := range []string{"source", "destination", "snapshot", "unknown"} {
			csv += id + ",a1," + time.Now().UTC().Format(time.RFC3339) + ",1\n"
		}
		bc := syntheticBulk(t, filename, csv)
		api := &client.Client{IP: bc.IP}
		client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{moduleKey: {"source": gjson.Parse(`"local-source"`)}}
		var collector prometheus.Collector = NewMetricVolumeCollector(api, bc, log.NewNopLogger())
		if group {
			collector = NewMetricVgCollector(api, bc, log.NewNopLogger())
		}
		reg := prometheus.NewPedanticRegistry()
		reg.MustRegister(Monitored(api.IP, "test", true, collector))
		families, err := reg.Gather()
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, f := range families {
			if f.GetName() == "powerstore_collector_success" && f.Metric[0].Gauge.GetValue() != 1 {
				t.Fatal("source-only collection failed")
			}
			if strings.HasSuffix(f.GetName(), "_avg_read_iops") {
				count += len(f.Metric)
			}
		}
		if count != 1 {
			t.Fatalf("got %d source samples", count)
		}
		delete(client.PowerstoreModuleID, api.IP)
	}
}

func TestSourceOnlyEmptyAndUnavailableInventory(t *testing.T) {
	if _, err := readBulkForMonitoredObjects(nil, "volume", "volume_id", nil, nil); err == nil {
		t.Fatal("missing inventory must fail")
	}
	data, err := readBulkForMonitoredObjects(nil, "volume", "volume_id", map[string]gjson.Result{}, nil)
	if err != nil || data != "[]" {
		t.Fatalf("empty eligible inventory: %q %v", data, err)
	}
}
