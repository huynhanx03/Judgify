package service

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/problem/constant"
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/mapper"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)


type testCaseService struct {
	testCaseRepo ports.TestCaseRepository
	problemRepo  ports.ProblemRepository
}

// NewTestCaseService creates a new TestCaseService instance.
func NewTestCaseService(testCaseRepo ports.TestCaseRepository, problemRepo ports.ProblemRepository) ports.TestCaseService {
	return &testCaseService{testCaseRepo: testCaseRepo, problemRepo: problemRepo}
}

// FindByProblemID retrieves test cases for a problem.
func (s *testCaseService) FindByProblemID(ctx context.Context, problemID int) ([]*dto.TestCaseResponse, error) {
	// Verify problem exists
	exists, err := s.problemRepo.Exists(ctx, problemID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjProblem), nil)
	}

	testCases, err := s.testCaseRepo.FindByProblemID(ctx, problemID)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.TestCaseResponse, len(testCases))
	for i, tc := range testCases {
		responses[i] = mapper.ToTestCaseResponse(tc)
	}
	return responses, nil
}

// Get retrieves a test case by ID.
func (s *testCaseService) Get(ctx context.Context, id int) (*dto.TestCaseResponse, error) {
	tc, err := s.testCaseRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToTestCaseResponse(tc), nil
}

// Create creates a new test case.
func (s *testCaseService) Create(ctx context.Context, req *dto.CreateTestCaseRequest) (*dto.TestCaseResponse, error) {
	// Verify problem exists
	exists, err := s.problemRepo.Exists(ctx, req.ProblemID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjProblem), nil)
	}

	tc := mapper.ToTestCaseEntityFromCreate(req)
	if err := s.testCaseRepo.Create(ctx, tc); err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("test case created successfully", zap.Int("test_case_id", tc.ID), zap.Int("problem_id", tc.ProblemID))

	return mapper.ToTestCaseResponse(tc), nil
}

// Update updates an existing test case.
func (s *testCaseService) Update(ctx context.Context, id int, req *dto.UpdateTestCaseRequest) (*dto.TestCaseResponse, error) {
	tc, err := s.testCaseRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Input != nil {
		tc.Input = *req.Input
	}
	if req.ExpectedOutput != nil {
		tc.ExpectedOutput = *req.ExpectedOutput
	}
	if req.IsHidden != nil {
		tc.IsHidden = *req.IsHidden
	}
	if req.OrderIndex != nil {
		tc.OrderIndex = *req.OrderIndex
	}

	tc.ID = id
	if err := s.testCaseRepo.Update(ctx, tc); err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("test case updated successfully", zap.Int("test_case_id", tc.ID), zap.Int("problem_id", tc.ProblemID))

	return mapper.ToTestCaseResponse(tc), nil
}

// Delete removes a test case by ID.
func (s *testCaseService) Delete(ctx context.Context, id int) error {
	exists, err := s.testCaseRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjTestCase), nil)
	}

	if err := s.testCaseRepo.Delete(ctx, id); err != nil {
		return err
	}

	logger.FromContext(ctx).Info("test case deleted successfully", zap.Int("test_case_id", id))
	return nil
}
