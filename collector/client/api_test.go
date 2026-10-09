package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"powerstore-metrics-exporter/utils"
)

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
