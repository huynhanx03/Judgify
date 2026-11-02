package realtime

import (
	"context"
	"sync"
)

type componentState uint8

const (
	componentNew componentState = iota
	componentRunning
	componentStopping
	componentStopped
)

// componentLifecycle centralizes the small, concurrency-sensitive lifecycle
// state machine shared by realtime components. It starts no goroutine itself.
type componentLifecycle struct {
	mu       sync.Mutex
	state    componentState
	cancel   context.CancelFunc
	done     chan struct{}
	doneOnce sync.Once
}

func newComponentLifecycle() componentLifecycle {
	return componentLifecycle{state: componentNew, done: make(chan struct{})}
}

func (lifecycle *componentLifecycle) start(ctx context.Context) (context.Context, error) {
	if ctx == nil {
		return nil, ErrInvalidLifecycle
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.state != componentNew {
		return nil, ErrInvalidLifecycle
	}
	runContext, cancel := context.WithCancel(ctx)
	lifecycle.cancel = cancel
	lifecycle.state = componentRunning
	return runContext, nil
}

func (lifecycle *componentLifecycle) finish() {
	lifecycle.mu.Lock()
	if lifecycle.cancel != nil {
		lifecycle.cancel()
	}
	lifecycle.state = componentStopped
	lifecycle.mu.Unlock()
	lifecycle.doneOnce.Do(func() { close(lifecycle.done) })
}

func (lifecycle *componentLifecycle) shutdown(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidLifecycle
	}
	lifecycle.mu.Lock()
	switch lifecycle.state {
	case componentNew:
		lifecycle.state = componentStopped
		lifecycle.mu.Unlock()
		lifecycle.doneOnce.Do(func() { close(lifecycle.done) })
		return nil
	case componentRunning:
		lifecycle.state = componentStopping
		lifecycle.cancel()
	case componentStopping, componentStopped:
	}
	done := lifecycle.done
	lifecycle.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (lifecycle *componentLifecycle) stopped() bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.state == componentStopping || lifecycle.state == componentStopped
}

func (lifecycle *componentLifecycle) running() bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.state == componentRunning
}
