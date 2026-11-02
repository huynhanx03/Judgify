package realtime

import (
	"context"
	"sync"

	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
)

// HubLifecycle gives the shared WebSocket hub a stable product component name
// without leaking naming concerns into the reusable go-common package.
type HubLifecycle struct {
	hub     *commonwebsocket.Hub
	mu      sync.RWMutex
	running bool
}

func NewHubLifecycle(hub *commonwebsocket.Hub) (*HubLifecycle, error) {
	if hub == nil {
		return nil, ErrInvalidConfig
	}
	return &HubLifecycle{hub: hub}, nil
}

func (*HubLifecycle) Name() string { return "realtime-websocket-hub" }

func (component *HubLifecycle) Run(ctx context.Context) error {
	if component == nil || component.hub == nil {
		return ErrInvalidConfig
	}
	component.mu.Lock()
	component.running = true
	component.mu.Unlock()
	defer func() {
		component.mu.Lock()
		component.running = false
		component.mu.Unlock()
	}()
	return component.hub.Run(ctx)
}

func (component *HubLifecycle) Shutdown(ctx context.Context) error {
	if component == nil || component.hub == nil {
		return ErrInvalidConfig
	}
	return component.hub.Shutdown(ctx)
}

// Check reports whether the lifecycle-owned WebSocket hub event loop is
// running. It performs no network operation and exposes no connection data.
func (component *HubLifecycle) Check(ctx context.Context) error {
	if component == nil || ctx == nil {
		return ErrHealthUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	component.mu.RLock()
	running := component.running
	component.mu.RUnlock()
	if !running {
		return ErrHealthUnavailable
	}
	return nil
}
