package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestMapProblemCreatePersistenceErrorClassifiesOnlyTheSlugUniqueConstraint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		constraint       string
		wrapped          bool
		wantSlugConflict bool
	}{
		{
			name:             "stable slug",
			constraint:       "problems_slug_key",
			wantSlugConflict: true,
		},
		{
			name:             "wrapped stable slug",
			constraint:       "problems_slug_key",
			wrapped:          true,
			wantSlugConflict: true,
		},
		{
			name:       "other unique constraint",
			constraint: "problems_pkey",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var persistenceError error = &pq.Error{
				Code:       "23505",
				Constraint: test.constraint,
			}
			if test.wrapped {
				persistenceError = fmt.Errorf("save problem: %w", persistenceError)
			}
			err := mapProblemCreatePersistenceError(persistenceError)

			require.Equal(
				t,
				test.wantSlugConflict,
				errors.Is(err, ports.ErrProblemSlugConflict),
			)
			var applicationError *apperr.AppError
			require.ErrorAs(t, err, &applicationError)
		})
	}
}

func TestMapProblemCreatePersistenceErrorDoesNotClassifyNonUniqueSlugConstraint(t *testing.T) {
	t.Parallel()

	err := mapProblemCreatePersistenceError(&pq.Error{
		Code:       "23514",
		Constraint: "problems_slug_key",
	})

	require.NotErrorIs(t, err, ports.ErrProblemSlugConflict)
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
}
