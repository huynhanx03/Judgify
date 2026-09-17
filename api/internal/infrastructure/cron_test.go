package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	problemMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func TestSchedulerCancelsRunningJobAndHonorsShutdownContext(t *testing.T) {
	controller := gomock.NewController(t)
	repository := problemMocks.NewMockProblemRepository(controller)
	jobStarted := make(chan struct{})
	repository.EXPECT().
		RecalculateStats(gomock.Any(), gomock.Nil()).
		DoAndReturn(func(ctx context.Context, _ *time.Time) error {
			close(jobStarted)
			<-ctx.Done()
			return ctx.Err()
		})

	scheduler, err := NewScheduler(repository, zap.NewNop())
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}
	scheduler.interval = time.Millisecond
	runResult := make(chan error, 1)
	go func() { runResult <- scheduler.Run(context.Background()) }()
	select {
	case <-jobStarted:
	case <-time.After(time.Second):
		t.Fatal("scheduled job did not start")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := scheduler.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := <-runResult; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestSchedulerReturnsDeadlineForUncooperativeJob(t *testing.T) {
	controller := gomock.NewController(t)
	repository := problemMocks.NewMockProblemRepository(controller)
	jobStarted := make(chan struct{})
	releaseJob := make(chan struct{})
	repository.EXPECT().
		RecalculateStats(gomock.Any(), gomock.Nil()).
		DoAndReturn(func(context.Context, *time.Time) error {
			close(jobStarted)
			<-releaseJob
			return nil
		})

	scheduler, err := NewScheduler(repository, zap.NewNop())
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}
	scheduler.interval = time.Millisecond
	runResult := make(chan error, 1)
	go func() { runResult <- scheduler.Run(context.Background()) }()
	select {
	case <-jobStarted:
	case <-time.After(time.Second):
		t.Fatal("scheduled job did not start")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := scheduler.Shutdown(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v, want context deadline exceeded", err)
	}
	close(releaseJob)
	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not exit after job returned")
	}
}
