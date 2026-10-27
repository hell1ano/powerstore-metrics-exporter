package generalCollector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
)

// Synthetic equivalent of the observed 196-row distribution. Private API
// responses are replayed outside the repository, never committed as fixtures.
func TestReplicationHTTPExpositionAcrossPagesAndSharedMetroFetch(t *testing.T) {
	for _, failSecond := range []bool{false, true} {
		t.Run(fmt.Sprintf("failSecond=%v", failSecond), func(t *testing.T) {
			utils.InitReqCounter(4)
			var sessionCalls, witnessCalls atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/rest/witness" {
					witnessCalls.Add(1)
					fmt.Fprint(w, `[]`)
					return
				}
				if r.URL.Path != "/api/rest/replication_session" {
					t.Errorf("extra endpoint %s", r.URL)
					w.WriteHeader(500)
					return
				}
				sessionCalls.Add(1)
				if r.URL.Query().Get("select") != "*,replication_rule(id,name,rpo,alert_threshold)" {
					t.Error("missing embedded RPO")
				}
				if r.URL.Query().Get("type") != "" || r.URL.Query().Get("role") != "" {
					t.Error("session type/role filtered")
				}
				start, end := 0, 100
				if r.URL.Query().Get("offset") == "100" {
					start, end = 100, 196
					if failSecond {
						w.WriteHeader(503)
						return
					}
				} else if r.URL.Query().Get("offset") != "0" {
					t.Error("bad page offset")
				}
				w.Header().Set("Content-Range", fmt.Sprintf("%d-%d/196", start, end-1))
				if start == 0 {
					w.WriteHeader(206)
				}
				rows := []map[string]interface{}{}
				for i := start; i < end; i++ {
					row := map[string]interface{}{"id": fmt.Sprintf("session-%03d", i), "type": "Metro_Active_Active", "role": "Metro_Non_Preferred", "resource_type": "volume", "state": "OK", "witness_details": map[string]string{"state": "Engaged"}}
					if i >= 105 {
						row["type"] = "Synchronous"
						row["role"] = "Destination"
						row["resource_type"] = "file_system"
						row["replication_rule"] = map[string]interface{}{"rpo": "Zero", "alert_threshold": 0}
					}
					if i >= 187 {
						row["type"] = "Asynchronous"
						row["role"] = "Source"
						row["resource_type"] = "nas_server"
						row["replication_rule"] = map[string]interface{}{"rpo": "One_Hour", "alert_threshold": 30}
					}
					rows = append(rows, row)
				}
				json.NewEncoder(w).Encode(rows)
			}))
			defer srv.Close()
			logger := log.NewNopLogger()
			api, err := client.NewConfiguredClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "fixture", Password: "fixture", Version: "v3", TLSInsecureSkipVerify: true}, logger)
			if err != nil {
				t.Fatal(err)
			}
			reg := prometheus.NewPedanticRegistry()
			reg.MustRegister(Monitored(api.IP, "replication", true, NewReplicationCollector(api, logger)))
			reg.MustRegister(Monitored(api.IP, "metro", true, NewMetroCollector(api, logger)))
			recorder := httptest.NewRecorder()
			promhttp.HandlerFor(reg, promhttp.HandlerOpts{ErrorHandling: promhttp.HTTPErrorOnError}).ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics/array/cluster", nil))
			if recorder.Code != 200 {
				t.Fatalf("exposition failed: %d %s", recorder.Code, recorder.Body.String())
			}
			var parser expfmt.TextParser
			families, err := parser.TextToMetricFamilies(strings.NewReader(recorder.Body.String()))
			if err != nil {
				t.Fatal(err)
			}
			wantHealth := 1.0
			if failSecond {
				wantHealth = 0
			}
			for _, metric := range families["powerstore_collector_success"].Metric {
				if metric.Gauge.GetValue() != wantHealth {
					t.Fatal("incorrect collection health")
				}
			}
			if sessionCalls.Load() != 2 {
				t.Fatalf("extra/missing session requests: %d", sessionCalls.Load())
			}
			if failSecond {
				if families["powerstore_replication_session_info"] != nil || families["powerstore_replication_session_count"] != nil {
					t.Fatal("partial sessions advertised as complete")
				}
				return
			}
			if witnessCalls.Load() != 1 {
				t.Fatal("unexpected witness requests")
			}
			if len(families["powerstore_replication_session_info"].Metric) != 196 || families["powerstore_replication_session_count"].Metric[0].Gauge.GetValue() != 196 {
				t.Fatal("lost sessions across pages")
			}
			if len(families["powerstore_replication_session_rpo_seconds"].Metric) != 91 {
				t.Fatal("lost RPO rows")
			}
			if len(families["powerstore_metro_status"].Metric) != 105 {
				t.Fatal("legacy Metro filtering broken")
			}
		})
	}
}
