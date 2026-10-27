package generalCollector

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/collector/client"
)

func coverageDescriptors(ip, collector string) (*prometheus.Desc, *prometheus.Desc) {
	labels := prometheus.Labels{"IP": ip, "collector": collector}
	variables := []string{"object_id", "name", "nas_server_id"}
	return prometheus.NewDesc("powerstore_object_sample_available", "Whether a supported numeric sample was returned, including measured zero.", variables, labels),
		prometheus.NewDesc("powerstore_object_missing_expected", "Whether an absent sample is explicitly allowed by configuration. Does not suppress request or archive errors.", variables, labels)
}

func describeCoverage(ch chan<- *prometheus.Desc, ip, collector string) {
	available, expected := coverageDescriptors(ip, collector)
	ch <- available
	ch <- expected
}

// Diagnostic gauges do not count as collected data samples.
type collectionDiagnostic struct{ prometheus.Metric }

type sampleCoverage struct {
	ch            chan<- prometheus.Metric
	api           *client.Client
	collector     string
	names, owners map[string]gjson.Result
}

func (c *sampleCoverage) emit(id string, available, expected float64) {
	if c == nil {
		return
	}
	sampleAvailable, missingExpected := coverageDescriptors(c.api.IP, c.collector)
	labels := []string{id, c.names[id].String(), c.owners[id].String()}
	c.ch <- collectionDiagnostic{prometheus.MustNewConstMetric(sampleAvailable, prometheus.GaugeValue, available, labels...)}
	c.ch <- collectionDiagnostic{prometheus.MustNewConstMetric(missingExpected, prometheus.GaugeValue, expected, labels...)}
}
func (c *sampleCoverage) present(id string) { c.emit(id, 1, 0) }
func (c *sampleCoverage) missing(id string) bool {
	allowed := c != nil && c.api.MissingMetricsExpected(c.collector, id)
	expected := 0.0
	if allowed {
		expected = 1
	}
	c.emit(id, 0, expected)
	return allowed
}
func firstCoverage(coverage []*sampleCoverage) *sampleCoverage {
	if len(coverage) == 0 {
		return nil
	}
	return coverage[0]
}
