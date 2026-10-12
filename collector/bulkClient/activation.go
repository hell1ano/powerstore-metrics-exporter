package bulkClient

import (
	"time"
)

// Activation retries have a capped delay. Success allows one minute for the
// array to prepare its first archive; download errors never trigger activation.
type bulkActivation struct {
	enabled     bool
	readyAt     time.Time
	nextAttempt time.Time
	delay       time.Duration
}

func (s *bulkActivation) advance(now time.Time, enable func() error) (bool, error) {
	if s.enabled {
		return !now.Before(s.readyAt), nil
	}
	if now.Before(s.nextAttempt) {
		return false, nil
	}
	if err := enable(); err != nil {
		if s.delay == 0 {
			s.delay = time.Minute
		} else {
			s.delay *= 2
		}
		if s.delay > 5*time.Minute {
			s.delay = 5 * time.Minute
		}
		s.nextAttempt = now.Add(s.delay)
		return false, err
	}
	s.enabled = true
	s.readyAt = now.Add(time.Minute)
	return false, nil
}

func (bc *BulkClient) EnsureEnabled() (bool, error) {
	bc.activationMu.Lock()
	defer bc.activationMu.Unlock()
	return bc.activation.advance(time.Now(), bc.BulkEnable)
}

func (bc *BulkClient) ActivationReady() bool {
	bc.activationMu.Lock()
	defer bc.activationMu.Unlock()
	return bc.activation.enabled && !time.Now().Before(bc.activation.readyAt)
}
