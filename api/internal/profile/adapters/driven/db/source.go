package db

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	cultivationdb "github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/element"
	"github.com/huynhanx03/judgify/internal/ent/generate/level"
	"github.com/huynhanx03/judgify/internal/ent/generate/rank"
	"github.com/huynhanx03/judgify/internal/ent/generate/userdifficultystats"
	"github.com/huynhanx03/judgify/internal/ent/generate/userelementexp"
	"github.com/huynhanx03/judgify/internal/ent/generate/userstats"
	"github.com/huynhanx03/judgify/internal/ent/generate/usertagstats"
	cultivationports "github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/profile"
)

var (
	ErrInvalidSource     = errors.New("profile storage: invalid source")
	ErrProjectionLimit   = errors.New("profile storage: projection limit exceeded")
	ErrCorruptProjection = errors.New("profile storage: corrupt projection")
)

// Source loads the cross-domain profile read model directly from Ent. It is a
// read adapter, not a domain repository: all rows are bounded, eagerly loaded,
// and converted to neutral profile data before leaving this package.
type Source struct {
	client         *dbent.EntClient
	rewardProfiles cultivationports.RewardProfileRepository
}

func NewSource(client *dbent.EntClient) (*Source, error) {
	if client == nil || client.Client() == nil {
		return nil, ErrInvalidSource
	}
	rewardProfiles, err := cultivationdb.NewRewardProfileRepository(client)
	if err != nil {
		return nil, ErrInvalidSource
	}
	return &Source{client: client, rewardProfiles: rewardProfiles}, nil
}

func (source *Source) Load(
	ctx context.Context,
	userID uuid.UUID,
) (profile.Projection, error) {
	if source == nil || source.client == nil || ctx == nil || userID == uuid.Nil {
		return profile.Projection{}, ErrInvalidSource
	}
	if err := ctx.Err(); err != nil {
		return profile.Projection{}, err
	}

	client := source.client.DB(ctx)
	stats, err := client.UserStats.Query().
		Where(userstats.UserID(userID)).
		Only(ctx)
	if generate.IsNotFound(err) {
		return profile.Projection{}, profile.ErrProfileNotProvisioned
	}
	if err != nil {
		return profile.Projection{}, fmt.Errorf("load user stats: %w", err)
	}
	if stats.UserID != userID || stats.TotalExp < 0 || stats.TotalSubmissions < 0 ||
		stats.AcceptedCount < 0 || stats.AcceptedCount > stats.TotalSubmissions {
		return profile.Projection{}, corrupt("invalid user stats")
	}

	projection := profile.Projection{
		Stats: profile.Stats{
			TotalExp:         stats.TotalExp,
			Rating:           stats.Rating,
			TotalSubmissions: stats.TotalSubmissions,
			AcceptedCount:    stats.AcceptedCount,
		},
	}
	if projection.RewardProfile, err = loadRewardProfile(
		ctx, source.rewardProfiles, userID,
	); err != nil {
		return profile.Projection{}, err
	}
	if projection.Elements, err = loadElements(ctx, client, userID); err != nil {
		return profile.Projection{}, err
	}
	if projection.Levels, err = loadLevels(ctx, client); err != nil {
		return profile.Projection{}, err
	}
	if projection.Ranks, err = loadRanks(ctx, client); err != nil {
		return profile.Projection{}, err
	}
	if projection.DifficultyStats, err = loadDifficultyStats(ctx, client, userID); err != nil {
		return profile.Projection{}, err
	}
	if projection.TagStats, err = loadTagStats(ctx, client, userID); err != nil {
		return profile.Projection{}, err
	}
	return projection, nil
}

