package generalCollector

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"os"
	"powerstore-metrics-exporter/collector/bulkClient"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/utils"
	"regexp"
	"strings"
	"testing"
	"time"
)

func syntheticBulk(t *testing.T, filename, csv string) *bulkClient.BulkClient {
	t.Helper()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: filename, Mode: 0600, Size: int64(len(csv))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(csv)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data.Bytes()) }))
	t.Cleanup(srv.Close)
	bc, err := bulkClient.NewBulkClient(utils.Storage{Ip: strings.TrimPrefix(srv.URL, "https://"), User: "test", Password: "test", Version: "v4", Bulk: true, TLSInsecureSkipVerify: true}, t.TempDir(), log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	bc.IP = "synthetic-array"
	if err = bc.DownloadBulkData(); err != nil {
		t.Fatal(err)
	}
	return bc
}

func TestFilesystemCSVToZabbixContract(t *testing.T) {
	template, err := os.ReadFile("../../templates/zabbix/zbx_export_templates_7.0.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fields := []string{"avg_read_latency", "avg_latency", "avg_write_latency", "avg_read_iops", "avg_read_bandwidth", "avg_total_iops", "avg_total_bandwidth", "avg_write_iops", "avg_write_bandwidth", "avg_size", "avg_read_size", "avg_write_size"}
	for _, optional := range []bool{false, true} {
		header := "file_system_id,timestamp," + strings.Join(fields, ",")
		row := "fs1," + time.Now().UTC().Format("2006-01-02 15:04:05-07") + strings.Repeat(",1", len(fields))
		if optional {
			header += ",avg_mirror_write_iops"
			row += ",2"
		}
		bc := syntheticBulk(t, "performance_metrics_by_file_system.csv", header+"\n"+row+"\n")
		api := &client.Client{IP: bc.IP}
		client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"filesystem": {"fs1": gjson.Parse(`"example"`)}, "filesystem_nas": {"fs1": gjson.Parse(`"nas1"`)}, "nas": {"nas1": gjson.Parse(`"example-nas"`)}}
		registry := prometheus.NewPedanticRegistry()
		registry.MustRegister(Monitored(api.IP, "filesystem_performance", true, NewMetricFilesystemCollector(api, bc, log.NewNopLogger())))
		metrics, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, m := range metrics {
			found[m.GetName()] = true
			if m.GetName() == "powerstore_collector_success" && m.Metric[0].Gauge.GetValue() != 1 {
				t.Fatal("collection failed")
			}
		}
		for _, m := range regexp.MustCompile(`powerstore_metricFilesystem_[a-z_]+`).FindAllString(string(template), -1) {
			if !found[m] {
				t.Errorf("template field missing from CSV exposition: %s", m)
			}
		}
		if found["powerstore_metricFilesystem_avg_mirror_write_iops"] != optional {
			t.Fatal("optional field presence lost")
		}
		delete(client.PowerstoreModuleID, api.IP)
	}
}

func TestBulkKnownObjectHealthAndRecovery(t *testing.T) {
	bc := syntheticBulk(t, "performance_metrics_by_volume.csv", "volume_id,appliance_id,timestamp,avg_read_iops\nv1,a1,"+time.Now().UTC().Format(time.RFC3339)+",0\n")
	api := &client.Client{IP: bc.IP}
	defer delete(client.PowerstoreModuleID, api.IP)
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(Monitored(api.IP, "volume_performance", true, NewMetricVolumeCollector(api, bc, log.NewNopLogger())))
	for _, missing := range []bool{true, false} {
		inventory := map[string]gjson.Result{"v1": gjson.Parse(`"one"`)}
		if missing {
			inventory["v2"] = gjson.Parse(`"two"`)
		}
		client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"volume": inventory}
		metrics, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range metrics {
			if m.GetName() == "powerstore_collector_success" && (m.Metric[0].Gauge.GetValue() == 0) != missing {
				t.Fatal("wrong coverage health")
			}
		}
	}
}

func TestBulkCapacityToZabbixContract(t *testing.T) {
	fields := []string{"logical_provisioned", "logical_used", "physical_total", "physical_used", "data_physical_used", "shared_logical_used", "efficiency_ratio", "data_reduction", "snapshot_savings", "thin_savings"}
	csv := "appliance_id,timestamp," + strings.Join(fields, ",") + "\na1," + time.Now().UTC().Format(time.RFC3339) + strings.Repeat(",1", len(fields)) + "\n"
	bc := syntheticBulk(t, "space_metrics_by_appliance.csv", csv)
	api := &client.Client{IP: bc.IP}
	client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"appliance": {"a1": gjson.Parse(`"appliance"`)}}
	defer delete(client.PowerstoreModuleID, api.IP)
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(Monitored(api.IP, "capacity", false, NewCapacityCollector(api, log.NewNopLogger(), bc)))
	metrics, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, m := range metrics {
		found[m.GetName()] = true
		if strings.HasPrefix(m.GetName(), "powerstore_cap_max_") {
			t.Fatal("invented daily maximum")
		}
		if m.GetName() == "powerstore_collector_success" && m.Metric[0].Gauge.GetValue() != 1 {
			t.Fatal("capacity failed")
		}
	}
	data, err := os.ReadFile("../../templates/zabbix/zbx_export_templates_7.0.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, metric := range regexp.MustCompile(`powerstore_cap_[a-z_]+`).FindAllString(string(data), -1) {
		if !found[metric] {
			t.Errorf("capacity selector absent: %s", metric)
		}
	}
}
