package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/material/viewer"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
)

type viewRepositoryStub struct {
	fingerprints  [][32]byte
	authenticated []bool
}

func (stub *viewRepositoryStub) RecordDailyView(_ context.Context, materialID uuid.UUID, day time.Time, fingerprint [32]byte, authenticated bool) (entity.DailyViewReceipt, error) {
	stub.authenticated = append(stub.authenticated, authenticated)
	for _, existing := range stub.fingerprints {
		if existing == fingerprint {
			return entity.DailyViewReceipt{
				ID:         uuid.NewSHA1(uuid.NameSpaceOID, fingerprint[:]),
				MaterialID: materialID, ViewDay: day,
			}, nil
		}
	}
	stub.fingerprints = append(stub.fingerprints, fingerprint)
	return entity.DailyViewReceipt{
		ID:         uuid.NewSHA1(uuid.NameSpaceOID, fingerprint[:]),
		MaterialID: materialID, ViewDay: day, Inserted: true,
	}, nil
}

func (*viewRepositoryStub) ApplyDailyViewProjection(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (*viewRepositoryStub) ReconcileDailyViewProjection(context.Context, int) (int, error) {
	return 0, nil
}

type directViewUnitOfWork struct{}

func (directViewUnitOfWork) Within(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

type recordingViewOutbox struct{ events []eventing.Event }

func (outbox *recordingViewOutbox) Append(_ context.Context, events ...eventing.Event) error {
	outbox.events = append(outbox.events, events...)
	return nil
}

func TestViewUsesStablePrivacyPreservingDailyHMAC(t *testing.T) {
	repository := &viewRepositoryStub{}
	outbox := &recordingViewOutbox{}
	serviceValue, err := NewViewService(ViewDependencies{
		Repository: repository, Transactions: directViewUnitOfWork{},
		Outbox: outbox, Secret: []byte("0123456789abcdef0123456789abcdef"),
	})
	require.NoError(t, err)
	service := serviceValue.(*viewService)
	service.now = func() time.Time { return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC) }
	materialID := uuid.New()
	viewerFingerprint := [32]byte{1, 2, 3, 4}
	ctx := viewer.WithAnonymousFingerprint(context.Background(), viewerFingerprint)

	counted, err := service.Record(ctx, materialID)
	require.NoError(t, err)
	require.True(t, counted)
	counted, err = service.Record(ctx, materialID)
	require.NoError(t, err)
	require.False(t, counted)
	require.Len(t, repository.fingerprints, 1)
	require.NotEqual(t, [32]byte{}, repository.fingerprints[0])
	require.NotEqual(t, viewerFingerprint, repository.fingerprints[0])
	require.Len(t, outbox.events, 1)
	require.Equal(t, entity.MaterialViewRecordedV1, outbox.events[0].Type)
	require.Equal(t, []bool{false, false}, repository.authenticated)
}

func TestViewPropagatesOnlyCanonicalLiveSessionAuthenticationToVisibilityCheck(t *testing.T) {
	repository := &viewRepositoryStub{}
	serviceValue, err := NewViewService(ViewDependencies{
		Repository: repository, Transactions: directViewUnitOfWork{},
		Outbox: &recordingViewOutbox{}, Secret: []byte("0123456789abcdef0123456789abcdef"),
	})
	require.NoError(t, err)
	now := time.Now().UTC()
	userID, sessionID := uuid.New(), uuid.New()
	principal := authentication.Principal{
		Authenticated: true, Subject: userID.String(), UserID: userID.String(),
		SessionID: sessionID.String(), TokenID: uuid.NewString(),
		AuthenticatedAt: now.Add(-time.Minute), SecurityVersion: 1,
		AuthenticatedUntil: now.Add(time.Hour),
	}
	ctx := authentication.WithPrincipal(context.Background(), principal)

	counted, err := serviceValue.Record(ctx, uuid.New())
	require.NoError(t, err)
	require.True(t, counted)
	require.Equal(t, []bool{true}, repository.authenticated)
}
