package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/go-kit/log"
	"github.com/tidwall/gjson"
	"powerstore-metrics-exporter/utils"
)

// Exercise every public collection method, including cached and filtered paths.
// The array deliberately returns fewer rows than requested: use its range, not
// the requested limit, to find the next page. Query filters must survive paging.
func TestAllRESTCollectionMethodsFollowServerPagination(t *testing.T) {
	utils.InitReqCounter(2)
	methods := []struct {
		name  string
		fetch func(*Client) (string, error)
	}{
		{"cluster", (*Client).GetCluster},
		{"ethernet", func(c *Client) (string, error) { return c.GetPort("eth_port") }},
		{"fc", func(c *Client) (string, error) { return c.GetPort("fc_port") }},
		{"hardware", func(c *Client) (string, error) { return c.GetHardware("Node") }},
		{"volume", (*Client).GetVolume}, {"appliance", (*Client).GetAppliance},
		{"nas", (*Client).GetNas}, {"nas detail", (*Client).GetNasDetail},
		{"volume group", (*Client).GetVolumeGroup}, {"metro", (*Client).GetMetro},
		{"sessions", (*Client).GetReplicationSessions}, {"witness", (*Client).GetWitness},
		{"appliance inventory", (*Client).GetApplianceId}, {"volume inventory", (*Client).GetVolumeId},
		{"group inventory", (*Client).GetVolumeGroupId}, {"ethernet inventory", (*Client).GetEthPortId},
		{"fc inventory", (*Client).GetFcPortId}, {"drive inventory", (*Client).GetDrivesId},
		{"nas inventory", (*Client).GetNasId}, {"filesystem inventory", (*Client).GetFilesystemId},
		{"hardware inventory", func(c *Client) (string, error) { return c.GetHardwareIDs("Battery") }},
	}
	for _, tc := range methods {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var firstQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				q := r.URL.Query()
				if r.Method != "GET" || q.Get("limit") != "2000" || q.Get("order") != "id" {
					t.Errorf("bad request %s %s", r.Method, r.URL)
				}
				offset, _ := strconv.Atoi(q.Get("offset"))
				q.Del("offset")
				if calls == 1 {
					firstQuery = q.Encode()
				} else if q.Encode() != firstQuery {
					t.Error("filters changed across pages")
				}
				if offset == 0 {
					w.Header().Set("Content-Range", "0-99/196")
					w.WriteHeader(206)
				} else if offset == 100 {
					w.Header().Set("Content-Range", "100-195/196")
				} else {
					t.Errorf("bad offset %d", offset)
					w.WriteHeader(500)
					return
				}
				end := offset + 100
				if end > 196 {
					end = 196
				}
				rows := []map[string]string{}
				for i := offset; i < end; i++ {
					rows = append(rows, map[string]string{"id": fmt.Sprintf("id-%04d", i), "type": "Metro_Active_Active"})
				}
				json.NewEncoder(w).Encode(rows)
			}))
			defer srv.Close()
			c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
			data, err := tc.fetch(c)
			if err != nil || len(gjson.Parse(data).Array()) != 196 || calls != 2 {
				t.Fatalf("rows=%d calls=%d err=%v", len(gjson.Parse(data).Array()), calls, err)
			}
		})
	}
}