func loadRewardProfile(
	ctx context.Context,
	repository cultivationports.RewardProfileRepository,
	userID uuid.UUID,
) (profile.RewardProfile, error) {
	stored, err := repository.Current(ctx, userID)
	if err != nil {
		return profile.RewardProfile{}, fmt.Errorf("load reward profile: %w", err)
	}
	if !stored.Valid() || len(stored.Traits) != profile.MaximumTraits {
		return profile.RewardProfile{}, corrupt("invalid reward profile")
	}
	result := profile.RewardProfile{
		ID:          stored.ID,
		Revision:    stored.Revision,
		Checksum:    hex.EncodeToString(stored.AssignmentChecksum[:]),
		Source:      stored.Source,
		EffectiveAt: stored.EffectiveAt,
		Traits:      make([]profile.Trait, len(stored.Traits)),
	}
	for index, assignment := range stored.Traits {
		effect := assignment.EffectRevision
		result.Traits[index] = profile.Trait{
			ID:                assignment.TraitID,
			Code:              assignment.TraitCode,
			Type:              assignment.TraitType,
			Name:              assignment.TraitName,
			RarityCode:        assignment.RarityCode,
			RarityName:        assignment.RarityName,
			Description:       assignment.Description,
			Slot:              assignment.Slot,
			SlotOrder:         assignment.SlotOrder,
			EffectRevisionID:  effect.ID,
			EffectRevision:    effect.Revision,
			EffectChecksum:    hex.EncodeToString(effect.Checksum[:]),
			EffectEffectiveAt: effect.EffectiveAt,
			Effect: profile.TraitEffect{
				Version:                    effect.Effect.Version,
				Kind:                       string(effect.Effect.Kind),
				Scope:                      string(effect.Effect.Scope),
				ElementCodes:               append([]string(nil), effect.Effect.ElementCodes...),
				MultiplierDeltaBasisPoints: cloneProfileInt64(effect.Effect.MultiplierDeltaBasisPoints),
				FlatBonus:                  cloneProfileInt64(effect.Effect.FlatBonus),
			},
		}
	}
	return result, nil
}

func loadElements(
	ctx context.Context,
	client *generate.Client,
	userID uuid.UUID,
) ([]profile.ElementExp, error) {
	rows, err := client.UserElementExp.Query().
		Where(userelementexp.UserID(userID)).
		WithElement().
		Order(userelementexp.ByElementID()).
		Limit(profile.MaximumElements + 1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load user element experience: %w", err)
	}
	if err := bounded("elements", len(rows), profile.MaximumElements); err != nil {
		return nil, err
	}

	result := make([]profile.ElementExp, 0, len(rows))
	for _, row := range rows {
		display, edgeErr := row.Edges.ElementOrErr()
		if edgeErr != nil || display == nil {
			return nil, corruptWith("missing element edge", edgeErr)
		}
		if row.Exp < 0 || blank(display.Code) || blank(display.Name) {
			return nil, corrupt("invalid element experience")
		}
		result = append(result, profile.ElementExp{
			Code: display.Code,
			Name: display.Name,
			Exp:  row.Exp,
		})
	}
	return result, nil
}

func loadLevels(ctx context.Context, client *generate.Client) ([]profile.Level, error) {
	rows, err := client.Level.Query().
		Order(level.ByMinExp(), level.ByID()).
		Limit(profile.MaximumLevels + 1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load profile levels: %w", err)
	}
	if err := bounded("levels", len(rows), profile.MaximumLevels); err != nil {
		return nil, err
	}
	result := make([]profile.Level, len(rows))
	for index, row := range rows {
		if row.MinExp < 0 || blank(row.Name) {
			return nil, corrupt("invalid level")
		}
		result[index] = profile.Level{Name: row.Name, MinExp: row.MinExp}
	}
	return result, nil
}

func loadRanks(ctx context.Context, client *generate.Client) ([]profile.Rank, error) {
	rows, err := client.Rank.Query().
		Order(rank.ByMinRating(), rank.ByID()).
		Limit(profile.MaximumRanks + 1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load profile ranks: %w", err)
	}
	if err := bounded("ranks", len(rows), profile.MaximumRanks); err != nil {
		return nil, err
	}
	result := make([]profile.Rank, len(rows))
	for index, row := range rows {
		if blank(row.Name) {
			return nil, corrupt("invalid rank")
		}
		result[index] = profile.Rank{Name: row.Name, MinRating: row.MinRating}
	}
	return result, nil
}

