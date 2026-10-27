package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-kit/log"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/utils"
)

func TestReplicationSharesMetroRequestAndIncludesAllRoles(t *testing.T) {
	utils.InitReqCounter(4)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/replication_session" || r.URL.Query().Get("type") != "" || r.URL.Query().Get("role") != "" {
			t.Errorf("filtered or unexpected request: %s", r.URL)
		}
		if r.URL.Query().Get("select") != "*,replication_rule(id,name,rpo,alert_threshold)" {
			t.Errorf("rule not embedded: %s", r.URL)
		}
		if r.URL.Query().Get("offset") == "0" {
			w.Header().Set("Content-Range", "0-1/3")
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, `[{"id":"a","type":"Asynchronous","role":"Destination"},{"id":"b","type":"Synchronous","role":"Source"}]`)
		} else {
			w.Header().Set("Content-Range", "2-2/3")
			fmt.Fprint(w, `[{"id":"c","type":"Metro_Active_Active","role":"Metro_Non_Preferred"}]`)
		}
	}))
	defer srv.Close()
	c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := c.GetReplicationSessions()
			if err != nil || len(gjson.Parse(data).Array()) != 3 {
				t.Errorf("sessions: %s, %v", data, err)
			}
		}()
	}
	wg.Wait()
	metro, err := c.GetMetro()
	if err != nil || len(gjson.Parse(metro).Array()) != 1 || gjson.Get(metro, "0.id").String() != "c" {
		t.Fatalf("metro: %s, %v", metro, err)
	}
	if requests.Load() != 2 {
		t.Fatalf("expected one two-page fetch; got %d requests", requests.Load())
	}
}

func TestReplicationCachesErrorsWithoutFallbackRequests(t *testing.T) {
	utils.InitReqCounter(2)
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(503) }))
	defer srv.Close()
	c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	for _, fetch := range []func() (string, error){c.GetReplicationSessions, c.GetMetro, c.GetReplicationSessions} {
		if data, err := fetch(); err == nil || data != "" {
			t.Fatal("failed request was not preserved")
		}
	}
	if requests != 1 {
		t.Fatalf("unexpected retry traffic: %d", requests)
	}
}
