package generalCollector

import (
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"testing"
	"time"

	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
)

func TestExpectedMissingBulkSamples(t *testing.T) {
	for _, kind := range []string{"volume_performance", "filesystem_capacity", "filesystem_performance"} {
		for _, mode := range []string{"unexpected", "expected", "measured_zero", "wrong_module", "stale"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				file, idField, field := "performance_metrics_by_volume.csv", "volume_id", "avg_read_iops"
				if kind == "filesystem_capacity" {
					file, idField, field = "space_metrics_by_file_system.csv", "file_system_id", "logical_used"
				}
				if kind == "filesystem_performance" {
					file, idField = "performance_metrics_by_file_system.csv", "file_system_id"
				}
				csv := idField + ",appliance_id,timestamp," + field + "\nother,a1," + time.Now().UTC().Format(time.RFC3339) + ",0\n"
				if mode == "measured_zero" {
					value := "0"

					csv = idField + ",appliance_id,timestamp," + field + "\nobj,a1," + time.Now().UTC().Format(time.RFC3339) + "," + value + "\n"
				}
				if mode == "wrong_module" {
					file = "space_metrics_by_appliance.csv"
					csv = "appliance_id,timestamp,logical_used\na1," + time.Now().UTC().Format(time.RFC3339) + ",1\n"
				}
				bc := syntheticBulk(t, file, csv)
				if mode == "stale" {
					bc.MaxAge = time.Nanosecond
				}
				config := utils.Storage{Ip: bc.IP, User: "test", Password: "test", Version: "v4"}
				if mode != "unexpected" {
					config.ExpectedMissingMetrics = map[string][]string{kind: {"obj"}}
				}
				api, err := client.NewConfiguredClient(config, log.NewNopLogger())
				if err != nil {
					t.Fatal(err)
				}
				client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{
					"volume": {"obj": gjson.Parse(`"example"`)}, "volume_appliance": {"obj": gjson.Parse(`"a1"`)},
					"filesystem":     {"obj": gjson.Parse(`"example"`), "other": gjson.Parse(`"other"`)},
					"filesystem_nas": {"obj": gjson.Parse(`"nas1"`), "other": gjson.Parse(`"nas1"`)}, "nas": {"nas1": gjson.Parse(`"nas"`)},
				}
				if mode == "measured_zero" {
					delete(client.PowerstoreModuleID[api.IP]["filesystem"], "other")
				}
				t.Cleanup(func() { delete(client.PowerstoreModuleID, api.IP) })
				var inner prometheus.Collector
				switch kind {
				case "volume_performance":
					inner = NewMetricVolumeCollector(api, bc, log.NewNopLogger())
				case "filesystem_capacity":
					inner = NewFileCollector(api, bc, log.NewNopLogger())
				default:
					inner = NewMetricFilesystemCollector(api, bc, log.NewNopLogger())
				}
				reg := prometheus.NewPedanticRegistry()
				reg.MustRegister(Monitored(api.IP, kind, true, inner))
				families, err := reg.Gather()
				if err != nil {
					t.Fatal(err)
				}
				values := map[string]float64{}
				for _, family := range families {
					for _, metric := range family.Metric {
						selected := true
						for _, label := range metric.Label {
							if label.GetName() == "object_id" && label.GetValue() != "obj" {
								selected = false
							}
						}
						if selected {
							values[family.GetName()] = metric.GetGauge().GetValue()
						}
					}
				}
				want := 0.0
				if mode == "expected" || mode == "measured_zero" {
					want = 1
				}
				if values["powerstore_collector_success"] != want {
					t.Fatalf("health: %v", values)
				}
				if mode == "expected" || mode == "unexpected" || mode == "measured_zero" {
					available, ok := values["powerstore_object_sample_available"]
					wantAvailable, wantExpected := 0.0, 0.0
					if mode == "measured_zero" {
						wantAvailable = 1
					}
					if mode == "expected" {
						wantExpected = 1
					}
					if !ok || available != wantAvailable || values["powerstore_object_missing_expected"] != wantExpected {
						t.Fatalf("coverage: %v", values)
					}
					wantSamples := 1.0
					if kind == "volume_performance" && mode != "measured_zero" {
						wantSamples = float64(len(metricVolumeCollectorMetric))
					}
					if values["powerstore_collector_samples"] != wantSamples {
						t.Fatalf("diagnostics counted as samples: %v", values)
					}
				}
			})
		}
	}
}
