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

package route

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"powerstore-metrics-exporter/collector/bulkClient"
	"powerstore-metrics-exporter/collector/client"
	"powerstore-metrics-exporter/collector/generalCollector"
	"powerstore-metrics-exporter/utils"
	"strconv"
	"time"
)

func Run(config *utils.Config, logger log.Logger) {
	maxAge := 15 * time.Minute
	if config.Exporter.BulkMaxAge != "" {
		var err error
		maxAge, err = time.ParseDuration(config.Exporter.BulkMaxAge)
		if err != nil || maxAge <= 0 {
			level.Error(logger).Log("msg", "bulkMaxAge must be a positive duration")
			return
		}
	}
	r := gin.New()
	r.Use(gin.Recovery())
	gin.SetMode(gin.ReleaseMode)
	for _, storage := range config.StorageList {
		client, err := client.NewClient(storage, logger)
		if err != nil {
			level.Error(logger).Log("msg", "init PowerStore client error", "err", err, "ip", storage.Ip)
			if client == nil {
				continue
			}
		}

		var bc = &bulkClient.BulkClient{
			IsEnable: false,
			IP:       storage.Ip,
		}
		if storage.Bulk {
			bc.IsEnable = true
			bc, err = bulkClient.NewBulkClient(storage, config.Exporter.BulkDir, logger)
			if err != nil {
				level.Error(logger).Log("msg", "init PowerStore bulk client error", "err", err, "ip", storage.Ip)
				continue
			}
			bc.MaxAge = maxAge
			err = bc.BulkEnable()
			if err != nil {
				level.Error(logger).Log("msg", "failed to enable batch request api", "err", err, "ip", storage.Ip)
			}
			// It takes at least one minute to start the API by opening the BULK API
			level.Info(logger).Log("msg", "The bulk api is starting...", "ip", storage.Ip)
			time.Sleep(60 * time.Second)
			level.Info(logger).Log("msg", "The bulk api startup is completed", "ip", storage.Ip)
			// Initialize download bulk api data
			if err := bc.DownloadBulkData(); err != nil {
				level.Error(logger).Log("msg", "error downloading batch data", "err", err, "ip", storage.Ip)
			}
			// Add timed tasks and download bulk API data regularly
			c := cron.New()
			var bulkCron string
			if config.Exporter.BulkCron == "" {
				bulkCron = "*/5 * * * *"
			} else {
				bulkCron = config.Exporter.BulkCron
			}
			_, err = c.AddFunc(bulkCron, func() {
				level.Info(logger).Log("msg", "Start a scheduled task", "ip", storage.Ip)
				if err := bc.DownloadBulkData(); err != nil {
					level.Error(logger).Log("msg", "error downloading batch data", "err", err, "ip", storage.Ip)
				}
			})
			if err != nil {
				level.Error(logger).Log("msg", "task addition failed", "err", err, "ip", storage.Ip)
			} else {
				// Start collecting bulk API data regularly
				c.Start()
			}
		}
		// Initialize the corresponding relationship between each component id and component name
		client.InitModuleID(logger)
		inventoryCron, err := inventoryRefreshJob(config.Exporter.InventoryRefresh, func() { client.InitModuleID(logger) })
		if err != nil {
			level.Error(logger).Log("msg", "invalid inventoryRefresh", "err", err)
			return
		}
		if inventoryCron != nil {
			inventoryCron.Start()
		}

		// Generate the registry for each component collector
		ClusterRegistry := prometheus.NewPedanticRegistry()
		PortRegistry := prometheus.NewPedanticRegistry()
		FileSystemRegistry := prometheus.NewPedanticRegistry()
		HardwareRegistry := prometheus.NewPedanticRegistry()
		VolumeRegistry := prometheus.NewPedanticRegistry()
		ApplianceRegistry := prometheus.NewPedanticRegistry()
		NasRegistry := prometheus.NewPedanticRegistry()
		VolumeGroupRegistry := prometheus.NewPedanticRegistry()
		CapacityRegistry := prometheus.NewPedanticRegistry()

		// The collector that registers each component in the registry
		ClusterRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "cluster", false, generalCollector.NewClusterCollector(client, logger)))
		ClusterRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "metro", true, generalCollector.NewMetroCollector(client, logger)))
		PortRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "port", true, generalCollector.NewPortCollector(client, logger)))
		HardwareRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "hardware", false, generalCollector.NewHardwareCollector(client, logger)))
		VolumeRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "volume", true, generalCollector.NewVolumeCollector(client, logger)))
		ApplianceRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "appliance", false, generalCollector.NewApplianceCollector(client, logger)))
		NasRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "nas", true, generalCollector.NewNasCollector(client, logger)))
		VolumeGroupRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "volume_group", true, generalCollector.NewVolumeGroupCollector(client, logger)))
		CapacityRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "capacity", false, generalCollector.NewCapacityCollector(client, logger, bc)))
		FileSystemRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "filesystem_capacity", true, generalCollector.NewFileCollector(client, bc, logger)))
		// Performance data
		ApplianceRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "appliance_performance", false, generalCollector.NewMetricApplianceCollector(client, bc, logger)))
		PortRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "fc_performance", true, generalCollector.NewMetricFcPortCollector(client, bc, logger)))
		PortRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "eth_performance", true, generalCollector.NewMetricEthPortCollector(client, bc, logger)))
		NasRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "nas_performance", true, generalCollector.NewMetricNasCollector(client, bc, logger)))
		FileSystemRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "filesystem_performance", true, generalCollector.NewMetricFilesystemCollector(client, bc, logger)))
		VolumeRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "volume_performance", true, generalCollector.NewMetricVolumeCollector(client, bc, logger)))
		VolumeGroupRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "volume_group_performance", true, generalCollector.NewMetricVgCollector(client, bc, logger)))
		HardwareRegistry.MustRegister(generalCollector.Monitored(storage.Ip, "drive_wear", true, generalCollector.NewWearMetricCollector(client, bc, logger)))

		BulkRegistry := prometheus.NewPedanticRegistry()
		BulkRegistry.MustRegister(bc)

		metricsGroup := r.Group(fmt.Sprintf("/metrics/%s", storage.Ip))
		{
			metricsGroup.GET("health", utils.PrometheusHandler(BulkRegistry, logger))
			metricsGroup.GET("cluster", utils.PrometheusHandler(ClusterRegistry, logger))
			metricsGroup.GET("port", utils.PrometheusHandler(PortRegistry, logger))
			metricsGroup.GET("file", utils.PrometheusHandler(FileSystemRegistry, logger))
			metricsGroup.GET("hardware", utils.PrometheusHandler(HardwareRegistry, logger))
			metricsGroup.GET("volume", utils.PrometheusHandler(VolumeRegistry, logger))
			metricsGroup.GET("appliance", utils.PrometheusHandler(ApplianceRegistry, logger))
			metricsGroup.GET("nas", utils.PrometheusHandler(NasRegistry, logger))
			metricsGroup.GET("volumeGroup", utils.PrometheusHandler(VolumeGroupRegistry, logger))
			metricsGroup.GET("capacity", utils.PrometheusHandler(CapacityRegistry, logger))
		}
		level.Info(logger).Log("msg", "The Powerstore is ready", "ip", storage.Ip)
	}

	// exporter Performance
	r.GET("/performance", func(context *gin.Context) {
		h := promhttp.Handler()
		h.ServeHTTP(context.Writer, context.Request)
	})

	httpPort := fmt.Sprintf(":%s", strconv.Itoa(config.Exporter.Port))
	level.Info(logger).Log("msg", "~~~~~~~~~~~~~Start PowerStore Exporter~~~~~~~~~~~~~~")
	level.Info(logger).Log("http-port", httpPort, "https", config.Exporter.Https.Enable)
	// Determine whether https is enabled
	if config.Exporter.Https.Enable {
		if config.Exporter.Https.CrtPath == "" || config.Exporter.Https.KeyPath == "" {
			level.Error(logger).Log("msg", "certificate missing", "crt", config.Exporter.Https.CrtPath, "key", config.Exporter.Https.KeyPath)
		}
		err := r.RunTLS(httpPort, config.Exporter.Https.CrtPath, config.Exporter.Https.KeyPath)
		if err != nil {
			level.Error(logger).Log("msg", "Service startup failed", "err", err)
		}
	} else {
		err := r.Run(httpPort)
		if err != nil {
			level.Error(logger).Log("msg", "Service startup failed", "err", err)
		}
	}
}

// Inventory is always loaded at startup. Scheduling a refresh is opt-in and is
// independent of bulk metric downloads and Zabbix's dependent discovery rules.
func inventoryRefreshJob(value string, refresh func()) (*cron.Cron, error) {
	if value == "" || value == "0" {
		return nil, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval < 0 {
		return nil, fmt.Errorf("inventoryRefresh must be 0 or a positive duration")
	}
	if interval == 0 {
		return nil, nil
	}
	if interval < time.Second {
		return nil, fmt.Errorf("inventoryRefresh must be at least one second")
	}
	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	if _, err := c.AddFunc("@every "+interval.String(), refresh); err != nil {
		return nil, err
	}
	return c, nil
}
