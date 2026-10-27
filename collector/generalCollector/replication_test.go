package generalCollector

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
)

func gatherReplication(t *testing.T, data string) map[string]*dto.MetricFamily {
	t.Helper()
	utils.InitReqCounter(4)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/rest/replication_session" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		fmt.Fprint(w, data)
	}))
	defer srv.Close()
	api, err := client.NewConfiguredClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v3", TLSInsecureSkipVerify: true}, log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(Monitored(api.IP, "replication", true, NewReplicationCollector(api, log.NewNopLogger())))
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]*dto.MetricFamily{}
	for _, f := range families {
		result[f.GetName()] = f
	}
	return result
}

func TestReplicationAllTypesResourcesRolesAndOptionalFields(t *testing.T) {
	data := `[
{"id":"nas","type":"Asynchronous","role":"Source","resource_type":"nas_server","state":"Partial_OK","last_sync_timestamp":"not-supported","replication_rule":{"rpo":"Fifteen_Minutes","alert_threshold":20}},
{"id":"fs","type":"Asynchronous","role":"Destination","resource_type":"file_system","state":"Paused","parent_replication_session_id":"nas"},
{"id":"sync","type":"Synchronous","role":"Source","resource_type":"volume_group","state":"OK","replication_rule":{"rpo":"Zero"},"data_connection_state":"OK"},
{"id":"metro","type":"Metro_Active_Active","role":"Metro_Non_Preferred","resource_type":"volume","state":"Fractured","last_sync_duration":1250,"last_sync_timestamp":"2026-10-27T12:00:00Z","failover_test_in_progress":false},
{"id":"future","type":"Future_Type","role":"New_Role","resource_type":"replication_group","state":"New_State"}
]`
	f := gatherReplication(t, data)
	if f["powerstore_collector_success"].Metric[0].Gauge.GetValue() != 1 {
		t.Fatal("valid session response failed")
	}
	if len(f["powerstore_replication_session_info"].Metric) != 5 || len(f["powerstore_replication_session_state"].Metric) != 5 {
		t.Fatal("sessions omitted")
	}
	if len(f["powerstore_replication_session_rpo_seconds"].Metric) != 3 {
		t.Fatal("RPO missing or fabricated")
	}
	if len(f["powerstore_replication_session_last_sync_timestamp_seconds"].Metric) != 1 {
		t.Fatal("NAS timestamp exported")
	}
	if f["powerstore_replication_session_last_sync_duration_seconds"].Metric[0].Gauge.GetValue() != 1.25 {
		t.Fatal("milliseconds not converted")
	}
	for _, m := range f["powerstore_replication_session_alert_threshold_seconds"].Metric {
		if m.Gauge.GetValue() != 1200 {
			t.Fatal("minutes not converted")
		}
	}
}

func TestReplicationEmptyAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		data    string
		success float64
	}{
		{`[]`, 1},
		{`[{"id":"broken"}]`, 0},
		{`[{"id":"s","type":"Asynchronous","role":"Source","resource_type":"volume","state":"OK","replication_rule":{"rpo":"unknown"}}]`, 0},
		{`[{"id":"s","type":"Asynchronous","role":"Source","resource_type":"volume","state":"OK","last_sync_duration":"oops"}]`, 0},
		{`[{"id":"s","type":"Asynchronous","role":"Source","resource_type":"volume","state":"OK","last_sync_timestamp":"oops"}]`, 0},
		{`[{"id":"s","type":"Asynchronous","role":"Source","resource_type":"volume","state":"OK"},{"id":"s","type":"Asynchronous","role":"Source","resource_type":"volume","state":"OK"}]`, 0},
	} {
		f := gatherReplication(t, tc.data)
		if got := f["powerstore_collector_success"].Metric[0].Gauge.GetValue(); got != tc.success {
			t.Errorf("%s: got %v", tc.data, got)
		}
	}
}

func TestUnknownWitnessStateIsNeverHealthyZero(t *testing.T) {
	// The original mapping used an absent 'other' key and silently returned 0.
	if got := getMetroFloatData("conn_state", gjson.Parse(`"future"`)); got != -1 {
		t.Fatal(got)
	}
}

func TestVolumeStatusDoesNotFabricateMissingMeasurements(t *testing.T) {
	utils.InitReqCounter(2)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/rest/volume" || r.URL.Query().Get("is_replication_destination") != "eq.false" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		fmt.Fprint(w, `[{"id":"v1","name":"source","appliance_id":"A1","state":"Ready","size":null,"logical_used":"invalid"}]`)
	}))
	defer srv.Close()
	api, err := client.NewConfiguredClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v3", TLSInsecureSkipVerify: true}, log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer delete(client.PowerstoreModuleID, api.IP)
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(Monitored(api.IP, "volume", true, NewVolumeCollector(api, log.NewNopLogger())))
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == "powerstore_volume_size" || f.GetName() == "powerstore_volume_logical_used" {
			t.Fatal("missing value fabricated")
		}
		if f.GetName() == "powerstore_collector_success" && f.Metric[0].Gauge.GetValue() != 0 {
			t.Fatal("missing values marked successful")
		}
	}
}