func TestCollectionLimitsAndMoreThanTwoThousandObjects(t *testing.T) {
	utils.InitReqCounter(2)
	for _, configured := range []int{0, 1, 100, 2000, 5000} {
		t.Run(strconv.Itoa(configured), func(t *testing.T) {
			want := configured
			if want == 0 || want > 2000 {
				want = 2000
			}
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("limit") != strconv.Itoa(want) {
					t.Error("limit not normalized")
				}
				offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
				end := offset + want
				if end > 2205 {
					end = 2205
				}
				w.Header().Set("Content-Range", fmt.Sprintf("%d-%d/2205", offset, end-1))
				if end < 2205 {
					w.WriteHeader(206)
				}
				rows := []map[string]string{}
				for i := offset; i < end; i++ {
					rows = append(rows, map[string]string{"id": fmt.Sprintf("id-%05d", i)})
				}
				json.NewEncoder(w).Encode(rows)
			}))
			defer srv.Close()
			c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger(), limit: configured}
			data, err := c.GetVolume()
			if err != nil || len(gjson.Parse(data).Array()) != 2205 || calls != (2205+want-1)/want {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestMalformedOrOverlappingPaginationNeverPublishesPartialData(t *testing.T) {
	utils.InitReqCounter(2)
	for _, tc := range []struct {
		name, header, body string
		status             int
	}{
		{"overlap", "2-2/3", `[{"id":"b"}]`, 206},
		{"overlap headerless final", "", `[{"id":"b"}]`, 200},
		{"missing ID", "2-2/3", `[{}]`, 206},
		{"wrong ID type", "2-2/3", `[{"id":42}]`, 206},
		{"trailing range garbage", "2-2/3garbage", `[{"id":"c"}]`, 206},
		{"range overflow", "2-2/9999999999999999999999", `[{"id":"c"}]`, 206},
		{"short final", "", `[]`, 200},
		{"long final", "", `[{"id":"c"},{"id":"d"}]`, 200},
		{"body not array", "2-2/3", `{"id":"c"}`, 206},
		{"range count mismatch", "2-2/3", `[]`, 206},
		{"missing range", "", `[{"id":"c"}]`, 206},
		{"later error", "", `[]`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("offset") == "0" {
					w.Header().Set("Content-Range", "0-1/3")
					w.WriteHeader(206)
					fmt.Fprint(w, `[{"id":"a"},{"id":"b"}]`)
					return
				}
				w.Header().Set("Content-Range", tc.header)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
			if data, err := c.GetVolume(); err == nil || data != "" {
				t.Fatalf("partial data accepted: %s %v", data, err)
			}
		})
	}
}

func TestAllGenerateMethodsRejectPartialResponsesWithoutGETPaging(t *testing.T) {
	utils.InitReqCounter(2)
	methods := []func(*Client, string) (string, error){(*Client).GetPerf, (*Client).GetCap, (*Client).GetMetricVg, (*Client).GetMetricVolume, (*Client).GetMetricFcPort, (*Client).GetMetricEthPort, (*Client).GetMetricAppliance, (*Client).GetWearMetricByDrive, (*Client).GetMetricByNas, (*Client).GetFilesystemCap, (*Client).GetMetricsFilesystem}
	for i, fetch := range methods {
		for _, partial := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/partial=%v", i, partial), func(t *testing.T) {
				calls := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != "POST" || r.URL.Path != "/metrics/generate" || len(r.URL.Query()) != 0 {
						t.Errorf("unexpected generate query: %s %s", r.Method, r.URL)
					}
					var body RequestBody
					json.NewDecoder(r.Body).Decode(&body)
					if body.EntityID != "object" || body.Entity == "" {
						t.Error("lost generate entity")
					}
					if partial {
						w.Header().Set("Content-Range", "0-0/2")
						w.WriteHeader(206)
					}
					fmt.Fprint(w, `[{"timestamp":"2026-10-27T12:00:00Z","value":1}]`)
				}))
				defer srv.Close()
				c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
				data, err := fetch(c, "object")
				if (err != nil) != partial || calls != 1 || (partial && data != "") {
					t.Fatalf("partial=%v calls=%d data=%s err=%v", partial, calls, data, err)
				}
			})
		}
	}
}

func TestAuthenticationRetryPreservesSecondPage(t *testing.T) {
	utils.InitReqCounter(2)
	secondAttempts := 0
	logins := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login_session" {
			logins++
			fmt.Fprint(w, `[]`)
			return
		}
		q, _ := url.ParseQuery(r.URL.RawQuery)
		if q.Get("offset") == "0" {
			w.Header().Set("Content-Range", "0-0/2")
			w.WriteHeader(206)
			fmt.Fprint(w, `[{"id":"a"}]`)
			return
		}
		if q.Get("offset") != "1" {
			t.Error("lost offset during retry")
		}
		secondAttempts++
		if secondAttempts == 1 {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `[{"id":"b"}]`)
	}))
	defer srv.Close()
	c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	data, err := c.GetVolume()
	if err != nil || len(gjson.Parse(data).Array()) != 2 || logins != 1 || secondAttempts != 2 {
		t.Fatalf("retry lost page: %s %v", data, err)
	}
}

func TestLoginDoesNotUseCollectionPagination(t *testing.T) {
	for _, code := range []int{200, 206} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/login_session" || len(r.URL.Query()) != 0 {
					t.Errorf("unexpected login query %s", r.URL)
				}
				w.WriteHeader(code)
			}))
			defer srv.Close()
			c := &Client{baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
			if err := c.InitLogin(); (err == nil) != (code == 200) || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}