func loadDifficultyStats(
	ctx context.Context,
	client *generate.Client,
	userID uuid.UUID,
) ([]profile.DifficultyStat, error) {
	rows, err := client.UserDifficultyStats.Query().
		Where(userdifficultystats.UserID(userID)).
		WithDifficulty().
		Order(userdifficultystats.ByDifficultyID()).
		Limit(profile.MaximumDifficultyStats + 1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load profile difficulty stats: %w", err)
	}
	if err := bounded("difficulty stats", len(rows), profile.MaximumDifficultyStats); err != nil {
		return nil, err
	}
	result := make([]profile.DifficultyStat, 0, len(rows))
	for _, row := range rows {
		difficulty, edgeErr := row.Edges.DifficultyOrErr()
		if edgeErr != nil || difficulty == nil {
			return nil, corruptWith("missing difficulty edge", edgeErr)
		}
		if row.SolvedCount < 0 || difficulty.Level <= 0 || blank(difficulty.Name) {
			return nil, corrupt("invalid difficulty stats")
		}
		result = append(result, profile.DifficultyStat{
			Name:        difficulty.Name,
			Level:       difficulty.Level,
			SolvedCount: row.SolvedCount,
		})
	}
	return result, nil
}

func loadTagStats(
	ctx context.Context,
	client *generate.Client,
	userID uuid.UUID,
) ([]profile.TagStat, error) {
	rows, err := client.UserTagStats.Query().
		Where(usertagstats.UserID(userID)).
		WithTag(func(query *generate.TagQuery) {
			query.WithElements(func(elements *generate.ElementQuery) {
				elements.Order(element.ByCode(), element.ByID()).
					Limit(profile.MaximumTagElements + 1)
			})
		}).
		Order(usertagstats.ByTagID()).
		Limit(profile.MaximumTagStats + 1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load profile tag stats: %w", err)
	}
	if err := bounded("tag stats", len(rows), profile.MaximumTagStats); err != nil {
		return nil, err
	}

	result := make([]profile.TagStat, 0, len(rows))
	totalElements := 0
	for _, row := range rows {
		tag, edgeErr := row.Edges.TagOrErr()
		if edgeErr != nil || tag == nil {
			return nil, corruptWith("missing tag edge", edgeErr)
		}
		elements, edgeErr := tag.Edges.ElementsOrErr()
		if edgeErr != nil {
			return nil, corruptWith("missing tag element edges", edgeErr)
		}
		if row.SolvedCount < 0 || blank(tag.Name) {
			return nil, corrupt("invalid tag stats")
		}
		totalElements += len(elements)
		if totalElements > profile.MaximumTagElements {
			return nil, limit("tag elements")
		}
		displays := make([]profile.ElementBrief, len(elements))
		for index, item := range elements {
			if item == nil || blank(item.Code) || blank(item.Name) {
				return nil, corrupt("invalid tag element")
			}
			displays[index] = profile.ElementBrief{Code: item.Code, Name: item.Name}
		}
		result = append(result, profile.TagStat{
			Name:        tag.Name,
			SolvedCount: row.SolvedCount,
			Elements:    displays,
		})
	}
	return result, nil
}

func bounded(name string, count, maximum int) error {
	if count <= maximum {
		return nil
	}
	return limit(name)
}

func limit(name string) error {
	return fmt.Errorf("%w: %s", ErrProjectionLimit, name)
}

func corrupt(detail string) error {
	return fmt.Errorf("%w: %s", ErrCorruptProjection, detail)
}

func corruptWith(detail string, cause error) error {
	if cause == nil {
		return corrupt(detail)
	}
	return fmt.Errorf("%w: %s: %v", ErrCorruptProjection, detail, cause)
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func cloneProfileInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

var _ profile.Source = (*Source)(nil)
