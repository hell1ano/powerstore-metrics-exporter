package client

import (
	"github.com/tidwall/gjson"
	"testing"
)

func TestInventoryReadiness(t *testing.T) {
	c := &Client{IP: "readiness-test"}
	defer delete(PowerstoreModuleID, c.IP)
	if c.InventoryReady() {
		t.Fatal("missing inventory ready")
	}
	snapshot := map[string]map[string]gjson.Result{}
	for _, module := range []string{"appliance", "volume", "volumegroup", "ethport", "fcport", "drive", "nas", "filesystem", "node", "fan", "power_supply", "battery"} {
		snapshot[module] = map[string]gjson.Result{}
	}
	PowerstoreModuleID[c.IP] = snapshot
	if c.InventoryReady() {
		t.Fatal("missing mandatory hardware ready")
	}
	snapshot["appliance"]["a1"] = gjson.Parse(`"appliance"`)
	snapshot["node"]["n1"] = gjson.Parse(`"node"`)
	if !c.InventoryReady() {
		t.Fatal("complete inventory not ready")
	}
	snapshot["fan"] = nil
	if c.InventoryReady() {
		t.Fatal("failed optional inventory ready")
	}
}
