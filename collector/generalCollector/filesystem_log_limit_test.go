package generalCollector

import (
	"bytes"
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"testing"
	"time"
)

func TestFilesystemFailureLogsDoNotHideHealth(t *testing.T) {
	gate := &filesystemFailureLog{}
	var output bytes.Buffer
	logger := log.NewLogfmtLogger(&output)
	ch := make(chan prometheus.Metric, 2)
	reportFilesystemError(ch, logger, "array", "filesystem_capacity", "", nil, fmt.Errorf("missing sample"), gate)
	size := output.Len()
	reportFilesystemError(ch, logger, "array", "filesystem_capacity", "", nil, fmt.Errorf("missing sample"), gate)
	if output.Len() != size || len(ch) != 2 {
		t.Fatal("repeat logged or failure health suppressed")
	}
	gate.reset()
	ch = make(chan prometheus.Metric, 1)
	reportFilesystemError(ch, logger, "array", "filesystem_capacity", "", nil, fmt.Errorf("missing sample"), gate)
	if output.Len() == size {
		t.Fatal("failure after recovery not logged")
	}
}
func TestFilesystemFailureLogReminder(t *testing.T) {
	gate := &filesystemFailureLog{}
	now := time.Now()
	if !gate.allow("a", now) || gate.allow("a", now.Add(time.Minute)) || !gate.allow("a", now.Add(5*time.Minute)) || !gate.allow("b", now.Add(6*time.Minute)) {
		t.Fatal("incorrect repeat/change logging")
	}
}
