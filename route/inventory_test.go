package route

import (
	"testing"
	"time"
)

func TestInventoryRefreshIsOptIn(t *testing.T) {
	for _, value := range []string{"", "0", "0s"} {
		job, err := inventoryRefreshJob(value, func() { t.Error("unexpected refresh") })
		if err != nil || job != nil {
			t.Fatalf("startup-only config scheduled refresh: %q", value)
		}
	}
	for _, value := range []string{"-1h", "invalid", "500ms"} {
		if _, err := inventoryRefreshJob(value, func() {}); err == nil {
			t.Errorf("accepted invalid interval %q", value)
		}
	}
	job, err := inventoryRefreshJob("24h", func() {})
	if err != nil {
		t.Fatal(err)
	}
	entries := job.Entries()
	if len(entries) != 1 {
		t.Fatal("expected one optional refresh job")
	}
	now := time.Now().Truncate(time.Second)
	if next := entries[0].Schedule.Next(now); next.Sub(now) != 24*time.Hour {
		t.Fatal("wrong refresh cadence")
	}
}
