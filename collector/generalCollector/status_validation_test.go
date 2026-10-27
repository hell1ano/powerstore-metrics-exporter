package generalCollector

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
)

func TestStatusValidationPreservesGoodSamples(t *testing.T) {
	utils.InitReqCounter(4)
	for _, tc := range []struct {
		name, kind, data string
		known            bool
		success, samples float64
	}{
		{"group empty", "group", `[]`, false, 1, 0},
		{"group missing", "group", `[]`, true, 0, 0},
		{"group zero", "group", `[{"id":"one","name":"group","appliance_ids":["a"],"logical_used":0,"logical_provisioned":10}]`, true, 1, 2},
		{"group invalid numeric", "group", `[{"id":"one","name":"group","appliance_ids":["a"],"logical_used":"invalid","logical_provisioned":10}]`, true, 0, 1},
		{"group missing numeric", "group", `[{"id":"one","name":"group","appliance_ids":["a"],"logical_provisioned":10}]`, true, 0, 1},
		{"group no appliances", "group", `[{"id":"one","name":"group","logical_used":0,"logical_provisioned":10}]`, true, 0, 0},
		{"group empty appliances", "group", `[{"id":"one","name":"group","appliance_ids":[],"logical_used":0,"logical_provisioned":10}]`, true, 0, 0},
		{"group invalid appliance", "group", `[{"id":"one","name":"group","appliance_ids":[null,"a"],"logical_used":0,"logical_provisioned":10}]`, true, 0, 2},
		{"group bad neighbor", "group", `[{"id":"one","name":"group","appliance_ids":["a"],"logical_used":0,"logical_provisioned":10},{"id":"two","name":"bad","appliance_ids":[]}]`, true, 0, 2},
		{"port empty", "port", `[]`, false, 1, 0},
		{"port missing", "port", `[]`, true, 0, 0},
		{"port down", "port", `[{"id":"one","name":"port","appliance_id":"a","is_link_up":false,"current_speed":null}]`, true, 1, 2},
		{"port up", "port", `[{"id":"one","name":"port","appliance_id":"a","is_link_up":true,"current_speed":"25_Gbps"}]`, true, 1, 2},
		{"port missing link", "port", `[{"id":"one","name":"port","appliance_id":"a","current_speed":"25_Gbps"}]`, true, 0, 1},
		{"port string link", "port", `[{"id":"one","name":"port","appliance_id":"a","is_link_up":"false","current_speed":"25_Gbps"}]`, true, 0, 1},
		{"port null link", "port", `[{"id":"one","name":"port","appliance_id":"a","is_link_up":null,"current_speed":"25_Gbps"}]`, true, 0, 1},
		{"port bad neighbor", "port", `[{"id":"one","name":"port","appliance_id":"a","is_link_up":true,"current_speed":"25_Gbps"},{"id":"two","name":"bad"}]`, true, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/rest/volume_group_list_cma_view" || r.URL.Path == "/api/rest/eth_port" {
					fmt.Fprint(w, tc.data)
				} else {
					fmt.Fprint(w, `[]`)
				}
			}))
			defer srv.Close()
			logger := log.NewNopLogger()
			api, err := client.NewConfiguredClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v3", TLSInsecureSkipVerify: true}, logger)
			if err != nil {
				t.Fatal(err)
			}
			expected := map[string]gjson.Result{}
			if tc.known {
				expected["one"] = gjson.Parse(`"known"`)
				if strings.Contains(tc.name, "neighbor") {
					expected["two"] = gjson.Parse(`"bad"`)
				}
			}
			client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"volumegroup": expected, "ethport": expected, "fcport": {}}
			defer delete(client.PowerstoreModuleID, api.IP)
			var collector prometheus.Collector = NewVolumeGroupCollector(api, logger)
			if tc.kind == "port" {
				collector = NewPortCollector(api, logger)
			}
			reg := prometheus.NewPedanticRegistry()
			reg.MustRegister(Monitored(api.IP, "validation", true, collector))
			families, err := reg.Gather()
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]float64{}
			for _, f := range families {
				if strings.HasPrefix(f.GetName(), "powerstore_collector_") {
					got[f.GetName()] = f.Metric[0].Gauge.GetValue()
				}
			}
			if got["powerstore_collector_success"] != tc.success || got["powerstore_collector_samples"] != tc.samples {
				t.Fatalf("health=%v; want success=%v samples=%v", got, tc.success, tc.samples)
			}
		})
	}
}
