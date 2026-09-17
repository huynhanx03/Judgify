package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/text/unicode/norm"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const problemDraftSavedEventV1 = problemcontract.EventDraftSavedV1

type DraftDependencies struct {
	Transactions platformtx.UnitOfWork
	Repository   ports.DraftRepository
	Authorizer   ports.DraftAuthorizer
	Catalog      ports.AuthoringCatalogFence
	Runtimes     ports.ExecutionCatalog
	Checkers     ports.CheckerCatalog
	Taxonomy     ports.TaxonomyCatalog
	Renderer     ports.DraftRenderer
	Content      contentrender.Repository
	Audit        audit.Appender
	Outbox       eventing.OutboxAppender
}

type DraftOption func(*draftService) error

func WithDraftClock(clock func() time.Time) DraftOption {
	return func(service *draftService) error {
		if clock == nil {
			return ports.ErrInvalidDraft
		}
		service.clock = clock
		return nil
	}
}

func WithDraftIDGenerator(generator func() uuid.UUID) DraftOption {
	return func(service *draftService) error {
		if generator == nil {
			return ports.ErrInvalidDraft
		}
		service.newID = generator
		return nil
	}
}

type draftService struct {
	dependencies DraftDependencies
	clock        func() time.Time
	newID        func() uuid.UUID
}

