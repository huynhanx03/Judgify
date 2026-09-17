package http

import (
	"fmt"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func toPublicSamplesProblemID(request *publicSamplesRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ProblemID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse problem id: %w", err)
	}
	return id, nil
}

func toSampleTestCaseResponses(results []*contract.SampleTestCaseResult) []*SampleTestCaseResponse {
	responses := make([]*SampleTestCaseResponse, len(results))
	for index, result := range results {
		responses[index] = &SampleTestCaseResponse{
			Input: result.Input, ExpectedOutput: result.ExpectedOutput, OrderIndex: result.OrderIndex,
		}
	}
	return responses
}
