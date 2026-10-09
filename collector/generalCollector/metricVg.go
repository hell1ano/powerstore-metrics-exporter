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

var metricVgCollectorMetric = []string{
	"avg_read_latency",
	"avg_latency",
	"avg_write_latency",
	"avg_read_iops",
	"avg_read_bandwidth",
	"avg_total_iops",
	"avg_total_bandwidth",
	"avg_write_iops",
	"avg_write_bandwidth",
	"avg_io_size",
	"avg_read_size",
	"avg_write_size",
}

var metricMetricVgDescMap = map[string]string{
	"avg_read_latency":    "Average read latency in microseconds,unit is ms",
	"avg_latency":         "Average read and write latency in microseconds,unit is ms",
	"avg_write_latency":   "Average write latency in microseconds,unit is ms",
	"avg_read_iops":       "Total read operations per second,unit is iops",
	"avg_read_bandwidth":  "Read rate in bytes per second,unit is bps",
	"avg_total_iops":      "Total read and write operations per second,unit is iops",
	"avg_total_bandwidth": "Total data transfer rate in bytes per second,unit is bps",
	"avg_write_iops":      "Total write operations per second,unit is iops",
	"avg_write_bandwidth": "Write rate in bytes per second,unit is bps",
	"avg_io_size":         "Average size of read and write operations in bytes.unit is bytes",
	"avg_write_size":      "Average write size in bytes.unit is bytes",
	"avg_read_size":       "Average read size in bytes.unit is bytes",
}

type metricVgCollector struct {
	client       *client.Client
	isEnableBulk bool
	bulkClient   *bulkClient.BulkClient
	metrics      map[string]*prometheus.Desc
	logger       log.Logger
}

func NewMetricVgCollector(api *client.Client, bulkApi *bulkClient.BulkClient, logger log.Logger) *metricVgCollector {
	metrics := getMetricVgfMetrics(api.IP)
	return &metricVgCollector{
		client:       api,
		isEnableBulk: bulkApi.IsEnable,
		bulkClient:   bulkApi,
		metrics:      metrics,
		logger:       logger,
	}
}

func (c *metricVgCollector) Collect(ch chan<- prometheus.Metric) {
	level.Info(c.logger).Log("msg", "Start collecting volume group performance data")
	startTime := time.Now()
	if c.isEnableBulk {
		volumeGroupArray := client.ModuleIDs(c.client.IP)
		if volumeGroupArray["volumegroup"] == nil {
			reportCollectionError(ch, fmt.Errorf("inventory unavailable"))
			return
		}
		volumeGroupData, err := readBulkForObjects(c.bulkClient, "PerformanceMetricsByVg", "vg_id", volumeGroupArray["volumegroup"], metricVgCollectorMetric)
		if err != nil {
			reportCollectionError(ch, err)
			level.Warn(c.logger).Log("msg", "get volume group performance data error", "err", err)
			return
		}
		volumeGroupDataJson := gjson.Parse(volumeGroupData)
		for _, data := range volumeGroupDataJson.Array() {
			volumeGroupID := data.Get("vg_id").String()
			volumeGroupName := volumeGroupArray["volumegroup"][volumeGroupID]
			for _, metricName := range metricVgCollectorMetric {
				metricValue := data.Get(metricName)
				metricDesc := c.metrics["vg"+"_"+metricName]
				if metricValue.Type == gjson.Number {
					ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, metricValue.Float(), volumeGroupName.String())
				}
			}
		}
	} else {
		var wg sync.WaitGroup
		vgArray := client.ModuleIDs(c.client.IP)
		if vgArray["volumegroup"] == nil {
			reportCollectionError(ch, fmt.Errorf("inventory unavailable"))
			return
		}
		for vgId, vgName := range vgArray["volumegroup"] {
			wg.Add(1)
			go func(vgId, vgName string) {
				defer wg.Done()
				metricVgData, err := c.client.GetMetricVg(vgId)
				if err != nil {
					reportCollectionError(ch, err)
					level.Warn(c.logger).Log("msg", "get volume group performance data error", "err", err)
					return
				}
				vgDataArray := gjson.Parse(metricVgData).Array()
				if len(vgDataArray) == 0 {
					reportCollectionError(ch, fmt.Errorf("no samples returned for known object"))
					level.Warn(c.logger).Log("msg", "get volume group performance data is null")
					return
				}
				vgData := vgDataArray[len(vgDataArray)-1]
				if !requireMeasurements(ch, c.logger, c.client.IP, vgId, vgData, metricVgCollectorMetric) {
					return
				}
				for _, metricName := range metricVgCollectorMetric {
					metricValue := vgData.Get(metricName)
					metricDesc := c.metrics["vg"+"_"+metricName]
					if metricValue.Type == gjson.Number {
						ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, metricValue.Float(), vgName)
					}
				}
			}(vgId, vgName.String())
		}
		wg.Wait()
	}
	level.Info(c.logger).Log("msg", "Obtaining the performance volume group is successful", "time", time.Since(startTime))
}

func (c *metricVgCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, descMap := range c.metrics {
		ch <- descMap
	}
}

func getMetricVgfMetrics(ip string) map[string]*prometheus.Desc {
	res := map[string]*prometheus.Desc{}
	for _, metricName := range metricVgCollectorMetric {
		res["vg"+"_"+metricName] = prometheus.NewDesc(
			"powerstore_metricVg_"+metricName,
			getMetricVgDescByType(metricName),
			[]string{"volume_group_id"},
			prometheus.Labels{"IP": ip})
	}
	return res
}

func getMetricVgDescByType(key string) string {
	if v, ok := metricMetricVgDescMap[key]; ok {
		return v
	} else {
		return key
	}
}
