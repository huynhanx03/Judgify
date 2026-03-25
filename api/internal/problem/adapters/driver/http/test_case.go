package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

// TestCaseHandler defines the test case HTTP handler interface.
type TestCaseHandler interface {
	FindByProblemID(ctx context.Context, req *dto.GetTestCaseRequest) ([]*dto.TestCaseResponse, error)
	Create(ctx context.Context, req *dto.CreateTestCaseRequest) (*dto.TestCaseResponse, error)
	Update(ctx context.Context, req *dto.UpdateTestCaseRequest) (*dto.TestCaseResponse, error)
	Delete(ctx context.Context, req *dto.DeleteTestCaseRequest) (*dto.TestCaseResponse, error)
}

type testCaseHandler struct {
	handler.BaseHandler
	testCaseService ports.TestCaseService
}

// NewTestCaseHandler creates a new TestCaseHandler instance.
func NewTestCaseHandler(testCaseService ports.TestCaseService) TestCaseHandler {
	return &testCaseHandler{testCaseService: testCaseService}
}

func (h *testCaseHandler) FindByProblemID(ctx context.Context, req *dto.GetTestCaseRequest) ([]*dto.TestCaseResponse, error) {
	return h.testCaseService.FindByProblemID(ctx, req.ProblemID)
}

func (h *testCaseHandler) Create(ctx context.Context, req *dto.CreateTestCaseRequest) (*dto.TestCaseResponse, error) {
	return h.testCaseService.Create(ctx, req)
}

func (h *testCaseHandler) Update(ctx context.Context, req *dto.UpdateTestCaseRequest) (*dto.TestCaseResponse, error) {
	return h.testCaseService.Update(ctx, req.ID, req)
}

func (h *testCaseHandler) Delete(ctx context.Context, req *dto.DeleteTestCaseRequest) (*dto.TestCaseResponse, error) {
	return nil, h.testCaseService.Delete(ctx, req.ID)
}