func NewDraftService(
	dependencies DraftDependencies,
	options ...DraftOption,
) (ports.DraftService, error) {
	if invalidDraftDependencies(dependencies) {
		return nil, ports.ErrInvalidDraft
	}
	service := &draftService{
		dependencies: dependencies,
		clock:        func() time.Time { return time.Now().UTC() },
		newID:        uuid.New,
	}
	for _, option := range options {
		if option == nil {
			return nil, ports.ErrInvalidDraft
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	return service, nil
}

var _ ports.DraftService = (*draftService)(nil)

type draftSavedEventPayload struct {
	ProblemID              string `json:"problem_id"`
	ProblemRevisionID      string `json:"problem_revision_id"`
	TestsetRevisionID      string `json:"testset_revision_id"`
	RenderSourceID         string `json:"render_source_id"`
	RenderArtifactID       string `json:"render_artifact_id"`
	ArtifactStatus         string `json:"artifact_status"`
	ContentChecksum        string `json:"content_checksum"`
	TestsetChecksum        string `json:"testset_checksum"`
	RewardSnapshotChecksum string `json:"reward_snapshot_checksum"`
	TestCount              int    `json:"test_count"`
	Version                int64  `json:"version"`
}

type preparedDraft struct {
	title       string
	statement   contentrender.NormalizedSource
	reward      ports.RewardIntent
	checker     json.RawMessage
	runtimeKeys []string
	revisionID  uuid.UUID
	testsetID   uuid.UUID
	source      contentrender.Source
	artifact    contentrender.Artifact
	groups      []draftGroupBlueprint
	assets      []entity.TestAsset
	savedAt     time.Time
	reason      string
}

type draftGroupBlueprint struct {
	id            uuid.UUID
	stopOnFailure bool
	cases         []draftCaseBlueprint
}

type draftCaseBlueprint struct {
	id               uuid.UUID
	testCaseID       uuid.UUID
	kind             string
	inputChecksum    [32]byte
	expectedChecksum [32]byte
}

func (service *draftService) SaveDraft(
	ctx context.Context,
	command ports.SaveDraftCommand,
) (ports.DraftResult, error) {
	if service == nil || invalidDraftDependencies(service.dependencies) ||
		service.clock == nil || service.newID == nil || ctx == nil {
		return ports.DraftResult{}, ports.ErrInvalidDraft
	}
	if err := ctx.Err(); err != nil {
		return ports.DraftResult{}, err
	}
	actorUserID, actorSessionID, err := draftActor(ctx)
	if err != nil {
		return ports.DraftResult{}, err
	}
	ctx = correlation.EnsureContext(ctx)
	prepared, err := service.prepare(ctx, command, actorUserID)
	if err != nil {
		return ports.DraftResult{}, err
	}

	logger.FromContext(ctx).Debug(
		"problem draft save started",
		zap.String("problem_id", command.ProblemID.String()),
		zap.String("catalog_version", command.CatalogVersion),
		zap.Int64("expected_version", command.ExpectedVersion),
		zap.Int("test_count", draftTestCount(command.Groups)),
	)

	var result ports.DraftResult
	transactionErr := service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
		aggregate, err := service.dependencies.Repository.LockAggregate(txCtx, command.ProblemID)
		if err != nil {
			return fmt.Errorf("lock problem aggregate: %w", err)
		}
		if err := aggregate.Validate(); err != nil {
			return err
		}
		if aggregate.Version != command.ExpectedVersion {
			return ports.ErrDraftConflict
		}
		if aggregate.Lifecycle == entity.LifecycleArchived {
			return entity.ErrProblemArchived
		}
		if err := service.dependencies.Authorizer.AuthorizeSaveDraft(txCtx, aggregate); err != nil {
			return errors.Join(ports.ErrDraftDenied, err)
		}
		if err := service.dependencies.Catalog.ValidateAuthoringCatalogVersion(
			txCtx,
			command.CatalogVersion,
		); err != nil {
			return fmt.Errorf("validate draft authoring catalog version: %w", err)
		}
		if err := service.dependencies.Runtimes.ValidateRuntimeKeys(txCtx, prepared.runtimeKeys); err != nil {
			return fmt.Errorf("validate draft runtimes: %w", err)
		}
		if err := service.dependencies.Checkers.ValidateChecker(
			txCtx,
			command.CheckerKey,
			command.CheckerVersion,
			append(json.RawMessage(nil), prepared.checker...),
		); err != nil {
			return fmt.Errorf("validate draft checker: %w", err)
		}
		if err := service.dependencies.Runtimes.ValidateExecutionPairings(
			txCtx,
			prepared.runtimeKeys,
			command.CheckerKey,
			command.CheckerVersion,
		); err != nil {
			return fmt.Errorf("validate draft execution capabilities: %w", err)
		}

		numbers, err := service.dependencies.Repository.NextRevisionNumbers(txCtx, aggregate.ID)
		if err != nil {
			return fmt.Errorf("allocate problem revision numbers: %w", err)
		}
		if !validDraftRevisionNumbers(numbers) {
			return ports.ErrInvalidDraft
		}
		facts, err := service.dependencies.Taxonomy.ResolveRewardFacts(txCtx, prepared.reward)
		if err != nil {
			return fmt.Errorf("resolve draft taxonomy facts: %w", err)
		}
		reward, err := entity.BuildRewardSnapshot(
			entity.RewardMappingIdentity{
				TagMappingID:     aggregate.TagMappingID,
				ElementMappingID: aggregate.ElementMappingID,
			},
			numbers.ProblemRevision,
			facts,
		)
		if err != nil {
			return fmt.Errorf("build draft reward snapshot: %w", err)
		}
		resolvedAssets, err := service.dependencies.Repository.ResolveAssets(
			txCtx,
			cloneTestAssets(prepared.assets),
		)
		if err != nil {
			return fmt.Errorf("resolve draft test assets: %w", err)
		}
		assetsByChecksum, err := validateResolvedAssets(prepared.assets, resolvedAssets)
		if err != nil {
			return err
		}
		testset, err := buildDraftTestset(
			txCtx,
			prepared,
			aggregate.ID,
			actorUserID,
			numbers.TestsetRevision,
			assetsByChecksum,
		)
		if err != nil {
			return err
		}
		if err := service.dependencies.Repository.StoreTestset(txCtx, testset); err != nil {
			return fmt.Errorf("store immutable testset revision: %w", err)
		}

		source, err := service.dependencies.Content.RegisterSource(txCtx, prepared.source)
		if err != nil {
			return fmt.Errorf("register problem render source: %w", err)
		}
		if !sameDraftSource(source, prepared.source) {
			return contentrender.ErrCorruptPersistence
		}
		revision, err := buildDraftRevision(
			prepared,
			command,
			reward,
			aggregate.ID,
			actorUserID,
			numbers.ProblemRevision,
			testset.ID,
		)
		if err != nil {
			return err
		}
		if err := service.dependencies.Repository.StoreRevision(txCtx, revision); err != nil {
			return fmt.Errorf("store immutable problem revision: %w", err)
		}

		artifact, err := service.dependencies.Content.StoreArtifact(
			txCtx,
			source,
			prepared.artifact,
		)
		if err != nil {
			return fmt.Errorf("store immutable render artifact: %w", err)
		}
		if !sameStoredDraftArtifact(artifact, prepared.artifact) ||
			artifact.ID == uuid.Nil {
			return contentrender.ErrCorruptPersistence
		}
		if err := artifact.ValidateFor(source); err != nil {
			return err
		}
		selection := entity.RenderSelection{
			RevisionID:     revision.ID,
			SourceID:       source.ID,
			SourceKind:     string(source.Kind),
			SourceChecksum: source.Checksum,
			ArtifactID:     artifact.ID,
		}
		if err := selection.Validate(); err != nil {
			return err
		}
		updated, err := service.dependencies.Repository.AdvanceDraft(
			txCtx,
			aggregate.ID,
			command.ExpectedVersion,
			selection,
			prepared.savedAt,
		)
		if err != nil {
			return fmt.Errorf("advance problem draft selection: %w", err)
		}
		if err := validateDraftAdvance(updated, aggregate, selection, prepared.savedAt); err != nil {
			return err
		}

		eventID, err := service.identifier()
		if err != nil {
			return err
		}
		payload := draftSavedEventPayload{
			ProblemID:              aggregate.ID.String(),
			ProblemRevisionID:      revision.ID.String(),
			TestsetRevisionID:      testset.ID.String(),
			RenderSourceID:         source.ID.String(),
			RenderArtifactID:       artifact.ID.String(),
			ArtifactStatus:         artifact.Status,
			ContentChecksum:        hex.EncodeToString(revision.ContentChecksum[:]),
			TestsetChecksum:        hex.EncodeToString(testset.Checksum[:]),
			RewardSnapshotChecksum: hex.EncodeToString(revision.Reward.Checksum[:]),
			TestCount:              testset.TestCount,
			Version:                updated.Version,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode problem draft event: %w", err)
		}
		if _, err := service.dependencies.Audit.Append(txCtx, audit.Entry{
			ID:             eventID,
			ActorUserID:    &actorUserID,
			ActorSessionID: &actorSessionID,
			Action:         problemDraftSavedEventV1,
			Resource:       problemAggregateType,
			ResourceID:     aggregate.ID.String(),
			Reason:         prepared.reason,
			Before: map[string]any{
				"lifecycle": string(aggregate.Lifecycle),
				"version":   aggregate.Version,
			},
			After: map[string]any{
				"lifecycle":           string(updated.Lifecycle),
				"version":             updated.Version,
				"problem_revision_id": revision.ID.String(),
				"testset_revision_id": testset.ID.String(),
				"render_source_id":    source.ID.String(),
				"render_artifact_id":  artifact.ID.String(),
				"render_status":       artifact.Status,
				"test_count":          testset.TestCount,
			},
			CorrelationID: correlation.FromContext(txCtx),
			OccurredAt:    prepared.savedAt,
		}); err != nil {
			return fmt.Errorf("append problem draft audit: %w", err)
		}
		if err := service.dependencies.Outbox.Append(txCtx, eventing.Event{
			ID:               eventID,
			Type:             problemDraftSavedEventV1,
			AggregateType:    problemAggregateType,
			AggregateID:      aggregate.ID,
			AggregateVersion: updated.Version,
			CorrelationID:    correlation.FromContext(txCtx),
			Payload:          payloadJSON,
			OccurredAt:       prepared.savedAt,
		}); err != nil {
			return fmt.Errorf("append problem draft event: %w", err)
		}
		result = ports.DraftResult{
			ProblemID:         aggregate.ID,
			RevisionID:        revision.ID,
			TestsetRevisionID: testset.ID,
			RenderSourceID:    source.ID,
			RenderArtifactID:  artifact.ID,
			ArtifactStatus:    artifact.Status,
			Diagnostics:       cloneDiagnostics(artifact.Diagnostics),
			Version:           updated.Version,
			SavedAt:           prepared.savedAt,
		}
		return nil
	})
	if transactionErr != nil {
		logger.FromContext(ctx).Debug(
			"problem draft save rejected",
			zap.String("problem_id", command.ProblemID.String()),
			zap.Int64("expected_version", command.ExpectedVersion),
			zap.Error(transactionErr),
		)
		return ports.DraftResult{}, transactionErr
	}
	logger.FromContext(ctx).Info(
		"problem draft saved",
		zap.String("problem_id", result.ProblemID.String()),
		zap.String("problem_revision_id", result.RevisionID.String()),
		zap.String("testset_revision_id", result.TestsetRevisionID.String()),
		zap.String("render_status", result.ArtifactStatus),
		zap.Int64("version", result.Version),
	)
	return result, nil
}

