package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	contestmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func TestPublicContestDetailFiltersHiddenMembershipsBeforeStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	base := contestmocks.NewMockContestRepository(ctrl)
	registrations := contestmocks.NewMockRegistrationRepository(ctrl)
	contestID := uuid.New()
	visibleProblemID, hiddenProblemID := uuid.New(), uuid.New()
	repository := &contestProblemProjectionRepository{
		MockContestRepository: base,
		projections: []ports.ContestProblemProjection{
			{
				ContestProblemID:   uuid.New(),
				ProblemID:          visibleProblemID,
				DisplayOrder:       1,
				Alias:              "A",
				VisibleBeforeStart: true,
			},
			{
				ContestProblemID: uuid.New(),
				ProblemID:        hiddenProblemID,
				DisplayOrder:     2,
				Alias:            "B",
			},
		},
	}
	base.EXPECT().GetPublic(gomock.Any(), contestID).Return(
		&entity.Contest{
			ID:     contestID,
			State:  entity.ContestUpcoming,
			Status: "upcoming",
		},
		nil,
	)
	base.EXPECT().GetReadModels(gomock.Any(), []uuid.UUID{contestID}, false).Return(
		map[uuid.UUID]ports.ContestReadModel{
			contestID: {ProblemCount: 2},
		},
		nil,
	)

	service := NewContestService(repository, registrations, nil)
	response, err := service.GetPublic(
		context.Background(),
		contestID,
		uuid.Nil,
	)
	if err != nil {
		t.Fatalf("GetPublic() error = %v", err)
	}
	if len(response.Problems) != 1 ||
		response.Problems[0].ProblemID != visibleProblemID {
		t.Fatalf("public contest projection = %+v", response)
	}
}

type contestProblemProjectionRepository struct {
	*contestmocks.MockContestRepository
	projections []ports.ContestProblemProjection
	err         error
}

func (repository *contestProblemProjectionRepository) GetContestProblemProjections(
	context.Context,
	uuid.UUID,
) ([]ports.ContestProblemProjection, error) {
	result := append(
		[]ports.ContestProblemProjection(nil),
		repository.projections...,
	)
	return result, repository.err
}
