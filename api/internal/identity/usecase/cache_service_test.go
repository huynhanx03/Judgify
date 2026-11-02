package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestSessionStateCacheLeaderCancellationDoesNotPoisonWaiters(t *testing.T) {
	now := time.Date(2026, 7, 19, 6, 0, 0, 0, time.UTC)
	cachePort, err := NewSessionStateCache(SessionStateCacheConfig{
		MaximumTTL:     30 * time.Second,
		MaximumEntries: 8,
		LoadTimeout:    time.Second,
		Clock:          func() time.Time { return now },
	})
	require.NoError(t, err)
	key := ports.SessionCacheKey{
		SessionID:       uuid.New(),
		UserID:          uuid.New(),
		TokenID:         uuid.NewString(),
		SecurityVersion: 1,
	}
	state := entity.SessionSecurityState{
		SessionID:              key.SessionID,
		UserID:                 key.UserID,
		SessionSecurityVersion: 1,
		UserSecurityVersion:    1,
		IdleExpiresAt:          now.Add(time.Hour),
		AbsoluteExpiresAt:      now.Add(2 * time.Hour),
	}
	loadStarted := make(chan struct{})
	releaseLoad := make(chan struct{})
	loaderCanceled := make(chan struct{})
	loader := func(ctx context.Context) (entity.SessionSecurityState, error) {
		close(loadStarted)
		select {
		case <-releaseLoad:
			return state, nil
		case <-ctx.Done():
			close(loaderCanceled)
			return entity.SessionSecurityState{}, ctx.Err()
		}
	}

	leaderContext, cancelLeader := context.WithCancel(context.Background())
	leaderDone := make(chan error, 1)
	go func() {
		_, resolveErr := cachePort.Resolve(leaderContext, key, now.Add(time.Minute), loader)
		leaderDone <- resolveErr
	}()
	<-loadStarted

	waiterStarted := make(chan struct{})
	waiterDone := make(chan error, 1)
	go func() {
		close(waiterStarted)
		_, resolveErr := cachePort.Resolve(
			context.Background(),
			key,
			now.Add(time.Minute),
			func(context.Context) (entity.SessionSecurityState, error) {
				return entity.SessionSecurityState{}, errors.New("waiter unexpectedly became loader")
			},
		)
		waiterDone <- resolveErr
	}()
	<-waiterStarted
	cancelLeader()
	require.ErrorIs(t, <-leaderDone, context.Canceled)
	select {
	case <-loaderCanceled:
		t.Fatal("leader cancellation reached shared loader")
	default:
	}
	close(releaseLoad)
	require.NoError(t, <-waiterDone)
}