func (service *draftService) prepare(
	ctx context.Context,
	command ports.SaveDraftCommand,
	actorUserID uuid.UUID,
) (preparedDraft, error) {
	if command.ProblemID == uuid.Nil || command.ExpectedVersion < 1 ||
		!ports.IsAuthoringCatalogVersion(command.CatalogVersion) {
		return preparedDraft{}, ports.ErrInvalidDraft
	}
	reason, err := normalizeDraftReason(command.Reason)
	if err != nil {
		return preparedDraft{}, err
	}
	title := norm.NFC.String(strings.TrimSpace(command.Title))
	if title == "" || !utf8.ValidString(title) || strings.IndexByte(title, 0) >= 0 ||
		len(title) > problemcontract.MaximumTitleBytes {
		return preparedDraft{}, ports.ErrInvalidDraft
	}
	statement, err := contentrender.NormalizeMarkdown(command.StatementMarkdown)
	if err != nil || len(statement.Bytes) == 0 ||
		len(statement.Bytes) > problemcontract.MaximumStatementMarkdownBytes ||
		strings.TrimSpace(string(statement.Bytes)) == "" {
		return preparedDraft{}, errors.Join(ports.ErrInvalidDraft, err)
	}
	reward, err := canonicalDraftRewardIntent(command.Reward)
	if err != nil {
		return preparedDraft{}, errors.Join(ports.ErrInvalidDraft, err)
	}
	if err := command.Limits.Validate(); err != nil {
		return preparedDraft{}, errors.Join(ports.ErrInvalidDraft, err)
	}
	checker, err := entity.CanonicalizeCheckerConfig(command.CheckerConfig)
	if err != nil {
		return preparedDraft{}, errors.Join(ports.ErrInvalidDraft, err)
	}
	runtimeKeys := append([]string(nil), command.AllowedRuntimeKeys...)
	slices.Sort(runtimeKeys)
	if !validDraftProtocolIdentifier(command.CheckerKey, problemcontract.MaximumCheckerKeyBytes) ||
		!validDraftProtocolIdentifier(
			command.CheckerVersion,
			problemcontract.MaximumCheckerVersionBytes,
		) ||
		!validDraftRuntimeKeys(runtimeKeys) {
		return preparedDraft{}, ports.ErrInvalidDraft
	}
	savedAt, err := service.timestamp()
	if err != nil {
		return preparedDraft{}, err
	}
	revisionID, err := service.identifier()
	if err != nil {
		return preparedDraft{}, err
	}
	testsetID, err := service.identifier()
	if err != nil {
		return preparedDraft{}, err
	}
	sourceID, err := service.identifier()
	if err != nil {
		return preparedDraft{}, err
	}
	groups, assets, err := service.prepareTestcases(
		command.Groups,
		savedAt,
	)
	if err != nil {
		return preparedDraft{}, err
	}
	source := contentrender.Source{
		ID:              sourceID,
		OwnerRevisionID: revisionID,
		Kind:            contentrender.ProblemStatement,
		Checksum:        statement.Checksum,
		CreatedBy:       cloneDraftUUID(&actorUserID),
		CreatedAt:       savedAt,
	}
	if err := source.Validate(); err != nil {
		return preparedDraft{}, errors.Join(ports.ErrInvalidDraft, err)
	}
	prepared := preparedDraft{
		title: title, statement: statement, reward: reward, checker: checker,
		runtimeKeys: runtimeKeys, revisionID: revisionID, testsetID: testsetID,
		source: source, groups: groups, assets: assets, savedAt: savedAt,
		reason: reason,
	}
	artifact, err := service.dependencies.Renderer.Render(ctx, contentrender.RenderRequest{
		SourceID:           source.ID,
		SourceKind:         source.Kind,
		SourceChecksum:     source.Checksum,
		NormalizedMarkdown: append([]byte(nil), statement.Bytes...),
	})
	if err != nil {
		return preparedDraft{}, fmt.Errorf("render problem draft: %w", err)
	}
	artifact.CreatedBy = cloneDraftUUID(&actorUserID)
	artifact.CreatedAt = savedAt
	if err := artifact.ValidateFor(source); err != nil {
		return preparedDraft{}, errors.Join(ports.ErrInvalidDraft, err)
	}
	prepared.artifact = artifact
	return prepared, nil
}

