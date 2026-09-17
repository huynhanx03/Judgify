package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToSubmissionIDParsesHTTPID(t *testing.T) {
	id := uuid.New()

	actual, err := toSubmissionID(&submissionIDRequest{ID: id.String()})

	require.NoError(t, err)
	require.Equal(t, id, actual)
}

func TestToSubmissionIDRejectsInvalidHTTPID(t *testing.T) {
	_, err := toSubmissionID(&submissionIDRequest{ID: "invalid"})
	require.Error(t, err)
}

func TestToProblemIDParsesNestedHTTPID(t *testing.T) {
	id := uuid.New()

	actual, err := toProblemID(&problemSubmissionsRequest{ProblemID: id.String()})

	require.NoError(t, err)
	require.Equal(t, id, actual)
}
