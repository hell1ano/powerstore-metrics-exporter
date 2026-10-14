package client

import (
	"fmt"
	"github.com/go-kit/log"
	"net/http"
	"net/http/httptest"
	"powerstore-metrics-exporter/utils"
	"testing"
)

func TestFilesystemInventoryFiltersSnapshotsAndKeepsOwnership(t *testing.T) {
	utils.InitReqCounter(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/file_system" {
			fmt.Fprint(w, `[]`)
			return
		}
		if r.URL.Query().Get("filesystem_type") != "eq.Primary" {
			t.Error("filesystem query must exclude snapshots")
			fmt.Fprint(w, `[{"id":"snapshot","name":"snapshot","nas_server_id":"nas1"}]`)
			return
		}
		if r.URL.Query().Get("select") != "id,name,nas_server_id" {
			t.Error("ownership fields missing")
		}
		fmt.Fprint(w, `[{"id":"primary","name":"data","nas_server_id":"nas1"},{"id":"clone","name":"clone","nas_server_id":"nas1"}]`)
	}))
	defer srv.Close()
	c := &Client{IP: "synthetic", baseUrl: srv.URL + "/", http: srv.Client(), logger: log.NewNopLogger()}
	defer delete(PowerstoreModuleID, c.IP)
	c.InitModuleID(c.logger)
	inventory := ModuleIDs(c.IP)
	if len(inventory["filesystem"]) != 2 || inventory["filesystem"]["snapshot"].Exists() {
		t.Fatal("incorrect filesystem inventory")
	}
	for _, id := range []string{"primary", "clone"} {
		if inventory["filesystem_nas"][id].String() != "nas1" {
			t.Fatal("ownership lost")
		}
	}
}
