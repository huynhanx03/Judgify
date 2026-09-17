package usecase

import (
	"context"

	"github.com/google/uuid"

	d "github.com/huynhanx03/go-common/pkg/dto"

	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

type userTraitService struct {
	userTraitRepo ports.UserTraitRepository
}

// NewUserTraitService creates a new UserTraitService instance.
func NewUserTraitService(userTraitRepo ports.UserTraitRepository) ports.UserTraitService {
	return &userTraitService{userTraitRepo: userTraitRepo}
}

func (s *userTraitService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.UserTraitResponse], error) {
	result, err := s.userTraitRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*contract.UserTraitResponse]{Records: &[]*contract.UserTraitResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*contract.UserTraitResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToUserTraitResponse(e)
	}

	return &d.Paginated[*contract.UserTraitResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *userTraitService) Get(ctx context.Context, id uuid.UUID) (*contract.UserTraitResponse, error) {
	e, err := s.userTraitRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToUserTraitResponse(e), nil
}
