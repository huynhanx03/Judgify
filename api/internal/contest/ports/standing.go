package ports

import (
	"context"
	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
)

// StandingRepository defines the standing data access interface.
type StandingRepository interface {
	Get(ctx context.Context, contestID, userID int) (*entity.ContestStanding, error)
	Upsert(ctx context.Context, e *entity.ContestStanding) error
	FindByContest(ctx context.Context, contestID int) ([]*entity.ContestStanding, error)
}

// StandingService defines the standing business logic interface.
type StandingService interface {
	GetStandings(ctx context.Context, contestID int) ([]*dto.StandingResponse, error)
	UpdateFromVerdict(ctx context.Context, contestID, userID, problemID int, accepted bool, submitTimeSec int) error
}
