package eventdelivery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/platform/eventing"
)

type fakeRelay struct {
	started chan struct{}
	done    chan struct{}
	once    sync.Once
}

func (relay *fakeRelay) Run(ctx context.Context) error {
	close(relay.started)
	select {
	case <-ctx.Done():
		return nil
	case <-relay.done:
		return nil
	}
}

func (relay *fakeRelay) Shutdown(context.Context) error {
	relay.once.Do(func() { close(relay.done) })
	return nil
}

type fakeReplayer struct {
	request eventing.ReplayDeadRequest
	result  bool
}

func (replayer *fakeReplayer) ReplayDead(
	_ context.Context,
	request eventing.ReplayDeadRequest,
) (bool, error) {
	replayer.request = request
	return replayer.result, nil
}

func TestRuntimeReconcilesBeforeServingAndSupportsFencedReplay(t *testing.T) {
	t.Parallel()

	relay := &fakeRelay{started: make(chan struct{}), done: make(chan struct{})}
	replayer := &fakeReplayer{result: true}
	calls := 0
	runtime, err := New(
		relay,
		replayer,
		[]Reconciler{{
			Name: "projection",
			Reconcile: func(context.Context, int) (int, error) {
				calls++
				return 0, nil
			},
		}},
		Options{
			ReconcileInterval: time.Hour,
			ReconcileBatch:    64,
			MaxBatchesPerRun:  2,
		},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- runtime.Run(ctx) }()
	<-relay.started
	if calls != 1 {
		t.Fatalf("startup reconcile calls = %d, want 1", calls)
	}
	if err := runtime.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	snapshot := runtime.Metrics()
	if snapshot.ObservedSince.IsZero() || snapshot.LastSuccessfulReconcileAt.IsZero() ||
		snapshot.LastReconcileAt.Before(snapshot.LastSuccessfulReconcileAt) {
		t.Fatalf("metrics snapshot = %+v", snapshot)
	}
	eventID := uuid.New()
	replayed, err := runtime.ReplayDead(context.Background(), eventID, 7)
	if err != nil || !replayed {
		t.Fatalf("ReplayDead = (%t, %v)", replayed, err)
	}
	if replayer.request.EventID != eventID ||
		replayer.request.ExpectedAttempts != 7 {
		t.Fatalf("replay request = %+v", replayer.request)
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRuntimeHealthReflectsRepairFailure(t *testing.T) {
	t.Parallel()

	relay := &fakeRelay{started: make(chan struct{}), done: make(chan struct{})}
	runtime, err := New(
		relay,
		&fakeReplayer{},
		[]Reconciler{{
			Name: "projection",
			Reconcile: func(context.Context, int) (int, error) {
				return 0, errors.New("database unavailable")
			},
		}},
		Options{
			ReconcileInterval: time.Hour,
			ReconcileBatch:    64,
			MaxBatchesPerRun:  2,
		},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- runtime.Run(ctx) }()
	<-relay.started
	if !errors.Is(runtime.Check(context.Background()), ErrRuntimeUnavailable) {
		t.Fatal("failed repair left runtime healthy")
	}
	snapshot := runtime.Metrics()
	if snapshot.ObservedSince.IsZero() || snapshot.LastReconcileAt.IsZero() ||
		!snapshot.LastSuccessfulReconcileAt.IsZero() {
		t.Fatalf("failed reconcile metrics snapshot = %+v", snapshot)
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("Run: %v", err)
	}
}
