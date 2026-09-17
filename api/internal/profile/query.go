package profile

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	cultivationentity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
	identityports "github.com/huynhanx03/judgify/internal/ports"
)

var (
	ErrInvalidProfileQuery      = errors.New("profile: invalid query")
	ErrInvalidProfileProjection = errors.New("profile: invalid projection")
)

type Query struct {
	users       identityports.UserService
	source      Source
	transaction tx.Manager
}

func NewQuery(
	users identityports.UserService,
	source Source,
	transaction tx.Manager,
) (*Query, error) {
	if users == nil || source == nil || transaction == nil {
		return nil, ErrInvalidProfileQuery
	}
	return &Query{users: users, source: source, transaction: transaction}, nil
}

// Get builds one all-or-error profile projection in the caller's bounded
// transaction context. Storage failures never become fabricated empty slices.
func (query *Query) Get(
	ctx context.Context,
	userID uuid.UUID,
) (*identitydto.ProfileResponse, error) {
	if query == nil || ctx == nil || userID == uuid.Nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid profile request", ErrInvalidProfileQuery)
	}
	var basic *identitydto.ProfileAttrs
	var projection Projection
	projectionReady := false
	err := query.transaction.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		basic, err = query.users.GetProfile(txCtx, userID)
		if err != nil {
			return err
		}
		projection, err = query.source.Load(txCtx, userID)
		if errors.Is(err, ErrProfileNotProvisioned) {
			return nil
		}
		if err != nil {
			return apperr.New(apperr.CodeInternalServer, "profile unavailable", err)
		}
		if err := validateProjection(projection); err != nil {
			return apperr.New(apperr.CodeInternalServer, "profile unavailable", err)
		}
		projectionReady = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if basic == nil || basic.ID != userID {
		return nil, apperr.New(apperr.CodeInternalServer, "profile unavailable", ErrInvalidProfileQuery)
	}
	if !projectionReady {
		logger.FromContext(ctx).Debug(
			"rich profile game projection is not provisioned",
			zap.String("user_id", userID.String()),
		)
		return buildProfileShell(basic), nil
	}
	return buildProfile(basic, projection), nil
}

func buildProfileShell(basic *identitydto.ProfileAttrs) *identitydto.ProfileResponse {
	return &identitydto.ProfileResponse{
		ProfileState: identitydto.ProfileStateUnprovisioned,
		ID:           basic.ID, Username: basic.Username, FirstName: basic.FirstName,
		LastName: basic.LastName, Gender: basic.Gender, Birthday: basic.Birthday,
		JoinedAt: basic.JoinedAt,
	}
}

