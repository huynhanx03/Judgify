package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

type publicSampleReader struct {
	publishedTestsets ports.PublishedTestsetRepository
	decoder           entity.AssetDecoder
}

// NewPublicSampleReader creates the public, immutable sample projection.
func NewPublicSampleReader(
	publishedTestsets ports.PublishedTestsetRepository,
	decoder entity.AssetDecoder,
) (ports.PublicSampleReader, error) {
	if publishedTestsets == nil || decoder == nil {
		return nil, fmt.Errorf("create public sample reader: invalid dependency")
	}
	return &publicSampleReader{
		publishedTestsets: publishedTestsets,
		decoder:           decoder,
	}, nil
}

// FindPublicSamples returns only visible examples from the exact immutable
// testset selected by the currently published problem revision.
func (reader *publicSampleReader) FindPublicSamples(
	ctx context.Context,
	problemID uuid.UUID,
) ([]*contract.SampleTestCaseResult, error) {
	if ctx == nil || problemID == uuid.Nil {
		return nil, apperr.For(constant.ProblemObjProblem).NotFound(nil)
	}
	testset, err := reader.publishedTestsets.LoadPublishedTestset(ctx, problemID)
	if err != nil {
		if errors.Is(err, ports.ErrPublicationNotFound) {
			return nil, apperr.For(constant.ProblemObjProblem).NotFound(err)
		}
		return nil, err
	}
	samples, err := entity.BuildPublicSamples(ctx, testset, reader.decoder)
	if err != nil {
		return nil, err
	}
	responses := make([]*contract.SampleTestCaseResult, len(samples))
	for index, sample := range samples {
		responses[index] = &contract.SampleTestCaseResult{
			Input:          sample.Input,
			ExpectedOutput: sample.ExpectedOutput,
			OrderIndex:     index,
		}
	}
	return responses, nil
}
