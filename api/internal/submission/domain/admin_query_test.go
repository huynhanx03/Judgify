package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeAdminSubmissionQueryAppliesBoundedDefaults(t *testing.T) {
	query, err := NormalizeAdminSubmissionQuery(AdminSubmissionQuery{
		Status:   " accepted ",
		Language: " go ",
	})
	if err != nil {
		t.Fatalf("normalize query: %v", err)
	}
	if query.Page != 1 || query.PageSize != DefaultAdminSubmissionPageSize ||
		query.Status != "accepted" || query.Language != "go" {
		t.Fatalf("unexpected normalized query: %+v", query)
	}
}

func TestAdminSubmissionQueryRejectsUnsafeValues(t *testing.T) {
	nilID := uuid.Nil
	tests := []AdminSubmissionQuery{
		{Page: -1, PageSize: 25},
		{Page: MaximumAdminSubmissionPage + 1, PageSize: 25},
		{Page: 1, PageSize: MaximumAdminSubmissionPageSize + 1},
		{Page: 1, PageSize: 25, Status: "unknown"},
		{Page: 1, PageSize: 25, Language: "rust"},
		{Page: 1, PageSize: 25, ProblemID: &nilID},
		{Page: 1, PageSize: 25, UserID: &nilID},
		{Page: 1, PageSize: 25, ContestID: &nilID},
	}
	for _, query := range tests {
		if err := query.Validate(); !errors.Is(err, ErrInvalidAdminSubmissionQuery) {
			t.Fatalf("query %+v: error = %v", query, err)
		}
	}
}

func TestAdminSubmissionQueryOffsetUsesValidatedWindow(t *testing.T) {
	query := AdminSubmissionQuery{Page: 3, PageSize: 25}
	if err := query.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if offset := query.Offset(); offset != 50 {
		t.Fatalf("offset = %d", offset)
	}
}

func TestAdminSubmissionQueryAcceptsProcessLimitVerdict(t *testing.T) {
	query := AdminSubmissionQuery{
		Page: 1, PageSize: 25, Status: "process_limit_exceeded",
	}
	if err := query.Validate(); err != nil {
		t.Fatalf("process-limit status must remain queryable: %v", err)
	}
}
