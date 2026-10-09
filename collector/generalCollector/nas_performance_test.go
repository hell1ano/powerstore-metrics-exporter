package generalCollector

import (
	"fmt"
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

func TestDirectNASUsesNASMetricFields(t *testing.T) {
	utils.InitReqCounter(1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/rest/metrics/generate" {
			fmt.Fprint(w, `[{"nas_server_id":"nas1","avg_size":4096,"avg_io_size":123}]`)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	logger := log.NewNopLogger()
	api, err := client.NewClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v3", TLSInsecureSkipVerify: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"nas": {"nas1": gjson.Parse(`"synthetic"`)}}
	defer delete(client.PowerstoreModuleID, api.IP)
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(Monitored(api.IP, "nas_performance", true, NewMetricNasCollector(api, &bulkClient.BulkClient{}, logger)))
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range metrics {
		if m.GetName() == "powerstore_metricNas_avg_size" {
			found = true
			if m.Metric[0].Gauge.GetValue() != 4096 {
				t.Fatal("wrong size")
			}
		}
	}
	if !found {
		t.Fatal("missing NAS size")
	}
}
