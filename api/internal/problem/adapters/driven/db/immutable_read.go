package db

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/difficultyrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ent/generate/problemrevision"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// LoadImmutableProblem resolves only exact, caller-supplied immutable
// identities. It intentionally ignores the aggregate's current published
// pointer so a contest remains stable after a later problem publication or
// archive.
func (repository *RevisionRepository) LoadImmutableProblem(
	ctx context.Context,
	problemID uuid.UUID,
	problemRevisionID uuid.UUID,
	testsetRevisionID uuid.UUID,
) (ports.ImmutableProblemProjection, error) {
	if invalidRevisionRepository(repository) || ctx == nil ||
		problemID == uuid.Nil || problemRevisionID == uuid.Nil ||
		testsetRevisionID == uuid.Nil {
		return ports.ImmutableProblemProjection{},
			ports.ErrInvalidImmutableProblem
	}
	if err := ctx.Err(); err != nil {
		return ports.ImmutableProblemProjection{}, err
	}

	stable, err := repository.client.Problem.Query().
		Where(problem.ID(problemID)).
		Only(ctx)
	if err != nil {
		return ports.ImmutableProblemProjection{},
			mapImmutableProblemReadError(err)
	}
	if stable.ID != problemID ||
		!entity.ValidProblemSlug(stable.Slug) ||
		stable.TagMappingID == uuid.Nil ||
		stable.ElementMappingID == uuid.Nil ||
		stable.SubmissionCount < 0 ||
		stable.AcceptedCount < 0 ||
		stable.AcceptedCount > stable.SubmissionCount {
		return ports.ImmutableProblemProjection{},
			ports.ErrInvalidImmutableProblem
	}

	revisionRecord, err := repository.client.ProblemRevision.Query().
		Where(
			problemrevision.ID(problemRevisionID),
			problemrevision.ProblemID(problemID),
			problemrevision.TestsetRevisionID(testsetRevisionID),
		).
		Only(ctx)
	if err != nil {
		return ports.ImmutableProblemProjection{},
			mapImmutableProblemReadError(err)
	}
	revision, err := revisionFromRecord(revisionRecord)
	if err != nil {
		return ports.ImmutableProblemProjection{}, errors.Join(
			ports.ErrInvalidImmutableProblem,
			err,
		)
	}
	if revision.ID != problemRevisionID ||
		revision.ProblemID != problemID ||
		revision.TestsetRevisionID != testsetRevisionID {
		return ports.ImmutableProblemProjection{},
			ports.ErrInvalidImmutableProblem
	}
	aggregate := entity.ProblemAggregate{
		ID:               stable.ID,
		TagMappingID:     stable.TagMappingID,
		ElementMappingID: stable.ElementMappingID,
	}
	if err := verifyRevisionRewardMappingOwnership(
		aggregate,
		revision,
	); err != nil {
		return ports.ImmutableProblemProjection{}, errors.Join(
			ports.ErrInvalidImmutableProblem,
			err,
		)
	}

	difficulty, tags, err := repository.loadImmutableTaxonomy(
		ctx,
		aggregate,
		revision.Reward,
	)
	if err != nil {
		return ports.ImmutableProblemProjection{}, err
	}
	testset, err := loadTestsetRevision(
		ctx,
		repository.client,
		problemID,
		testsetRevisionID,
	)
	if err != nil {
		if errors.Is(err, ports.ErrPublicationNotFound) {
			return ports.ImmutableProblemProjection{},
				ports.ErrImmutableProblemNotFound
		}
		return ports.ImmutableProblemProjection{}, errors.Join(
			ports.ErrInvalidImmutableProblem,
			err,
		)
	}
	samples, err := entity.BuildPublicSamples(
		ctx,
		testset,
		repository.sampleDecoder,
	)
	if err != nil {
		return ports.ImmutableProblemProjection{}, errors.Join(
			ports.ErrInvalidImmutableProblem,
			err,
		)
	}

	return ports.ImmutableProblemProjection{
		ProblemID:         problemID,
		ProblemSlug:       stable.Slug,
		ProblemRevisionID: revision.ID,
		TestsetRevisionID: testset.ID,
		RevisionNumber:    revision.RevisionNumber,
		Title:             revision.Title,
		StatementMarkdown: string(revision.StatementMarkdown),
		Difficulty:        difficulty,
		Tags:              tags,
		Limits:            revision.Limits,
		AllowedRuntimeKeys: append(
			[]string(nil),
			revision.AllowedRuntimeKeys...,
		),
		SubmissionCount: stable.SubmissionCount,
		AcceptedCount:   stable.AcceptedCount,
		CreatedAt:       revision.CreatedAt,
		Samples:         samples,
	}, nil
}

