package generalCollector

import (
	"encoding/json"
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/collector/bulkClient"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
	"strings"
	"testing"
	"time"
)

func TestFilesystemOwnershipInBothCollectionModes(t *testing.T) {
	utils.InitReqCounter(4)
	for _, bulk := range []bool{false, true} {
		for _, performance := range []bool{false, true} {
			t.Run(fmt.Sprintf("bulk=%v/performance=%v", bulk, performance), func(t *testing.T) {
				nasName := "NAS-one"
				missingOwner := false
				srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/rest/file_system":
						if r.URL.Query().Get("select") != "id,name,nas_server_id" {
							t.Error("ownership not requested")
						}
						fmt.Fprint(w, `[{"id":"fs1","name":"shared","nas_server_id":"nas1"},{"id":"fs2","name":"shared","nas_server_id":"nas2"}]`)
					case "/api/rest/nas_server":
						if missingOwner {
							fmt.Fprint(w, `[]`)
						} else {
							fmt.Fprintf(w, `[{"id":"nas1","name":%q},{"id":"nas2","name":"NAS-two"}]`, nasName)
						}
					case "/api/rest/metrics/generate":
						var body client.RequestBody
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						fmt.Fprintf(w, `[{"file_system_id":%q,"logical_used":2,"logical_provisioned":10,"avg_total_iops":3}]`, body.EntityID)
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
				bc := &bulkClient.BulkClient{}
				metric := "powerstore_filesystem_logical_used"
				filename, field := "space_metrics_by_file_system.csv", "logical_used"
				if performance {
					filename, field = "performance_metrics_by_file_system.csv", "avg_total_iops"
					metric = "powerstore_metricFilesystem_avg_total_iops"
				}
				if bulk {
					timestamp := time.Now().UTC().Format(time.RFC3339)
					bc = syntheticBulk(t, filename, "file_system_id,timestamp,"+field+"\nfs1,"+timestamp+",2\nfs2,"+timestamp+",3\n")
				}
				defer delete(client.PowerstoreModuleID, api.IP)
				registry := prometheus.NewPedanticRegistry()
				var collector prometheus.Collector = NewFileCollector(api, bc, logger)
				if performance {
					collector = NewMetricFilesystemCollector(api, bc, logger)
				}
				registry.MustRegister(Monitored(api.IP, "filesystem", true, collector))
				for _, stage := range []string{"initial", "renamed", "missing", "recovered"} {
					if stage == "renamed" {
						nasName = "NAS-renamed"
					}
					missingOwner = stage == "missing"
					api.InitModuleID(logger)
					families, err := registry.Gather()
					if err != nil {
						t.Fatal(err)
					}
					seen := map[string]string{}
					health := -1.0
					for _, family := range families {
						if family.GetName() == "powerstore_collector_success" {
							health = family.Metric[0].Gauge.GetValue()
						}
						if family.GetName() != metric {
							continue
						}
						for _, sample := range family.Metric {
							labels := map[string]string{}
							for _, label := range sample.Label {
								labels[label.GetName()] = label.GetValue()
							}
							if labels["name"] != "shared" {
								t.Fatal("filesystem name lost")
							}
							seen[labels["file_system_id"]] = labels["nas_server_id"] + "/" + labels["nas_server_name"]
						}
					}
					if missingOwner {
						if health != 0 || len(seen) != 0 {
							t.Fatal("missing ownership reported success")
						}
					} else {
						if health != 1 || len(seen) != 2 || seen["fs1"] != "nas1/"+nasName || seen["fs2"] != "nas2/NAS-two" {
							t.Fatalf("incorrect ownership at %s: %v", stage, seen)
						}
					}
				}
			})
		}
	}
}
