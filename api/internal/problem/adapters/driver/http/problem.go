package http

import (
	"context"
	"errors"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// ProblemHandler defines the problem HTTP handler interface.
type ProblemHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*ProblemResponse], error)
	Get(ctx context.Context, req *problemIDRequest) (*ProblemResponse, error)
	FindAdmin(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*ProblemResponse], error)
	GetAdmin(ctx context.Context, req *problemIDRequest) (*ProblemResponse, error)
	Create(ctx context.Context, req *CreateProblemRequest) (*ProblemResponse, error)
	GetAuthoringCatalog(ctx context.Context) (*ProblemAuthoringCatalogResponse, error)
	GetAuthoring(ctx context.Context, req *problemIDRequest) (*ProblemAuthoringDraftResponse, error)
	SaveDraft(ctx context.Context, req *SaveProblemDraftRequest) (*ProblemDraftResponse, error)
	Publish(ctx context.Context, req *PublishProblemRequest) (*ProblemPublicationResponse, error)
	Archive(ctx context.Context, req *ArchiveProblemRequest) (*ProblemPublicationResponse, error)
}

type problemHandler struct {
	handler.BaseHandler
	problemService   ports.ProblemService
	authoring        ports.AuthoringQuery
	authoringCatalog ports.AuthoringCatalogQuery
	drafts           ports.DraftService
	publication      ports.PublicationService
}

// NewProblemHandler creates a new ProblemHandler instance.
func NewProblemHandler(problemService ports.ProblemService) ProblemHandler {
	return &problemHandler{problemService: problemService}
}

func NewProblemHandlerWithAuthoring(
	problemService ports.ProblemService,
	authoring ports.AuthoringQuery,
	authoringCatalog ports.AuthoringCatalogQuery,
	drafts ports.DraftService,
	publication ports.PublicationService,
) ProblemHandler {
	return &problemHandler{
		problemService:   problemService,
		authoring:        authoring,
		authoringCatalog: authoringCatalog,
		drafts:           drafts,
		publication:      publication,
	}
}

func (h *problemHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*ProblemResponse], error) {
	results, err := h.problemService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	responses := make([]*ProblemResponse, 0)
	if results.Records != nil {
		responses = make([]*ProblemResponse, len(*results.Records))
		for index, result := range *results.Records {
			responses[index] = toProblemResponse(result)
		}
	}
	return &d.Paginated[*ProblemResponse]{Records: &responses, Pagination: results.Pagination}, nil
}

func (h *problemHandler) Get(ctx context.Context, req *problemIDRequest) (*ProblemResponse, error) {
	id, err := toProblemID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.problemService.Get(ctx, id)
	return toProblemResponse(result), err
}

func (h *problemHandler) FindAdmin(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*ProblemResponse], error) {
	results, err := h.problemService.FindAdmin(ctx, req)
	if err != nil {
		return nil, err
	}
	responses := make([]*ProblemResponse, 0)
	if results.Records != nil {
		responses = make([]*ProblemResponse, len(*results.Records))
		for index, result := range *results.Records {
			responses[index] = toProblemResponse(result)
		}
	}
	return &d.Paginated[*ProblemResponse]{Records: &responses, Pagination: results.Pagination}, nil
}

func (h *problemHandler) GetAdmin(ctx context.Context, req *problemIDRequest) (*ProblemResponse, error) {
	id, err := toProblemID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.problemService.GetAdmin(ctx, id)
	return toProblemResponse(result), err
}

func (h *problemHandler) Create(ctx context.Context, req *CreateProblemRequest) (*ProblemResponse, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "user id not found in context", nil)
	}

	command, err := toCreateProblemCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.problemService.Create(ctx, userID, &command)
	return toProblemResponse(result), err
}

