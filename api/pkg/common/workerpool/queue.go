package workerpool

import "time"

// QueueType indicates the type of the worker queue.
type QueueType int

const (
	// QueueTypeLoopQueue indicates the use of a circular loop queue.
	QueueTypeLoopQueue QueueType = iota

	// QueueTypeStack indicates the use of a LIFO stack.
	QueueTypeStack
)

// Queue is the interface for holding workers.
type Queue interface {
	len() int
	isEmpty() bool
	insert(Worker) error
	detach() Worker
	refresh(duration time.Duration) []Worker
	reset()
}

func newQueue(qType QueueType, size int) Queue {
	switch qType {
	case QueueTypeStack:
		return newStack(size)
	case QueueTypeLoopQueue:
		return newLoopQueue(size)
	default:
		return newLoopQueue(size)
	}
}
