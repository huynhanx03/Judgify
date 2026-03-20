package workerpool

import (
	"testing"
	"time"
)

func TestNewStack(t *testing.T) {
	size := 100
	q := newStack(size)
	if q.len() != 0 {
		t.Errorf("NewStack len = %d, want 0", q.len())
	}
	if !q.isEmpty() {
		t.Errorf("NewStack isEmpty = false, want true")
	}
	if q.detach() != nil {
		t.Errorf("NewStack detach should be nil")
	}
}

func TestStack_InsertDetach(t *testing.T) {
	size := 10
	q := newStack(size)

	for i := 0; i < 5; i++ {
		_ = q.insert(&mockWorker{workerCommon{lastUsed: time.Now().UnixNano()}})
	}
	if q.len() != 5 {
		t.Errorf("Len = %d, want 5", q.len())
	}

	_ = q.detach()
	if q.len() != 4 {
		t.Errorf("Len after detach = %d, want 4", q.len())
	}

	for i := 0; i < 6; i++ {
		_ = q.insert(&mockWorker{workerCommon{lastUsed: time.Now().UnixNano()}})
	}
	if q.len() != 10 {
		t.Errorf("Len = %d, want 10", q.len())
	}
}

func TestStack_Refresh(t *testing.T) {
	size := 10
	q := newStack(size)
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
