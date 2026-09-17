package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type oversizedSessionListRepository struct {
	*sessionManagementMemoryRepository
	userID uuid.UUID
}

func (repository *oversizedSessionListRepository) List(
	context.Context,
	ports.SessionListQuery,
) (ports.SessionListResult, error) {
	now := time.Now().UTC()
	userID := repository.userID
	sessions := make([]entity.Session, 0, defaultSessionPageLimit+1)
	for index := 0; index <= defaultSessionPageLimit; index++ {
		sessions = append(sessions, entity.Session{
			ID: uuid.New(), UserID: userID, Status: entity.SessionStatusActive,
			CreatedAt:  now.Add(-time.Duration(index) * time.Second),
			LastSeenAt: now, LastReauthenticatedAt: now,
			IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
		})
	}
	return ports.SessionListResult{Sessions: sessions}, nil
}

func TestSessionManagementRejectsOversizedRepositoryPage(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	currentID := uuid.New()
	base := newSessionManagementMemoryRepository(userID, currentID, uuid.New(), now)
	repository := &oversizedSessionListRepository{sessionManagementMemoryRepository: base, userID: userID}
	fixture := newSessionManagementFixture(t, base, now, &securityMutationJournalCapture{})
	fixture.service.repository = repository

	_, err := fixture.service.List(
		context.Background(), userID, currentID, &dto.SessionListRequest{Limit: defaultSessionPageLimit},
	)
	requireAppErrorCode(t, err, apperr.CodeInternalServer)
}
