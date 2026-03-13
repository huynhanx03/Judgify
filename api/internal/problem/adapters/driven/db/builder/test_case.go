package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// BuildCreateTestCase builds the create mutation for TestCase entity.
func BuildCreateTestCase(ctx context.Context, e *entity.TestCase) *generate.TestCaseCreate {
	return global.EntClient.DB(ctx).TestCase.Create().
		SetProblemID(e.ProblemID).
		SetInput(e.Input).
		SetExpectedOutput(e.ExpectedOutput).
		SetIsSample(e.IsSample).
		SetOrderIndex(e.OrderIndex)
}

// BuildUpdateTestCase builds the update mutation for TestCase entity.
func BuildUpdateTestCase(ctx context.Context, e *entity.TestCase) *generate.TestCaseUpdateOne {
	return global.EntClient.DB(ctx).TestCase.UpdateOneID(e.ID).
		SetInput(e.Input).
		SetExpectedOutput(e.ExpectedOutput).
		SetIsSample(e.IsSample).
		SetOrderIndex(e.OrderIndex)
}
