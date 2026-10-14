package client

import (
	"errors"
	"github.com/go-kit/log"
	"powerstore-metrics-exporter/utils"
	"sync"
	"testing"
	"time"
)

func TestNASCacheSharesConcurrentRefresh(t *testing.T) {
	s := responseCache{}
	calls := 0
	now := time.Now()
	fetch := func() (string, error) { calls++; return "fresh", nil }
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err, _ := s.get(time.Minute, func() time.Time { return now }, fetch)
			if data != "fresh" || err != nil {
				t.Error("bad response")
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("got %d refreshes", calls)
	}
	now = now.Add(time.Minute)
	s.get(time.Minute, func() time.Time { return now }, fetch)
	if calls != 2 {
		t.Fatal("expired cache did not refresh")
	}
}
func TestNASCacheFailureNeverServesOldDataAndRecovers(t *testing.T) {
	s := responseCache{}
	now := time.Now()
	clock := func() time.Time { return now }
	calls := 0
	failed := false
	fetch := func() (string, error) {
		calls++
		if failed {
			return "partial", errors.New("offline")
		}
		return "fresh", nil
	}
	s.get(time.Minute, clock, fetch)
	failed = true
	now = now.Add(time.Minute)
	for i := 0; i < 5; i++ {
		data, err, _ := s.get(time.Minute, clock, fetch)
		if data != "" || err == nil {
			t.Fatal("failure hidden by cached data")
		}
	}
	if calls != 2 {
		t.Fatal("failed refresh caused retry storm")
	}
	failed = false
	now = now.Add(time.Minute)
	data, err, _ := s.get(time.Minute, clock, fetch)
	if data != "fresh" || err != nil || calls != 3 {
		t.Fatal("cache did not recover")
	}
}
func TestNASCacheConfiguration(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
		fail  bool
	}{{"", time.Minute, false}, {"60s", time.Minute, false}, {"2m", 2 * time.Minute, false}, {"0", 0, true}, {"-1s", 0, true}, {"bad", 0, true}, {"1ms", 0, true}} {
		c, err := NewConfiguredClient(utils.Storage{Ip: "synthetic", User: "test", Password: "test", Version: "v3", NASStatusCacheTTL: tc.value}, log.NewNopLogger())
		if (err != nil) != tc.fail {
			t.Fatalf("%q: %v", tc.value, err)
		}
		if err == nil && c.nasStatusTTL != tc.want {
			t.Fatal("incorrect interval")
		}
	}
}