func validateProjection(projection Projection) error {
	if projection.Stats.TotalExp < 0 || projection.Stats.TotalSubmissions < 0 ||
		projection.Stats.AcceptedCount < 0 ||
		projection.Stats.AcceptedCount > projection.Stats.TotalSubmissions {
		return invalidProjection("invalid aggregate stats")
	}
	limits := []struct {
		name    string
		count   int
		maximum int
	}{
		{name: "traits", count: len(projection.RewardProfile.Traits), maximum: MaximumTraits},
		{name: "elements", count: len(projection.Elements), maximum: MaximumElements},
		{name: "levels", count: len(projection.Levels), maximum: MaximumLevels},
		{name: "ranks", count: len(projection.Ranks), maximum: MaximumRanks},
		{name: "difficulty stats", count: len(projection.DifficultyStats), maximum: MaximumDifficultyStats},
		{name: "tag stats", count: len(projection.TagStats), maximum: MaximumTagStats},
	}
	for _, limit := range limits {
		if limit.count > limit.maximum {
			return invalidProjection(limit.name + " exceed limit")
		}
	}

	if projection.RewardProfile.ID == uuid.Nil || projection.RewardProfile.Revision < 1 ||
		len(projection.RewardProfile.Checksum) != 64 ||
		(projection.RewardProfile.Source != "onboarding" &&
			projection.RewardProfile.Source != "administrative") ||
		projection.RewardProfile.EffectiveAt.IsZero() ||
		len(projection.RewardProfile.Traits) != MaximumTraits {
		return invalidProjection("invalid reward profile")
	}
	rootBones := 0
	talentOrders := make(map[int]struct{}, 3)
	for _, trait := range projection.RewardProfile.Traits {
		if trait.Type != "root_bone" && trait.Type != "talent" {
			return invalidProjection("invalid trait type")
		}
		if trait.Type == "root_bone" {
			if trait.Slot != "root_bone" || trait.SlotOrder != 0 {
				return invalidProjection("invalid root-bone slot")
			}
			rootBones++
			if rootBones > 1 {
				return invalidProjection("multiple root bones")
			}
		} else {
			if trait.Slot != "talent" || trait.SlotOrder < 0 || trait.SlotOrder > 2 {
				return invalidProjection("invalid talent slot")
			}
			if _, duplicate := talentOrders[trait.SlotOrder]; duplicate {
				return invalidProjection("duplicate talent slot")
			}
			talentOrders[trait.SlotOrder] = struct{}{}
		}
		if trait.ID == uuid.Nil || profileBlank(trait.Code) || profileBlank(trait.Name) ||
			profileBlank(trait.RarityCode) || profileBlank(trait.RarityName) ||
			trait.EffectRevisionID == uuid.Nil || trait.EffectRevision < 1 ||
			len(trait.EffectChecksum) != 64 || trait.EffectEffectiveAt.IsZero() ||
			!validProfileTraitEffect(trait.Effect) {
			return invalidProjection("invalid trait display data")
		}
	}
	if rootBones != 1 || len(talentOrders) != 3 {
		return invalidProjection("incomplete reward profile")
	}
	for _, element := range projection.Elements {
		if element.Exp < 0 || profileBlank(element.Code) || profileBlank(element.Name) {
			return invalidProjection("invalid element experience")
		}
	}
	for _, level := range projection.Levels {
		if level.MinExp < 0 || profileBlank(level.Name) {
			return invalidProjection("invalid level")
		}
	}
	for _, rank := range projection.Ranks {
		if profileBlank(rank.Name) {
			return invalidProjection("invalid rank")
		}
	}
	for _, difficulty := range projection.DifficultyStats {
		if difficulty.Level <= 0 || difficulty.SolvedCount < 0 || profileBlank(difficulty.Name) {
			return invalidProjection("invalid difficulty stats")
		}
	}
	tagElements := 0
	for _, tag := range projection.TagStats {
		if tag.SolvedCount < 0 || profileBlank(tag.Name) {
			return invalidProjection("invalid tag stats")
		}
		if len(tag.Elements) > MaximumTagElements-tagElements {
			return invalidProjection("tag elements exceed limit")
		}
		tagElements += len(tag.Elements)
		for _, element := range tag.Elements {
			if profileBlank(element.Code) || profileBlank(element.Name) {
				return invalidProjection("invalid tag element")
			}
		}
	}
	return nil
}

func invalidProjection(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidProfileProjection, detail)
}

func profileBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func validProfileTraitEffect(value TraitEffect) bool {
	return (cultivationentity.TraitEffectSpec{
		Version:                    value.Version,
		Kind:                       cultivationentity.TraitEffectKind(value.Kind),
		Scope:                      cultivationentity.TraitEffectScope(value.Scope),
		ElementCodes:               append([]string(nil), value.ElementCodes...),
		MultiplierDeltaBasisPoints: cloneProfileInt64Value(value.MultiplierDeltaBasisPoints),
		FlatBonus:                  cloneProfileInt64Value(value.FlatBonus),
	}).Valid()
}