func (h *problemHandler) GetAuthoringCatalog(
	ctx context.Context,
) (*ProblemAuthoringCatalogResponse, error) {
	if h == nil || h.authoringCatalog == nil || ctx == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"problem authoring catalog is unavailable",
			ports.ErrInvalidAuthoringCatalog,
		)
	}
	catalog, err := h.authoringCatalog.GetCatalog(ctx)
	if err != nil {
		return nil, err
	}
	runtimes := make(
		[]ProblemAuthoringRuntimeResponse,
		len(catalog.Runtimes),
	)
	for index, runtime := range catalog.Runtimes {
		runtimes[index] = ProblemAuthoringRuntimeResponse{
			Key:            runtime.Key,
			Language:       runtime.Language,
			DisplayName:    runtime.DisplayName,
			ProfileVersion: runtime.ProfileVersion,
		}
	}
	checkers := make(
		[]ProblemAuthoringCheckerResponse,
		len(catalog.Checkers),
	)
	for index, checker := range catalog.Checkers {
		checkers[index] = ProblemAuthoringCheckerResponse{
			Key:        checker.Key,
			Version:    checker.Version,
			ConfigKind: string(checker.ConfigKind),
		}
	}
	capabilities := make(
		[]ProblemAuthoringCapabilityResponse,
		len(catalog.Capabilities),
	)
	for index, capability := range catalog.Capabilities {
		capabilities[index] = ProblemAuthoringCapabilityResponse{
			RuntimeKey:     capability.RuntimeKey,
			CheckerKey:     capability.CheckerKey,
			CheckerVersion: capability.CheckerVersion,
		}
	}
	return &ProblemAuthoringCatalogResponse{
		ReleaseID:      catalog.ReleaseID,
		CatalogVersion: catalog.CatalogVersion,
		Runtimes:       runtimes,
		Checkers:       checkers,
		Capabilities:   capabilities,
		Policy: ProblemAuthoringPolicyResponse{
			SlugMaximumBytes:      catalog.Policy.SlugMaximumBytes,
			TitleMaximumBytes:     catalog.Policy.TitleMaximumBytes,
			StatementMaximumBytes: catalog.Policy.StatementMaximumBytes,
			ReasonMaximumBytes:    catalog.Policy.ReasonMaximumBytes,
			TestAssetMaximumBytes: catalog.Policy.TestAssetMaximumBytes,
			MaximumTestcases:      catalog.Policy.MaximumTestcases,
			MaximumGroups:         catalog.Policy.MaximumGroups,
			MaximumTags:           catalog.Policy.MaximumTags,
			MaximumRuntimes:       catalog.Policy.MaximumRuntimes,
			CPUTimeMilliseconds:   numericPolicyResponse(catalog.Policy.CPUTimeMilliseconds),
			WallTimeMilliseconds:  numericPolicyResponse(catalog.Policy.WallTimeMilliseconds),
			MemoryKilobytes:       numericPolicyResponse(catalog.Policy.MemoryKilobytes),
			OutputBytes:           numericPolicyResponse(catalog.Policy.OutputBytes),
			ProcessCount:          numericPolicyResponse(catalog.Policy.ProcessCount),
		},
	}, nil
}

func numericPolicyResponse(
	value ports.AuthoringNumericPolicy,
) ProblemAuthoringNumericPolicyResponse {
	return ProblemAuthoringNumericPolicyResponse{
		Default: value.Default,
		Minimum: value.Minimum,
		Maximum: value.Maximum,
	}
}

func (h *problemHandler) GetAuthoring(
	ctx context.Context,
	req *problemIDRequest,
) (*ProblemAuthoringDraftResponse, error) {
	if h == nil || h.authoring == nil || req == nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"problem authoring query is required",
			nil,
		)
	}
	problemID, err := toProblemID(req)
	if err != nil {
		return nil, err
	}
	draft, err := h.authoring.GetDraft(ctx, problemID)
	if err != nil {
		return nil, authoringHTTPError(err)
	}
	groups := make(
		[]AuthoringTestcaseGroupResponse,
		len(draft.Groups),
	)
	for groupIndex, group := range draft.Groups {
		groups[groupIndex] = AuthoringTestcaseGroupResponse{
			StopOnFailure: group.StopOnFailure,
			Cases: make(
				[]AuthoringTestcaseResponse,
				len(group.Cases),
			),
		}
		for caseIndex, testcase := range group.Cases {
			groups[groupIndex].Cases[caseIndex] =
				AuthoringTestcaseResponse{
					TestCaseID:     testcase.TestCaseID.String(),
					Kind:           testcase.Kind,
					Input:          testcase.Input,
					ExpectedOutput: testcase.ExpectedOutput,
				}
		}
	}
	return &ProblemAuthoringDraftResponse{
		ProblemID:         draft.ProblemID.String(),
		Slug:              draft.Slug,
		Lifecycle:         string(draft.Lifecycle),
		Version:           draft.Version,
		RevisionID:        draft.RevisionID.String(),
		TestsetRevisionID: draft.TestsetRevisionID.String(),
		Title:             draft.Title,
		StatementMarkdown: draft.StatementMarkdown,
		DifficultyID:      draft.DifficultyID.String(),
		TagIDs:            stringIDs(draft.TagIDs),
		CPUTimeMs:         draft.Limits.CPUTimeMilliseconds,
		WallTimeMs:        draft.Limits.WallTimeMilliseconds,
		MemoryLimitKb:     draft.Limits.MemoryKilobytes,
		OutputLimitBytes:  draft.Limits.OutputBytes,
		ProcessLimit:      draft.Limits.ProcessCount,
		CheckerKey:        draft.CheckerKey,
		CheckerVersion:    draft.CheckerVersion,
		CheckerConfig:     append([]byte(nil), draft.CheckerConfig...),
		AllowedRuntimeKeys: append(
			[]string(nil),
			draft.AllowedRuntimeKeys...,
		),
		ArtifactStatus: draft.ArtifactStatus,
		ArtifactDiagnostics: append(
			[]string{},
			draft.ArtifactDiagnostics...,
		),
		SavedAt: draft.SavedAt,
		Groups:  groups,
	}, nil
}

