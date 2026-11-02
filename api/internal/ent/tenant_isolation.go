package ent

import (
	"context"
	"errors"
	"fmt"

	"entgo.io/ent"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedintercept "github.com/huynhanx03/judgify/internal/ent/generate/intercept"
	"github.com/huynhanx03/judgify/internal/tenantctx"
)

var ErrTenantScopeRequired = errors.New("tenant scope required for tenant-owned data")

var tenantOwnedTypes = map[string]struct{}{
	"AttributeDefinition":              {},
	"AttributeDefinitionRevision":      {},
	"ContentRenderArtifact":            {},
	"ContentRenderSource":              {},
	"Contest":                          {},
	"ContestAnnouncement":              {},
	"ContestClarification":             {},
	"ContestContentCommand":            {},
	"ContestContentRevision":           {},
	"ContestDraftProblem":              {},
	"ContestProblem":                   {},
	"ContestProblemResult":             {},
	"ContestRatingResult":              {},
	"ContestRatingRun":                 {},
	"ContestRegistration":              {},
	"ContestRegistrationTransition":    {},
	"ContestScoringAttempt":            {},
	"ContestStanding":                  {},
	"ContestTransition":                {},
	"ContestUserRating":                {},
	"CultivationRankingPrivacyChange":  {},
	"CultivationRankingProjection":     {},
	"CultivationRankingState":          {},
	"JudgeAttempt":                     {},
	"JudgeJob":                         {},
	"JudgeSpec":                        {},
	"JudgeTestResult":                  {},
	"Judgement":                        {},
	"Material":                         {},
	"MaterialCategory":                 {},
	"MaterialRevision":                 {},
	"MaterialSlug":                     {},
	"MaterialRevisionTag":              {},
	"MaterialViewDaily":                {},
	"Notification":                     {},
	"NotificationCampaign":             {},
	"NotificationCampaignConfirmation": {},
	"NotificationCampaignExplicitUser": {},
	"NotificationCampaignRecipient":    {},
	"NotificationCampaignRevision":     {},
	"NotificationInboxState":           {},
	"NotificationPreference":           {},
	"OnboardingTraitOffer":             {},
	"OnboardingTraitOfferTalent":       {},
	"Operation":                        {},
	"OperationBatch":                   {},
	"OperationControlReceipt":          {},
	"Problem":                          {},
	"ProblemRevision":                  {},
	"ProfileSchemaState":               {},
	"ProgressionLedgerEntry":           {},
	"RatingHistory":                    {},
	"RatingProjectionGeneration":       {},
	"RatingRunChain":                   {},
	"RatingRunChainEntry":              {},
	"RatingSequenceSlot":               {},
	"RatingState":                      {},
	"RewardRuleRevision":               {},
	"SolveRewardCursor":                {},
	"SolveRewardProcessedTransition":   {},
	"Submission":                       {},
	"TenantRole":                       {},
	"TestAsset":                        {},
	"TestCase":                         {},
	"TestcaseGroup":                    {},
	"TestcaseRevision":                 {},
	"TestsetRevision":                  {},
	"UserAttributeValue":               {},
	"UserDifficultyStats":              {},
	"UserElementExp":                   {},
	"UserProblemSolve":                 {},
	"UserProblemSolveTransition":       {},
	"UserRewardProfileRevision":        {},
	"UserRewardProfileTrait":           {},
	"UserSolvedProblem":                {},
	"UserStats":                        {},
	"UserTagStats":                     {},
	"UserTrait":                        {},
}

func installTenantIsolation(client *generate.Client) {
	client.Intercept(generatedintercept.Func(func(ctx context.Context, query generatedintercept.Query) error {
		if _, tenantOwned := tenantOwnedTypes[query.Type()]; !tenantOwned {
			return nil
		}
		scope, ok := tenantctx.FromContext(ctx)
		if !ok {
			return ErrTenantScopeRequired
		}
		ids := scope.TenantIDs()
		if scope.IsTenant() {
			id, _ := scope.TenantID()
			query.WhereP(func(s *entsql.Selector) { s.Where(entsql.EQ(s.C("tenant_id"), id)) })
			return nil
		}
		query.WhereP(func(s *entsql.Selector) { s.Where(entsql.In(s.C("tenant_id"), uuidValues(ids)...)) })
		return nil
	}))
	client.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if _, tenantOwned := tenantOwnedTypes[mutation.Type()]; !tenantOwned {
				return next.Mutate(ctx, mutation)
			}
			scope, ok := tenantctx.FromContext(ctx)
			if !ok || !scope.IsTenant() {
				return nil, ErrTenantScopeRequired
			}
			tenantID, _ := scope.TenantID()
			if mutation.Op().Is(ent.OpCreate) {
				if existing, exists := mutation.Field("tenant_id"); exists && existing != tenantID {
					return nil, fmt.Errorf("tenant-owned %s cannot be created for a different tenant", mutation.Type())
				}
				if err := mutation.SetField("tenant_id", tenantID); err != nil {
					return nil, fmt.Errorf("set tenant owner: %w", err)
				}
			} else {
				for _, changed := range mutation.Fields() {
					if changed == "tenant_id" {
						return nil, fmt.Errorf("tenant ownership is immutable")
					}
				}
				where, ok := mutation.(interface {
					WhereP(...func(*entsql.Selector))
				})
				if !ok {
					return nil, fmt.Errorf("tenant-owned %s mutation does not support scoped predicates", mutation.Type())
				}
				where.WhereP(func(s *entsql.Selector) { s.Where(entsql.EQ(s.C("tenant_id"), tenantID)) })
			}
			return next.Mutate(ctx, mutation)
		})
	})
}

func uuidValues(ids []uuid.UUID) []any {
	values := make([]any, len(ids))
	for i, id := range ids {
		values[i] = id
	}
	return values
}
