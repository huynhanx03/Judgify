package realtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAuthorizationTailerFencesOnlyNewCommittedRevisions(t *testing.T) {
	source := &fakeAuthorizationSource{change: authorizationChange(11)}
	publisher := &fakeAuthorizationPublisher{}
	tailer := testAuthorizationTailer(t, source, publisher)
	tailer.revision.Store(10)

	published, err := tailer.pollOnce(context.Background())
	require.NoError(t, err)
	require.True(t, published)
	require.Equal(t, int64(11), tailer.CurrentRevision())
	require.Empty(t, publisher.snapshot())

	published, err = tailer.pollOnce(context.Background())
	require.NoError(t, err)
	require.False(t, published)
	require.Empty(t, publisher.snapshot())
}

func TestAuthorizationTailerDoesNotUseLegacyPublisher(t *testing.T) {
	publishErr := errors.New("legacy publisher must be ignored")
	tailer := testAuthorizationTailer(
		t,
		&fakeAuthorizationSource{change: authorizationChange(12)},
		&fakeAuthorizationPublisher{err: publishErr},
	)
	tailer.revision.Store(11)

	published, err := tailer.pollOnce(context.Background())
	require.NoError(t, err)
	require.True(t, published)
	require.Equal(t, int64(12), tailer.CurrentRevision())
}

func TestAuthorizationTailerKeepsFenceWhenRevocationFails(t *testing.T) {
	revokeErr := errors.New("hub revocation unavailable")
	publisher := &fakeAuthorizationPublisher{}
	tailer, err := NewAuthorizationTailer(
		AuthorizationTailerConfig{PollInterval: time.Second},
		&fakeAuthorizationSource{change: authorizationChange(12)},
		publisher,
		&fakeConnectionRevoker{err: revokeErr},
		zap.NewNop(),
	)
	require.NoError(t, err)
	tailer.revision.Store(11)

	published, err := tailer.pollOnce(context.Background())
	require.ErrorIs(t, err, revokeErr)
	require.False(t, published)
	require.Equal(t, int64(11), tailer.CurrentRevision())
	require.Empty(t, publisher.snapshot())
}

func TestAuthorizationTailerRejectsRevisionRegression(t *testing.T) {
	tailer := testAuthorizationTailer(
		t,
		&fakeAuthorizationSource{change: authorizationChange(9)},
		&fakeAuthorizationPublisher{},
	)
	tailer.revision.Store(10)

	_, err := tailer.pollOnce(context.Background())
	require.ErrorIs(t, err, ErrInvalidChange)
	require.Equal(t, int64(10), tailer.CurrentRevision())
}

func TestAuthorizationTailerBootstrapsWithoutReplayingHead(t *testing.T) {
	source := &fakeAuthorizationSource{change: authorizationChange(21), called: make(chan struct{}, 1)}
	publisher := &fakeAuthorizationPublisher{}
	tailer, err := NewAuthorizationTailer(
		AuthorizationTailerConfig{PollInterval: time.Millisecond},
		source,
		publisher,
		&fakeConnectionRevoker{},
		zap.NewNop(),
	)
	require.NoError(t, err)
	runDone := make(chan error, 1)
	go func() { runDone <- tailer.Run(context.Background()) }()
	select {
	case <-source.called:
	case <-time.After(time.Second):
		t.Fatal("authorization tailer did not bootstrap")
	}
	require.Eventually(t, func() bool {
		return tailer.CurrentRevision() == 21 && tailer.Check(context.Background()) == nil
	}, time.Second, time.Millisecond)
	require.Empty(t, publisher.snapshot())
	require.NoError(t, tailer.Shutdown(context.Background()))
	require.NoError(t, <-runDone)
}

func testAuthorizationTailer(
	t *testing.T,
	source AuthorizationRevisionSource,
	publisher AuthorizationChangePublisher,
) *AuthorizationTailer {
	t.Helper()
	tailer, err := NewAuthorizationTailer(
		AuthorizationTailerConfig{PollInterval: time.Second},
		source,
		publisher,
		&fakeConnectionRevoker{},
		zap.NewNop(),
	)
	require.NoError(t, err)
	return tailer
}

type fakeConnectionRevoker struct {
	mu       sync.Mutex
	allCalls int
	err      error
}

func (revoker *fakeConnectionRevoker) RevokeAllAuthenticated(context.Context) (int, error) {
	revoker.mu.Lock()
	defer revoker.mu.Unlock()
	revoker.allCalls++
	if revoker.err != nil {
		return 0, revoker.err
	}
	return 3, nil
}

func (*fakeConnectionRevoker) RevokeSubject(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}

func (*fakeConnectionRevoker) RevokeSession(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}

func authorizationChange(revision int64) AuthorizationRevisionChange {
	return AuthorizationRevisionChange{Event: AuthorizationCapabilitiesChangedV1{
		Revision:  revision,
		ChangedAt: time.Date(2026, time.July, 22, 9, 0, int(revision%60), 0, time.UTC),
	}}
}

type fakeAuthorizationSource struct {
	mu     sync.Mutex
	change AuthorizationRevisionChange
	err    error
	called chan struct{}
}

func (source *fakeAuthorizationSource) Current(context.Context) (AuthorizationRevisionChange, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.called != nil {
		select {
		case source.called <- struct{}{}:
		default:
		}
	}
	return source.change, source.err
}

type fakeAuthorizationPublisher struct {
	mu      sync.Mutex
	changes []AuthorizationRevisionChange
	err     error
}

func (publisher *fakeAuthorizationPublisher) PublishAuthorization(
	_ context.Context,
	change AuthorizationRevisionChange,
) error {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.err != nil {
		return publisher.err
	}
	publisher.changes = append(publisher.changes, change)
	return nil
}

func (publisher *fakeAuthorizationPublisher) snapshot() []AuthorizationRevisionChange {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	return append([]AuthorizationRevisionChange(nil), publisher.changes...)
}