func (h *problemHandler) SaveDraft(ctx context.Context, req *SaveProblemDraftRequest) (*ProblemDraftResponse, error) {
	if h == nil || h.drafts == nil || req == nil {
		return nil, apperr.New(apperr.CodeBadRequest, "problem draft request is required", nil)
	}
	command, err := toSaveDraftCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.drafts.SaveDraft(ctx, command)
	if err != nil {
		return nil, authoringHTTPError(err)
	}
	diagnostics := make([]string, len(result.Diagnostics))
	for index, diagnostic := range result.Diagnostics {
		diagnostics[index] = diagnostic.Code
	}
	return &ProblemDraftResponse{ProblemID: result.ProblemID.String(), RevisionID: result.RevisionID.String(), TestsetRevisionID: result.TestsetRevisionID.String(), Version: result.Version, ArtifactStatus: result.ArtifactStatus, ArtifactDiagnostics: diagnostics, SavedAt: result.SavedAt}, nil
}

func (h *problemHandler) Publish(ctx context.Context, req *PublishProblemRequest) (*ProblemPublicationResponse, error) {
	if h == nil || h.publication == nil || req == nil {
		return nil, apperr.New(apperr.CodeBadRequest, "problem publish request is required", nil)
	}
	command, err := toPublishCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.publication.Publish(ctx, command)
	if err != nil {
		return nil, authoringHTTPError(err)
	}
	revisionID, testsetRevisionID := result.PublishedRevisionID.String(), result.TestsetRevisionID.String()
	return &ProblemPublicationResponse{ProblemID: result.ProblemID.String(), RevisionID: &revisionID, TestsetRevisionID: &testsetRevisionID, Version: result.Version, PublishedAt: &result.PublishedAt, Lifecycle: string(entity.LifecyclePublished)}, nil
}

func (h *problemHandler) Archive(ctx context.Context, req *ArchiveProblemRequest) (*ProblemPublicationResponse, error) {
	if h == nil || h.publication == nil || req == nil {
		return nil, apperr.New(apperr.CodeBadRequest, "problem archive request is required", nil)
	}
	command, err := toArchiveCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.publication.Archive(ctx, command)
	if err != nil {
		return nil, authoringHTTPError(err)
	}
	return &ProblemPublicationResponse{ProblemID: result.ID.String(), Version: result.Version, Lifecycle: string(result.Lifecycle)}, nil
}

func authoringHTTPError(err error) error {
	switch {
	case errors.Is(err, ports.ErrDraftConflict), errors.Is(err, ports.ErrPublicationConflict):
		return apperr.New(apperr.CodeConflict, "problem changed; refresh and try again", err)
	case errors.Is(err, ports.ErrRuntimeProfileUnavailable),
		errors.Is(err, ports.ErrCheckerProfileUnavailable),
		errors.Is(err, ports.ErrExecutionCapabilityUnavailable):
		return apperr.New(apperr.CodeConflict, "problem authoring catalog changed; refresh and try again", err)
	case errors.Is(err, ports.ErrAuthoringCatalogChanged):
		return apperr.New(
			apperr.CodeConflict,
			"problem authoring catalog changed; refresh and try again",
			err,
		).WithDetails(map[string]any{"reason": "authoring_catalog_changed"})
	case errors.Is(err, ports.ErrDraftNotFound), errors.Is(err, ports.ErrPublicationNotFound):
		return apperr.New(apperr.CodeNotFound, "problem draft was not found", err)
	case errors.Is(err, ports.ErrAuthoringDraftNotFound):
		return apperr.New(apperr.CodeNotFound, "problem authoring draft was not found", err)
	case errors.Is(err, ports.ErrDraftDenied), errors.Is(err, ports.ErrPublicationDenied):
		return apperr.New(apperr.CodeForbidden, "problem operation is not allowed", err)
	case errors.Is(err, ports.ErrAuthoringDraftDenied):
		return apperr.New(apperr.CodeForbidden, "problem authoring draft is not allowed", err)
	case errors.Is(err, ports.ErrInvalidDraft), errors.Is(err, ports.ErrInvalidPublication), errors.Is(err, ports.ErrInvalidAuthoringQuery), errors.Is(err, ports.ErrInvalidCheckerConfiguration), errors.Is(err, entity.ErrProblemArchived):
		return apperr.New(apperr.CodeBadRequest, "problem authoring request is invalid", err)
	default:
		return err
	}
}