func (service *draftService) prepareTestcases(
	groups []ports.DraftTestcaseGroup,
	savedAt time.Time,
) ([]draftGroupBlueprint, []entity.TestAsset, error) {
	if len(groups) == 0 || len(groups) > problemcontract.MaximumTestcaseGroups {
		return nil, nil, ports.ErrInvalidDraft
	}
	preparedGroups := make([]draftGroupBlueprint, len(groups))
	assets := make([]entity.TestAsset, 0)
	assetIndex := make(map[[32]byte]int)
	testCaseIDs := make(map[uuid.UUID]struct{})
	testCount := 0
	hiddenCount := 0
	var totalBytes int64
	for groupIndex, group := range groups {
		if len(group.Cases) == 0 {
			return nil, nil, ports.ErrInvalidDraft
		}
		groupID, err := service.identifier()
		if err != nil {
			return nil, nil, err
		}
		preparedGroups[groupIndex] = draftGroupBlueprint{
			id: groupID, stopOnFailure: group.StopOnFailure,
			cases: make([]draftCaseBlueprint, len(group.Cases)),
		}
		for caseIndex, testcase := range group.Cases {
			testCount++
			if testCount > problemcontract.MaximumTestCases ||
				!problemcontract.IsTestKind(testcase.Kind) {
				return nil, nil, ports.ErrInvalidDraft
			}
			if testcase.Kind == problemcontract.TestKindHidden {
				hiddenCount++
			}
			testCaseID := uuid.Nil
			if testcase.TestCaseID != nil {
				testCaseID = *testcase.TestCaseID
			}
			if testCaseID == uuid.Nil {
				testCaseID, err = service.identifier()
				if err != nil {
					return nil, nil, err
				}
			}
			if _, duplicate := testCaseIDs[testCaseID]; duplicate {
				return nil, nil, ports.ErrInvalidDraft
			}
			testCaseIDs[testCaseID] = struct{}{}
			caseRevisionID, err := service.identifier()
			if err != nil {
				return nil, nil, err
			}
			inputChecksum, err := service.prepareAsset(
				testcase.Input,
				savedAt,
				assetIndex,
				&assets,
			)
			if err != nil {
				return nil, nil, err
			}
			expectedChecksum, err := service.prepareAsset(
				testcase.ExpectedOutput,
				savedAt,
				assetIndex,
				&assets,
			)
			if err != nil {
				return nil, nil, err
			}
			totalBytes += int64(len(testcase.Input)) + int64(len(testcase.ExpectedOutput))
			if totalBytes > problemcontract.MaximumTestsetUncompressedBytes {
				return nil, nil, ports.ErrInvalidDraft
			}
			preparedGroups[groupIndex].cases[caseIndex] = draftCaseBlueprint{
				id: caseRevisionID, testCaseID: testCaseID, kind: testcase.Kind,
				inputChecksum: inputChecksum, expectedChecksum: expectedChecksum,
			}
		}
	}
	if hiddenCount == 0 {
		return nil, nil, errors.Join(ports.ErrInvalidDraft, entity.ErrHiddenTestRequired)
	}
	return preparedGroups, assets, nil
}

