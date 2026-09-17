package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

type AuthoringQueryDependencies struct {
	Repository ports.AuthoringQueryRepository
	Authorizer ports.AuthoringQueryAuthorizer
}

type authoringQuery struct {
	dependencies AuthoringQueryDependencies
}

func NewAuthoringQuery(
	dependencies AuthoringQueryDependencies,
) (ports.AuthoringQuery, error) {
	if nilAuthoringQueryDependency(dependencies.Repository) ||
		nilAuthoringQueryDependency(dependencies.Authorizer) {
		return nil, ports.ErrInvalidAuthoringQuery
	}
	return &authoringQuery{dependencies: dependencies}, nil
}

var _ ports.AuthoringQuery = (*authoringQuery)(nil)

func (query *authoringQuery) GetDraft(
	ctx context.Context,
	problemID uuid.UUID,
) (ports.AuthoringDraft, error) {
	if query == nil ||
		nilAuthoringQueryDependency(query.dependencies.Repository) ||
		nilAuthoringQueryDependency(query.dependencies.Authorizer) ||
		ctx == nil || problemID == uuid.Nil {
		return ports.AuthoringDraft{}, ports.ErrInvalidAuthoringQuery
	}
	if err := ctx.Err(); err != nil {
		return ports.AuthoringDraft{}, err
	}

	log := logger.FromContext(ctx)
	log.Debug("loading problem authoring draft",
		zap.String("problem_id", problemID.String()),
	)

	aggregate, err := query.dependencies.Repository.LoadAuthoringAggregate(ctx, problemID)
	if err != nil {
		return ports.AuthoringDraft{}, err
	}
	if aggregate.ID != problemID {
		return ports.AuthoringDraft{}, ports.ErrInvalidAuthoringQuery
	}
	if aggregate.Draft == nil {
		return ports.AuthoringDraft{}, ports.ErrAuthoringDraftNotFound
	}
	if err := query.dependencies.Authorizer.AuthorizeReadDraft(ctx, aggregate); err != nil {
		log.Warn("problem authoring draft access denied",
			zap.String("problem_id", problemID.String()),
		)
		return ports.AuthoringDraft{}, err
	}

	draft, err := query.dependencies.Repository.LoadAuthoringDraft(ctx, aggregate)
	if err != nil {
		return ports.AuthoringDraft{}, err
	}
	if err := validateAuthoringDraftProjection(aggregate, draft); err != nil {
		return ports.AuthoringDraft{}, err
	}

	log.Debug("problem authoring draft loaded",
		zap.String("problem_id", problemID.String()),
		zap.String("problem_revision_id", draft.RevisionID.String()),
		zap.String("testset_revision_id", draft.TestsetRevisionID.String()),
		zap.Int64("problem_version", draft.Version),
	)
	return cloneAuthoringDraft(draft), nil
}

func validateAuthoringDraftProjection(
	aggregate entity.ProblemAggregate,
	draft ports.AuthoringDraft,
) error {
	if err := aggregate.Validate(); err != nil ||
		aggregate.Draft == nil ||
		draft.ProblemID != aggregate.ID ||
		draft.Slug != aggregate.Slug ||
		draft.Lifecycle != aggregate.Lifecycle ||
		draft.Version != aggregate.Version ||
		draft.RevisionID != aggregate.Draft.RevisionID ||
		draft.TestsetRevisionID == uuid.Nil ||
		draft.DifficultyID == uuid.Nil ||
		draft.SavedAt.IsZero() ||
		!validAuthoringText(draft.Title, problemcontract.MaximumTitleBytes, true) ||
		!validAuthoringText(
			draft.StatementMarkdown,
			problemcontract.MaximumStatementMarkdownBytes,
			true,
		) ||
		draft.Limits.Validate() != nil ||
		!validAuthoringIdentifier(
			draft.CheckerKey,
			problemcontract.MaximumCheckerKeyBytes,
		) ||
		!validAuthoringIdentifier(
			draft.CheckerVersion,
			problemcontract.MaximumCheckerVersionBytes,
		) {
		return ports.ErrInvalidAuthoringQuery
	}

	canonicalConfig, err := entity.CanonicalizeCheckerConfig(draft.CheckerConfig)
	if err != nil || !bytes.Equal(canonicalConfig, draft.CheckerConfig) {
		return ports.ErrInvalidAuthoringQuery
	}
	if !validAuthoringRuntimeKeys(draft.AllowedRuntimeKeys) ||
		!validAuthoringTagIDs(draft.TagIDs) ||
		!validAuthoringArtifact(draft) ||
		!validAuthoringGroups(draft.Groups) {
		return ports.ErrInvalidAuthoringQuery
	}
	return nil
}

