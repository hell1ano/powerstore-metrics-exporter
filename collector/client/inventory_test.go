package client

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/utils"
	"sync"
	"testing"
)

func TestPagination(t *testing.T) {
	utils.InitReqCounter(2)
	for _, tc := range []struct {
		name       string
		finalCode  int
		finalRange string
		fail       bool
	}{
		{"partial final", 206, "2-2/3", false}, {"OK final", 200, "", false},
		{"failed page", 500, "", true}, {"repeated range", 206, "0-0/3", true},
		{"changed total", 206, "2-2/4", true}, {"missing range", 206, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("order") != "id" || r.URL.Query().Get("limit") != "2000" {
					t.Error("missing bounded stable ordering")
				}
				if r.URL.Query().Get("offset") == "0" {
					w.Header().Set("Content-Range", "0-1/3")
					w.WriteHeader(206)
					fmt.Fprint(w, `[{"id":"a"},{"id":"b"}]`)
					return
				}
				if r.URL.Query().Get("offset") != "2" {
					t.Error("wrong offset")
				}
				w.Header().Set("Content-Range", tc.finalRange)
				w.WriteHeader(tc.finalCode)
				fmt.Fprint(w, `[{"id":"c"}]`)
			}))
			defer srv.Close()
			c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger(), limit: 5000}
			data, err := c.GetVolume()
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected result: %v", err)
			}
			if !tc.fail && len(gjson.Parse(data).Array()) != 3 {
				t.Fatal("partial inventory")
			}
			if calls != 2 {
				t.Fatalf("unexpected page count %d", calls)
			}
		})
	}
}

func TestInventoryRefreshReplacesSnapshotAndReportsFailures(t *testing.T) {
	utils.InitReqCounter(2)
	var mu sync.Mutex
	name := "old"
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			w.WriteHeader(500)
			return
		}
		fmt.Fprintf(w, `[{"id":"id1","name":%q}]`, name)
	}))
	defer srv.Close()
	c := &Client{IP: "inventory-test", baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	defer delete(PowerstoreModuleID, c.IP)
	c.InitModuleID(c.logger)
	old := ModuleIDs(c.IP)
	mu.Lock()
	name = "renamed"
	mu.Unlock()
	c.InitModuleID(c.logger)
	if old["volume"]["id1"].String() != "old" || ModuleIDs(c.IP)["volume"]["id1"].String() != "renamed" {
		t.Fatal("snapshot mutated or not refreshed")
	}
	mu.Lock()
	fail = true
	mu.Unlock()
	c.InitModuleID(c.logger)
	if ModuleIDs(c.IP)["volume"] != nil {
		t.Fatal("failed inventory looks valid")
	}
	mu.Lock()
	fail = false
	mu.Unlock()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			c.InitModuleID(c.logger)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = ModuleIDs(c.IP)["volume"]
		}
	}()
	wg.Wait()
}

func TestAuthenticationRetryIsBounded(t *testing.T) {
	utils.InitReqCounter(1)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/login_session" {
			fmt.Fprint(w, `[]`)
			return
		}
		w.WriteHeader(401)
	}))
	defer srv.Close()
	c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	if _, err := c.GetVolume(); err == nil {
		t.Fatal("accepted failed authentication")
	}
	if requests != 3 {
		t.Fatalf("authentication retried %d requests", requests)
	}
}

func TestRetryOnlyFailedInventoryClasses(t *testing.T) {
	utils.InitReqCounter(2)
	fail := true
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		if r.URL.Path == "/file_system" {
			if fail {
				w.WriteHeader(503)
				return
			}
			fmt.Fprint(w, `[{"id":"fs1","name":"filesystem","nas_server_id":"nas1"}]`)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	c := &Client{IP: "retry-test", baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	defer delete(PowerstoreModuleID, c.IP)
	c.InitModuleID(c.logger)
	before := ModuleIDs(c.IP)
	if before["filesystem"] != nil || before["volume"] == nil {
		t.Fatal("incorrect failure/empty distinction")
	}
	fail = false
	c.RetryMissingInventory(c.logger)
	recovered := ModuleIDs(c.IP)
	if recovered["filesystem"]["fs1"].String() != "filesystem" || recovered["filesystem_nas"]["fs1"].String() != "nas1" {
		t.Fatal("inventory did not recover")
	}
	if before["filesystem"] != nil {
		t.Fatal("published snapshot mutated")
	}
	c.RetryMissingInventory(c.logger)
	for path, count := range calls {
		want := 1
		if path == "/file_system" {
			want = 2
		}
		if count != want {
			t.Errorf("%s called %d times, want %d", path, count, want)
		}
	}
}
