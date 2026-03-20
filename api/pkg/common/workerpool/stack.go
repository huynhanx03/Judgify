package workerpool

import "time"

var _ Queue = (*stack)(nil)

type stack struct {
	items  []Worker
	expiry []Worker
}

func newStack(size int) *stack {
	return &stack{
		items: make([]Worker, 0, size),
	}
}

// len returns the number of workers in the queue.
func (ws *stack) len() int {
	return len(ws.items)
}

// isEmpty returns true if the queue is empty.
func (ws *stack) isEmpty() bool {
	return len(ws.items) == 0
}

// insert inserts a worker into the queue.
func (ws *stack) insert(w Worker) error {
	ws.items = append(ws.items, w)
	return nil
}

// detach removes and returns the worker at the end of the queue.
func (ws *stack) detach() Worker {
	l := ws.len()
	if l == 0 {
		return nil
	}

	w := ws.items[l-1]
	ws.items[l-1] = nil // avoid memory leaks
	ws.items = ws.items[:l-1]

	return w
}

// refresh retrieves and removes all expired workers.
func (ws *stack) refresh(duration time.Duration) []Worker {
	n := ws.len()
	if n == 0 {
		return nil
	}

	expiryTime := time.Now().Add(-duration).UnixNano()

	// Find the index of the first valid (non-expired) worker.
	// Since items are sorted by time (oldest at 0), this gives us the split point.
	l, r := 0, n-1
	for l <= r {
		mid := l + (r-l)/2
		if expiryTime < ws.items[mid].lastUsedTime() {
			r = mid - 1
		} else {
			l = mid + 1
		}
	}

	lastExpiredIndex := r
	if lastExpiredIndex < 0 {
		return nil
	}

	ws.expiry = ws.expiry[:0]
	ws.expiry = append(ws.expiry, ws.items[:lastExpiredIndex+1]...)

	// Shift remaining valid workers to the beginning of the slice
	copy(ws.items, ws.items[lastExpiredIndex+1:])

	// Release references to the checked-out workers to avoid memory leaks
	newLen := n - (lastExpiredIndex + 1)
	for i := newLen; i < n; i++ {
		ws.items[i] = nil
	}
	ws.items = ws.items[:newLen]

	return ws.expiry
}

// reset resets the queue.
func (ws *stack) reset() {
	for i := 0; i < ws.len(); i++ {
		ws.items[i].finish()
		ws.items[i] = nil
	}
	ws.items = ws.items[:0]
}
