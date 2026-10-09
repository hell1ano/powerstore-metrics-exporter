package generalCollector

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
	"strings"
	"testing"
)

func TestNodeDoesNotRequireStartupInventory(t *testing.T) {
	utils.InitReqCounter(1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "eq.Node" {
			fmt.Fprint(w, `[{"name":"node1","appliance_id":"new-appliance","serial_number":"synthetic","lifecycle_state":"Ready"}]`)
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
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(Monitored(api.IP, "hardware", false, NewHardwareCollector(api, logger)))
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range metrics {
		if m.GetName() == "powerstore_hardware_node_state" {
			found = true
		}
		if m.GetName() == "powerstore_collector_success" && m.Metric[0].Gauge.GetValue() != 1 {
			t.Fatal("node collection failed")
		}
	}
	if !found {
		t.Fatal("missing node metric")
	}
}
