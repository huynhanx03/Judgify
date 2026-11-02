package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/testutil"
)

func TestToProblemEntityPreservesEagerLoadedTagVersion(t *testing.T) {
	now := time.Now().UTC()
	model := &generate.Problem{
		ID: testutil.ID(1),
		Edges: generate.ProblemEdges{
			Tags: []*generate.Tag{{
				ID:        testutil.ID(2),
				Name:      "graphs",
				Version:   3,
				CreatedAt: now,
				UpdatedAt: now,
			}},
		},
	}

	entity := ToProblemEntity(model)
	require.Len(t, entity.Tags, 1)
	require.EqualValues(t, 3, entity.Tags[0].Version)
}
