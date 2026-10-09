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
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

var capCollectorMetric = []string{
	"last_logical_provisioned",
	"last_logical_used",
	"last_physical_total",
	"last_physical_used",
	"max_logical_provisioned",
	"max_logical_used",
	"max_physical_total",
	"max_physical_used",
	"last_data_physical_used",
	"max_data_physical_used",
	"last_efficiency_ratio",
	"last_data_reduction",
	"last_snapshot_savings",
	"last_thin_savings",
	"max_efficiency_ratio",
	"max_data_reduction",
	"max_snapshot_savings",
	"max_thin_savings",
	"last_shared_logical_used",
	"max_shared_logical_used",
}

var metricCapDescMap = map[string]string{
	"last_logical_provisioned": "Last logical total space during the period,unit is B",
	"last_logical_used":        "Last logical used space during the period,unit is B",
	"last_physical_total":      "Last physical total space during the period,unit is B",
	"last_physical_used":       "Last physical used space during the period,unit is B",
	"max_logical_provisioned":  "Maxiumum logical total space during the period,unit is B",
	"max_logical_used":         "Maxiumum logical used space during the period,unit is B",
	"max_physical_total":       "Maximum physical total space during the period,unit is B",
	"max_physical_used":        "Maximum physical used space during the period,unit is B",
	"last_data_physical_used":  "Last physical used space for data during the period,unit is B",
	"max_data_physical_used":   "Maximum physical used space for data during the period,unit is B",
	"last_efficiency_ratio":    "Last efficiency ratio during the period.",
	"last_data_reduction":      "Last data reduction space during the period.unit is B",
	"last_snapshot_savings":    "Last snapshot savings space during the period.",
	"last_thin_savings":        "Last thin savings ratio during the period.",
	"max_efficiency_ratio":     "Maximum efficiency ratio during the period.",
	"max_data_reduction":       "Maximum data reduction space during the period,unit is B",
	"max_snapshot_savings":     "Maximum snapshot savings space during the period.",
	"max_thin_savings":         "Maximum thin savings ratio during the period.",
	"last_shared_logical_used": "Last shared logical used during the period,unit is B",
	"max_shared_logical_used":  "Max shared logical used during the period,unit is B",
}

type capacityCollector struct {
	bulkClient *bulkClient.BulkClient
	client     *client.Client
	metrics    map[string]*prometheus.Desc
	logger     log.Logger
}

func NewCapacityCollector(api *client.Client, logger log.Logger, bulk ...*bulkClient.BulkClient) *capacityCollector {
	metrics := getCapacityMetrics(api.IP)
	var bc *bulkClient.BulkClient
	if len(bulk) > 0 {
		bc = bulk[0]
	}
	return &capacityCollector{
		client:     api,
		bulkClient: bc,
		metrics:    metrics,
		logger:     logger,
	}
}

func (c *capacityCollector) Collect(ch chan<- prometheus.Metric) {
	level.Info(c.logger).Log("msg", "Start collecting capacity data")
	startTime := time.Now()
	applianceArray := client.ModuleIDs(c.client.IP)
	if applianceArray["appliance"] == nil {
		reportCollectionError(ch, fmt.Errorf("inventory unavailable"))
		return
	}
	if c.bulkClient != nil && c.bulkClient.IsEnable {
		data, err := readBulkForObjects(c.bulkClient, "SpaceMetricsByAppliance", "appliance_id", applianceArray["appliance"], []string{"physical_used", "physical_total", "logical_used", "logical_provisioned"})
		if err != nil {
			reportCollectionError(ch, err)
			return
		}
		for _, row := range gjson.Parse(data).Array() {
			c.emitCapacity(ch, row)
		}
		return
	}
	for applianceID, _ := range applianceArray["appliance"] {
		capacityData, err := c.client.GetCap(applianceID)
		if err != nil {
			reportCollectionError(ch, err)
			level.Warn(c.logger).Log("msg", "get capacity data error", "err", err)
			return
		}
		capacityDataArray := gjson.Parse(capacityData).Array()
		if len(capacityDataArray) == 0 {
			reportCollectionError(ch, fmt.Errorf("no capacity samples"))
			continue
		}
		capacity := capacityDataArray[len(capacityDataArray)-1]
		c.emitCapacity(ch, capacity)
	}
	level.Info(c.logger).Log("msg", "Obtaining the cluster capacity is successful", "time", time.Since(startTime))
}

// Latest-sample aliases preserve existing consumers. A five-minute sample is
// never fabricated as a daily maximum; max_* is emitted only if supplied.
func (c *capacityCollector) emitCapacity(ch chan<- prometheus.Metric, capacity gjson.Result) {
	for _, field := range []string{"logical_provisioned", "logical_used", "physical_total", "physical_used"} {
		if capacity.Get(field).Type != gjson.Number && capacity.Get("last_"+field).Type != gjson.Number {
			reportCollectionError(ch, fmt.Errorf("capacity sample missing required measurement"))
			return
		}
	}
	for _, metricName := range capCollectorMetric {
		value := capacity.Get(metricName)
		if !value.Exists() && strings.HasPrefix(metricName, "last_") {
			value = capacity.Get(strings.TrimPrefix(metricName, "last_"))
		}
		if value.Type == gjson.Number {
			ch <- prometheus.MustNewConstMetric(c.metrics[metricName], prometheus.GaugeValue, value.Float(), capacity.Get("appliance_id").String())
		}
	}
}

func (c *capacityCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, descMap := range c.metrics {
		ch <- descMap
	}
}

func getCapacityMetrics(ip string) map[string]*prometheus.Desc {
	res := map[string]*prometheus.Desc{}
	for _, metricName := range capCollectorMetric {
		res[metricName] = prometheus.NewDesc(
			"powerstore_cap_"+metricName,
			getCapacityDescByType(metricName),
			[]string{"appliance_id"},
			prometheus.Labels{"IP": ip})
	}
	return res
}

func getCapacityDescByType(key string) string {
	if v, ok := metricCapDescMap[key]; ok {
		return v
	} else {
		return key
	}
}
