package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/lifecycle"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/testutil"
	contestMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

var _ lifecycle.Component = (*ContestOrchestrator)(nil)

type ratingSettlementFake struct {
	pendingIDs []uuid.UUID
	findErr    error
}

func (fake *ratingSettlementFake) FindPending(
	context.Context,
	int,
) ([]uuid.UUID, error) {
	return append([]uuid.UUID(nil), fake.pendingIDs...), fake.findErr
}

func (*ratingSettlementFake) LockPending(
	context.Context,
	uuid.UUID,
) (bool, error) {
	return false, nil
}

func (*ratingSettlementFake) Complete(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	return nil
}

func newOrchestratorTestEnv(t *testing.T) (
	*ContestOrchestrator,
	*contestMocks.MockContestRepository,
	*contestMocks.MockRatingService,
) {
	t.Helper()

	ctrl := gomock.NewController(t)
	repository := contestMocks.NewMockContestRepository(ctrl)
	ratingService := contestMocks.NewMockRatingService(ctrl)
	settlement := &ratingSettlementFake{}
	orchestrator := NewContestOrchestrator(repository, ratingService, settlement, zap.NewNop())
	orchestrator.tickInterval = time.Millisecond
	return orchestrator, repository, ratingService
}

func TestContestOrchestratorComponentIdentity(t *testing.T) {
	orchestrator, _, _ := newOrchestratorTestEnv(t)
	if got := orchestrator.Name(); got != "contest-orchestrator" {
		t.Fatalf("Name() = %q, want contest-orchestrator", got)
	}
}

func TestContestOrchestratorReconcilesImmediatelyOnStartup(t *testing.T) {
	orchestrator, repository, _ := newOrchestratorTestEnv(t)
	orchestrator.tickInterval = time.Hour
	reconciled := make(chan struct{})

	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusUpcoming).
		DoAndReturn(func(context.Context, string) ([]uuid.UUID, error) {
			close(reconciled)
			return nil, nil
		})
	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusRunning).
		Return(nil, nil)

	runResult := make(chan error, 1)
	go func() { runResult <- orchestrator.Run(context.Background()) }()
	select {
	case <-reconciled:
	case <-time.After(time.Second):
		t.Fatal("Run waited for the first periodic tick before reconciliation")
	}
	require.NoError(t, orchestrator.Shutdown(context.Background()))
	require.NoError(t, <-runResult)
}

func TestContestOrchestratorRunAndConcurrentShutdown(t *testing.T) {
	orchestrator, repository, _ := newOrchestratorTestEnv(t)
	started := make(chan struct{})
	var startedOnce sync.Once

	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusUpcoming).
		DoAndReturn(func(context.Context, string) ([]uuid.UUID, error) {
			startedOnce.Do(func() { close(started) })
			return nil, nil
		}).AnyTimes()
	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusRunning).
		Return(nil, nil).AnyTimes()

	runResult := make(chan error, 1)
	go func() { runResult <- orchestrator.Run(context.Background()) }()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Run did not start the transition loop")
	}
	select {
	case err := <-runResult:
		t.Fatalf("Run exited before shutdown: %v", err)
	default:
	}

	const callers = 32
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	results := make(chan error, callers)
	var callersWG sync.WaitGroup
	callersWG.Add(callers)
	for range callers {
		go func() {
			defer callersWG.Done()
			results <- orchestrator.Shutdown(shutdownCtx)
		}()
	}
	callersWG.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Errorf("concurrent Shutdown() = %v, want nil", err)
		}
	}

	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() = %v, want nil after shutdown", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after shutdown")
	}
	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("repeated Shutdown() = %v, want nil", err)
	}
}

func TestContestOrchestratorShutdownHonorsDeadline(t *testing.T) {
	orchestrator, repository, _ := newOrchestratorTestEnv(t)
	transitionStarted := make(chan struct{})
	releaseTransition := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseTransition) }) }
	t.Cleanup(release)

	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusUpcoming).
		DoAndReturn(func(context.Context, string) ([]uuid.UUID, error) {
			close(transitionStarted)
			<-releaseTransition
			return nil, nil
		})
	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusRunning).
		Return(nil, nil)

	runResult := make(chan error, 1)
	go func() { runResult <- orchestrator.Run(context.Background()) }()
	select {
	case <-transitionStarted:
	case <-time.After(time.Second):
		t.Fatal("transition did not start")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := orchestrator.Shutdown(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() = %v, want context deadline exceeded", err)
	}

	release()
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after the blocked transition was released")
	}
	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() after Run exit = %v, want nil", err)
	}
}

