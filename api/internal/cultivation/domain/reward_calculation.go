package domain

import (
	"errors"
	"math"

	"github.com/google/uuid"
)

const maximumCombinedRewardMultiplier = int64(1_000_000)

var ErrRewardCalculation = errors.New("cultivation: reward calculation failed")

type RewardElementInput struct {
	ElementID         uuid.UUID
	WeightBasisPoints int
}

type RewardElementResult struct {
	ElementID  uuid.UUID `json:"element_id"`
	Experience int64     `json:"experience"`
}

type CalculatedSolveReward struct {
	TotalExperience int64                 `json:"total_experience"`
	Elements        []RewardElementResult `json:"elements"`
}

// CalculateSolveReward implements formula v1 with checked integers and one
// half-away-from-zero rounding step per result. Element mapping weight is part
// of its immutable Problem input and is applied in the same rational product,
// avoiding double-rounding drift.
func CalculateSolveReward(
	baseExperience int64,
	rule RewardRuleRevision,
	profile UserRewardProfileRevision,
	elements []RewardElementInput,
) (CalculatedSolveReward, error) {
	if baseExperience < 0 || !rule.Valid() || !profile.Valid() || len(elements) > 256 {
		return CalculatedSolveReward{}, rewardCalculationError(profile)
	}
	seenElements := make(map[uuid.UUID]struct{}, len(elements))
	totalWeight := 0
	for _, element := range elements {
		if element.ElementID == uuid.Nil || element.WeightBasisPoints < 1 ||
			element.WeightBasisPoints > int(RewardBasisPoints) {
			return CalculatedSolveReward{}, ErrRewardCalculation
		}
		if _, duplicate := seenElements[element.ElementID]; duplicate {
			return CalculatedSolveReward{}, ErrRewardCalculation
		}
		seenElements[element.ElementID] = struct{}{}
		totalWeight += element.WeightBasisPoints
	}
	if len(elements) > 0 && totalWeight != int(RewardBasisPoints) {
		return CalculatedSolveReward{}, ErrRewardCalculation
	}

	totalMultiplier, totalBonus, err := rewardModifiers(
		rule, profile, func(revision TraitEffectRevision) bool {
			if revision.Effect.Scope == TraitEffectScopeAll {
				return true
			}
			for _, target := range revision.TargetElementIDs {
				if _, matched := seenElements[target]; matched {
					return true
				}
			}
			return false
		},
	)
	if err != nil {
		return CalculatedSolveReward{}, err
	}
	total, err := checkedRoundedProduct(baseExperience, totalMultiplier, RewardBasisPoints)
	if err != nil {
		return CalculatedSolveReward{}, err
	}
	total, err = checkedRewardBonus(total, totalBonus)
	if err != nil {
		return CalculatedSolveReward{}, err
	}

	result := CalculatedSolveReward{
		TotalExperience: total,
		Elements:        make([]RewardElementResult, 0, len(elements)),
	}
	for _, element := range elements {
		multiplier, bonus, modifierErr := rewardModifiers(
			rule, profile, func(revision TraitEffectRevision) bool {
				if revision.Effect.Scope == TraitEffectScopeAll {
					return true
				}
				for _, target := range revision.TargetElementIDs {
					if target == element.ElementID {
						return true
					}
				}
				return false
			},
		)
		if modifierErr != nil {
			return CalculatedSolveReward{}, modifierErr
		}
		experience, modifierErr := checkedRoundedTripleProduct(
			baseExperience, int64(element.WeightBasisPoints), multiplier,
			RewardBasisPoints*RewardBasisPoints,
		)
		if modifierErr != nil {
			return CalculatedSolveReward{}, modifierErr
		}
		experience, modifierErr = checkedRewardBonus(experience, bonus)
		if modifierErr != nil {
			return CalculatedSolveReward{}, modifierErr
		}
		result.Elements = append(result.Elements, RewardElementResult{
			ElementID: element.ElementID, Experience: experience,
		})
	}
	return result, nil
}

func rewardModifiers(
	rule RewardRuleRevision,
	profile UserRewardProfileRevision,
	matched func(TraitEffectRevision) bool,
) (int64, int64, error) {
	multiplier := rule.Configuration.MultiplierBasisPoints
	bonus := rule.Configuration.FlatBonus
	for _, assignment := range profile.Traits {
		revision := assignment.EffectRevision
		if !matched(revision) {
			continue
		}
		switch revision.Effect.Kind {
		case TraitEffectExperienceMultiplier:
			if revision.Effect.MultiplierDeltaBasisPoints == nil ||
				addOverflows(multiplier, *revision.Effect.MultiplierDeltaBasisPoints) {
				return 0, 0, ErrRewardCalculation
			}
			multiplier += *revision.Effect.MultiplierDeltaBasisPoints
		case TraitEffectExperienceBonus:
			if revision.Effect.FlatBonus == nil || addOverflows(bonus, *revision.Effect.FlatBonus) {
				return 0, 0, ErrRewardCalculation
			}
			bonus += *revision.Effect.FlatBonus
		default:
			return 0, 0, ErrRewardCalculation
		}
	}
	if multiplier < 0 || multiplier > maximumCombinedRewardMultiplier {
		return 0, 0, ErrRewardCalculation
	}
	return multiplier, bonus, nil
}

func checkedRoundedProduct(value, multiplier, denominator int64) (int64, error) {
	if value < 0 || multiplier < 0 || denominator <= 0 ||
		(multiplier != 0 && value > math.MaxInt64/multiplier) {
		return 0, ErrRewardCalculation
	}
	return roundHalfAwayFromZero(value*multiplier, denominator), nil
}

func checkedRoundedTripleProduct(
	first, second, third, denominator int64,
) (int64, error) {
	if first < 0 || second < 0 || third < 0 || denominator <= 0 ||
		(second != 0 && first > math.MaxInt64/second) {
		return 0, ErrRewardCalculation
	}
	product := first * second
	if third != 0 && product > math.MaxInt64/third {
		return 0, ErrRewardCalculation
	}
	return roundHalfAwayFromZero(product*third, denominator), nil
}

func roundHalfAwayFromZero(numerator, denominator int64) int64 {
	quotient := numerator / denominator
	remainder := numerator % denominator
	if remainder*2 >= denominator {
		return quotient + 1
	}
	if remainder*2 <= -denominator {
		return quotient - 1
	}
	return quotient
}

func checkedRewardBonus(value, bonus int64) (int64, error) {
	if addOverflows(value, bonus) {
		return 0, ErrRewardCalculation
	}
	value += bonus
	if value < 0 {
		return 0, nil
	}
	return value, nil
}

func addOverflows(left, right int64) bool {
	return (right > 0 && left > math.MaxInt64-right) ||
		(right < 0 && left < math.MinInt64-right)
}

func rewardCalculationError(profile UserRewardProfileRevision) error {
	if !profile.Valid() {
		return ErrInvalidRewardProfile
	}
	return ErrRewardCalculation
}
