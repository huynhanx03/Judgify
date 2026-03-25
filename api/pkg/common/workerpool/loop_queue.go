package workerpool

import (
	"time"
)

var _ Queue = (*loopQueue)(nil)

type loopQueue struct {
	items  []Worker
	expiry []Worker
	head   int
	tail   int
	size   int
	isFull bool
}

func newLoopQueue(size int) *loopQueue {
	if size <= 0 {
		return nil
	}
	return &loopQueue{
		items: make([]Worker, size),
		size:  size,
	}
}

// len returns the number of workers in the queue.
func (wq *loopQueue) len() int {
	if wq.size == 0 || wq.isEmpty() {
		return 0
	}

	if wq.head == wq.tail && wq.isFull {
		return wq.size
	}

	if wq.tail > wq.head {
		return wq.tail - wq.head
	}

	return wq.size - wq.head + wq.tail
}

// isEmpty returns true if the queue is empty.
func (wq *loopQueue) isEmpty() bool {
	return wq.head == wq.tail && !wq.isFull
}

// insert inserts a worker into the queue.
func (wq *loopQueue) insert(w Worker) error {
	if wq.size == 0 {
		return nil
	}
	if wq.isFull {
		return ErrQueueIsFull
	}
	wq.items[wq.tail] = w
	wq.tail = (wq.tail + 1) % wq.size

	if wq.tail == wq.head {
		wq.isFull = true
	}

	return nil
}

// detach removes and returns the worker at the head of the queue.
func (wq *loopQueue) detach() Worker {
	if wq.isEmpty() {
		return nil
	}

	w := wq.items[wq.head]
	wq.items[wq.head] = nil
	wq.head = (wq.head + 1) % wq.size

	wq.isFull = false

	return w
}

// refresh retrieves and removes all expired workers from the queue.
func (wq *loopQueue) refresh(duration time.Duration) []Worker {
	expiryTime := time.Now().Add(-duration).UnixNano()
	index := wq.binarySearch(expiryTime)
	if index == -1 {
		return nil
	}
	wq.expiry = wq.expiry[:0]

	if wq.head <= index {
		// No wrap-around: expired workers are in a contiguous block [head, index]
		wq.expiry = append(wq.expiry, wq.items[wq.head:index+1]...)
		for i := wq.head; i < index+1; i++ {
			wq.items[i] = nil
		}
	} else {
		// Wrap-around: expired workers are in [head, end] and [0, index]
		wq.expiry = append(wq.expiry, wq.items[wq.head:]...)
		wq.expiry = append(wq.expiry, wq.items[0:index+1]...)
		for i := 0; i < index+1; i++ {
			wq.items[i] = nil
		}
		for i := wq.head; i < wq.size; i++ {
			wq.items[i] = nil
		}
	}
	head := (index + 1) % wq.size
	wq.head = head
	if len(wq.expiry) > 0 {
		wq.isFull = false
	}

	return wq.expiry
}

func (wq *loopQueue) binarySearch(expiryTime int64) int {
	nlen := len(wq.items)

	if wq.isEmpty() || expiryTime < wq.items[wq.head].lastUsedTime() {
		return -1
	}

	// map head and tail to effective left and right
	// Example from ants:
	// r is the logical index of the last element
	r := (wq.tail - 1 - wq.head + nlen) % nlen
	basel := wq.head
	l := 0
	for l <= r {
		mid := l + ((r - l) >> 1)
		// calculate true mid position from mapped mid position
		tmid := (mid + basel) % nlen
		if expiryTime < wq.items[tmid].lastUsedTime() {
			r = mid - 1
		} else {
			l = mid + 1
		}
	}
	// return true position from mapped position
	return (r + basel) % nlen
}

// reset resets the queue.
func (wq *loopQueue) reset() {
	if wq.isEmpty() {
		return
	}

retry:
	if w := wq.detach(); w != nil {
		w.finish()
		goto retry
	}
	wq.head = 0
	wq.tail = 0
}
