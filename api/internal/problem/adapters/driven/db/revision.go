package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ent/generate/problemrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/testasset"
	"github.com/huynhanx03/judgify/internal/ent/generate/testcase"
	"github.com/huynhanx03/judgify/internal/ent/generate/testcasegroup"
	"github.com/huynhanx03/judgify/internal/ent/generate/testcaserevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/testsetrevision"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var ErrInvalidRevisionRepository = errors.New("problem revision repository: invalid dependency")

// RevisionRepository is the transaction-bound persistence adapter for exact
// problem publication. It never opens a transaction and never returns Ent
// entities across the domain boundary.
type RevisionRepository struct {
	client        *generate.Client
	content       contentrender.Repository
	sampleDecoder entity.AssetDecoder
}

func NewRevisionRepository(
	client *generate.Client,
	content contentrender.Repository,
	sampleDecoders ...entity.AssetDecoder,
) (*RevisionRepository, error) {
	if client == nil || nilRevisionDependency(content) ||
		len(sampleDecoders) > 1 {
		return nil, ErrInvalidRevisionRepository
	}
	var sampleDecoder entity.AssetDecoder = entity.IdentityAssetDecoder{}
	if len(sampleDecoders) == 1 {
		if nilRevisionDependency(sampleDecoders[0]) {
			return nil, ErrInvalidRevisionRepository
		}
		sampleDecoder = sampleDecoders[0]
	}
	return &RevisionRepository{
		client:        client,
		content:       content,
		sampleDecoder: sampleDecoder,
	}, nil
}

func invalidRevisionRepository(repository *RevisionRepository) bool {
	return repository == nil ||
		repository.client == nil ||
		nilRevisionDependency(repository.content) ||
		nilRevisionDependency(repository.sampleDecoder)
}

func nilRevisionDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var (
	_ ports.PublicationRepository    = (*RevisionRepository)(nil)
	_ ports.DraftRepository          = (*RevisionRepository)(nil)
	_ ports.AuthoringQueryRepository = (*RevisionRepository)(nil)
	_ ports.ImmutableProblemReader   = (*RevisionRepository)(nil)
)

