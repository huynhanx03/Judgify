package usecase

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
)

func TestFindPublicSamplesUsesPublishedImmutableTestset(t *testing.T) {
	problemID := testutil.ID(1)
	repository := &publishedTestsetRepositoryStub{
		testset: entity.TestsetRevision{
			ProblemID: problemID,
			Groups: []entity.TestcaseGroup{{
				GroupIndex: 0,
				Cases: []entity.TestcaseRevision{{
					Kind:          problemcontract.TestKindSample,
					InputAsset:    publicSampleAsset("1 2"),
					ExpectedAsset: publicSampleAsset("3"),
				}},
			}},
		},
	}
	reader, err := NewPublicSampleReader(repository, entity.IdentityAssetDecoder{})
	require.NoError(t, err)

	response, err := reader.FindPublicSamples(context.Background(), problemID)

	require.NoError(t, err)
	require.Len(t, response, 1)
	require.Equal(t, "1 2", response[0].Input)
	require.Equal(t, "3", response[0].ExpectedOutput)
	require.Zero(t, response[0].OrderIndex)
}

func TestFindPublicSamplesRejectsUnpublishedProblem(t *testing.T) {
	problemID := testutil.ID(2)
	notFound := apperr.New(apperr.CodeNotFound, "problem not found", nil)
	reader, err := NewPublicSampleReader(
		&publishedTestsetRepositoryStub{
			err: errors.Join(ports.ErrPublicationNotFound, notFound),
		},
		entity.IdentityAssetDecoder{},
	)
	require.NoError(t, err)

	response, err := reader.FindPublicSamples(context.Background(), problemID)

	require.Nil(t, response)
	require.ErrorIs(t, err, ports.ErrPublicationNotFound)
}

type publishedTestsetRepositoryStub struct {
	testset entity.TestsetRevision
	err     error
}

func (stub *publishedTestsetRepositoryStub) LoadPublishedTestset(
	context.Context,
	uuid.UUID,
) (entity.TestsetRevision, error) {
	return stub.testset, stub.err
}

func publicSampleAsset(content string) entity.TestAsset {
	bytes := []byte(content)
	return entity.TestAsset{
		ID:               uuid.New(),
		SHA256:           sha256.Sum256(bytes),
		Encoding:         problemcontract.AssetEncodingIdentity,
		Content:          bytes,
		UncompressedSize: int64(len(bytes)),
		CreatedAt:        time.Now().UTC(),
	}
}
