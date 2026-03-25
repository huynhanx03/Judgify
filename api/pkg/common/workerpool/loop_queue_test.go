package workerpool

import (
	"errors"
	"testing"
	"time"
)

func TestNewLoopQueue(t *testing.T) {
	size := 100
	q := newLoopQueue(size)
	if q.len() != 0 {
		t.Errorf("NewLoopQueue len = %d, want 0", q.len())
	}
	if !q.isEmpty() {
		t.Errorf("NewLoopQueue isEmpty = false, want true")
	}
	if q.detach() != nil {
		t.Errorf("NewLoopQueue detach should be nil")
	}
	if newLoopQueue(0) != nil {
		t.Errorf("NewLoopQueue(0) should be nil")
	}
}

func TestLoopQueue_InsertDetach(t *testing.T) {
	size := 10
	q := newLoopQueue(size)

	for i := 0; i < 5; i++ {
		err := q.insert(&mockWorker{workerCommon{lastUsed: time.Now().UnixNano()}})
		if err != nil {
			t.Errorf("Insert failed: %v", err)
		}
	}
	if q.len() != 5 {
		t.Errorf("Len = %d, want 5", q.len())
	}

	_ = q.detach()
	if q.len() != 4 {
		t.Errorf("Len after detach = %d, want 4", q.len())
	}

	for i := 0; i < 6; i++ {
		err := q.insert(&mockWorker{workerCommon{lastUsed: time.Now().UnixNano()}})
		if err != nil {
			t.Errorf("Insert failed at %d: %v", i, err)
		}
	}
	if q.len() != 10 {
		t.Errorf("Len = %d, want 10", q.len())
	}

	err := q.insert(&mockWorker{workerCommon{lastUsed: time.Now().UnixNano()}})
	if !errors.Is(err, ErrQueueIsFull) {
		t.Errorf("Insert on full queue should return ErrQueueIsFull, got %v", err)
	}
}

func TestLoopQueue_Refresh(t *testing.T) {
	size := 10
	q := newLoopQueue(size)
	duration := 100 * time.Millisecond

	// Insert workers that are expired (lastUsed = now - 2*duration)
	expiredCount := 5
	for i := 0; i < expiredCount; i++ {
		_ = q.insert(&mockWorker{workerCommon{lastUsed: time.Now().Add(-200 * time.Millisecond).UnixNano()}})
	}

	// Insert workers that are fresh (lastUsed = now)
	freshCount := 5
	for i := 0; i < freshCount; i++ {
		_ = q.insert(&mockWorker{workerCommon{lastUsed: time.Now().UnixNano()}})
	}

	if q.len() != 10 {
		t.Fatalf("Queue should be full, got %d", q.len())
	}

	expiredWorkers := q.refresh(duration)
	if len(expiredWorkers) != expiredCount {
		t.Errorf("Expected %d expired workers, got %d", expiredCount, len(expiredWorkers))
	}
	if q.len() != freshCount {
		t.Errorf("Queue len after refresh = %d, want %d", q.len(), freshCount)
	}
}

func TestLoopQueue_Refresh_WrapAround(t *testing.T) {
	qReal := newLoopQueue(5)
	now := time.Now()
	oldTime := now.Add(-5 * time.Hour).UnixNano()
	midTime := now.Add(-3 * time.Hour).UnixNano()
	newTime := now.UnixNano()

	// 1. Insert 5 old
	for i := 0; i < 5; i++ {
		_ = qReal.insert(&mockWorker{workerCommon{lastUsed: oldTime}})
	}
	// 2. Detach 3
	_ = qReal.detach()
	_ = qReal.detach()
	_ = qReal.detach()
	// 3. Insert 2 mid
	_ = qReal.insert(&mockWorker{workerCommon{lastUsed: midTime}})
	_ = qReal.insert(&mockWorker{workerCommon{lastUsed: midTime}})
	// 4. Insert 1 new (will wrap)
	_ = qReal.insert(&mockWorker{workerCommon{lastUsed: newTime}})

	// Expect 4 expired (2 old + 2 mid). 1 remaining.
	expired := qReal.refresh(1 * time.Hour)

	if len(expired) != 4 {
		t.Errorf("Expected 4 expired, got %d", len(expired))
	}

	if qReal.len() != 1 {
		t.Errorf("Expected 1 remaining, got %d", qReal.len())
	}

	// Verify the remaining one is the New one
	w := qReal.detach()
	if w.lastUsedTime() != newTime {
		t.Error("Remaining worker is not the newest one")
	}
}
