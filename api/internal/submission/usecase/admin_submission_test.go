package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commondto "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
)

func TestFindAdminUsesDefaultsAndReturnsSafeListProjection(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	createdAt := time.Date(2026, 7, 22, 1, 2, 3, 0, time.UTC)
	repository.EXPECT().FindAdmin(context.Background(), entity.AdminSubmissionQuery{
		Page: 1, PageSize: entity.DefaultAdminSubmissionPageSize,
	}).Return(&commondto.Paginated[*entity.Submission]{
		Records: &[]*entity.Submission{{
			ID: testutil.ID(1), ProblemID: testutil.ID(2), UserID: testutil.ID(3),
			Language: "go", Status: "accepted", SourceCode: "must not leak",
			ErrorMessage: stringPointer("must not leak"), CreatedAt: createdAt,
		}},
		Pagination: &commondto.PaginationMeta{
			CurrentPage: 1, PageSize: entity.DefaultAdminSubmissionPageSize,
			TotalPages: 1, TotalItems: 1,
		},
	}, nil)

	response, err := service.FindAdmin(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, *response.Records, 1)
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "source_code")
	require.NotContains(t, string(encoded), "error_message")
	require.NotContains(t, string(encoded), "must not leak")
	require.NotContains(t, string(encoded), "expected_output")
}

func TestFindAdminNormalizesTypedFilters(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	status, language := " accepted ", " go "
	problemID, userID, contestID := testutil.ID(10), testutil.ID(11), testutil.ID(12)
	repository.EXPECT().FindAdmin(context.Background(), entity.AdminSubmissionQuery{
		Page: 2, PageSize: 50, Status: "accepted", Language: "go",
		ProblemID: &problemID, UserID: &userID, ContestID: &contestID,
	}).Return(&commondto.Paginated[*entity.Submission]{
		Records: &[]*entity.Submission{},
		Pagination: &commondto.PaginationMeta{
			CurrentPage: 2, PageSize: 50, TotalPages: 2,
		},
	}, nil)

	_, err := service.FindAdmin(context.Background(), &contract.AdminSubmissionFindRequest{
		Pagination: &commondto.PaginationOptions{Page: 2, PageSize: 50},
		Status:     &status, Language: &language,
		ProblemID: &problemID, UserID: &userID, ContestID: &contestID,
	})
	require.NoError(t, err)
}

func TestFindAdminRejectsUnknownStatusBeforeRepository(t *testing.T) {
	service, _ := newSubmissionTestEnv(t)
	status := "rejudging"
	response, err := service.FindAdmin(context.Background(), &contract.AdminSubmissionFindRequest{
		Status: &status,
	})
	require.Nil(t, response)
	require.ErrorIs(t, err, entity.ErrInvalidAdminSubmissionQuery)
}

func TestGetAdminReturnsExplicitSourceWithoutJudgeInternals(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	id := testutil.ID(1)
	repository.EXPECT().Get(context.Background(), id).Return(&entity.Submission{
		ID: id, ProblemID: testutil.ID(2), UserID: testutil.ID(3),
		Language: "go", Status: "compile_error", SourceCode: "package main",
		ErrorMessage: stringPointer("compiler diagnostic"),
	}, nil)

	response, err := service.GetAdmin(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "package main", response.SourceCode)
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "SourceCode")
	require.NotContains(t, string(encoded), "judge_bundle")
	require.NotContains(t, string(encoded), "judge_lease")
	require.NotContains(t, string(encoded), "expected_output")
}

func stringPointer(value string) *string { return &value }