func (service *draftService) prepareAsset(
	content []byte,
	savedAt time.Time,
	assetIndex map[[32]byte]int,
	assets *[]entity.TestAsset,
) ([32]byte, error) {
	if len(content) > problemcontract.MaximumTestAssetBytes {
		return [32]byte{}, ports.ErrInvalidDraft
	}
	owned := append([]byte(nil), content...)
	checksum := sha256.Sum256(owned)
	if index, exists := assetIndex[checksum]; exists {
		if !bytes.Equal((*assets)[index].Content, owned) {
			return [32]byte{}, entity.ErrTestAssetIntegrity
		}
		return checksum, nil
	}
	id, err := service.identifier()
	if err != nil {
		return [32]byte{}, err
	}
	asset := entity.TestAsset{
		ID: id, SHA256: checksum, Encoding: problemcontract.AssetEncodingIdentity,
		Content: owned, UncompressedSize: int64(len(owned)), CreatedAt: savedAt,
	}
	if err := asset.ValidateMetadata(); err != nil {
		return [32]byte{}, errors.Join(ports.ErrInvalidDraft, err)
	}
	assetIndex[checksum] = len(*assets)
	*assets = append(*assets, asset)
	return checksum, nil
}

func buildDraftTestset(
	ctx context.Context,
	prepared preparedDraft,
	problemID uuid.UUID,
	actorUserID uuid.UUID,
	revisionNumber int64,
	assets map[[32]byte]entity.TestAsset,
) (entity.TestsetRevision, error) {
	if ctx == nil {
		return entity.TestsetRevision{}, ports.ErrInvalidDraft
	}
	if err := ctx.Err(); err != nil {
		return entity.TestsetRevision{}, err
	}
	testset := entity.TestsetRevision{
		ID: prepared.testsetID, ProblemID: problemID, RevisionNumber: revisionNumber,
		CreatedBy: actorUserID, CreatedAt: prepared.savedAt,
		Groups: make([]entity.TestcaseGroup, len(prepared.groups)),
	}
	for groupIndex, group := range prepared.groups {
		if err := ctx.Err(); err != nil {
			return entity.TestsetRevision{}, err
		}
		testset.Groups[groupIndex] = entity.TestcaseGroup{
			ID: group.id, TestsetRevisionID: testset.ID, GroupIndex: groupIndex,
			StopOnFailure: group.stopOnFailure, CreatedAt: prepared.savedAt,
			Cases: make([]entity.TestcaseRevision, len(group.cases)),
		}
		for caseIndex, testcase := range group.cases {
			if err := ctx.Err(); err != nil {
				return entity.TestsetRevision{}, err
			}
			input, inputOK := assets[testcase.inputChecksum]
			expected, expectedOK := assets[testcase.expectedChecksum]
			if !inputOK || !expectedOK {
				return entity.TestsetRevision{}, entity.ErrTestAssetIntegrity
			}
			testset.Groups[groupIndex].Cases[caseIndex] = entity.TestcaseRevision{
				ID: testcase.id, TestCaseID: testcase.testCaseID,
				TestsetRevisionID: testset.ID, ProblemID: problemID,
				Kind: testcase.kind, GroupIndex: groupIndex, OrderIndex: caseIndex,
				InputAsset: input, ExpectedAsset: expected, Weight: 1,
				StopOnFailure: group.stopOnFailure, CreatedAt: prepared.savedAt,
			}
			testset.TestCount++
			testset.TotalUncompressedBytes +=
				input.UncompressedSize + expected.UncompressedSize
		}
	}
	checksum, err := testset.ComputeChecksum()
	if err != nil {
		return entity.TestsetRevision{}, err
	}
	testset.Checksum = checksum
	if err := testset.ValidateStructure(); err != nil {
		return entity.TestsetRevision{}, err
	}
	if err := testset.VerifyAssets(ctx, entity.IdentityAssetVerifier{}); err != nil {
		return entity.TestsetRevision{}, err
	}
	return testset, nil
}

