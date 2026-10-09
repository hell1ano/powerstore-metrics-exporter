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
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/bulkClient"
	"powerstore-metrics-exporter/collector/client"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var metricFileSystemCollector = []string{
	"logical_provisioned",
	"logical_used",
	"thin_savings",
}

// file description
var metricFileSystemDescMap = map[string]string{
	"logical_provisioned": "Last logical provisioned space during the period.",
	"logical_used":        "Last logical used space during the period.",
	"thin_savings":        "Last thin savings ratio during the period.",
}

type fileSystemCollector struct {
	client       *client.Client
	isEnableBulk bool
	bulkClient   *bulkClient.BulkClient
	metrics      map[string]*prometheus.Desc
	logger       log.Logger
}

func NewFileCollector(api *client.Client, bulkApi *bulkClient.BulkClient, logger log.Logger) *fileSystemCollector {
	metrics := getFileSystemMetrics(api.IP)
	return &fileSystemCollector{
		client:       api,
		isEnableBulk: bulkApi.IsEnable,
		bulkClient:   bulkApi,
		metrics:      metrics,
		logger:       logger,
	}
}

func (c *fileSystemCollector) Collect(ch chan<- prometheus.Metric) {
	level.Info(c.logger).Log("msg", "Start collecting filesystem data")
	startTime := time.Now()
	if c.isEnableBulk {
		filesystemArray := client.ModuleIDs(c.client.IP)
		if filesystemArray["filesystem"] == nil {
			reportCollectionError(ch, fmt.Errorf("inventory unavailable"))
			return
		}
		filesystemData, err := readBulkForObjects(c.bulkClient, "SpaceMetricsByFilesystem", "file_system_id", filesystemArray["filesystem"], metricFileSystemCollector)
		if err != nil {
			reportCollectionError(ch, err)
			level.Warn(c.logger).Log("msg", "get filesystem space data error", "err", err)
			return
		}
		filesystemDataJson := gjson.Parse(filesystemData)
		for _, data := range filesystemDataJson.Array() {
			filesystemID := data.Get("file_system_id").String()
			labels, err := filesystemLabels(filesystemArray, filesystemID)
			if err != nil {
				reportCollectionError(ch, err)
				continue
			}
			for _, metricName := range metricFileSystemCollector {
				metricValue := data.Get(metricName)
				metricDesc := c.metrics["filesystem"+"_"+metricName]
				if metricValue.Exists() && metricValue.Type != gjson.Null {
					ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, metricValue.Float(), labels...)
				}
			}
		}
	} else {
		moduleIDArray := client.ModuleIDs(c.client.IP)
		if moduleIDArray["filesystem"] == nil {
			reportCollectionError(ch, fmt.Errorf("inventory unavailable"))
			return
		}
		for filesystemID := range moduleIDArray["filesystem"] {
			labels, err := filesystemLabels(moduleIDArray, filesystemID)
			if err != nil {
				reportCollectionError(ch, err)
				continue
			}
			filesystemData, err := c.client.GetFilesystemCap(filesystemID)
			if err != nil {
				reportCollectionError(ch, err)
				level.Warn(c.logger).Log("msg", "get filesystem data error", "err", err)
				return
			}
			filesystemArray := gjson.Parse(filesystemData).Array()
			if len(filesystemArray) == 0 {
				reportCollectionError(ch, fmt.Errorf("no samples returned for known object"))
				continue
			}
			for _, metricName := range metricFileSystemCollector {
				metricValue := filesystemArray[len(filesystemArray)-1].Get(metricName)
				metricDesc := c.metrics["filesystem_"+metricName]
				if metricValue.Exists() && metricValue.Type != gjson.Null {
					ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, metricValue.Float(), labels...)
				}
			}
		}
	}
	level.Info(c.logger).Log("msg", "Obtaining the filesystem is successful", "time", time.Since(startTime))
}

func (c *fileSystemCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, descMap := range c.metrics {
		ch <- descMap
	}
}

func getFileSystemMetrics(ip string) map[string]*prometheus.Desc {
	res := map[string]*prometheus.Desc{}
	for _, metricName := range metricFileSystemCollector {
		res["filesystem_"+metricName] = prometheus.NewDesc(
			"powerstore_filesystem_"+metricName,
			getFileSystemDescByType(metricName),
			[]string{"name", "file_system_id", "nas_server_id", "nas_server_name"},
			prometheus.Labels{"IP": ip})
	}
	return res
}

func getFileSystemDescByType(key string) string {
	if v, ok := metricFileSystemDescMap[key]; ok {
		return v
	} else {
		return key
	}
}
