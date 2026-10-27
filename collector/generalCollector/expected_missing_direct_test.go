package generalCollector

import (
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/collector/bulkClient"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
	"strings"
	"testing"
)

func TestExpectedMissingDirectAndRecovery(t *testing.T) {
	utils.InitReqCounter(4)
	for _, kind := range []string{"volume_performance", "filesystem_capacity", "filesystem_performance"} {
		t.Run(kind, func(t *testing.T) {
			body, status := "[]", http.StatusOK
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			api, err := client.NewConfiguredClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v4", TLSInsecureSkipVerify: true, ExpectedMissingMetrics: map[string][]string{kind: {"obj"}}}, log.NewNopLogger())
			if err != nil {
				t.Fatal(err)
			}
			client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{
				"volume": {"obj": gjson.Parse(`"example"`)}, "filesystem": {"obj": gjson.Parse(`"example"`)},
				"filesystem_nas": {"obj": gjson.Parse(`"nas1"`)}, "nas": {"nas1": gjson.Parse(`"nas"`)},
			}
			defer delete(client.PowerstoreModuleID, api.IP)
			var inner prometheus.Collector
			bc := &bulkClient.BulkClient{}
			switch kind {
			case "volume_performance":
				inner = NewMetricVolumeCollector(api, bc, log.NewNopLogger())
			case "filesystem_capacity":
				inner = NewFileCollector(api, bc, log.NewNopLogger())
			default:
				inner = NewMetricFilesystemCollector(api, bc, log.NewNopLogger())
			}
			registry := prometheus.NewPedanticRegistry()
			registry.MustRegister(Monitored(api.IP, kind, true, inner))
			for _, step := range []struct {
				body                                 string
				status                               int
				health, available, expected, samples float64
			}{
				{"[]", 200, 1, 0, 1, 0},
				{`[{"appliance_id":"a1","logical_used":0,"avg_read_iops":0}]`, 200, 1, 1, 0, 1},
				{`[{"avg_read_iops":null}]`, 200, 0, 0, 0, 0},
				{"[]", 500, 0, 0, 0, 0},
				{"[]", 200, 1, 0, 1, 0},
			} {
				body, status = step.body, step.status
				families, err := registry.Gather()
				if err != nil {
					t.Fatal(err)
				}
				values := map[string]float64{}
				for _, f := range families {
					values[f.GetName()] = f.Metric[0].GetGauge().GetValue()
				}
				if values["powerstore_collector_success"] != step.health || values["powerstore_object_sample_available"] != step.available || values["powerstore_object_missing_expected"] != step.expected || values["powerstore_collector_samples"] != step.samples {
					t.Fatalf("step %+v: %v", step, values)
				}
			}
		})
	}
}