func buildDraftRevision(
	prepared preparedDraft,
	command ports.SaveDraftCommand,
	reward entity.RewardSnapshot,
	problemID uuid.UUID,
	actorUserID uuid.UUID,
	revisionNumber int64,
	testsetID uuid.UUID,
) (entity.Revision, error) {
	revision := entity.Revision{
		ID: prepared.revisionID, ProblemID: problemID, RevisionNumber: revisionNumber,
		Title: prepared.title, StatementMarkdown: append([]byte(nil), prepared.statement.Bytes...),
		RenderSourceID: prepared.source.ID, RenderSourceKind: string(prepared.source.Kind),
		RenderSourceChecksum: prepared.source.Checksum, Reward: cloneReward(reward),
		Limits: command.Limits, CheckerKey: command.CheckerKey,
		CheckerVersion:     command.CheckerVersion,
		CheckerConfig:      append(json.RawMessage(nil), prepared.checker...),
		TestsetRevisionID:  testsetID,
		AllowedRuntimeKeys: append([]string(nil), prepared.runtimeKeys...),
		CreatedBy:          actorUserID, CreatedAt: prepared.savedAt,
	}
	checksum, err := revision.ComputeContentChecksum()
	if err != nil {
		return entity.Revision{}, err
	}
	revision.ContentChecksum = checksum
	if err := revision.Validate(); err != nil {
		return entity.Revision{}, err
	}
	return revision, nil
}