func (repository *RevisionRepository) loadImmutableTaxonomy(
	ctx context.Context,
	aggregate entity.ProblemAggregate,
	snapshot entity.RewardSnapshot,
) (
	ports.ImmutableDifficultyProjection,
	[]ports.ImmutableTagProjection,
	error,
) {
	difficultyRecord, err := repository.client.DifficultyRevision.Query().
		Where(
			difficultyrevision.DifficultyID(snapshot.DifficultyID),
			difficultyrevision.Revision(snapshot.DifficultyRevision),
		).
		Only(ctx)
	if err != nil {
		return ports.ImmutableDifficultyProjection{}, nil,
			mapImmutableProblemReadError(err)
	}
	if err := validateDifficultyRevisionRecord(difficultyRecord); err != nil ||
		!validImmutableLabel(
			difficultyRecord.Name,
			50,
		) ||
		difficultyRecord.Level < 1 {
		return ports.ImmutableDifficultyProjection{}, nil,
			ports.ErrInvalidImmutableProblem
	}

	tagRecords, err := resolveExactHistoricalTagRevisions(
		ctx,
		repository.client,
		snapshot.Tags.Items,
	)
	if err != nil {
		return ports.ImmutableDifficultyProjection{}, nil, errors.Join(
			ports.ErrInvalidImmutableProblem,
			err,
		)
	}
	elementFacts, err := resolveTagRevisionElementFacts(
		ctx,
		repository.client,
		tagRecords,
	)
	if err != nil {
		return ports.ImmutableDifficultyProjection{}, nil, errors.Join(
			ports.ErrInvalidImmutableProblem,
			err,
		)
	}
	tagFacts := make(
		[]entity.TaxonomyRevisionFact,
		len(tagRecords),
	)
	tags := make(
		[]ports.ImmutableTagProjection,
		len(tagRecords),
	)
	for index, record := range tagRecords {
		if record == nil ||
			!validImmutableLabel(record.Name, 50) {
			return ports.ImmutableDifficultyProjection{}, nil,
				ports.ErrInvalidImmutableProblem
		}
		tagFacts[index] = entity.TaxonomyRevisionFact{
			ID:       record.TagID,
			Revision: record.Revision,
		}
		tags[index] = ports.ImmutableTagProjection{
			ID:       record.TagID,
			Revision: record.Revision,
			Name:     record.Name,
		}
	}
	facts := entity.RewardFacts{
		Difficulty: entity.DifficultyRewardFact{
			ID:             difficultyRecord.DifficultyID,
			Revision:       difficultyRecord.Revision,
			BaseExperience: difficultyRecord.ExpReward,
		},
		Tags:     tagFacts,
		Elements: elementFacts,
	}
	identity := entity.RewardMappingIdentity{
		TagMappingID:     aggregate.TagMappingID,
		ElementMappingID: aggregate.ElementMappingID,
	}
	expected, err := entity.BuildRewardSnapshot(
		identity,
		snapshot.Tags.Revision,
		facts,
	)
	if err != nil || !reflect.DeepEqual(expected, snapshot) {
		return ports.ImmutableDifficultyProjection{}, nil,
			ports.ErrInvalidImmutableProblem
	}

	return ports.ImmutableDifficultyProjection{
		ID:          difficultyRecord.DifficultyID,
		Revision:    difficultyRecord.Revision,
		Name:        difficultyRecord.Name,
		Level:       difficultyRecord.Level,
		Description: difficultyRecord.Description,
	}, tags, nil
}

func validImmutableLabel(value string, maximumBytes int) bool {
	return value != "" &&
		len(value) <= maximumBytes &&
		utf8.ValidString(value)
}

func mapImmutableProblemReadError(err error) error {
	if err == nil {
		return nil
	}
	if generate.IsNotFound(err) {
		return ports.ErrImmutableProblemNotFound
	}
	return fmt.Errorf("read immutable problem projection: %w", err)
}
