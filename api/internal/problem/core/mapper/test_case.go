package mapper

import (
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToTestCaseResponse converts TestCase entity to TestCaseResponse DTO.
func ToTestCaseResponse(e *entity.TestCase) *dto.TestCaseResponse {
	if e == nil {
		return nil
	}
	return &dto.TestCaseResponse{
		ID:             e.ID,
		ProblemID:      e.ProblemID,
		Input:          e.Input,
		ExpectedOutput: e.ExpectedOutput,
		IsSample:       e.IsSample,
		OrderIndex:     e.OrderIndex,
	}
}

// ToTestCaseEntityFromCreate converts CreateTestCaseRequest to TestCase entity.
func ToTestCaseEntityFromCreate(req *dto.CreateTestCaseRequest) *entity.TestCase {
	e := &entity.TestCase{
		ProblemID:      req.ProblemID,
		Input:          req.Input,
		ExpectedOutput: req.ExpectedOutput,
		OrderIndex:     req.OrderIndex,
	}
	if req.IsSample != nil {
		e.IsSample = *req.IsSample
	}
	return e
}
