package utils

import (
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"net/http/httptest"
	"testing"
)

type repeatedMetric struct{ desc *prometheus.Desc }

func (c repeatedMetric) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }
func (c repeatedMetric) Collect(ch chan<- prometheus.Metric) {
	for i := 0; i < 2; i++ {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1)
	}
}

func TestExpositionErrorsAreNotHTTPSuccess(t *testing.T) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(repeatedMetric{prometheus.NewDesc("duplicate", "test", nil, nil)})
	router := gin.New()
	router.GET("/metrics", PrometheusHandler(registry, log.NewNopLogger()))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 500 {
		t.Fatalf("registry error returned HTTP %d", w.Code)
	}
}