func (repository *RevisionRepository) LockAggregate(
	ctx context.Context,
	problemID uuid.UUID,
) (entity.ProblemAggregate, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || problemID == uuid.Nil {
		return entity.ProblemAggregate{}, ErrInvalidRevisionRepository
	}
	if err := ctx.Err(); err != nil {
		return entity.ProblemAggregate{}, err
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	record, err := client.Problem.Query().
		Where(problem.ID(problemID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, mapPublicationReadError(err)
	}
	return aggregateFromRecord(record)
}

func (repository *RevisionRepository) NextRevisionNumbers(
	ctx context.Context,
	problemID uuid.UUID,
) (ports.DraftRevisionNumbers, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || problemID == uuid.Nil {
		return ports.DraftRevisionNumbers{}, ErrInvalidRevisionRepository
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return ports.DraftRevisionNumbers{}, err
	}
	problemCount, err := client.ProblemRevision.Query().
		Where(problemrevision.ProblemID(problemID)).
		Count(ctx)
	if err != nil {
		return ports.DraftRevisionNumbers{}, fmt.Errorf("count problem revisions: %w", err)
	}
	testsetCount, err := client.TestsetRevision.Query().
		Where(testsetrevision.ProblemID(problemID)).
		Count(ctx)
	if err != nil {
		return ports.DraftRevisionNumbers{}, fmt.Errorf("count testset revisions: %w", err)
	}
	if int64(problemCount) >= problemcontract.MaximumProblemRevisionNumber ||
		int64(testsetCount) >= problemcontract.MaximumTestsetRevisionNumber {
		return ports.DraftRevisionNumbers{}, ErrInvalidRevisionRepository
	}
	return ports.DraftRevisionNumbers{
		ProblemRevision: int64(problemCount) + 1,
		TestsetRevision: int64(testsetCount) + 1,
	}, nil
}

func (repository *RevisionRepository) ResolveAssets(
	ctx context.Context,
	candidates []entity.TestAsset,
) ([]entity.TestAsset, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || len(candidates) == 0 {
		return nil, ErrInvalidRevisionRepository
	}
	if len(candidates) > problemcontract.MaximumTestCases*2 {
		return nil, ErrInvalidRevisionRepository
	}
	seen := make(map[[sha256Size]byte]struct{}, len(candidates))
	for _, candidate := range candidates {
		if err := candidate.ValidateMetadata(); err != nil {
			return nil, err
		}
		if _, duplicate := seen[candidate.SHA256]; duplicate {
			return nil, entity.ErrTestAssetIntegrity
		}
		seen[candidate.SHA256] = struct{}{}
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return nil, err
	}
	resolved := make([]entity.TestAsset, 0, len(candidates))
	for _, candidate := range candidates {
		if err := client.TestAsset.Create().
			SetID(candidate.ID).
			SetCreatedAt(candidate.CreatedAt.UTC()).
			SetSha256(append([]byte(nil), candidate.SHA256[:]...)).
			SetEncoding(testasset.Encoding(candidate.Encoding)).
			SetContent(append([]byte(nil), candidate.Content...)).
			SetUncompressedSize(candidate.UncompressedSize).
			OnConflictColumns(testasset.FieldSha256).
			DoNothing().
			Exec(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("insert content-addressed test asset: %w", err)
		}
		record, err := client.TestAsset.Query().
			Where(testasset.Sha256(candidate.SHA256[:])).
			Only(ctx)
		if err != nil {
			return nil, mapDraftReadError(err)
		}
		asset, err := assetFromRecord(record)
		if err != nil {
			return nil, err
		}
		if asset.SHA256 != candidate.SHA256 ||
			asset.Encoding != candidate.Encoding ||
			asset.UncompressedSize != candidate.UncompressedSize ||
			!bytes.Equal(asset.Content, candidate.Content) {
			return nil, entity.ErrTestAssetIntegrity
		}
		resolved = append(resolved, asset)
	}
	return resolved, nil
}

func (repository *RevisionRepository) StoreTestset(
	ctx context.Context,
	testset entity.TestsetRevision,
) error {
	if invalidRevisionRepository(repository) ||
		ctx == nil {
		return ErrInvalidRevisionRepository
	}
	if err := testset.ValidateStructure(); err != nil {
		return err
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return err
	}
	for _, group := range testset.Groups {
		for _, item := range group.Cases {
			err := client.TestCase.Create().
				SetID(item.TestCaseID).
				SetCreatedAt(testset.CreatedAt.UTC()).
				SetUpdatedAt(testset.CreatedAt.UTC()).
				SetProblemID(testset.ProblemID).
				OnConflictColumns(testcase.FieldID).
				DoNothing().
				Exec(ctx)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("insert stable testcase lineage: %w", err)
			}
			record, err := client.TestCase.Query().
				Where(
					testcase.ID(item.TestCaseID),
					testcase.ProblemID(testset.ProblemID),
				).
				Only(ctx)
			if err != nil {
				if generate.IsNotFound(err) {
					return entity.ErrInvalidTestsetRevision
				}
				return fmt.Errorf("verify stable testcase lineage: %w", err)
			}
			if record.ID != item.TestCaseID || record.ProblemID != testset.ProblemID {
				return entity.ErrInvalidTestsetRevision
			}
		}
	}
	if err := client.TestsetRevision.Create().
		SetID(testset.ID).
		SetCreatedAt(testset.CreatedAt.UTC()).
		SetProblemID(testset.ProblemID).
		SetRevisionNumber(testset.RevisionNumber).
		SetChecksum(append([]byte(nil), testset.Checksum[:]...)).
		SetTestCount(testset.TestCount).
		SetTotalUncompressedBytes(testset.TotalUncompressedBytes).
		SetCreatedBy(testset.CreatedBy).
		Exec(ctx); err != nil {
		return fmt.Errorf("insert immutable testset revision: %w", err)
	}

	groupBuilders := make([]*generate.TestcaseGroupCreate, 0, len(testset.Groups))
	caseBuilders := make([]*generate.TestcaseRevisionCreate, 0, testset.TestCount)
	for _, group := range testset.Groups {
		groupBuilders = append(groupBuilders, client.TestcaseGroup.Create().
			SetID(group.ID).
			SetCreatedAt(group.CreatedAt.UTC()).
			SetTestsetRevisionID(group.TestsetRevisionID).
			SetGroupIndex(group.GroupIndex).
			SetStopOnFailure(group.StopOnFailure))
		for _, item := range group.Cases {
			caseBuilders = append(caseBuilders, client.TestcaseRevision.Create().
				SetID(item.ID).
				SetCreatedAt(item.CreatedAt.UTC()).
				SetTestCaseID(item.TestCaseID).
				SetProblemID(item.ProblemID).
				SetTestsetRevisionID(item.TestsetRevisionID).
				SetGroupID(group.ID).
				SetKind(testcaserevision.Kind(item.Kind)).
				SetGroupIndex(item.GroupIndex).
				SetOrderIndex(item.OrderIndex).
				SetInputAssetID(item.InputAsset.ID).
				SetExpectedAssetID(item.ExpectedAsset.ID).
				SetWeight(item.Weight).
				SetStopOnFailure(item.StopOnFailure))
		}
	}
	if err := client.TestcaseGroup.CreateBulk(groupBuilders...).Exec(ctx); err != nil {
		return fmt.Errorf("insert immutable testcase groups: %w", err)
	}
	if err := client.TestcaseRevision.CreateBulk(caseBuilders...).Exec(ctx); err != nil {
		return fmt.Errorf("insert immutable testcase revisions: %w", err)
	}
	return nil
}

func (repository *RevisionRepository) StoreRevision(
	ctx context.Context,
	revision entity.Revision,
) error {
	if invalidRevisionRepository(repository) ||
		ctx == nil {
		return ErrInvalidRevisionRepository
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	tagSnapshot, err := json.Marshal(revision.Reward.Tags)
	if err != nil || len(tagSnapshot) > problemcontract.MaximumRewardSnapshotJSONBytes {
		return entity.ErrInvalidProblemRevision
	}
	var elementSnapshot json.RawMessage
	if revision.Reward.Elements != nil {
		elementSnapshot, err = json.Marshal(revision.Reward.Elements)
		if err != nil || len(elementSnapshot) > problemcontract.MaximumRewardSnapshotJSONBytes {
			return entity.ErrInvalidProblemRevision
		}
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return err
	}
	if err := verifyProblemRewardMappingOwnership(ctx, client, revision); err != nil {
		return err
	}
	builder := client.ProblemRevision.Create().
		SetID(revision.ID).
		SetCreatedAt(revision.CreatedAt.UTC()).
		SetProblemID(revision.ProblemID).
		SetRevisionNumber(revision.RevisionNumber).
		SetTitle(revision.Title).
		SetStatementMarkdown(string(revision.StatementMarkdown)).
		SetRenderSourceID(revision.RenderSourceID).
		SetRenderSourceKind(problemrevision.RenderSourceKind(revision.RenderSourceKind)).
		SetRenderSourceChecksum(append([]byte(nil), revision.RenderSourceChecksum[:]...)).
		SetDifficultyID(revision.Reward.DifficultyID).
		SetDifficultyRevision(revision.Reward.DifficultyRevision).
		SetBaseExp(revision.Reward.BaseExperience).
		SetTagMappingID(revision.Reward.Tags.MappingID).
		SetTagMappingRevision(revision.Reward.Tags.Revision).
		SetTagSnapshot(append(json.RawMessage(nil), tagSnapshot...)).
		SetRewardSnapshotChecksum(append([]byte(nil), revision.Reward.Checksum[:]...)).
		SetCPUTimeLimitMs(revision.Limits.CPUTimeMilliseconds).
		SetWallTimeLimitMs(revision.Limits.WallTimeMilliseconds).
		SetMemoryLimitKB(revision.Limits.MemoryKilobytes).
		SetOutputLimitBytes(revision.Limits.OutputBytes).
		SetProcessLimit(revision.Limits.ProcessCount).
		SetCheckerKey(revision.CheckerKey).
		SetCheckerVersion(revision.CheckerVersion).
		SetCheckerConfig(append(json.RawMessage(nil), revision.CheckerConfig...)).
		SetTestsetRevisionID(revision.TestsetRevisionID).
		SetAllowedRuntimeKeys(append([]string(nil), revision.AllowedRuntimeKeys...)).
		SetCreatedBy(revision.CreatedBy).
		SetContentChecksum(append([]byte(nil), revision.ContentChecksum[:]...))
	if revision.Reward.Elements != nil {
		builder.
			SetElementMappingID(revision.Reward.Elements.MappingID).
			SetElementMappingRevision(revision.Reward.Elements.Revision).
			SetElementSnapshot(append(json.RawMessage(nil), elementSnapshot...))
	}
	if err := builder.Exec(ctx); err != nil {
		return fmt.Errorf("insert immutable problem revision: %w", err)
	}
	return nil
}

func (repository *RevisionRepository) AdvanceDraft(
	ctx context.Context,
	problemID uuid.UUID,
	expectedVersion int64,
	selection entity.RenderSelection,
	savedAt time.Time,
) (entity.ProblemAggregate, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || problemID == uuid.Nil || expectedVersion < 1 ||
		savedAt.IsZero() {
		return entity.ProblemAggregate{}, ErrInvalidRevisionRepository
	}
	if err := selection.Validate(); err != nil {
		return entity.ProblemAggregate{}, err
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	affected, err := client.Problem.Update().
		Where(
			problem.ID(problemID),
			problem.Version(expectedVersion),
			problem.LifecycleNEQ(problem.LifecycleArchived),
		).
		SetVersion(expectedVersion + 1).
		SetUpdatedAt(savedAt.UTC()).
		SetDraftRevisionID(selection.RevisionID).
		SetDraftRenderSourceID(selection.SourceID).
		SetDraftRenderSourceKind(problem.DraftRenderSourceKind(selection.SourceKind)).
		SetDraftRenderSourceChecksum(append([]byte(nil), selection.SourceChecksum[:]...)).
		SetDraftRenderArtifactID(selection.ArtifactID).
		Save(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, fmt.Errorf("update problem draft selection: %w", err)
	}
	if affected != 1 {
		return entity.ProblemAggregate{}, ports.ErrDraftConflict
	}
	record, err := client.Problem.Query().Where(problem.ID(problemID)).Only(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, mapDraftReadError(err)
	}
	return aggregateFromRecord(record)
}

func (repository *RevisionRepository) LoadPublicationCandidate(
	ctx context.Context,
	aggregate entity.ProblemAggregate,
	revisionID uuid.UUID,
	testsetRevisionID uuid.UUID,
) (entity.PublicationCandidate, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || aggregate.ID == uuid.Nil || revisionID == uuid.Nil ||
		testsetRevisionID == uuid.Nil {
		return entity.PublicationCandidate{}, ErrInvalidRevisionRepository
	}
	if err := ctx.Err(); err != nil {
		return entity.PublicationCandidate{}, err
	}
	if aggregate.Draft == nil ||
		aggregate.Draft.RevisionID != revisionID {
		return entity.PublicationCandidate{}, ports.ErrPublicationNotFound
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return entity.PublicationCandidate{}, err
	}
	return repository.loadCandidate(
		ctx,
		client,
		aggregate,
		revisionID,
		testsetRevisionID,
	)
}

func (repository *RevisionRepository) LoadAuthoringAggregate(
	ctx context.Context,
	problemID uuid.UUID,
) (entity.ProblemAggregate, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || problemID == uuid.Nil {
		return entity.ProblemAggregate{}, ErrInvalidRevisionRepository
	}
	if err := ctx.Err(); err != nil {
		return entity.ProblemAggregate{}, err
	}
	record, err := repository.client.Problem.Query().
		Where(problem.ID(problemID)).
		Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return entity.ProblemAggregate{}, ports.ErrAuthoringDraftNotFound
		}
		return entity.ProblemAggregate{}, fmt.Errorf(
			"load problem authoring aggregate: %w",
			err,
		)
	}
	aggregate, err := aggregateFromRecord(record)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	if aggregate.Draft == nil {
		return entity.ProblemAggregate{}, ports.ErrAuthoringDraftNotFound
	}
	return aggregate, nil
}

func (repository *RevisionRepository) LoadAuthoringDraft(
	ctx context.Context,
	aggregate entity.ProblemAggregate,
) (ports.AuthoringDraft, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || aggregate.ID == uuid.Nil || aggregate.Draft == nil {
		return ports.AuthoringDraft{}, ErrInvalidRevisionRepository
	}
	if err := ctx.Err(); err != nil {
		return ports.AuthoringDraft{}, err
	}
	candidate, err := repository.loadCandidate(
		ctx,
		repository.client,
		aggregate,
		aggregate.Draft.RevisionID,
		uuid.Nil,
	)
	if err != nil {
		if errors.Is(err, ports.ErrPublicationNotFound) {
			return ports.AuthoringDraft{}, ports.ErrAuthoringDraftNotFound
		}
		return ports.AuthoringDraft{}, err
	}
	return projectAuthoringDraft(ctx, candidate)
}

func (repository *RevisionRepository) LoadPublishedTestset(
	ctx context.Context,
	problemID uuid.UUID,
) (entity.TestsetRevision, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || problemID == uuid.Nil {
		return entity.TestsetRevision{}, ErrInvalidRevisionRepository
	}
	if err := ctx.Err(); err != nil {
		return entity.TestsetRevision{}, err
	}
	client := repository.client
	record, err := client.Problem.Query().
		Where(
			problem.ID(problemID),
			problem.LifecycleEQ(problem.LifecyclePublished),
			problem.IsPublished(true),
			problem.PublishedRevisionIDNotNil(),
		).
		Only(ctx)
	if err != nil {
		return entity.TestsetRevision{}, mapPublicationReadError(err)
	}
	aggregate, err := aggregateFromRecord(record)
	if err != nil || aggregate.Published == nil {
		return entity.TestsetRevision{}, errors.Join(
			ports.ErrPublicationNotFound,
			err,
		)
	}
	revisionRecord, err := client.ProblemRevision.Query().
		Where(
			problemrevision.ID(aggregate.Published.RevisionID),
			problemrevision.ProblemID(problemID),
		).
		Only(ctx)
	if err != nil {
		return entity.TestsetRevision{}, mapPublicationReadError(err)
	}
	revision, err := revisionFromRecord(revisionRecord)
	if err != nil {
		return entity.TestsetRevision{}, err
	}
	if revision.RenderSourceID != aggregate.Published.SourceID ||
		revision.RenderSourceKind != aggregate.Published.SourceKind ||
		revision.RenderSourceChecksum != aggregate.Published.SourceChecksum {
		return entity.TestsetRevision{}, entity.ErrInvalidProblemRevision
	}
	if err := verifyRevisionRewardMappingOwnership(aggregate, revision); err != nil {
		return entity.TestsetRevision{}, err
	}
	return loadTestsetRevision(
		ctx,
		client,
		problemID,
		revision.TestsetRevisionID,
	)
}

func (repository *RevisionRepository) loadCandidate(
	ctx context.Context,
	client *generate.Client,
	aggregate entity.ProblemAggregate,
	revisionID uuid.UUID,
	expectedTestsetRevisionID uuid.UUID,
) (entity.PublicationCandidate, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || client == nil || aggregate.ID == uuid.Nil ||
		revisionID == uuid.Nil || aggregate.Draft == nil ||
		aggregate.Draft.RevisionID != revisionID {
		return entity.PublicationCandidate{}, ErrInvalidRevisionRepository
	}
	revisionQuery := client.ProblemRevision.Query().
		Where(
			problemrevision.ID(revisionID),
			problemrevision.ProblemID(aggregate.ID),
		)
	if expectedTestsetRevisionID != uuid.Nil {
		revisionQuery.Where(
			problemrevision.TestsetRevisionID(expectedTestsetRevisionID),
		)
	}
	revisionRecord, err := revisionQuery.Only(ctx)
	if err != nil {
		return entity.PublicationCandidate{}, mapPublicationReadError(err)
	}
	revision, err := revisionFromRecord(revisionRecord)
	if err != nil {
		return entity.PublicationCandidate{}, err
	}
	testsetRevisionID := revision.TestsetRevisionID
	if expectedTestsetRevisionID != uuid.Nil &&
		testsetRevisionID != expectedTestsetRevisionID {
		return entity.PublicationCandidate{}, ports.ErrPublicationNotFound
	}
	if err := verifyRevisionRewardMappingOwnership(aggregate, revision); err != nil {
		return entity.PublicationCandidate{}, err
	}

	testset, err := loadTestsetRevision(
		ctx,
		client,
		aggregate.ID,
		testsetRevisionID,
	)
	if err != nil {
		return entity.PublicationCandidate{}, err
	}

	source, err := repository.content.GetSource(ctx, aggregate.Draft.SourceID)
	if err != nil {
		return entity.PublicationCandidate{}, fmt.Errorf("load problem render source: %w", err)
	}
	artifact, err := repository.content.GetArtifact(ctx, aggregate.Draft.ArtifactID)
	if err != nil {
		return entity.PublicationCandidate{}, fmt.Errorf("load problem render artifact: %w", err)
	}
	candidate := entity.PublicationCandidate{
		Aggregate: aggregate,
		Revision:  revision,
		Testset:   testset,
		Source:    source,
		Artifact:  artifact,
	}
	if err := candidate.Validate(); err != nil {
		return entity.PublicationCandidate{}, err
	}
	return candidate, nil
}

func loadTestsetRevision(
	ctx context.Context,
	client *generate.Client,
	problemID uuid.UUID,
	testsetRevisionID uuid.UUID,
) (entity.TestsetRevision, error) {
	if ctx == nil || client == nil || problemID == uuid.Nil ||
		testsetRevisionID == uuid.Nil {
		return entity.TestsetRevision{}, ErrInvalidRevisionRepository
	}
	testsetRecord, err := client.TestsetRevision.Query().
		Where(
			testsetrevision.ID(testsetRevisionID),
			testsetrevision.ProblemID(problemID),
		).
		WithGroups(func(groupQuery *generate.TestcaseGroupQuery) {
			groupQuery.
				Order(generate.Asc(testcasegroup.FieldGroupIndex)).
				WithTestcaseRevisions(func(caseQuery *generate.TestcaseRevisionQuery) {
					caseQuery.
						Order(generate.Asc(testcaserevision.FieldOrderIndex)).
						WithInputAsset().
						WithExpectedAsset()
				})
		}).
		Only(ctx)
	if err != nil {
		return entity.TestsetRevision{}, mapPublicationReadError(err)
	}
	testset, err := testsetFromRecord(testsetRecord)
	if err != nil {
		return entity.TestsetRevision{}, err
	}
	return testset, nil
}

func projectAuthoringDraft(
	ctx context.Context,
	candidate entity.PublicationCandidate,
) (ports.AuthoringDraft, error) {
	if ctx == nil {
		return ports.AuthoringDraft{}, ErrInvalidRevisionRepository
	}
	if err := candidate.Validate(); err != nil {
		return ports.AuthoringDraft{}, err
	}
	tagIDs := make([]uuid.UUID, len(candidate.Revision.Reward.Tags.Items))
	for index, item := range candidate.Revision.Reward.Tags.Items {
		tagIDs[index] = item.TagID
	}
	diagnostics := make([]string, len(candidate.Artifact.Diagnostics))
	for index, diagnostic := range candidate.Artifact.Diagnostics {
		diagnostics[index] = diagnostic.Code
	}
	groups := make(
		[]ports.AuthoringTestcaseGroup,
		len(candidate.Testset.Groups),
	)
	decoder := entity.IdentityAssetDecoder{}
	for groupIndex, group := range candidate.Testset.Groups {
		groups[groupIndex] = ports.AuthoringTestcaseGroup{
			StopOnFailure: group.StopOnFailure,
			Cases:         make([]ports.AuthoringTestcase, len(group.Cases)),
		}
		for caseIndex, testcase := range group.Cases {
			input, err := decoder.Decode(ctx, testcase.InputAsset)
			if err != nil {
				return ports.AuthoringDraft{}, fmt.Errorf(
					"decode authoring testcase input: %w",
					err,
				)
			}
			expected, err := decoder.Decode(ctx, testcase.ExpectedAsset)
			if err != nil {
				return ports.AuthoringDraft{}, fmt.Errorf(
					"decode authoring testcase output: %w",
					err,
				)
			}
			groups[groupIndex].Cases[caseIndex] = ports.AuthoringTestcase{
				TestCaseID:     testcase.TestCaseID,
				Kind:           testcase.Kind,
				Input:          string(input),
				ExpectedOutput: string(expected),
			}
		}
	}
	return ports.AuthoringDraft{
		ProblemID:           candidate.Aggregate.ID,
		Slug:                candidate.Aggregate.Slug,
		Lifecycle:           candidate.Aggregate.Lifecycle,
		Version:             candidate.Aggregate.Version,
		RevisionID:          candidate.Revision.ID,
		TestsetRevisionID:   candidate.Testset.ID,
		Title:               candidate.Revision.Title,
		StatementMarkdown:   string(candidate.Revision.StatementMarkdown),
		DifficultyID:        candidate.Revision.Reward.DifficultyID,
		TagIDs:              tagIDs,
		Limits:              candidate.Revision.Limits,
		CheckerKey:          candidate.Revision.CheckerKey,
		CheckerVersion:      candidate.Revision.CheckerVersion,
		CheckerConfig:       append(json.RawMessage(nil), candidate.Revision.CheckerConfig...),
		AllowedRuntimeKeys:  append([]string(nil), candidate.Revision.AllowedRuntimeKeys...),
		ArtifactStatus:      candidate.Artifact.Status,
		ArtifactDiagnostics: diagnostics,
		SavedAt:             candidate.Revision.CreatedAt,
		Groups:              groups,
	}, nil
}

// verifyProblemRewardMappingOwnership is intentionally separate from taxonomy
// history verification. TaxonomyCatalog proves immutable catalog facts; this
// repository proves the stable Problem-owned mapping identities used by a
// revision belong to that exact aggregate.
func verifyProblemRewardMappingOwnership(
	ctx context.Context,
	client *generate.Client,
	revision entity.Revision,
) error {
	if client == nil || revision.ProblemID == uuid.Nil {
		return entity.ErrInvalidProblemRevision
	}
	aggregate, err := client.Problem.Query().Where(problem.ID(revision.ProblemID)).Only(ctx)
	if err != nil {
		return mapDraftReadError(err)
	}
	return verifyRevisionRewardMappingOwnership(
		entity.ProblemAggregate{
			ID:               aggregate.ID,
			TagMappingID:     aggregate.TagMappingID,
			ElementMappingID: aggregate.ElementMappingID,
		},
		revision,
	)
}

func verifyRevisionRewardMappingOwnership(
	aggregate entity.ProblemAggregate,
	revision entity.Revision,
) error {
	if revision.RevisionNumber < 1 ||
		revision.Reward.Tags.Revision != revision.RevisionNumber ||
		(revision.Reward.Elements != nil &&
			revision.Reward.Elements.Revision != revision.RevisionNumber) {
		return entity.ErrInvalidProblemRevision
	}
	return verifyAggregateRewardMappingOwnership(aggregate, revision.Reward)
}

func verifyAggregateRewardMappingOwnership(
	aggregate entity.ProblemAggregate,
	reward entity.RewardSnapshot,
) error {
	if aggregate.ID == uuid.Nil || aggregate.TagMappingID == uuid.Nil ||
		aggregate.ElementMappingID == uuid.Nil || reward.Tags.MappingID != aggregate.TagMappingID {
		return entity.ErrInvalidProblemRevision
	}
	if reward.Elements != nil && reward.Elements.MappingID != aggregate.ElementMappingID {
		return entity.ErrInvalidProblemRevision
	}
	return nil
}

func (repository *RevisionRepository) AdvancePublished(
	ctx context.Context,
	problemID uuid.UUID,
	expectedVersion int64,
	selection entity.RenderSelection,
	projection ports.PublishedProblemProjection,
	publishedAt time.Time,
) (entity.ProblemAggregate, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || problemID == uuid.Nil || expectedVersion < 1 ||
		publishedAt.IsZero() ||
		!validPublishedProjection(selection, projection) {
		return entity.ProblemAggregate{}, ErrInvalidRevisionRepository
	}
	if err := selection.Validate(); err != nil {
		return entity.ProblemAggregate{}, err
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	update := client.Problem.Update().
		Where(
			problem.ID(problemID),
			problem.Version(expectedVersion),
			problem.LifecycleNEQ(problem.LifecycleArchived),
			problem.DraftRevisionID(selection.RevisionID),
			problem.DraftRenderSourceID(selection.SourceID),
			problem.DraftRenderSourceKindEQ(problem.DraftRenderSourceKind(selection.SourceKind)),
			problem.DraftRenderSourceChecksumEQ(selection.SourceChecksum[:]),
			problem.DraftRenderArtifactID(selection.ArtifactID),
		).
		SetLifecycle(problem.LifecyclePublished).
		SetVersion(expectedVersion + 1).
		SetPublishedRevisionID(selection.RevisionID).
		SetPublishedRenderSourceID(selection.SourceID).
		SetPublishedRenderSourceKind(problem.PublishedRenderSourceKind(selection.SourceKind)).
		SetPublishedRenderSourceChecksum(append([]byte(nil), selection.SourceChecksum[:]...)).
		SetPublishedRenderArtifactID(selection.ArtifactID).
		SetPublishedAt(publishedAt.UTC()).
		ClearArchivedAt().
		SetIsPublished(true).
		SetTitle(projection.Title).
		SetDescription(projection.StatementMarkdown).
		SetDifficultyID(projection.DifficultyID).
		SetTimeLimitMs(projection.CPUTimeMs).
		SetMemoryLimitKB(projection.MemoryLimitKb).
		ClearTags()
	if len(projection.TagIDs) > 0 {
		update.AddTagIDs(projection.TagIDs...)
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, fmt.Errorf("update problem publication: %w", err)
	}
	if affected != 1 {
		return entity.ProblemAggregate{}, ports.ErrPublicationConflict
	}
	record, err := client.Problem.Query().Where(problem.ID(problemID)).Only(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, mapPublicationReadError(err)
	}
	return aggregateFromRecord(record)
}

func validPublishedProjection(
	selection entity.RenderSelection,
	projection ports.PublishedProblemProjection,
) bool {
	if projection.RevisionID != selection.RevisionID ||
		projection.DifficultyID == uuid.Nil ||
		strings.TrimSpace(projection.Title) == "" ||
		strings.TrimSpace(projection.StatementMarkdown) == "" ||
		!utf8.ValidString(projection.Title) ||
		!utf8.ValidString(projection.StatementMarkdown) ||
		strings.IndexByte(projection.Title, 0) >= 0 ||
		strings.IndexByte(projection.StatementMarkdown, 0) >= 0 ||
		len(projection.Title) > problemcontract.MaximumTitleBytes ||
		len(projection.StatementMarkdown) >
			problemcontract.MaximumStatementMarkdownBytes ||
		len(projection.TagIDs) > problemcontract.MaximumTags {
		return false
	}
	if err := (entity.Limits{
		CPUTimeMilliseconds:  projection.CPUTimeMs,
		WallTimeMilliseconds: projection.CPUTimeMs,
		MemoryKilobytes:      projection.MemoryLimitKb,
		OutputBytes:          problemcontract.MinimumOutputLimitBytes,
		ProcessCount:         problemcontract.MinimumProcessLimit,
	}).Validate(); err != nil {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(projection.TagIDs))
	for _, tagID := range projection.TagIDs {
		if tagID == uuid.Nil {
			return false
		}
		if _, duplicate := seen[tagID]; duplicate {
			return false
		}
		seen[tagID] = struct{}{}
	}
	return true
}

func (repository *RevisionRepository) Archive(
	ctx context.Context,
	problemID uuid.UUID,
	expectedVersion int64,
	archivedAt time.Time,
) (entity.ProblemAggregate, error) {
	if invalidRevisionRepository(repository) ||
		ctx == nil || problemID == uuid.Nil || expectedVersion < 1 ||
		archivedAt.IsZero() {
		return entity.ProblemAggregate{}, ErrInvalidRevisionRepository
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	affected, err := client.Problem.Update().
		Where(
			problem.ID(problemID),
			problem.Version(expectedVersion),
			problem.LifecycleEQ(problem.LifecyclePublished),
			problem.PublishedRevisionIDNotNil(),
		).
		SetLifecycle(problem.LifecycleArchived).
		SetVersion(expectedVersion + 1).
		SetArchivedAt(archivedAt.UTC()).
		SetIsPublished(false).
		Save(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, fmt.Errorf("archive problem aggregate: %w", err)
	}
	if affected != 1 {
		return entity.ProblemAggregate{}, ports.ErrPublicationConflict
	}
	record, err := client.Problem.Query().Where(problem.ID(problemID)).Only(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, mapPublicationReadError(err)
	}
	return aggregateFromRecord(record)
}

func aggregateFromRecord(record *generate.Problem) (entity.ProblemAggregate, error) {
	if record == nil {
		return entity.ProblemAggregate{}, ports.ErrPublicationNotFound
	}
	draft, err := selectionFromRecord(
		record.DraftRevisionID,
		record.DraftRenderSourceID,
		record.DraftRenderSourceKind,
		record.DraftRenderSourceChecksum,
		record.DraftRenderArtifactID,
	)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	published, err := selectionFromRecord(
		record.PublishedRevisionID,
		record.PublishedRenderSourceID,
		record.PublishedRenderSourceKind,
		record.PublishedRenderSourceChecksum,
		record.PublishedRenderArtifactID,
	)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	aggregate := entity.ProblemAggregate{
		ID:               record.ID,
		Slug:             record.Slug,
		AuthorID:         record.AuthorID,
		TagMappingID:     record.TagMappingID,
		ElementMappingID: record.ElementMappingID,
		Lifecycle:        entity.Lifecycle(record.Lifecycle),
		Version:          record.Version,
		Draft:            draft,
		Published:        published,
		SubmissionCount:  int64(record.SubmissionCount),
		AcceptedCount:    int64(record.AcceptedCount),
		CreatedAt:        record.CreatedAt.UTC(),
		UpdatedAt:        record.UpdatedAt.UTC(),
		PublishedAt:      cloneTime(record.PublishedAt),
		ArchivedAt:       cloneTime(record.ArchivedAt),
	}
	if err := aggregate.Validate(); err != nil {
		return entity.ProblemAggregate{}, err
	}
	return aggregate, nil
}

type sourceKind interface{ String() string }

func selectionFromRecord[K sourceKind](
	revisionID *uuid.UUID,
	sourceID *uuid.UUID,
	kind *K,
	checksum *[]byte,
	artifactID *uuid.UUID,
) (*entity.RenderSelection, error) {
	// Ent represents a nullable PostgreSQL bytea as a pointer to a nil slice
	// when the database value is NULL. Normalize that generated-client detail
	// at the adapter boundary. A pointer to a non-nil empty slice remains
	// present and is rejected below as a corrupt digest.
	if checksum != nil && *checksum == nil {
		checksum = nil
	}
	nulls := 0
	if revisionID == nil {
		nulls++
	}
	if sourceID == nil {
		nulls++
	}
	if kind == nil {
		nulls++
	}
	if checksum == nil {
		nulls++
	}
	if artifactID == nil {
		nulls++
	}
	if nulls == 5 {
		return nil, nil
	}
	if nulls != 0 {
		return nil, entity.ErrInvalidRenderSelection
	}
	digest, err := digestFromBytes(*checksum)
	if err != nil {
		return nil, err
	}
	selection := &entity.RenderSelection{
		RevisionID:     *revisionID,
		SourceID:       *sourceID,
		SourceKind:     (*kind).String(),
		SourceChecksum: digest,
		ArtifactID:     *artifactID,
	}
	if err := selection.Validate(); err != nil {
		return nil, err
	}
	return selection, nil
}

func revisionFromRecord(record *generate.ProblemRevision) (entity.Revision, error) {
	if record == nil {
		return entity.Revision{}, ports.ErrPublicationNotFound
	}
	renderChecksum, err := digestFromBytes(record.RenderSourceChecksum)
	if err != nil {
		return entity.Revision{}, err
	}
	rewardChecksum, err := digestFromBytes(record.RewardSnapshotChecksum)
	if err != nil {
		return entity.Revision{}, err
	}
	contentChecksum, err := digestFromBytes(record.ContentChecksum)
	if err != nil {
		return entity.Revision{}, err
	}
	checkerConfig, err := entity.CanonicalizeCheckerConfig(record.CheckerConfig)
	if err != nil {
		return entity.Revision{}, err
	}
	var tags entity.TagMappingSnapshot
	if err := decodeJSON(record.TagSnapshot, &tags); err != nil {
		return entity.Revision{}, err
	}
	if tags.MappingID != record.TagMappingID || tags.Revision != record.TagMappingRevision {
		return entity.Revision{}, entity.ErrInvalidProblemRevision
	}
	var elements *entity.ElementMappingSnapshot
	switch {
	case record.ElementMappingID == nil && record.ElementMappingRevision == nil &&
		len(record.ElementSnapshot) == 0:
	case record.ElementMappingID != nil && record.ElementMappingRevision != nil &&
		len(record.ElementSnapshot) > 0:
		var decoded entity.ElementMappingSnapshot
		if err := decodeJSON(record.ElementSnapshot, &decoded); err != nil {
			return entity.Revision{}, err
		}
		if decoded.MappingID != *record.ElementMappingID ||
			decoded.Revision != *record.ElementMappingRevision {
			return entity.Revision{}, entity.ErrInvalidProblemRevision
		}
		elements = &decoded
	default:
		return entity.Revision{}, entity.ErrInvalidProblemRevision
	}
	createdBy := uuid.Nil
	if record.CreatedBy != nil {
		createdBy = *record.CreatedBy
	}
	revision := entity.Revision{
		ID:                   record.ID,
		ProblemID:            record.ProblemID,
		RevisionNumber:       record.RevisionNumber,
		Title:                record.Title,
		StatementMarkdown:    []byte(record.StatementMarkdown),
		RenderSourceID:       record.RenderSourceID,
		RenderSourceKind:     string(record.RenderSourceKind),
		RenderSourceChecksum: renderChecksum,
		Reward: entity.RewardSnapshot{
			DifficultyID:       record.DifficultyID,
			DifficultyRevision: record.DifficultyRevision,
			BaseExperience:     record.BaseExp,
			Tags:               tags,
			Elements:           elements,
			Checksum:           rewardChecksum,
		},
		Limits: entity.Limits{
			CPUTimeMilliseconds:  record.CPUTimeLimitMs,
			WallTimeMilliseconds: record.WallTimeLimitMs,
			MemoryKilobytes:      record.MemoryLimitKB,
			OutputBytes:          record.OutputLimitBytes,
			ProcessCount:         record.ProcessLimit,
		},
		CheckerKey:         record.CheckerKey,
		CheckerVersion:     record.CheckerVersion,
		CheckerConfig:      checkerConfig,
		TestsetRevisionID:  record.TestsetRevisionID,
		AllowedRuntimeKeys: append([]string(nil), record.AllowedRuntimeKeys...),
		CreatedBy:          createdBy,
		CreatedAt:          record.CreatedAt.UTC(),
		ContentChecksum:    contentChecksum,
	}
	if err := revision.Validate(); err != nil {
		return entity.Revision{}, err
	}
	return revision, nil
}

func testsetFromRecord(record *generate.TestsetRevision) (entity.TestsetRevision, error) {
	if record == nil {
		return entity.TestsetRevision{}, ports.ErrPublicationNotFound
	}
	checksum, err := digestFromBytes(record.Checksum)
	if err != nil {
		return entity.TestsetRevision{}, err
	}
	createdBy := uuid.Nil
	if record.CreatedBy != nil {
		createdBy = *record.CreatedBy
	}
	groups := make([]entity.TestcaseGroup, len(record.Edges.Groups))
	for groupIndex, groupRecord := range record.Edges.Groups {
		if groupRecord == nil {
			return entity.TestsetRevision{}, entity.ErrInvalidTestsetRevision
		}
		cases := make([]entity.TestcaseRevision, len(groupRecord.Edges.TestcaseRevisions))
		for caseIndex, caseRecord := range groupRecord.Edges.TestcaseRevisions {
			if caseRecord == nil ||
				caseRecord.Edges.InputAsset == nil ||
				caseRecord.Edges.ExpectedAsset == nil ||
				caseRecord.GroupID != groupRecord.ID ||
				caseRecord.InputAssetID != caseRecord.Edges.InputAsset.ID ||
				caseRecord.ExpectedAssetID != caseRecord.Edges.ExpectedAsset.ID {
				return entity.TestsetRevision{}, entity.ErrInvalidTestsetRevision
			}
			input, err := assetFromRecord(caseRecord.Edges.InputAsset)
			if err != nil {
				return entity.TestsetRevision{}, err
			}
			expected, err := assetFromRecord(caseRecord.Edges.ExpectedAsset)
			if err != nil {
				return entity.TestsetRevision{}, err
			}
			cases[caseIndex] = entity.TestcaseRevision{
				ID:                caseRecord.ID,
				TestCaseID:        caseRecord.TestCaseID,
				TestsetRevisionID: caseRecord.TestsetRevisionID,
				ProblemID:         caseRecord.ProblemID,
				Kind:              string(caseRecord.Kind),
				GroupIndex:        caseRecord.GroupIndex,
				OrderIndex:        caseRecord.OrderIndex,
				InputAsset:        input,
				ExpectedAsset:     expected,
				Weight:            caseRecord.Weight,
				StopOnFailure:     caseRecord.StopOnFailure,
				CreatedAt:         caseRecord.CreatedAt.UTC(),
			}
		}
		groups[groupIndex] = entity.TestcaseGroup{
			ID:                groupRecord.ID,
			TestsetRevisionID: groupRecord.TestsetRevisionID,
			GroupIndex:        groupRecord.GroupIndex,
			StopOnFailure:     groupRecord.StopOnFailure,
			CreatedAt:         groupRecord.CreatedAt.UTC(),
			Cases:             cases,
		}
	}
	testset := entity.TestsetRevision{
		ID:                     record.ID,
		ProblemID:              record.ProblemID,
		RevisionNumber:         record.RevisionNumber,
		Checksum:               checksum,
		TestCount:              record.TestCount,
		TotalUncompressedBytes: record.TotalUncompressedBytes,
		CreatedBy:              createdBy,
		CreatedAt:              record.CreatedAt.UTC(),
		Groups:                 groups,
	}
	if err := testset.ValidateStructure(); err != nil {
		return entity.TestsetRevision{}, err
	}
	return testset, nil
}

func assetFromRecord(record *generate.TestAsset) (entity.TestAsset, error) {
	if record == nil {
		return entity.TestAsset{}, ports.ErrPublicationNotFound
	}
	checksum, err := digestFromBytes(record.Sha256)
	if err != nil {
		return entity.TestAsset{}, err
	}
	asset := entity.TestAsset{
		ID:               record.ID,
		SHA256:           checksum,
		Encoding:         string(record.Encoding),
		Content:          append([]byte(nil), record.Content...),
		UncompressedSize: record.UncompressedSize,
		CreatedAt:        record.CreatedAt.UTC(),
	}
	if err := asset.ValidateMetadata(); err != nil {
		return entity.TestAsset{}, err
	}
	return asset, nil
}

func decodeJSON(encoded json.RawMessage, target any) error {
	if len(encoded) == 0 || target == nil {
		return entity.ErrInvalidProblemRevision
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.Join(entity.ErrInvalidProblemRevision, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return entity.ErrInvalidProblemRevision
	}
	return nil
}

func digestFromBytes(value []byte) ([32]byte, error) {
	if len(value) != sha256Size {
		return [32]byte{}, entity.ErrInvalidProblemRevision
	}
	var digest [sha256Size]byte
	copy(digest[:], value)
	if digest == ([sha256Size]byte{}) {
		return [32]byte{}, entity.ErrInvalidProblemRevision
	}
	return digest, nil
}

const sha256Size = 32

func mapPublicationReadError(err error) error {
	if generate.IsNotFound(err) {
		return ports.ErrPublicationNotFound
	}
	return err
}

func mapDraftReadError(err error) error {
	if generate.IsNotFound(err) {
		return ports.ErrDraftNotFound
	}
	return err
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
