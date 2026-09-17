package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"

	"github.com/huynhanx03/judgify/internal/ports"
)

// TestCaseHandler defines the test case HTTP handler interface.
type TestCaseHandler interface {
	FindPublicSamples(ctx context.Context, req *publicSamplesRequest) ([]*SampleTestCaseResponse, error)
}

type testCaseHandler struct {
	handler.BaseHandler
	publicSamples ports.PublicSampleReader
}

// NewTestCaseHandler creates the public sample handler.
func NewTestCaseHandler(publicSamples ports.PublicSampleReader) TestCaseHandler {
	return &testCaseHandler{publicSamples: publicSamples}
}

func (h *testCaseHandler) FindPublicSamples(ctx context.Context, req *publicSamplesRequest) ([]*SampleTestCaseResponse, error) {
	problemID, err := toPublicSamplesProblemID(req)
	if err != nil {
		return nil, err
	}
	results, err := h.publicSamples.FindPublicSamples(ctx, problemID)
	if err != nil {
		return nil, err
	}
	return toSampleTestCaseResponses(results), nil
}
