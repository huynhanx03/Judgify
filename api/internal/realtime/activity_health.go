package realtime

import (
	"sync"
	"time"
)

// activityHealth keeps only the latest successful and failed activity. It is
// intentionally independent of the operation being monitored so long-running
// realtime loops can share the same bounded readiness semantics.
type activityHealth struct {
	mu          sync.RWMutex
	clock       func() time.Time
	lastSuccess time.Time
	lastFailure time.Time
}

func newActivityHealth(clock func() time.Time) (*activityHealth, error) {
	if clock == nil {
		return nil, ErrInvalidConfig
	}
	return &activityHealth{clock: clock}, nil
}

func (health *activityHealth) succeeded() {
	health.record(true)
}

func (health *activityHealth) failed() {
	health.record(false)
}

func (health *activityHealth) record(success bool) {
	if health == nil || health.clock == nil {
		return
	}
	now := health.clock()
	health.mu.Lock()
	if success {
		health.lastSuccess = now
	} else {
		health.lastFailure = now
	}
	health.mu.Unlock()
}

func (health *activityHealth) available(staleAfter time.Duration) bool {
	if health == nil || health.clock == nil || staleAfter <= 0 {
		return false
	}
	health.mu.RLock()
	lastSuccess := health.lastSuccess
	health.mu.RUnlock()
	if lastSuccess.IsZero() {
		return false
	}
	now := health.clock()
	if now.Before(lastSuccess) {
		return true
	}
	return now.Sub(lastSuccess) < staleAfter
}
