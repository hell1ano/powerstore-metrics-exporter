package generalCollector

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

type fixtureCollector struct{ fail, empty, panicNow bool }

var fixtureDesc = prometheus.NewDesc("fixture_value", "Fixture.", nil, nil)

func (f *fixtureCollector) Describe(ch chan<- *prometheus.Desc) { ch <- fixtureDesc }
func (f *fixtureCollector) Collect(ch chan<- prometheus.Metric) {
	if f.panicNow {
		panic("bad API response")
	}
	if !f.empty {
		ch <- prometheus.MustNewConstMetric(fixtureDesc, prometheus.GaugeValue, 42)
	}
	if f.fail {
		reportCollectionError(ch, errors.New("partial API failure"))
	}
}

func TestCollectionHealth(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fixture    fixtureCollector
		allowEmpty bool
		want       float64
	}{
		{"success", fixtureCollector{}, false, 1},
		{"partial failure", fixtureCollector{fail: true}, false, 0},
		{"empty required", fixtureCollector{empty: true}, false, 0},
		{"empty optional", fixtureCollector{empty: true}, true, 1},
		{"panic", fixtureCollector{panicNow: true}, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := prometheus.NewPedanticRegistry()
			registry.MustRegister(Monitored("array", "fixture", tc.allowEmpty, &tc.fixture))
			metrics, err := registry.Gather()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, m := range metrics {
				if m.GetName() == "powerstore_collector_success" {
					found = true
					if m.Metric[0].Gauge.GetValue() != tc.want {
						t.Fatal("wrong health")
					}
				}
			}
			if !found {
				t.Fatal("missing health metric")
			}
		})
	}
}

func TestLastSuccessSurvivesFailureAndRecovers(t *testing.T) {
	fixture := &fixtureCollector{}
	wrapped := Monitored("array", "fixture", false, fixture).(*monitoredCollector)
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(wrapped)
	if _, err := registry.Gather(); err != nil {
		t.Fatal(err)
	}
	last := wrapped.lastSuccess
	fixture.fail = true
	if _, err := registry.Gather(); err != nil {
		t.Fatal(err)
	}
	if !wrapped.lastSuccess.Equal(last) {
		t.Fatal("failure refreshed last success")
	}
	fixture.fail = false
	if _, err := registry.Gather(); err != nil {
		t.Fatal(err)
	}
	if wrapped.lastSuccess.Before(last) {
		t.Fatal("invalid recovery time")
	}
}