func cloneProfileInt64Value(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func buildProfile(
	basic *identitydto.ProfileAttrs,
	projection Projection,
) *identitydto.ProfileResponse {
	levels := slices.Clone(projection.Levels)
	ranks := slices.Clone(projection.Ranks)
	slices.SortFunc(levels, func(left, right Level) int {
		return compareInt64(left.MinExp, right.MinExp)
	})
	slices.SortFunc(ranks, func(left, right Rank) int {
		return compareInt(left.MinRating, right.MinRating)
	})

	cultivation := identitydto.CultivationInfo{
		TotalExp: projection.Stats.TotalExp,
		Rating:   projection.Stats.Rating,
		Level:    resolveLevel(levels, projection.Stats.TotalExp),
		Rank:     resolveRank(ranks, projection.Stats.Rating),
		RewardProfile: &identitydto.RewardProfileInfo{
			ID:          projection.RewardProfile.ID,
			Revision:    projection.RewardProfile.Revision,
			Checksum:    projection.RewardProfile.Checksum,
			Source:      projection.RewardProfile.Source,
			EffectiveAt: projection.RewardProfile.EffectiveAt.UTC().Format(time.RFC3339Nano),
		},
		Talents:  make([]identitydto.TraitInfo, 0, 3),
		Elements: make([]identitydto.ElementExp, 0, len(projection.Elements)),
	}
	for _, trait := range projection.RewardProfile.Traits {
		info := identitydto.TraitInfo{
			ID: trait.ID, Code: trait.Code, Type: trait.Type,
			Name: trait.Name, RarityCode: trait.RarityCode, RarityName: trait.RarityName,
			Description: trait.Description, Slot: trait.Slot, SlotOrder: trait.SlotOrder,
			EffectRevisionID:  trait.EffectRevisionID,
			EffectRevision:    trait.EffectRevision,
			EffectChecksum:    trait.EffectChecksum,
			EffectEffectiveAt: trait.EffectEffectiveAt.UTC().Format(time.RFC3339Nano),
			Effect: identitydto.TraitEffectInfo{
				Version:                    trait.Effect.Version,
				Kind:                       trait.Effect.Kind,
				Scope:                      trait.Effect.Scope,
				ElementCodes:               append([]string(nil), trait.Effect.ElementCodes...),
				MultiplierDeltaBasisPoints: cloneProfileInt64Value(trait.Effect.MultiplierDeltaBasisPoints),
				FlatBonus:                  cloneProfileInt64Value(trait.Effect.FlatBonus),
			},
		}
		if trait.Type == "root_bone" {
			copy := info
			cultivation.RootBone = &copy
			continue
		}
		cultivation.Talents = append(cultivation.Talents, info)
	}
	for _, element := range projection.Elements {
		cultivation.Elements = append(cultivation.Elements, identitydto.ElementExp{
			Code: element.Code, Name: element.Name, Exp: element.Exp,
		})
	}

	problemStats := identitydto.ProblemStats{
		TotalSubmissions: projection.Stats.TotalSubmissions,
		AcceptedCount:    projection.Stats.AcceptedCount,
		ByDifficulty:     make([]identitydto.DiffStat, 0, len(projection.DifficultyStats)),
		ByTag:            make([]identitydto.TagStat, 0, len(projection.TagStats)),
	}
	for _, difficulty := range projection.DifficultyStats {
		problemStats.ByDifficulty = append(problemStats.ByDifficulty, identitydto.DiffStat{
			Name: difficulty.Name, Level: difficulty.Level, SolvedCount: difficulty.SolvedCount,
		})
	}
	for _, tag := range projection.TagStats {
		elements := make([]identitydto.ElementBrief, len(tag.Elements))
		for index, element := range tag.Elements {
			elements[index] = identitydto.ElementBrief{Code: element.Code, Name: element.Name}
		}
		problemStats.ByTag = append(problemStats.ByTag, identitydto.TagStat{
			Name: tag.Name, SolvedCount: tag.SolvedCount, Elements: elements,
		})
	}
	return &identitydto.ProfileResponse{
		ProfileState: identitydto.ProfileStateReady,
		ID:           basic.ID, Username: basic.Username, FirstName: basic.FirstName,
		LastName: basic.LastName, Gender: basic.Gender, Birthday: basic.Birthday,
		JoinedAt: basic.JoinedAt, Cultivation: &cultivation, ProblemStats: &problemStats,
	}
}

func resolveLevel(levels []Level, totalExp int64) identitydto.LevelInfo {
	if len(levels) == 0 {
		return identitydto.LevelInfo{Name: "—"}
	}
	index := 0
	for candidate, level := range levels {
		if level.MinExp > totalExp {
			break
		}
		index = candidate
	}
	current := levels[index]
	info := identitydto.LevelInfo{Name: current.Name, TierIndex: index}
	if index+1 >= len(levels) {
		info.Progress = 100
		return info
	}
	next := levels[index+1]
	info.NextName = next.Name
	info.NextTierIndex = index + 1
	difference := next.MinExp - current.MinExp
	if difference > 0 {
		progress := float64(totalExp-current.MinExp) / float64(difference) * 100
		info.Progress = math.Max(0, math.Min(progress, 100))
	}
	info.ExpToNext = max(0, next.MinExp-totalExp)
	return info
}

func resolveRank(ranks []Rank, rating int) identitydto.RankInfo {
	if len(ranks) == 0 {
		return identitydto.RankInfo{Name: "—"}
	}
	index := 0
	for candidate, rank := range ranks {
		if rank.MinRating > rating {
			break
		}
		index = candidate
	}
	current := ranks[index]
	info := identitydto.RankInfo{Name: current.Name, TierIndex: index}
	if index+1 >= len(ranks) {
		info.Progress = 100
		return info
	}
	next := ranks[index+1]
	info.NextName = next.Name
	info.NextTierIndex = index + 1
	difference := next.MinRating - current.MinRating
	if difference > 0 {
		progress := float64(rating-current.MinRating) / float64(difference) * 100
		info.Progress = math.Max(0, math.Min(progress, 100))
	}
	info.RatingToNext = max(0, next.MinRating-rating)
	return info
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
