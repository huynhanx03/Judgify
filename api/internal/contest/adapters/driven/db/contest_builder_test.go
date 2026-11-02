package db

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

func TestBuildUpdateContestNeverMutatesLifecycleStatus(t *testing.T) {
	update := BuildUpdateContest(generate.NewClient(), &entity.Contest{
		ID:     uuid.New(),
		Status: "running",
	})

	_, statusWasMutated := update.Mutation().Status()
	require.False(t, statusWasMutated, "generic contest updates must not own lifecycle transitions")
}
