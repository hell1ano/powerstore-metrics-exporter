package client

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/utils"
	"testing"
)

func TestSourceInventoryAndVolumeStatusQueries(t *testing.T) {
	utils.InitReqCounter(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volume" && r.URL.Path != "/volume_group" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "in.(Primary,Clone)" || r.URL.Query().Get("is_replication_destination") != "eq.false" {
			t.Error("source-only filters missing")
		}
		fmt.Fprint(w, `[{"id":"source","name":"local"},{"id":"clone","name":"local-clone"}]`)
	}))
	defer srv.Close()
	for _, version := range []string{"v1", "v3"} {
		c := &Client{version: version, baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
		for _, load := range []func() (string, error){c.GetVolume, c.GetVolumeId, c.GetVolumeGroupId} {
			data, err := load()
			if err != nil {
				t.Fatal(err)
			}
			if len(gjson.Parse(data).Array()) != 2 {
				t.Fatal("source/clone omitted")
			}
		}
	}
}

func TestVolumeInventoryApplianceIdentity(t *testing.T) {
	utils.InitReqCounter(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/volume" {
			if r.URL.Query().Get("select") != "id,name,appliance_id" {
				t.Error("appliance identity not requested")
			}
			fmt.Fprint(w, `[{"id":"v1","name":"volume-one","appliance_id":"A1"}]`)
		} else {
			fmt.Fprint(w, `[]`)
		}
	}))
	defer srv.Close()
	c := &Client{IP: "volume-appliance-test", baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	defer delete(PowerstoreModuleID, c.IP)
	c.InitModuleID(log.NewNopLogger())
	if ModuleIDs(c.IP)["volume_appliance"]["v1"].String() != "A1" {
		t.Fatal("missing appliance inventory")
	}
}

func TestVolumeStatusRefreshesSourceSelectionWithoutChangingOldSnapshot(t *testing.T) {
	c := &Client{IP: "role-change-test"}
	defer delete(PowerstoreModuleID, c.IP)
	if err := c.UpdateVolumeInventory(`[{"id":"old-source","name":"old","appliance_id":"A1"}]`); err != nil {
		t.Fatal(err)
	}
	old := ModuleIDs(c.IP)
	if err := c.UpdateVolumeInventory(`[{"id":"new-source","name":"new","appliance_id":"A2"}]`); err != nil {
		t.Fatal(err)
	}
	current := ModuleIDs(c.IP)
	if len(current["volume"]) != 1 || current["volume"]["new-source"].String() != "new" || old["volume"]["old-source"].String() != "old" {
		t.Fatal("source selection stale or old snapshot mutated")
	}
	if err := c.UpdateVolumeInventory(`[{"id":"invalid"}]`); err == nil {
		t.Fatal("invalid inventory accepted")
	}
	if ModuleIDs(c.IP)["volume"]["new-source"].String() != "new" {
		t.Fatal("failed update erased inventory")
	}
}
