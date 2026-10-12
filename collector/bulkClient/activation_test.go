package bulkClient

import (
	"errors"
	"testing"
	"time"
)

func TestActivationBackoffAndRecovery(t *testing.T) {
	now := time.Now()
	s := bulkActivation{}
	calls := 0
	fail := true
	enable := func() error {
		calls++
		if fail {
			return errors.New("unavailable")
		}
		return nil
	}
	for _, delay := range []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute} {
		ready, err := s.advance(now, enable)
		if ready || err == nil {
			t.Fatal("failed activation accepted")
		}
		n := calls
		s.advance(now.Add(delay-time.Second), enable)
		if calls != n {
			t.Fatal("retried before backoff elapsed")
		}
		now = now.Add(delay)
	}
	fail = false
	if ready, err := s.advance(now, enable); ready || err != nil {
		t.Fatal("warmup not respected")
	}
	n := calls
	if ready, err := s.advance(now.Add(time.Minute), enable); !ready || err != nil {
		t.Fatal("activation did not recover")
	}
	s.advance(now.Add(time.Hour), enable)
	if calls != n {
		t.Fatal("successful activation repeated")
	}
}
