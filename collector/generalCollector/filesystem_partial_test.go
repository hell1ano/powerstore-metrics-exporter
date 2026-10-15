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

func TestPartialFilesystemBulk(t *testing.T) {
	for _, performance := range []bool{false, true} {
		t.Run(fmt.Sprint(performance), func(t *testing.T) {
			filename, field, metric := "space_metrics_by_file_system.csv", "logical_used", "powerstore_filesystem_logical_used"
			if performance {
				filename, field, metric = "performance_metrics_by_file_system.csv", "avg_total_iops", "powerstore_metricFilesystem_avg_total_iops"
			}
			csv := "file_system_id,timestamp," + field + "\n"
			for _, id := range []string{"good", "orphan", "unknown", "destination"} {
				value := "42"
				if id == "nonumeric" {
					value = ""
				}
				csv += id + "," + time.Now().UTC().Format(time.RFC3339) + "," + value + "\n"
			}
			bc := syntheticBulk(t, filename, csv)
			api := &client.Client{IP: bc.IP}
			str := func(s string) gjson.Result { return gjson.Parse(fmt.Sprintf("%q", s)) }
			inv := map[string]map[string]gjson.Result{
				"filesystem":      {"good": str("healthy"), "orphan": str(""), "destination": str("replica"), "missing": str("absent"), "nonumeric": str("empty")},
				"filesystem_nas":  {"good": str("nas"), "destination": str("dest")},
				"nas":             {"nas": str("source"), "dest": str("replication")},
				"nas_destination": {"dest": gjson.Parse("true")},
			}
			client.PowerstoreModuleID[api.IP] = inv
			defer delete(client.PowerstoreModuleID, api.IP)
			var logs bytes.Buffer
			logger := log.NewLogfmtLogger(&logs)
			var c prometheus.Collector = NewFileCollector(api, bc, logger)
			if performance {
				c = NewMetricFilesystemCollector(api, bc, logger)
			}
			reg := prometheus.NewPedanticRegistry()
			reg.MustRegister(Monitored(api.IP, "filesystem", true, c))
			families, err := reg.Gather()
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			health := -1.0
			for _, f := range families {
				if f.GetName() == "powerstore_collector_success" {
					health = f.Metric[0].Gauge.GetValue()
				}
				if f.GetName() == "powerstore_collector_last_success_timestamp_seconds" && f.Metric[0].Gauge.GetValue() != 0 {
					t.Fatal("partial scrape advanced last success")
				}
				if f.GetName() == metric {
					for _, m := range f.Metric {
						for _, l := range m.Label {
							if l.GetName() == "file_system_id" {
								seen[l.GetValue()] = true
							}
						}
					}
				}
			}
			if health != 0 || len(seen) != 3 || !seen["good"] || !seen["orphan"] || !seen["unknown"] {
				t.Fatalf("partial data/health incorrect: %v %v", seen, health)
			}
			for _, id := range []string{"missing", "nonumeric", "orphan", "unknown"} {
				if !strings.Contains(logs.String(), "file_system_id="+id) {
					t.Errorf("no diagnostic for %s", id)
				}
			}
			before := logs.Len()
			if _, err := reg.Gather(); err != nil {
				t.Fatal(err)
			}
			// Info messages may repeat, but object warnings must be suppressed independently.
			if strings.Contains(logs.String()[before:], "filesystem collection failed") {
				t.Fatal("repeated per-object errors not throttled")
			}
		})
	}
}
