package generalCollector

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
	"strings"
	"testing"
)

func TestHardwareMissingClassesAndRecovery(t *testing.T) {
	utils.InitReqCounter(2)
	for _, missing := range []string{"Node", "Drive", "Fan", "Power_Supply", "Battery", ""} {
		t.Run(missing, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				kind := strings.TrimPrefix(r.URL.Query().Get("type"), "eq.")
				if r.URL.Path != "/api/rest/hardware" || kind == missing {
					fmt.Fprint(w, `[]`)
					return
				}
				fmt.Fprintf(w, `[{"id":%q,"name":%q,"appliance_id":"a1","serial_number":"test","lifecycle_state":"Healthy","type":%q}]`, kind, kind, kind)
			}))
			defer srv.Close()
			logger := log.NewNopLogger()
			api, err := client.NewClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v3", TLSInsecureSkipVerify: true}, logger)
			if err != nil {
				t.Fatal(err)
			}
			inventory := map[string]map[string]gjson.Result{}
			for _, kind := range []string{"Node", "Drive", "Fan", "Power_Supply", "Battery"} {
				inventory[strings.ToLower(kind)] = map[string]gjson.Result{kind: gjson.Parse(`"test"`)}
			}
			client.PowerstoreModuleID[api.IP] = inventory
			defer delete(client.PowerstoreModuleID, api.IP)
			reg := prometheus.NewPedanticRegistry()
			reg.MustRegister(Monitored(api.IP, "hardware", false, NewHardwareCollector(api, logger)))
			families, err := reg.Gather()
			if err != nil {
				t.Fatal(err)
			}
			want := 0.0
			if missing == "" {
				want = 1
			}
			for _, f := range families {
				if f.GetName() == "powerstore_collector_success" && f.Metric[0].Gauge.GetValue() != want {
					t.Fatalf("missing %q: wrong health", missing)
				}
			}
		})
	}
}