func validAuthoringText(value string, maximumBytes int, required bool) bool {
	if maximumBytes < 1 || len(value) > maximumBytes || !utf8.ValidString(value) ||
		strings.IndexByte(value, 0) >= 0 {
		return false
	}
	return !required || strings.TrimSpace(value) != ""
}

func validAuthoringIdentifier(value string, maximumBytes int) bool {
	if maximumBytes < 1 || len(value) == 0 || len(value) > maximumBytes {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validAuthoringRuntimeKeys(values []string) bool {
	if len(values) == 0 || len(values) > problemcontract.MaximumAllowedRuntimes {
		return false
	}
	previous := ""
	for index, value := range values {
		if !validAuthoringIdentifier(value, problemcontract.MaximumRuntimeKeyBytes) ||
			(index > 0 && previous >= value) {
			return false
		}
		previous = value
	}
	return true
}

func validAuthoringTagIDs(values []uuid.UUID) bool {
	if len(values) > problemcontract.MaximumTags {
		return false
	}
	previous := uuid.Nil
	for index, value := range values {
		if value == uuid.Nil ||
			(index > 0 && bytes.Compare(previous[:], value[:]) >= 0) {
			return false
		}
		previous = value
	}
	return true
}

func validAuthoringArtifact(draft ports.AuthoringDraft) bool {
	switch draft.ArtifactStatus {
	case contentrender.StatusReady:
		return len(draft.ArtifactDiagnostics) == 0
	case contentrender.StatusRejected:
		if len(draft.ArtifactDiagnostics) == 0 ||
			len(draft.ArtifactDiagnostics) > contentrender.DefaultMaxDiagnostics {
			return false
		}
		for _, diagnostic := range draft.ArtifactDiagnostics {
			if !rendercontract.IsDiagnosticCode(diagnostic) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validAuthoringGroups(groups []ports.AuthoringTestcaseGroup) bool {
	if len(groups) == 0 || len(groups) > problemcontract.MaximumTestcaseGroups {
		return false
	}
	seen := make(map[uuid.UUID]struct{})
	testCount := 0
	totalBytes := 0
	hidden := 0
	for _, group := range groups {
		if len(group.Cases) == 0 {
			return false
		}
		for _, testcase := range group.Cases {
			if testcase.TestCaseID == uuid.Nil ||
				!problemcontract.IsTestKind(testcase.Kind) ||
				!validAuthoringText(
					testcase.Input,
					problemcontract.MaximumTestAssetBytes,
					true,
				) ||
				!validAuthoringText(
					testcase.ExpectedOutput,
					problemcontract.MaximumTestAssetBytes,
					true,
				) {
				return false
			}
			if _, duplicate := seen[testcase.TestCaseID]; duplicate {
				return false
			}
			seen[testcase.TestCaseID] = struct{}{}
			testCount++
			totalBytes += len(testcase.Input) + len(testcase.ExpectedOutput)
			if testcase.Kind == problemcontract.TestKindHidden {
				hidden++
			}
			if testCount > problemcontract.MaximumTestCases ||
				totalBytes > problemcontract.MaximumTestsetUncompressedBytes {
				return false
			}
		}
	}
	return hidden > 0
}

func cloneAuthoringDraft(value ports.AuthoringDraft) ports.AuthoringDraft {
	value.TagIDs = append([]uuid.UUID(nil), value.TagIDs...)
	value.CheckerConfig = append(json.RawMessage(nil), value.CheckerConfig...)
	value.AllowedRuntimeKeys = append([]string(nil), value.AllowedRuntimeKeys...)
	value.ArtifactDiagnostics = append([]string(nil), value.ArtifactDiagnostics...)
	value.Groups = append([]ports.AuthoringTestcaseGroup(nil), value.Groups...)
	for groupIndex := range value.Groups {
		value.Groups[groupIndex].Cases = append(
			[]ports.AuthoringTestcase(nil),
			value.Groups[groupIndex].Cases...,
		)
	}
	return value
}

func nilAuthoringQueryDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
