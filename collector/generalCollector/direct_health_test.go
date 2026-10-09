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

func TestIncompleteDirectCollectionFailsHealth(t *testing.T) {
	utils.InitReqCounter(4)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/rest/metrics/generate":
			fmt.Fprint(w, `[{"volume_id":"v1","appliance_id":"a1","avg_read_iops":null}]`)
		case "/api/rest/nas_server":
			fmt.Fprint(w, `[{"id":"n1","name":"example"}]`)
		case "/api/rest/hardware":
			if r.URL.Query().Get("type") == "eq.Node" {
				fmt.Fprint(w, `[{"id":"n1","name":"node","appliance_id":"a1","serial_number":"synthetic","lifecycle_state":"Healthy"}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	defer srv.Close()
	logger := log.NewNopLogger()
	api, err := client.NewClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v3", TLSInsecureSkipVerify: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"volume": {"v1": gjson.Parse(`"example"`)}, "nas": {"n1": gjson.Parse(`"example"`)}, "drive": {"d1": gjson.Parse(`"drive"`)}}
	defer delete(client.PowerstoreModuleID, api.IP)
	for _, tc := range []struct {
		name      string
		collector prometheus.Collector
		empty     bool
	}{
		{"known volume with null measurements", NewMetricVolumeCollector(api, &bulkClient.BulkClient{}, logger), true},
		{"known NAS with missing status", NewNasCollector(api, logger), true},
		{"hardware omits all known drives", NewHardwareCollector(api, logger), false},
	} {
		registry := prometheus.NewPedanticRegistry()
		registry.MustRegister(Monitored(api.IP, "review", tc.empty, tc.collector))
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		for _, family := range families {
			if family.GetName() == "powerstore_collector_success" || family.GetName() == "powerstore_collector_samples" {
				if family.GetName() == "powerstore_collector_success" && family.Metric[0].Gauge.GetValue() != 0 {
					t.Errorf("%s incorrectly succeeded", tc.name)
				}
			}
		}
	}
}

func TestHealthCoverageOptionalAndMissingObjects(t *testing.T) {
	empty := map[string]gjson.Result{}
	known := map[string]gjson.Result{"n1": gjson.Parse(`"nas"`)}
	for _, tc := range []struct {
		data     string
		expected map[string]gjson.Result
		fail     bool
	}{
		{`[]`, empty, false}, {`[]`, nil, true}, {`[]`, known, true},
		{`[{"id":"n1","name":"nas","operational_status":"Started"}]`, known, false},
		{`[{"id":"n1","name":"nas","operational_status":null}]`, known, true},
		{`[{"id":"n1","name":"nas","operational_status":1}]`, known, true},
	} {
		if err := checkHealthCoverage(tc.data, tc.expected, []string{"id", "name", "operational_status"}); (err != nil) != tc.fail {
			t.Errorf("%s: %v", tc.data, err)
		}
	}
}
