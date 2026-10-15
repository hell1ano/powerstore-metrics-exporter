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
	labels := []string{name, id, nasID, nasName}
	if name == "" {
		labels[0] = id
	}
	if nasName == "" {
		labels[3] = "unknown"
		if nasID != "" {
			labels[3] = nasID
		}
	}
	if _, exists := inventory["filesystem"][id]; !exists {
		return labels, fmt.Errorf("filesystem ID absent from inventory; displaying available data by ID")
	}
	if name == "" {
		return labels, fmt.Errorf("filesystem inventory name is empty; displaying ID")
	}
	if nasID == "" {
		return labels, fmt.Errorf("filesystem inventory has no nas_server_id")
	}
	if inventory["nas"] == nil {
		return labels, fmt.Errorf("NAS inventory unavailable")
	}
	if _, exists := inventory["nas"][nasID]; !exists {
		return labels, fmt.Errorf("NAS ID absent from NAS inventory")
	}
	if nasName == "" {
		return labels, fmt.Errorf("NAS inventory name is empty")
	}
	return labels, nil
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
	last map[string]time.Time
}

func (s *filesystemFailureLog) allow(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = make(map[string]time.Time)
	}
	if last, ok := s.last[key]; ok && now.Sub(last) < 5*time.Minute {
		return false
	}
	// Bound memory while retaining independent suppression for different objects.
	if len(s.last) >= 4096 {
		oldestKey := ""
		oldest := now
		for k, t := range s.last {
			if !t.After(oldest) {
				oldestKey, oldest = k, t
			}
		}
		delete(s.last, oldestKey)
	}
	s.last[key] = now
	return true
}
func (s *filesystemFailureLog) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = nil
}
