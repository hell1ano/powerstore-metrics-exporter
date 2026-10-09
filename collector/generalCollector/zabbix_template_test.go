package generalCollector

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
	"powerstore-metrics-exporter/collector/bulkClient"
	"powerstore-metrics-exporter/collector/client"
)

// Check selectors against actual exporter descriptors, not a duplicate list of
// expected metric names. This catches both the old filesystem names and NAS label bug.
func TestZabbix7MetricContract(t *testing.T) {
	api := &client.Client{IP: "test-array"}
	client.PowerstoreModuleID[api.IP] = map[string]map[string]gjson.Result{"appliance": {"a1": gjson.Parse(`"appliance"`)}}
	defer delete(client.PowerstoreModuleID, api.IP)
	bc := &bulkClient.BulkClient{IP: api.IP}
	logger := log.NewNopLogger()
	collectors := []prometheus.Collector{
		NewClusterCollector(api, logger), NewMetroCollector(api, logger), NewPortCollector(api, logger),
		NewHardwareCollector(api, logger), NewVolumeCollector(api, logger), NewApplianceCollector(api, logger),
		NewNasCollector(api, logger), NewVolumeGroupCollector(api, logger), NewCapacityCollector(api, logger),
		NewFileCollector(api, bc, logger), NewMetricApplianceCollector(api, bc, logger), NewMetricFcPortCollector(api, bc, logger),
		NewMetricEthPortCollector(api, bc, logger), NewMetricNasCollector(api, bc, logger), NewMetricFilesystemCollector(api, bc, logger),
		NewMetricVolumeCollector(api, bc, logger), NewMetricVgCollector(api, bc, logger), NewWearMetricCollector(api, bc, logger),
		bc, Monitored(api.IP, "fixture", true, &fixtureCollector{}),
	}
	contracts := map[string]map[string]bool{}
	descName := regexp.MustCompile(`fqName: "([^"]+)"`)
	variableLabels := regexp.MustCompile(`variableLabels: \[([^]]*)\]`)
	for _, c := range collectors {
		ch := make(chan *prometheus.Desc, 128)
		c.Describe(ch)
		close(ch)
		for d := range ch {
			text := d.String()
			metric := descName.FindStringSubmatch(text)[1]
			labels := map[string]bool{"IP": true}
			for _, label := range strings.Fields(variableLabels.FindStringSubmatch(text)[1]) {
				labels[label] = true
			}
			if strings.Contains(text, `collector=`) {
				labels["collector"] = true
			}
			contracts[metric] = labels
		}
	}
	data, err := os.ReadFile("../../templates/zabbix/zbx_export_templates_7.0.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]interface{}
	if err = yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	labelPattern := regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*(?:=~|!~|!=|=)"`)
	namePattern := regexp.MustCompile(`__name__=~?"\^?([A-Za-z0-9_]+)`)
	pathPattern := regexp.MustCompile(`labels\["([^"]+)"\]`)
	var check func(interface{})
	check = func(value interface{}) {
		switch v := value.(type) {
		case map[string]interface{}:
			if steps, ok := v["preprocessing"].([]interface{}); ok {
				for _, step := range steps {
					s := step.(map[string]interface{})
					kind := s["type"]
					if kind != "PROMETHEUS_PATTERN" && kind != "PROMETHEUS_TO_JSON" {
						continue
					}
					selector := s["parameters"].([]interface{})[0].(string)
					metric := strings.SplitN(selector, "{", 2)[0]
					if metric == "" {
						match := namePattern.FindStringSubmatch(selector)
						if match == nil {
							t.Errorf("unknown selector %s", selector)
							continue
						}
						metric = match[1]
					}
					labels, ok := contracts[metric]
					if !ok {
						t.Errorf("template selects nonexistent metric %s", metric)
						continue
					}
					for _, m := range labelPattern.FindAllStringSubmatch(selector, -1) {
						if m[1] != "__name__" && !labels[m[1]] {
							t.Errorf("%s has no label %s", metric, m[1])
						}
					}
					if kind == "PROMETHEUS_PATTERN" {
						params := s["parameters"].([]interface{})
						if params[1] == "label" && !labels[params[2].(string)] {
							t.Errorf("%s cannot extract %v", metric, params[2])
						}
					}
					if kind == "PROMETHEUS_TO_JSON" {
						paths, _ := v["lld_macro_paths"].([]interface{})
						for _, path := range paths {
							p := path.(map[string]interface{})
							m := pathPattern.FindStringSubmatch(p["path"].(string))
							if m != nil && !labels[m[1]] {
								t.Errorf("%s discovery expects missing label %s", metric, m[1])
							}
						}
					}
				}
			}
			for _, x := range v {
				check(x)
			}
		case []interface{}:
			for _, x := range v {
				check(x)
			}
		}
	}
	check(document)
}

func TestZabbix7StructureAndTriggers(t *testing.T) {
	read := func(path string) map[string]interface{} {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var d map[string]interface{}
		if e = yaml.Unmarshal(b, &d); e != nil {
			t.Fatal(e)
		}
		return d
	}
	old := read("../../templates/zabbix/zbx_export_templates.yaml")
	current := read("../../templates/zabbix/zbx_export_templates_7.0.yaml")
	oldIDs := map[string]bool{}
	ids := map[string]bool{}
	keys := map[string]bool{}
	var masters []string
	var walk func(interface{}, bool)
	walk = func(value interface{}, legacy bool) {
		switch v := value.(type) {
		case map[string]interface{}:
			if id, ok := v["uuid"].(string); ok {
				if legacy {
					oldIDs[id] = true
				} else {
					if ids[id] || oldIDs[id] {
						t.Errorf("UUID collision: %s", id)
					}
					ids[id] = true
				}
			}
			if !legacy {
				if key, ok := v["key"].(string); ok {
					if _, isItem := v["type"]; isItem {
						if keys[key] {
							t.Errorf("duplicate key %s", key)
						}
						keys[key] = true
					}
				}
				if master, ok := v["master_item"].(map[string]interface{}); ok {
					masters = append(masters, master["key"].(string))
				}
				if v["type"] == "HTTP_AGENT" {
					if !strings.HasPrefix(v["url"].(string), "{$POWERSTORE.EXPORTER.URL}/") {
						t.Error("hardcoded exporter scheme")
					}
					if v["verify_peer"] != "YES" || v["verify_host"] != "YES" {
						t.Error("TLS verification disabled")
					}
				}
				if expression, ok := v["expression"].(string); ok {
					if strings.Contains(expression, "change(") || strings.Contains(expression, ",6h)") {
						t.Errorf("unsafe trigger %s", expression)
					}
				}
			}
			for _, x := range v {
				walk(x, legacy)
			}
		case []interface{}:
			for _, x := range v {
				walk(x, legacy)
			}
		}
	}
	walk(old, true)
	walk(current, false)
	for _, key := range masters {
		if !keys[key] {
			t.Errorf("missing master %s", key)
		}
	}
	root := current["zabbix_export"].(map[string]interface{})
	if root["version"] != "7.0" || root["template_groups"] == nil || root["date"] != nil {
		t.Error("incorrect Zabbix 7 export envelope")
	}
}
