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
	"math"
	"powerstore-metrics-exporter/collector/client"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

var portTypes = []string{
	"eth_port",
	"fc_port",
}

var portCollectorMetrics = []string{
	"is_link_up",
	"current_speed",
}

// port description
var metricPortDescMap = map[string]string{
	"is_link_up":    "Indicates whether the port's link is up:true is 1,false is 0",
	"current_speed": "Current port transmission speed in Gbps; zero means no numeric speed is available",
}

type portCollector struct {
	client  *client.Client
	metrics map[string]*prometheus.Desc
	logger  log.Logger
}

func NewPortCollector(api *client.Client, logger log.Logger) *portCollector {
	metrics := getPortMetrics(api.IP)
	return &portCollector{
		client:  api,
		metrics: metrics,
		logger:  logger,
	}
}

func (c *portCollector) Collect(ch chan<- prometheus.Metric) {
	level.Info(c.logger).Log("msg", "Start collecting port data")
	startTime := time.Now()
	inventory := client.ModuleIDs(c.client.IP)
	for _, portType := range portTypes {
		portTypeData, err := c.client.GetPort(portType)
		if err != nil {
			reportCollectionError(ch, err)
			level.Warn(c.logger).Log("msg", "get "+portType+" data error", "err", err)
			continue
		}
		module := map[string]string{"eth_port": "ethport", "fc_port": "fcport"}[portType]
		if err := checkHealthCoverage(portTypeData, inventory[module], []string{"id", "name", "appliance_id"}); err != nil {
			reportCollectionError(ch, err)
			level.Warn(c.logger).Log("msg", "incomplete port coverage", "ip", c.client.IP, "type", portType, "err", err)
		}
		for _, data := range gjson.Parse(portTypeData).Array() {
			if err := checkStringFields(data, []string{"id", "name", "appliance_id"}); err != nil {
				continue // Retain other ports while reporting incomplete coverage.
			}
			name := data.Get("name").String()
			id := data.Get("appliance_id").String()
			for _, metricName := range portCollectorMetrics {
				metricValue, err := getPortFloatDate(metricName, data.Get(metricName))
				if err != nil {
					reportCollectionError(ch, err)
					level.Warn(c.logger).Log("msg", "invalid port measurement", "ip", c.client.IP, "port", name, "metric", metricName, "err", err)
					continue
				}
				metricDesc := c.metrics[portType+metricName]
				ch <- prometheus.MustNewConstMetric(metricDesc, prometheus.GaugeValue, metricValue, id, name)
			}
		}
	}
	level.Info(c.logger).Log("msg", "Port collection finished", "time", time.Since(startTime))
}

func (c *portCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, descMap := range c.metrics {
		ch <- descMap
	}
}

func getPortFloatDate(key string, value gjson.Result) (float64, error) {
	if key == "is_link_up" {
		switch value.Type {
		case gjson.True:
			return 1, nil
		case gjson.False:
			return 0, nil
		default:
			return 0, fmt.Errorf("missing or invalid port link status")
		}
	}
	if key != "current_speed" {
		return 0, fmt.Errorf("unsupported port metric %q", key)
	}
	// Auto and null mean no numeric negotiated speed is available. Keep the
	// established zero convention so link state remains independently usable.
	if value.Type == gjson.Null || value.String() == "Auto" {
		return 0, nil
	}
	number, unit, found := strings.Cut(value.String(), "_")
	if !found || (unit != "Mbps" && unit != "Gbps") {
		return 0, fmt.Errorf("unsupported port speed %q", value.String())
	}
	speed, err := strconv.ParseFloat(number, 64)
	if err != nil || math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0 {
		return 0, fmt.Errorf("invalid port speed %q", value.String())
	}
	if unit == "Mbps" {
		speed /= 1000
	}
	return speed, nil
}

func getPortMetrics(ip string) map[string]*prometheus.Desc {
	res := map[string]*prometheus.Desc{}
	for _, portType := range portTypes {
		for _, metricName := range portCollectorMetrics {
			res[portType+metricName] = prometheus.NewDesc(
				"powerstore_"+portType+"_"+metricName,
				getPortDescByType(metricName),
				[]string{"appliance_id", portType + "_id"},
				prometheus.Labels{"IP": ip})
		}
	}
	return res
}

func getPortDescByType(key string) string {
	if v, ok := metricPortDescMap[key]; ok {
		return v
	} else {
		return key
	}
}
