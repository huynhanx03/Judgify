package domain

import (
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/pkg/judging"
)

const (
	DefaultAdminSubmissionPageSize = 25
	MaximumAdminSubmissionPageSize = 100
	// Bound offset scans on the administrative endpoint. Operational exports
	// belong on a separate asynchronous surface, not an arbitrarily deep page.
	MaximumAdminSubmissionPage = 10_000
)

var ErrInvalidAdminSubmissionQuery = errors.New("submission: invalid admin query")

// AdminSubmissionQuery is the closed, typed filter set supported by the
// monitoring read model. It deliberately cannot express arbitrary columns,
// operators, or ordering supplied by an HTTP client.
type AdminSubmissionQuery struct {
	Page      int
	PageSize  int
	Status    string
	Language  string
	ProblemID *uuid.UUID
	UserID    *uuid.UUID
	ContestID *uuid.UUID
}

// NormalizeAdminSubmissionQuery applies safe defaults and validates every
// value before it can reach persistence. Repository adapters validate again.
func NormalizeAdminSubmissionQuery(query AdminSubmissionQuery) (AdminSubmissionQuery, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = DefaultAdminSubmissionPageSize
	}
	query.Status = strings.TrimSpace(query.Status)
	query.Language = strings.TrimSpace(query.Language)
	if err := query.Validate(); err != nil {
		return AdminSubmissionQuery{}, err
	}
	return query, nil
}

func (query AdminSubmissionQuery) Validate() error {
	if query.Page < 1 || query.Page > MaximumAdminSubmissionPage ||
		query.PageSize < 1 || query.PageSize > MaximumAdminSubmissionPageSize {
		return ErrInvalidAdminSubmissionQuery
	}
	if query.Status != "" && !IsSubmissionStatus(query.Status) {
		return ErrInvalidAdminSubmissionQuery
	}
	if query.Language != "" {
		if _, supported := judging.ParseLanguage(query.Language); !supported {
			return ErrInvalidAdminSubmissionQuery
		}
	}
	for _, id := range []*uuid.UUID{query.ProblemID, query.UserID, query.ContestID} {
		if id != nil && *id == uuid.Nil {
			return ErrInvalidAdminSubmissionQuery
		}
	}
	return nil
}

func (query AdminSubmissionQuery) Offset() int {
	return (query.Page - 1) * query.PageSize
}

// IsSubmissionStatus owns validation of the complete persisted submission
// state set without exposing the mutable map used by verdict validation.
func IsSubmissionStatus(status string) bool {
	switch status {
	case "pending", "judging", "accepted", "wrong_answer", "compile_error",
		"runtime_error", "time_limit_exceeded", "memory_limit_exceeded",
		"process_limit_exceeded", "output_limit_exceeded", "internal_error":
		return true
	default:
		return false
	}
}
