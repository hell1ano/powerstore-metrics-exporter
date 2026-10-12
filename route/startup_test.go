package route

import (
	"github.com/go-kit/log"
	"powerstore-metrics-exporter/utils"
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