func canonicalDraftRewardIntent(value ports.RewardIntent) (ports.RewardIntent, error) {
	if value.DifficultyID == uuid.Nil || len(value.TagIDs) > problemcontract.MaximumTags {
		return ports.RewardIntent{}, ports.ErrInvalidDraft
	}
	tagIDs := append([]uuid.UUID(nil), value.TagIDs...)
	sort.Slice(tagIDs, func(first, second int) bool {
		return bytes.Compare(tagIDs[first][:], tagIDs[second][:]) < 0
	})
	result := ports.RewardIntent{
		DifficultyID: value.DifficultyID,
		TagIDs:       tagIDs[:0],
	}
	for _, tagID := range tagIDs {
		if tagID == uuid.Nil {
			return ports.RewardIntent{}, ports.ErrInvalidDraft
		}
		if len(result.TagIDs) == 0 || result.TagIDs[len(result.TagIDs)-1] != tagID {
			result.TagIDs = append(result.TagIDs, tagID)
		}
	}
	return result, nil
}

func validDraftRuntimeKeys(values []string) bool {
	if len(values) == 0 || len(values) > problemcontract.MaximumAllowedRuntimes {
		return false
	}
	for index, value := range values {
		if !validDraftProtocolIdentifier(value, problemcontract.MaximumRuntimeKeyBytes) ||
			(index > 0 && values[index-1] == value) {
			return false
		}
	}
	return true
}

func validDraftProtocolIdentifier(value string, maximumBytes int) bool {
	if len(value) == 0 || len(value) > maximumBytes {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			(index > 0 && (character == '.' || character == '_' || character == '-')) {
			continue
		}
		return false
	}
	return true
}

func validateResolvedAssets(
	candidates []entity.TestAsset,
	resolved []entity.TestAsset,
) (map[[32]byte]entity.TestAsset, error) {
	if len(resolved) != len(candidates) {
		return nil, entity.ErrTestAssetIntegrity
	}
	expected := make(map[[32]byte]entity.TestAsset, len(candidates))
	for _, candidate := range candidates {
		expected[candidate.SHA256] = candidate
	}
	result := make(map[[32]byte]entity.TestAsset, len(resolved))
	for _, asset := range resolved {
		candidate, exists := expected[asset.SHA256]
		if !exists || asset.ID == uuid.Nil ||
			asset.Encoding != candidate.Encoding ||
			asset.UncompressedSize != candidate.UncompressedSize ||
			!bytes.Equal(asset.Content, candidate.Content) {
			return nil, entity.ErrTestAssetIntegrity
		}
		if err := asset.ValidateMetadata(); err != nil {
			return nil, err
		}
		if _, duplicate := result[asset.SHA256]; duplicate {
			return nil, entity.ErrTestAssetIntegrity
		}
		result[asset.SHA256] = cloneTestAssetValue(asset)
	}
	return result, nil
}

func validateDraftAdvance(
	current entity.ProblemAggregate,
	previous entity.ProblemAggregate,
	selection entity.RenderSelection,
	savedAt time.Time,
) error {
	if !sameStableProblemAggregate(current, previous) ||
		current.Version != previous.Version+1 ||
		current.Lifecycle != previous.Lifecycle ||
		current.Draft == nil || *current.Draft != selection ||
		!sameOptionalSelection(current.Published, previous.Published) ||
		!sameOptionalTime(current.PublishedAt, previous.PublishedAt) ||
		!sameOptionalTime(current.ArchivedAt, previous.ArchivedAt) ||
		!current.UpdatedAt.Equal(savedAt) {
		return entity.ErrInvalidProblemAggregate
	}
	return current.Validate()
}

