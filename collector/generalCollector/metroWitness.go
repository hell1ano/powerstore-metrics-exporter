/*
 Copyright (c) 2024-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package generalCollector

import (
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var statusMetroMetricsMap = map[string]map[string]int{
	"state":         {"Initializing": 0, "Disengaged": 1, "Engaged": 2, "Remote_Witness_Invalid_Or_Unavailable": 3, "Failed_To_Initialize": 4, "Unconfigure_In_Progress": 5},
	"witness_state": {"OK": 0, "Partially_Connected": 1, "Disconnected": 2, "Deleting": 3, "Initializing": 4},
	"conn_state":    {"OK": 0, "Disconnected": 1, "Initializing": 2},
}

var metricMetroDescMap = map[string]string{
	"state":         "Witness state of replication;Initializing - 0 Disengaged - 1 Engaged - 2 Remote_Witness_Invalid_Or_Unavailable - 3 Failed_To_Initialize - 4 Unconfigure_In_Progress - 5",
	"witness_state": "Possible witness states;OK - 0 Partially_Connected - 1 Disconnected - 2 Deleting - 3 Initializing - 4",
	"conn_state":    "Possible witness connection states;OK - 0 Disconnected - 1 Initializing - 2",
}

type metroCollector struct {
	client  *client.Client
	metrics map[string]*prometheus.Desc
	logger  log.Logger
}

func NewMetroCollector(api *client.Client, logger log.Logger) *metroCollector {
	metrics := getMetroMetrics(api.IP)
	return &metroCollector{
		client:  api,
		metrics: metrics,
		logger:  logger,
	}
}

func (c *metroCollector) Collect(ch chan<- prometheus.Metric) {
	level.Info(c.logger).Log("msg", "Start collecting metro and witness data")
	startTime := time.Now()
	metroData, err := c.client.GetMetro()
	if err != nil {
		reportCollectionError(ch, err)
		level.Warn(c.logger).Log("msg", "get metro data error", "err", err)
		return
	}

	for _, data := range gjson.Parse(metroData).Array() {
		id := data.Get("id").String()
		metroType := data.Get("type").String()
		metricValue := data.Get("witness_details").Get("state")
		metricDesc := c.metrics["metro_status"]
		if metricValue.Exists() && metricValue.Type != gjson.Null {
			ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, getMetroFloatData("state", metricValue), id, metroType)
		}
	}

	witnessData, err := c.client.GetWitness()
	if err != nil {
		reportCollectionError(ch, err)
		level.Warn(c.logger).Log("msg", "get witness data error", "err", err)
		return
	}

	for _, data := range gjson.Parse(witnessData).Array() {
		witnessName := data.Get("name").String()
		witnessState := data.Get("state")
		witnessAddr := data.Get("address").String()
		for _, connData := range data.Get("connections").Array() {
			nodeID := connData.Get("node_id").String()
			nodeStatus := connData.Get("state")
			metricDesc := c.metrics["metro_witness_conn_status"]
			if witnessState.Exists() && witnessState.Type != gjson.Null {
				ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, getMetroFloatData("conn_state", nodeStatus), witnessName, nodeID)
			}
		}
		metricDesc := c.metrics["metro_witness_status"]
		if witnessState.Exists() && witnessState.Type != gjson.Null {
			ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, getMetroFloatData("witness_state", witnessState), witnessName, witnessAddr)
		}
	}

	level.Info(c.logger).Log("msg", "Obtaining the metro and witness is successful", "time", time.Since(startTime))
}

func (c *metroCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, descMap := range c.metrics {
		ch <- descMap
	}
}

func getMetroMetrics(ip string) map[string]*prometheus.Desc {
	res := map[string]*prometheus.Desc{}
	res["metro_status"] = prometheus.NewDesc(
		"powerstore_metro_status",
		getMetroDescByType("state"),
		[]string{"id", "metro_type"},
		prometheus.Labels{"IP": ip})

	res["metro_witness_conn_status"] = prometheus.NewDesc(
		"powerstore_metro_witness_conn_status",
		getMetroDescByType("conn_state"),
		[]string{"name", "node_id"},
		prometheus.Labels{"IP": ip})

	res["metro_witness_status"] = prometheus.NewDesc(
		"powerstore_metro_witness_status",
		getMetroDescByType("witness_state"),
		[]string{"name", "witness_addr"},
		prometheus.Labels{"IP": ip})
	return res
}

func getMetroDescByType(key string) string {
	if v, ok := metricMetroDescMap[key]; ok {
		return v
	} else {
		return key
	}
}

func getMetroFloatData(key string, value gjson.Result) float64 {
	if v, ok := statusMetroMetricsMap[key]; ok {
		if res, ok2 := v[value.String()]; ok2 {
			return float64(res)
		} else {
			return float64(v["other"])
		}
	} else {
		return value.Float()
	}
}
