package generalCollector

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var collectionFailure = prometheus.NewDesc("powerstore_collection_error", "Internal collector error.", nil, nil)

func reportCollectionError(ch chan<- prometheus.Metric, err error) {
	ch <- prometheus.NewInvalidMetric(collectionFailure, err)
}

// Monitored adds health to the same response as the data. Errors from a partial
// collection cannot be masked by a successful HTTP response or another collector.
func Monitored(ip, name string, allowEmpty bool, inner prometheus.Collector) prometheus.Collector {
	labels := prometheus.Labels{"IP": ip, "collector": name}
	return &monitoredCollector{
		inner: inner, allowEmpty: allowEmpty,
		success:         prometheus.NewDesc("powerstore_collector_success", "Whether this collection completed without errors.", nil, labels),
		lastSuccessDesc: prometheus.NewDesc("powerstore_collector_last_success_timestamp_seconds", "Unix time of last successful collection; this is not source sample time.", nil, labels),
		samples:         prometheus.NewDesc("powerstore_collector_samples", "Number of data samples emitted during this collection.", nil, labels),
	}
}

type monitoredCollector struct {
	inner                             prometheus.Collector
	allowEmpty                        bool
	mu                                sync.Mutex
	lastSuccess                       time.Time
	success, lastSuccessDesc, samples *prometheus.Desc
}

func (c *monitoredCollector) Describe(ch chan<- *prometheus.Desc) {
	c.inner.Describe(ch)
	ch <- c.success
	ch <- c.lastSuccessDesc
	ch <- c.samples
}

func (c *monitoredCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data := make(chan prometheus.Metric)
	go func() {
		defer close(data)
		defer func() {
			if failure := recover(); failure != nil {
				reportCollectionError(data, fmt.Errorf("collector panic: %v", failure))
			}
		}()
		c.inner.Collect(data)
	}()
	success, samples := 1.0, 0.0
	for metric := range data {
		// Invalid metrics are converted to an explicit health value, not silently dropped.
		if err := metric.Write(&dto.Metric{}); err != nil {
			success = 0
			continue
		}
		samples++
		ch <- metric
	}
	if samples == 0 && !c.allowEmpty {
		success = 0
	}
	if success == 1 {
		c.lastSuccess = time.Now()
	}
	last := 0.0
	if !c.lastSuccess.IsZero() {
		last = float64(c.lastSuccess.Unix())
	}
	ch <- prometheus.MustNewConstMetric(c.success, prometheus.GaugeValue, success)
	ch <- prometheus.MustNewConstMetric(c.samples, prometheus.GaugeValue, samples)
	ch <- prometheus.MustNewConstMetric(c.lastSuccessDesc, prometheus.GaugeValue, last)
}
