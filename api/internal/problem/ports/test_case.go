package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// TestCaseRepository defines the test case data access interface.
type TestCaseRepository interface {
	FindByProblemID(ctx context.Context, problemID int) ([]*entity.TestCase, error)
	Get(ctx context.Context, id int) (*entity.TestCase, error)
	Create(ctx context.Context, e *entity.TestCase) error
	Update(ctx context.Context, e *entity.TestCase) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
	CountByProblemID(ctx context.Context, problemID int) (int, error)
}

// TestCaseService defines the test case business logic interface.
type TestCaseService interface {
	FindByProblemID(ctx context.Context, problemID int) ([]*dto.TestCaseResponse, error)
	Get(ctx context.Context, id int) (*dto.TestCaseResponse, error)
	Create(ctx context.Context, req *dto.CreateTestCaseRequest) (*dto.TestCaseResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateTestCaseRequest) (*dto.TestCaseResponse, error)
	Delete(ctx context.Context, id int) error
}
