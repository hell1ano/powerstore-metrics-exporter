package generalCollector

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
	"sync"
	"time"
)

// Resolve all labels from the same inventory snapshot. Names are for display;
// stable filesystem IDs distinguish equal names on different NAS servers.
func filesystemLabels(inventory map[string]map[string]gjson.Result, id string) ([]string, error) {
	name := inventory["filesystem"][id].String()
	nasID := inventory["filesystem_nas"][id].String()
	nasName := inventory["nas"][nasID].String()
	if id == "" {
		return nil, fmt.Errorf("sample has no file_system_id")
	}
	if _, exists := inventory["filesystem"][id]; !exists {
		return nil, fmt.Errorf("filesystem ID absent from inventory")
	}
	if name == "" {
		return nil, fmt.Errorf("filesystem inventory name is empty")
	}
	if nasID == "" {
		return nil, fmt.Errorf("filesystem inventory has no nas_server_id")
	}
	if inventory["nas"] == nil {
		return nil, fmt.Errorf("NAS inventory unavailable")
	}
	if _, exists := inventory["nas"][nasID]; !exists {
		return nil, fmt.Errorf("NAS ID absent from NAS inventory")
	}
	if nasName == "" {
		return nil, fmt.Errorf("NAS inventory name is empty")
	}
	return []string{name, id, nasID, nasName}, nil
}

// Log local troubleshooting context as well as marking collection unsuccessful.
// Do not log credentials or full API responses. Bulk coverage errors include
// a bounded sample of affected IDs when there is no single object for the failure.
func reportFilesystemError(ch chan<- prometheus.Metric, logger log.Logger, ip, collector, id string, inventory map[string]map[string]gjson.Result, err error, gate ...*filesystemFailureLog) {
	reportCollectionError(ch, err)
	if len(gate) > 0 && !gate[0].allow(id+":"+err.Error(), time.Now()) {
		return
	}
	level.Warn(logger).Log("msg", "filesystem collection failed", "ip", ip, "collector", collector,
		"file_system_id", id, "filesystem_name", inventory["filesystem"][id].String(),
		"nas_server_id", inventory["filesystem_nas"][id].String(), "err", err)
}

// Health fails on every scrape. Only identical repeated log details are throttled.
type filesystemFailureLog struct {
	mu   sync.Mutex
	key  string
	last time.Time
}

func (s *filesystemFailureLog) allow(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.key == key && now.Sub(s.last) < 5*time.Minute {
		return false
	}
	s.key = key
	s.last = now
	return true
}
func (s *filesystemFailureLog) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = ""
	s.last = time.Time{}
}
