package generalCollector

import (
	"bytes"
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
	"strings"
	"testing"
	"time"
)

func TestVolumePartialResultsAndRecovery(t *testing.T) {
	csv := "volume_id,appliance_id,timestamp,avg_read_iops\n"
	for _, row := range []struct{ id, value string }{{"good", "42"}, {"idle", "0"}, {"excluded", "99"}} {
		csv += row.id + ",a1," + time.Now().UTC().Format(time.RFC3339) + "," + row.value + "\n"
	}
	bc := syntheticBulk(t, "performance_metrics_by_volume.csv", csv)
	api := &client.Client{IP: bc.IP}
	expected := map[string]gjson.Result{}
	for _, id := range []string{"good", "idle", "empty"} {
		expected[id] = gjson.Parse(fmt.Sprintf("%q", id))
	}
	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("missing-%02d", i)
		expected[id] = gjson.Parse(fmt.Sprintf("%q", "name-"+id))
	}
	client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"volume": expected}
	defer delete(client.PowerstoreModuleID, api.IP)
	var logs bytes.Buffer
	c := NewMetricVolumeCollector(api, bc, log.NewLogfmtLogger(&logs))
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(Monitored(api.IP, "volume_performance", true, c))
	gather := func(wantSuccess float64) float64 {
		t.Helper()
		fs, err := reg.Gather()
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]float64{}
		success, last := -1.0, -1.0
		for _, f := range fs {
			switch f.GetName() {
			case "powerstore_collector_success":
				success = f.Metric[0].Gauge.GetValue()
			case "powerstore_collector_last_success_timestamp_seconds":
				last = f.Metric[0].Gauge.GetValue()
			case "powerstore_metricVolume_avg_read_iops":
				for _, m := range f.Metric {
					for _, l := range m.Label {
						if l.GetName() == "volume_id" {
							values[l.GetValue()] = m.Gauge.GetValue()
						}
					}
				}
			}
		}
		if success != wantSuccess || len(values) != 2 || values["good"] != 42 {
			t.Fatalf("success=%v values=%v", success, values)
		}
		if v, ok := values["idle"]; !ok || v != 0 {
			t.Fatal("real zero lost")
		}
		return last
	}
	if gather(0) != 0 {
		t.Fatal("partial scrape advanced last success")
	}
	for i := 0; i < 40; i++ {
		if !strings.Contains(logs.String(), fmt.Sprintf("volume_id=missing-%02d volume_name=name-missing-%02d", i, i)) {
			t.Fatal("missing named diagnostic")
		}
	}
	before := logs.Len()
	gather(0)
	if strings.Contains(logs.String()[before:], "collection incomplete") {
		t.Fatal("warnings not throttled")
	}
	for id := range expected {
		if id != "good" && id != "idle" {
			delete(expected, id)
		}
	}
	last := gather(1)
	if last <= 0 {
		t.Fatal("recovery did not advance success")
	}
	expected["missing-00"] = gjson.Parse(`"name-missing-00"`)
	before = logs.Len()
	if gather(0) != last {
		t.Fatal("partial scrape changed last success")
	}
	if !strings.Contains(logs.String()[before:], "volume_id=missing-00") {
		t.Fatal("recovery did not reset warning suppression")
	}
}

func TestVolumePartialInventoryRequirements(t *testing.T) {
	report := func(string, error) { t.Fatal("unexpected object error") }
	if _, err := readVolumePerformanceBulk(nil, nil, report); err == nil {
		t.Fatal("missing inventory accepted")
	}
	if data, err := readVolumePerformanceBulk(nil, map[string]gjson.Result{}, report); err != nil || data != "[]" {
		t.Fatal("empty selected inventory failed")
	}
}
