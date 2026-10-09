package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"powerstore-metrics-exporter/utils"
)

func TestCapacityUsesConfiguredInterval(t *testing.T) {
	utils.InitReqCounter(1)
	for _, interval := range []string{"", "Five_Mins", "One_Hour", "One_Day"} {
		want := interval
		if want == "" {
			want = "Five_Mins"
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body RequestBody
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Interval != want || body.Entity != "space_metrics_by_appliance" {
				t.Error("wrong capacity request")
			}
			_, _ = w.Write([]byte(`[]`))
		}))
		c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), capacityInterval: interval, logger: log.NewNopLogger()}
		if _, err := c.GetCap("a1"); err != nil {
			t.Fatal(err)
		}
		srv.Close()
	}
}

func TestRejectInvalidCollectionResponses(t *testing.T) {
	utils.InitReqCounter(1)
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`[{"id":"one"}]`, true}, {`[]`, true}, {`{"error":"unavailable"}`, false}, {`invalid`, false}, {`null`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer server.Close()
			c := &Client{baseUrl: server.URL + "/", http: server.Client(), logger: log.NewNopLogger()}
			_, err := c.GetCluster()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
		})
	}
}