func TestContestOrchestratorRatingWorkStaysInsideLifecycleContext(t *testing.T) {
	orchestrator, repository, ratingService := newOrchestratorTestEnv(t)
	contestID := testutil.ID(42)
	ratingStarted := make(chan struct{})
	transitionReturned := make(chan struct{})

	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusUpcoming).
		Return(nil, nil)
	repository.EXPECT().
		FindIDsByStatus(gomock.Any(), constant.ContestStatusRunning).
		Return([]uuid.UUID{contestID}, nil)
	repository.EXPECT().Get(gomock.Any(), contestID).Return(&entity.Contest{
		ID:      contestID,
		EndTime: time.Now().Add(-time.Minute),
	}, nil)

	type contextKey struct{}
	key := contextKey{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key, "owned"))
	orchestrator.settlementRepo.(*ratingSettlementFake).pendingIDs = []uuid.UUID{contestID}
	ratingService.EXPECT().
		CalculateRating(gomock.Any(), contestID).
		DoAndReturn(func(ratingCtx context.Context, _ uuid.UUID) error {
			if got := ratingCtx.Value(key); got != "owned" {
				t.Errorf("rating context value = %v, want owned lifecycle context", got)
			}
			close(ratingStarted)
			<-ratingCtx.Done()
			return ratingCtx.Err()
		})

	go func() {
		orchestrator.transitionStatuses(ctx)
		close(transitionReturned)
	}()
	select {
	case <-ratingStarted:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("rating calculation did not start")
	}

	returnedEarly := false
	select {
	case <-transitionReturned:
		returnedEarly = true
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case <-transitionReturned:
	case <-time.After(time.Second):
		t.Fatal("transition did not release after lifecycle cancellation")
	}
	if returnedEarly {
		t.Fatal("transition returned while detached rating work was still running")
	}
}

func TestContestOrchestratorRetriesEndedPendingContest(t *testing.T) {
	orchestrator, repository, ratingService := newOrchestratorTestEnv(t)
	contestID := testutil.ID(51)
	ctx := context.Background()

	repository.EXPECT().FindIDsByStatus(ctx, constant.ContestStatusUpcoming).Return(nil, nil)
	repository.EXPECT().FindIDsByStatus(ctx, constant.ContestStatusRunning).Return(nil, nil)
	orchestrator.settlementRepo.(*ratingSettlementFake).pendingIDs = []uuid.UUID{contestID}
	ratingService.EXPECT().CalculateRating(gomock.Any(), contestID).Return(nil)

	orchestrator.transitionStatuses(ctx)
}

func TestContestOrchestratorContinuesAfterPendingRatingFailure(t *testing.T) {
	orchestrator, repository, ratingService := newOrchestratorTestEnv(t)
	firstID := testutil.ID(61)
	secondID := testutil.ID(62)
	ctx := context.Background()

	repository.EXPECT().FindIDsByStatus(ctx, constant.ContestStatusUpcoming).Return(nil, nil)
	repository.EXPECT().FindIDsByStatus(ctx, constant.ContestStatusRunning).Return(nil, nil)
	orchestrator.settlementRepo.(*ratingSettlementFake).pendingIDs = []uuid.UUID{firstID, secondID}
	ratingService.EXPECT().CalculateRating(gomock.Any(), firstID).Return(errors.New("transient failure"))
	ratingService.EXPECT().CalculateRating(gomock.Any(), secondID).Return(nil)

	orchestrator.transitionStatuses(ctx)
}

func TestContestOrchestratorPreservesUpcomingTransition(t *testing.T) {
	orchestrator, repository, _ := newOrchestratorTestEnv(t)
	contestID := testutil.ID(7)
	ctx := context.Background()

	repository.EXPECT().
		FindIDsByStatus(ctx, constant.ContestStatusUpcoming).
		Return([]uuid.UUID{contestID}, nil)
	repository.EXPECT().Get(gomock.Any(), contestID).Return(&entity.Contest{
		ID:        contestID,
		Version:   1,
		StartTime: time.Now().Add(-time.Minute),
	}, nil)
	orchestrator.lifecycle = &automaticLifecycleFake{}
	repository.EXPECT().
		FindIDsByStatus(ctx, constant.ContestStatusRunning).
		Return(nil, nil)

	orchestrator.transitionStatuses(ctx)
}

func TestContestOrchestratorAutomaticTransitionUsesSystemCommand(t *testing.T) {
	contestID := testutil.ID(71)
	lifecycle := &automaticLifecycleFake{}
	orchestrator := NewContestOrchestrator(
		nil,
		nil,
		nil,
		zap.NewNop(),
		lifecycle,
	)
	err := orchestrator.transitionSystem(context.Background(), &entity.Contest{ID: contestID, Version: 7}, entity.ContestRunning)
	require.NoError(t, err)
	require.True(t, lifecycle.command.System)
	require.Equal(t, uuid.Nil, lifecycle.command.ActorUserID)
	require.Nil(t, lifecycle.command.ActorSessionID)
	require.Equal(t, int64(7), lifecycle.command.ExpectedVersion)
	require.Equal(t, systemContestLifecycleReason, lifecycle.command.Reason)
	firstID := lifecycle.command.IdempotencyKey
	require.NoError(t, orchestrator.transitionSystem(context.Background(), &entity.Contest{ID: contestID, Version: 7}, entity.ContestRunning))
	require.Equal(t, firstID, lifecycle.command.IdempotencyKey)
}

type automaticLifecycleFake struct{ command ports.TransitionCommand }

func (fake *automaticLifecycleFake) Transition(
	_ context.Context,
	command ports.TransitionCommand,
) (ports.TransitionResult, error) {
	fake.command = command
	return ports.TransitionResult{ContestID: command.ContestID, To: command.Target, Version: command.ExpectedVersion + 1}, nil
}
