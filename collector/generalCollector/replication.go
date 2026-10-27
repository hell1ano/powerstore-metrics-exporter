package generalCollector

import (
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
)

// Session identities never depend on source-only volume inventory. Raw states
// preserve new enum values instead of silently mapping them to a healthy zero.
type replicationCollector struct {
	client  *client.Client
	logger  log.Logger
	metrics map[string]*prometheus.Desc
}

func NewReplicationCollector(api *client.Client, logger log.Logger) *replicationCollector {
	c := &replicationCollector{client: api, logger: logger, metrics: map[string]*prometheus.Desc{}}
	add := func(name, help string, labels ...string) {
		c.metrics[name] = prometheus.NewDesc("powerstore_replication_session_"+name, help, labels, prometheus.Labels{"IP": api.IP})
	}
	add("count", "Number of replication sessions returned by the array, including all types and roles.")
	add("info", "Replication session identity; value is 1.", "session_id", "type", "role", "resource_type", "local_resource_id", "remote_resource_id", "remote_system_id", "parent_session_id", "replication_rule_id")
	for _, field := range []string{"state", "data_transfer_state", "data_connection_state", "local_resource_state", "auto_failover_state", "error_code"} {
		add(field, "Current replication session "+field+" as a label; value is 1.", "session_id", field)
	}
	add("rpo_seconds", "Configured RPO target in seconds, not measured destination lag.", "session_id")
	add("rpo_available", "Whether a configured RPO target is available for this session.", "session_id")
	add("alert_threshold_seconds", "Configured delay before an RPO compliance alert, in seconds.", "session_id")
	add("last_sync_duration_seconds", "Last synchronization duration; Metro updates only in asynchronous copy phases. Unsupported for NAS server sessions.", "session_id")
	add("progress_percentage", "Current replication operation progress, when supplied by the source. Unsupported for NAS server sessions.", "session_id")
	for _, field := range []string{"last_sync", "next_sync", "estimated_completion"} {
		add(field+"_timestamp_seconds", "Reported "+field+" timestamp; not destination recovery-point age. Unsupported for NAS server sessions.", "session_id")
	}
	add("failover_test_in_progress", "Whether a failover test is in progress, when reported.", "session_id")
	return c
}

func (c *replicationCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.metrics {
		ch <- desc
	}
}

var replicationRPOSeconds = map[string]float64{
	"Zero": 0, "Five_Minutes": 300, "Fifteen_Minutes": 900, "Thirty_Minutes": 1800,
	"One_Hour": 3600, "Six_Hours": 21600, "Twelve_Hours": 43200, "One_Day": 86400,
}

func (c *replicationCollector) Collect(ch chan<- prometheus.Metric) {
	report := func(id string, err error) {
		reportCollectionError(ch, err)
		level.Warn(c.logger).Log("msg", "replication session collection incomplete", "ip", c.client.IP, "session_id", id, "err", err)
	}
	data, err := c.client.GetReplicationSessions()
	if err != nil {
		report("", err)
		return
	}
	emit := func(name string, value float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(c.metrics[name], prometheus.GaugeValue, value, labels...)
	}
	rows := gjson.Parse(data).Array()
	byID := map[string]gjson.Result{}
	for _, row := range rows {
		byID[row.Get("id").String()] = row
	}
	seen := map[string]bool{}
	for _, row := range rows {
		id := row.Get("id").String()
		valid := true
		for _, field := range []string{"id", "type", "role", "resource_type", "state"} {
			v := row.Get(field)
			if v.Type != gjson.String || v.String() == "" {
				report(id, fmt.Errorf("missing or invalid %s", field))
				valid = false
			}
		}
		if !valid {
			continue
		}
		if seen[id] {
			report(id, fmt.Errorf("duplicate session ID"))
			continue
		}
		seen[id] = true
		emit("info", 1, id, row.Get("type").String(), row.Get("role").String(), row.Get("resource_type").String(), row.Get("local_resource_id").String(), row.Get("remote_resource_id").String(), row.Get("remote_system_id").String(), row.Get("parent_replication_session_id").String(), row.Get("replication_rule_id").String())
		for _, field := range []string{"state", "data_transfer_state", "data_connection_state", "local_resource_state", "auto_failover_state", "error_code"} {
			v := row.Get(field)
			if v.Type == gjson.Null {
				continue
			}
			if v.Type != gjson.String {
				report(id, fmt.Errorf("invalid %s", field))
				continue
			}
			if v.String() != "" {
				emit(field, 1, id, v.String())
			}
		}
		// Child filesystem sessions can inherit the parent's rule within this
		// response; no per-session or per-rule API lookup is performed.
		ruleRow := row
		visited := map[string]bool{id: true}
		for ruleRow.Get("replication_rule.rpo").Type == gjson.Null {
			parent := ruleRow.Get("parent_replication_session_id").String()
			if parent == "" || visited[parent] {
				break
			}
			visited[parent] = true
			ruleRow = byID[parent]
		}
		rpo := ruleRow.Get("replication_rule.rpo")
		available := 0.0
		if rpo.Type != gjson.Null {
			seconds, ok := replicationRPOSeconds[rpo.String()]
			if rpo.Type != gjson.String || !ok {
				report(id, fmt.Errorf("unknown RPO value %q", rpo.String()))
			} else {
				emit("rpo_seconds", seconds, id)
				available = 1
			}
		}
		emit("rpo_available", available, id)
		threshold := ruleRow.Get("replication_rule.alert_threshold")
		if threshold.Type != gjson.Null {
			if threshold.Type != gjson.Number || threshold.Float() < 0 {
				report(id, fmt.Errorf("invalid alert_threshold"))
			} else {
				emit("alert_threshold_seconds", threshold.Float()*60, id)
			}
		}
		// PowerStore 5.0 explicitly does not support these for NAS parents.
		if row.Get("resource_type").String() == "nas_server" {
			continue
		}
		for _, field := range []string{"last_sync", "next_sync", "estimated_completion"} {
			v := row.Get(field + "_timestamp")
			if v.Type == gjson.Null {
				continue
			}
			ts, err := time.Parse(time.RFC3339Nano, v.String())
			if v.Type != gjson.String || err != nil {
				report(id, fmt.Errorf("invalid %s timestamp", field))
				continue
			}
			emit(field+"_timestamp_seconds", float64(ts.UnixNano())/1e9, id)
		}
		for _, spec := range []struct {
			field, metric string
			scale         float64
		}{
			{"last_sync_duration", "last_sync_duration_seconds", .001}, {"progress_percentage", "progress_percentage", 1},
		} {
			v := row.Get(spec.field)
			if v.Type == gjson.Null {
				continue
			}
			if v.Type != gjson.Number || v.Float() < 0 {
				report(id, fmt.Errorf("invalid %s", spec.field))
				continue
			}
			emit(spec.metric, v.Float()*spec.scale, id)
		}
		v := row.Get("failover_test_in_progress")
		if v.Type == gjson.True {
			emit("failover_test_in_progress", 1, id)
		} else if v.Type == gjson.False {
			emit("failover_test_in_progress", 0, id)
		} else if v.Type != gjson.Null {
			report(id, fmt.Errorf("invalid failover_test_in_progress"))
		}
	}
	emit("count", float64(len(rows)))
}