func validDraftRevisionNumbers(value ports.DraftRevisionNumbers) bool {
	return value.ProblemRevision >= 1 &&
		value.ProblemRevision <= problemcontract.MaximumProblemRevisionNumber &&
		value.TestsetRevision >= 1 &&
		value.TestsetRevision <= problemcontract.MaximumTestsetRevisionNumber
}

func sameDraftSource(first, second contentrender.Source) bool {
	return first.ID == second.ID &&
		first.OwnerRevisionID == second.OwnerRevisionID &&
		first.Kind == second.Kind &&
		first.Checksum == second.Checksum &&
		sameUUIDPointer(first.CreatedBy, second.CreatedBy) &&
		first.CreatedAt.Equal(second.CreatedAt)
}

func sameStoredDraftArtifact(first, second contentrender.Artifact) bool {
	return first.RendererVersion == second.RendererVersion &&
		first.SanitizerPolicyVersion == second.SanitizerPolicyVersion &&
		first.SanitizedHTML == second.SanitizedHTML &&
		first.ArtifactChecksum == second.ArtifactChecksum &&
		first.Status == second.Status &&
		reflect.DeepEqual(first.Diagnostics, second.Diagnostics) &&
		sameUUIDPointer(first.CreatedBy, second.CreatedBy) &&
		first.CreatedAt.Equal(second.CreatedAt)
}

func sameUUIDPointer(first, second *uuid.UUID) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}

func cloneReward(value entity.RewardSnapshot) entity.RewardSnapshot {
	result := value
	result.Tags.Items = append([]entity.TagSnapshotItem(nil), value.Tags.Items...)
	if value.Elements != nil {
		elements := *value.Elements
		elements.Items = append([]entity.ElementSnapshotItem(nil), value.Elements.Items...)
		result.Elements = &elements
	}
	return result
}

func cloneTestAssets(values []entity.TestAsset) []entity.TestAsset {
	result := make([]entity.TestAsset, len(values))
	for index := range values {
		result[index] = cloneTestAssetValue(values[index])
	}
	return result
}

func cloneTestAssetValue(value entity.TestAsset) entity.TestAsset {
	result := value
	result.Content = append([]byte(nil), value.Content...)
	return result
}

func cloneDiagnostics(values []contentrender.Diagnostic) []contentrender.Diagnostic {
	if values == nil {
		return nil
	}
	result := make([]contentrender.Diagnostic, len(values))
	copy(result, values)
	return result
}

func cloneDraftUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func draftActor(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !userOK || !sessionOK || userID == uuid.Nil || sessionID == uuid.Nil {
		return uuid.Nil, uuid.Nil, ports.ErrDraftDenied
	}
	return userID, sessionID, nil
}

func normalizeDraftReason(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" || !utf8.ValidString(normalized) ||
		strings.IndexByte(normalized, 0) >= 0 ||
		len(normalized) > problemcontract.MaximumPublicationReasonBytes {
		return "", ports.ErrInvalidDraft
	}
	return normalized, nil
}

func draftTestCount(groups []ports.DraftTestcaseGroup) int {
	total := 0
	for _, group := range groups {
		total += len(group.Cases)
	}
	return total
}

func (service *draftService) timestamp() (time.Time, error) {
	value := service.clock().UTC()
	if value.IsZero() {
		return time.Time{}, ports.ErrInvalidDraft
	}
	return value, nil
}

func (service *draftService) identifier() (uuid.UUID, error) {
	value := service.newID()
	if value == uuid.Nil {
		return uuid.Nil, ports.ErrInvalidDraft
	}
	return value, nil
}

func invalidDraftDependencies(dependencies DraftDependencies) bool {
	return nilDependency(dependencies.Transactions) ||
		nilDependency(dependencies.Repository) ||
		nilDependency(dependencies.Authorizer) ||
		nilDependency(dependencies.Catalog) ||
		nilDependency(dependencies.Runtimes) ||
		nilDependency(dependencies.Checkers) ||
		nilDependency(dependencies.Taxonomy) ||
		nilDependency(dependencies.Renderer) ||
		nilDependency(dependencies.Content) ||
		nilDependency(dependencies.Audit) ||
		nilDependency(dependencies.Outbox)
}
