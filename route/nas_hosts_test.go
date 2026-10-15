package route

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/tidwall/gjson"
	"net/http/httptest"
	"powerstore-metrics-exporter/collector/client"
	"strings"
	"sync"
	"testing"
	"time"
)

func nasInventory(id, name string, destination bool, policy string) map[string]map[string]gjson.Result {
	return map[string]map[string]gjson.Result{"nas": {id: gjson.Parse(fmt.Sprintf("%q", name))}, "nas_destination": {id: gjson.Parse(fmt.Sprint(destination))}, "nas_policy": {id: gjson.Parse(policy)}}
}
func TestNASDiscoveryIdentityMoveAndConflicts(t *testing.T) {
	snapshots := map[string]map[string]map[string]gjson.Result{"a": nasInventory("id-a", "same NAS", false, `"policy"`), "b": nasInventory("id-b", "same NAS", true, `"policy"`)}
	load := func(ip string) map[string]map[string]gjson.Result { return snapshots[ip] }
	before, err := discoverNAS([]string{"a", "b"}, load)
	if err != nil || len(before) != 1 || before[0].IP != "a" {
		t.Fatalf("initial: %v %v", before, err)
	}
	snapshots["a"] = nasInventory("id-a", "same NAS", true, `"policy"`)
	snapshots["b"] = nasInventory("id-b", "same NAS", false, `"policy"`)
	after, err := discoverNAS([]string{"a", "b"}, load)
	if err != nil || len(after) != 1 || after[0].Key != before[0].Key || after[0].IP != "b" || after[0].ID != "id-b" {
		t.Fatalf("identity changed on move: %v %v", after, err)
	}
	snapshots["a"] = nasInventory("id-a", "same NAS", false, `"policy"`)
	if _, err := discoverNAS([]string{"a", "b"}, load); err == nil {
		t.Fatal("ambiguous source selected")
	}
	snapshots["a"] = nil
	if _, err := discoverNAS([]string{"a", "b"}, load); err == nil {
		t.Fatal("partial inventory accepted as complete")
	}
	snapshots["a"] = nasInventory("orphan", "retained flag", true, "null")
	rows, err := discoverNAS([]string{"a", "b"}, load)
	if err != nil || len(rows) != 2 {
		t.Fatalf("no-policy exception lost: %v", err)
	}
	delete(snapshots["a"]["nas_destination"], "orphan")
	if _, err := discoverNAS([]string{"a", "b"}, load); err == nil {
		t.Fatal("unknown role accepted")
	}
}
func fixtureFamily(name string, labels map[string]string, value float64) *dto.MetricFamily {
	typ := dto.MetricType_GAUGE
	m := &dto.Metric{Gauge: &dto.Gauge{Value: &value}}
	for k, v := range labels {
		k, v := k, v
		m.Label = append(m.Label, &dto.LabelPair{Name: &k, Value: &v})
	}
	return &dto.MetricFamily{Name: &name, Type: &typ, Metric: []*dto.Metric{m}}
}
func TestNASMetricIsolation(t *testing.T) {
	owner := nasOwner{Name: "NAS-one", ID: "id-one"}
	fs := fixtureFamily("powerstore_filesystem_logical_used", map[string]string{"nas_server_id": "id-one"}, 1)
	other := fixtureFamily("powerstore_filesystem_logical_used", map[string]string{"nas_server_id": "id-two"}, 2)
	fs.Metric = append(fs.Metric, other.Metric...)
	health := fixtureFamily("powerstore_collector_success", map[string]string{"collector": "filesystem_capacity"}, 0)
	filtered := nasMetricSubset([]*dto.MetricFamily{fs, health}, owner, "file")
	if len(filtered) != 2 || len(filtered[0].Metric) != 1 || len(fs.Metric) != 2 {
		t.Fatal("scope filtering changed cache or leaked another NAS")
	}
	status := fixtureFamily("powerstore_nas_server_operational_status", map[string]string{"name": "NAS-one"}, 1)
	perf := fixtureFamily("powerstore_metricNas_avg_total_iops", map[string]string{"nas_id": "NAS-one"}, 3)
	unrelated := fixtureFamily("powerstore_metricNas_avg_total_iops", map[string]string{"nas_id": "NAS-two"}, 5)
	if got := nasMetricSubset([]*dto.MetricFamily{status, perf, unrelated}, owner, "nas"); len(got) != 2 {
		t.Fatal("NAS status/performance selection incorrect")
	}
}
func TestNASGatherCacheSharesPollsAndFailures(t *testing.T) {
	calls := 0
	cache := &nasGatherCache{source: prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) { calls++; return nil, fmt.Errorf("unavailable") })}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.Gather(); err == nil {
				t.Error("failure hidden")
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("got %d upstream collections", calls)
	}
	cache.at = time.Now().Add(-2 * time.Minute)
	_, _ = cache.Gather()
	if calls != 2 {
		t.Fatal("cache did not refresh")
	}
}
func TestNASRoutesFollowOwner(t *testing.T) {
	a, b := "nas-test-a", "nas-test-b"
	client.PowerstoreModuleID[a] = nasInventory("id-a", "moving", false, `"policy"`)
	client.PowerstoreModuleID[b] = nasInventory("id-b", "moving", true, `"policy"`)
	defer delete(client.PowerstoreModuleID, a)
	defer delete(client.PowerstoreModuleID, b)
	source := func(value float64) *nasGatherCache {
		return &nasGatherCache{source: prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) {
			return []*dto.MetricFamily{fixtureFamily("powerstore_nas_server_operational_status", map[string]string{"name": "moving"}, value)}, nil
		})}
	}
	r := gin.New()
	registerNASHosts(r, []string{a, b}, map[string]nasArrayMetrics{a: {nas: source(1)}, b: {nas: source(2)}}, log.NewNopLogger())
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	d := get("/discovery/nas")
	var rows []nasOwner
	if err := json.Unmarshal(d.Body.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatal(d.Body.String())
	}
	path := "/nas-hosts/" + rows[0].Key + "/nas"
	if w := get(path); w.Code != 200 || !strings.Contains(w.Body.String(), "} 1") {
		t.Fatal(w.Body.String())
	}
	client.PowerstoreModuleID[a] = nasInventory("id-a", "moving", true, `"policy"`)
	client.PowerstoreModuleID[b] = nasInventory("id-b", "moving", false, `"policy"`)
	if w := get(path); w.Code != 200 || !strings.Contains(w.Body.String(), "} 2") {
		t.Fatal("stable URL did not move: " + w.Body.String())
	}
	client.PowerstoreModuleID[a] = nil
	if w := get("/discovery/nas"); w.Code != 503 || w.Body.String() == "[]" {
		t.Fatal("incomplete discovery looked empty")
	}
	if w := get(path); w.Code != 503 {
		t.Fatal("incomplete inventory still routed")
	}
	if w := get("/nas-hosts/invalid/unsupported"); w.Code != 404 {
		t.Fatal("unsupported route accepted")
	}
}
