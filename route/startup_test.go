package route

import (
	"github.com/go-kit/log"
	"net"
	"powerstore-metrics-exporter/utils"
	"strings"
	"testing"
)

func TestStartupRejectsInvalidConfiguration(t *testing.T) {
	for _, change := range []func(*utils.Config){
		func(c *utils.Config) { c.Exporter.Port = 0 },
		func(c *utils.Config) { c.StorageList = nil },
		func(c *utils.Config) { c.Exporter.BulkMaxAge = "invalid" },
		func(c *utils.Config) { c.Exporter.BulkCron = "invalid" },
		func(c *utils.Config) { c.Exporter.InventoryRefresh = "invalid" },
		func(c *utils.Config) { c.Exporter.Https.Enable = true },
		func(c *utils.Config) { c.StorageList = append(c.StorageList, c.StorageList[0]) },
	} {
		c := &utils.Config{Exporter: utils.Exporter{Port: 9010}, StorageList: []utils.Storage{{Ip: "synthetic", User: "test", Password: "test", Version: "v3"}}}
		change(c)
		if err := Run(c, log.NewNopLogger()); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}

func TestListenerFailureReturnsError(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	c := &utils.Config{Exporter: utils.Exporter{Port: listener.Addr().(*net.TCPAddr).Port}, StorageList: []utils.Storage{{Ip: "127.0.0.1:1", User: "test", Password: "test", Version: "v3"}}}
	if err := Run(c, log.NewNopLogger()); err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("listener error not propagated: %v", err)
	}
}
