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
	"fmt"
	"powerstore-metrics-exporter/collector/bulkClient"
	"powerstore-metrics-exporter/collector/client"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

var metricFcPortCollectorMetric = []string{
	"avg_read_latency",
	"avg_latency",
	"avg_write_latency",
	"avg_total_iops",
	"avg_total_bandwidth",
	"avg_dumped_frames_ps",
	"avg_loss_of_signal_count_ps",
	"avg_invalid_crc_count_ps",
	"avg_loss_of_sync_count_ps",
	"avg_invalid_tx_word_count_ps",
	"avg_prim_seq_prot_err_count_ps",
	"avg_link_failure_count_ps",
}

var metricMetricFcPortDescMap = map[string]string{
	"avg_read_latency":               "Average read latency in microseconds,unit is ms",
	"avg_latency":                    "Average read and write latency in microseconds,unit is ms",
	"avg_write_latency":              "Average write latency in microseconds,unit is ms",
	"avg_total_iops":                 "Total read and write operations per second,unit is iops",
	"avg_total_bandwidth":            "Total data transfer rate in bytes per second,unit is bps",
	"avg_dumped_frames_ps":           "count of dumped frames in a second",
	"avg_loss_of_signal_count_ps":    "count of loss of signal in a second",
	"avg_invalid_crc_count_ps":       "count of invalid useless in a second",
	"avg_loss_of_sync_count_ps":      "count of loss of sync in a second",
	"avg_invalid_tx_word_count_ps":   "count of invalid send word in a second",
	"avg_prim_seq_prot_err_count_ps": "count of prim seq prot err in a second",
	"avg_link_failure_count_ps":      "count of link failure in a second",
}

type metricFcPortCollector struct {
	client       *client.Client
	isEnableBulk bool
	bulkClient   *bulkClient.BulkClient
	metrics      map[string]*prometheus.Desc
	logger       log.Logger
}

func NewMetricFcPortCollector(api *client.Client, bulkApi *bulkClient.BulkClient, logger log.Logger) *metricFcPortCollector {
	metrics := getMetricFcPortMetrics(api.IP)
	return &metricFcPortCollector{
		client:       api,
		isEnableBulk: bulkApi.IsEnable,
		bulkClient:   bulkApi,
		metrics:      metrics,
		logger:       logger,
	}
}

func (c *metricFcPortCollector) Collect(ch chan<- prometheus.Metric) {
	level.Info(c.logger).Log("msg", "Start collecting fcPort performance data")
	startTime := time.Now()
	if c.isEnableBulk {
		fcPortArray := client.ModuleIDs(c.client.IP)
		if fcPortArray["fcport"] == nil {
			reportCollectionError(ch, fmt.Errorf("inventory unavailable"))
			return
		}
		fcPortData, err := readBulkForObjects(c.bulkClient, "PerformanceMetricsByFeFcPort", "fe_port_id", fcPortArray["fcport"], metricFcPortCollectorMetric)
		if err != nil {
			reportCollectionError(ch, err)
			level.Warn(c.logger).Log("msg", "get fc port performance data error", "err", err)
			return
		}
		ethDataJson := gjson.Parse(fcPortData)
		for _, data := range ethDataJson.Array() {
			applianceID := data.Get("appliance_id").String()
			fcPortID := data.Get("fe_port_id").String()
			fcPortName := fcPortArray["fcport"][fcPortID]
			for _, metricName := range metricFcPortCollectorMetric {
				metricValue := data.Get(metricName)
				metricDesc := c.metrics["fcport"+"_"+metricName]
				if metricValue.Exists() && metricValue.Type != gjson.Null {
					ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, metricValue.Float(), fcPortName.String(), applianceID)
				}
			}
		}
	} else {
		var wg sync.WaitGroup
		fcPortArray := client.ModuleIDs(c.client.IP)
		if fcPortArray["fcport"] == nil {
			reportCollectionError(ch, fmt.Errorf("inventory unavailable"))
			return
		}
		for portId, portName := range fcPortArray["fcport"] {
			wg.Add(1)
			go func(portId, portName string) {
				defer wg.Done()
				fcPortsData, err := c.client.GetMetricFcPort(portId)
				if err != nil {
					reportCollectionError(ch, err)
					level.Warn(c.logger).Log("msg", "get fcPort performance data error", "err", err)
					return
				}
				fcPortDataArray := gjson.Parse(fcPortsData).Array()
				if len(fcPortDataArray) == 0 {
					reportCollectionError(ch, fmt.Errorf("no samples returned for known object"))
					level.Warn(c.logger).Log("msg", "get fcPort performance data is null")
					return
				}
				fcPortData := fcPortDataArray[len(fcPortDataArray)-1]
				applianceID := fcPortData.Get("appliance_id").String()
				for _, metricName := range metricFcPortCollectorMetric {
					metricValue := fcPortData.Get(metricName)
					metricDesc := c.metrics["fcport"+"_"+metricName]
					if metricValue.Exists() && metricValue.Type != gjson.Null {
						ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, metricValue.Float(), portName, applianceID)
					}
				}
			}(portId, portName.String())
		}
		wg.Wait()
	}
	level.Info(c.logger).Log("msg", "Obtaining the performance fc port is successful", "time", time.Since(startTime))
}

func (c *metricFcPortCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, descMap := range c.metrics {
		ch <- descMap
	}
}

func getMetricFcPortMetrics(ip string) map[string]*prometheus.Desc {
	res := map[string]*prometheus.Desc{}

	for _, metricName := range metricFcPortCollectorMetric {
		res["fcport"+"_"+metricName] = prometheus.NewDesc(
			"powerstore_metricFcPort_"+metricName,
			getMetricFcPortDescByType(metricName),
			[]string{"fc_port_id", "appliance_id"},
			prometheus.Labels{"IP": ip})
	}
	return res
}

func getMetricFcPortDescByType(key string) string {
	if v, ok := metricMetricFcPortDescMap[key]; ok {
		return v
	} else {
		return key
	}
}
