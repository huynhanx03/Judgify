package workerpool

// Worker is the interface for a worker that runs tasks.
type Worker interface {
	run()
	finish()
	lastUsedTime() int64
	setLastUsedTime(t int64)
	inputFunc(func())
	inputParam(any) // Added for Generic Pool support
}

// workerCommon contains common fields and methods for workers.
type workerCommon struct {
	lastUsed int64
}

// lastUsedTime returns the last used time of the worker.
func (w *workerCommon) lastUsedTime() int64 {
	return w.lastUsed
}

// setLastUsedTime sets the last used time of the worker.
func (w *workerCommon) setLastUsedTime(t int64) {
	w.lastUsed = t
}
