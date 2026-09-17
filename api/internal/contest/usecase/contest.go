package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/constant"
	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
	problemports "github.com/huynhanx03/judgify/internal/ports"
)

const (
	contestDraftResource     = "contest"
	contestDraftAggregate    = "contest"
	contestDraftCreateAction = "contest.draft.create"
	contestDraftUpdateAction = "contest.draft.update"
	contestDraftDeleteAction = "contest.draft.delete"
	contestDraftCreatedEvent = contestcontract.EventDraftCreatedV1
	contestDraftUpdatedEvent = contestcontract.EventDraftUpdatedV1
	contestDraftDeletedEvent = contestcontract.EventDraftDeletedV1
)

type contestService struct {
	contestRepo ports.ContestRepository
	regRepo     ports.RegistrationProjectionReader
	problems    problemports.ImmutableProblemReader
	journal     *mutation.Journal
}

// NewContestServiceWithJournal creates the production service whose draft
// writes, immutable audit evidence, and outbox events share one transaction.
func NewContestServiceWithJournal(
	contestRepo ports.ContestRepository,
	regRepo ports.RegistrationProjectionReader,
	problems problemports.ImmutableProblemReader,
	journal *mutation.Journal,
) ports.ContestService {
	return &contestService{
		contestRepo: contestRepo,
		regRepo:     regRepo,
		problems:    problems,
		journal:     journal,
	}
}

// NewContestService creates a contest application service.
func NewContestService(
	contestRepo ports.ContestRepository,
	regRepo ports.RegistrationProjectionReader,
	problems problemports.ImmutableProblemReader,
) ports.ContestService {
	return &contestService{
		contestRepo: contestRepo,
		regRepo:     regRepo,
		problems:    problems,
	}
}

// FindPublic retrieves only publicly visible contests. The repository owns the
// access predicate, so caller-supplied filters cannot widen the result.
func (service *contestService) FindPublic(
	ctx context.Context,
	options *d.QueryOptions,
) (*d.Paginated[*contract.ContestResponse], error) {
	contests, err := service.contestRepo.FindPublic(ctx, options)
	if err != nil {
		return nil, err
	}
	return service.mapContestPage(ctx, contests, false)
}

// FindAdmin retrieves every lifecycle status for authorized administration.
func (service *contestService) FindAdmin(
	ctx context.Context,
	options *d.QueryOptions,
) (*d.Paginated[*contract.ContestResponse], error) {
	contests, err := service.contestRepo.Find(ctx, options)
	if err != nil {
		return nil, err
	}
	return service.mapContestPage(ctx, contests, true)
}

func (service *contestService) mapContestPage(
	ctx context.Context,
	contests *d.Paginated[*entity.Contest],
	includeDraftProblems bool,
) (*d.Paginated[*contract.ContestResponse], error) {
	if contests == nil {
		return nil, invalidContestReadModel()
	}
	if contests.Records == nil || len(*contests.Records) == 0 {
		records := make([]*contract.ContestResponse, 0)
		return &d.Paginated[*contract.ContestResponse]{
			Records:    &records,
			Pagination: contests.Pagination,
		}, nil
	}

	entities := *contests.Records
	contestIDs := make([]uuid.UUID, len(entities))
	for index, contest := range entities {
		if contest == nil || contest.ID == uuid.Nil {
			return nil, invalidContestReadModel()
		}
		contestIDs[index] = contest.ID
	}
	readModels, err := service.contestRepo.GetReadModels(
		ctx,
		contestIDs,
		includeDraftProblems,
	)
	if err != nil {
		return nil, err
	}

	responses := make([]*contract.ContestResponse, len(entities))
	for index, contest := range entities {
		readModel, exists := readModels[contest.ID]
		if !exists {
			return nil, invalidContestReadModel()
		}
		var draftProblems []entity.ContestDraftProblem
		problemCount := readModel.ProblemCount
		if includeDraftProblems &&
			contest.EffectiveState() == entity.ContestDraft {
			draftProblems = readModel.DraftProblems
			if draftProblems == nil {
				draftProblems = make([]entity.ContestDraftProblem, 0)
			}
			problemCount = len(draftProblems)
		}
		responses[index] = ToContestResponse(
			contest,
			readModel.ParticipantCount,
			problemCount,
			draftProblems,
		)
	}
	return &d.Paginated[*contract.ContestResponse]{
		Records:    &responses,
		Pagination: contests.Pagination,
	}, nil
}

// GetPublic retrieves a non-draft contest and personalized registration state.
func (service *contestService) GetPublic(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*contract.ContestResponse, error) {
	contest, err := service.contestRepo.GetPublic(ctx, id)
	if err != nil {
		return nil, err
	}
	return service.mapContest(ctx, contest, userID, false)
}

// GetProblem resolves a contest-owned frozen membership before asking the
// Problem domain for exact immutable content. This ordering prevents arbitrary
// revision enumeration through the public route.
func (service *contestService) GetProblem(
	ctx context.Context,
	contestID uuid.UUID,
	contestProblemID uuid.UUID,
) (*contract.ContestProblemDetailResponse, error) {
	if service == nil || service.contestRepo == nil ||
		service.problems == nil || ctx == nil ||
		contestID == uuid.Nil || contestProblemID == uuid.Nil {
		return nil, invalidContestReadModel()
	}
	reference, err := service.contestRepo.GetContestProblemSnapshotRef(
		ctx,
		contestID,
		contestProblemID,
	)
	if err != nil {
		if errors.Is(err, entity.ErrContestNotFound) {
			return nil, apperr.For(
				constant.ContestObjContestProblem,
			).NotFound(err)
		}
		return nil, err
	}
	if !contestProblemVisible(reference) {
		return nil, apperr.For(
			constant.ContestObjContestProblem,
		).NotFound(entity.ErrContestProblemUnavailable)
	}

	problem, err := service.problems.LoadImmutableProblem(
		ctx,
		reference.ProblemID,
		reference.ProblemRevisionID,
		reference.TestsetRevisionID,
	)
	if err != nil {
		return nil, invalidContestProblemReadModel(err)
	}
	if problem.ProblemID != reference.ProblemID ||
		problem.ProblemRevisionID != reference.ProblemRevisionID ||
		problem.TestsetRevisionID != reference.TestsetRevisionID ||
		problem.ProblemSlug != reference.ProblemSlug ||
		problem.RevisionNumber < 1 ||
		problem.SubmissionCount < 0 ||
		problem.AcceptedCount < 0 ||
		problem.AcceptedCount > problem.SubmissionCount {
		return nil, invalidContestProblemReadModel(
			problemports.ErrInvalidImmutableProblem,
		)
	}

	tags := make(
		[]contract.ContestProblemTagResponse,
		len(problem.Tags),
	)
	for index, tag := range problem.Tags {
		if tag.ID == uuid.Nil || tag.Revision < 1 || tag.Name == "" {
			return nil, invalidContestProblemReadModel(
				problemports.ErrInvalidImmutableProblem,
			)
		}
		tags[index] = contract.ContestProblemTagResponse{
			ID:       tag.ID,
			Revision: tag.Revision,
			Name:     tag.Name,
		}
	}
	samples := make(
		[]contract.ContestProblemSampleResponse,
		len(problem.Samples),
	)
	for index, sample := range problem.Samples {
		samples[index] = contract.ContestProblemSampleResponse{
			Input:          sample.Input,
			ExpectedOutput: sample.ExpectedOutput,
			OrderIndex:     index,
		}
	}
	acceptanceRate := float64(0)
	if problem.SubmissionCount > 0 {
		acceptanceRate = float64(problem.AcceptedCount) *
			100 /
			float64(problem.SubmissionCount)
	}
	response := &contract.ContestProblemDetailResponse{
		ContestID:         reference.ContestID,
		ContestProblemID:  reference.ContestProblemID,
		ProblemID:         problem.ProblemID,
		ProblemSlug:       problem.ProblemSlug,
		RevisionNumber:    problem.RevisionNumber,
		Title:             problem.Title,
		StatementMarkdown: problem.StatementMarkdown,
		Difficulty: contract.ContestProblemDifficultyResult{
			ID:          problem.Difficulty.ID,
			Revision:    problem.Difficulty.Revision,
			Name:        problem.Difficulty.Name,
			Level:       problem.Difficulty.Level,
			Description: problem.Difficulty.Description,
		},
		Tags:             tags,
		CPUTimeMs:        problem.Limits.CPUTimeMilliseconds,
		WallTimeMs:       problem.Limits.WallTimeMilliseconds,
		MemoryLimitKb:    problem.Limits.MemoryKilobytes,
		OutputLimitBytes: problem.Limits.OutputBytes,
		ProcessLimit:     problem.Limits.ProcessCount,
		AllowedRuntimeKeys: append(
			[]string(nil),
			problem.AllowedRuntimeKeys...,
		),
		SubmissionCount: problem.SubmissionCount,
		AcceptedCount:   problem.AcceptedCount,
		AcceptanceRate:  acceptanceRate,
		DisplayOrder:    reference.DisplayOrder,
		Alias:           reference.Alias,
		Points:          cloneInt(reference.Points),
		Samples:         samples,
		CreatedAt:       problem.CreatedAt,
	}
	logger.FromContext(ctx).Debug(
		"contest problem snapshot resolved",
		zap.String("contest_id", contestID.String()),
		zap.String(
			"contest_problem_id",
			contestProblemID.String(),
		),
		zap.String("problem_id", problem.ProblemID.String()),
		zap.Int64("problem_revision", problem.RevisionNumber),
	)
	return response, nil
}

func contestProblemVisible(
	reference ports.ContestProblemSnapshotRef,
) bool {
	switch reference.State {
	case entity.ContestRunning, entity.ContestEnded:
		return true
	case entity.ContestUpcoming:
		return reference.VisibleBeforeStart
	case entity.ContestCancelled:
		return reference.VisibleBeforeStart ||
			(reference.CancelledAt != nil &&
				!reference.CancelledAt.Before(reference.StartTime))
	default:
		return false
	}
}

// GetAdmin retrieves a contest regardless of lifecycle state.
func (service *contestService) GetAdmin(
	ctx context.Context,
	id uuid.UUID,
) (*contract.ContestResponse, error) {
	contest, err := service.contestRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return service.mapContest(ctx, contest, uuid.Nil, true)
}

func (service *contestService) mapContest(
	ctx context.Context,
	contest *entity.Contest,
	userID uuid.UUID,
	includeHiddenProblems bool,
) (*contract.ContestResponse, error) {
	if contest == nil || contest.ID == uuid.Nil {
		return nil, invalidContestReadModel()
	}
	readModels, err := service.contestRepo.GetReadModels(
		ctx,
		[]uuid.UUID{contest.ID},
		includeHiddenProblems &&
			contest.EffectiveState() == entity.ContestDraft,
	)
	if err != nil {
		return nil, err
	}
	readModel, exists := readModels[contest.ID]
	if !exists {
		return nil, invalidContestReadModel()
	}
	var draftProblems []entity.ContestDraftProblem
	problemCount := readModel.ProblemCount
	if includeHiddenProblems &&
		contest.EffectiveState() == entity.ContestDraft {
		draftProblems = readModel.DraftProblems
		if draftProblems == nil {
			draftProblems = make([]entity.ContestDraftProblem, 0)
		}
		problemCount = len(draftProblems)
	}
	response := ToContestResponse(
		contest,
		readModel.ParticipantCount,
		problemCount,
		draftProblems,
	)

	if contest.EffectiveState() != entity.ContestDraft {
		problems, err := service.contestRepo.GetContestProblemProjections(
			ctx,
			contest.ID,
		)
		if err != nil {
			return nil, err
		}
		response.Problems = make(
			[]contract.ContestProblemResponse,
			0,
			len(problems),
		)
		showAll := includeHiddenProblems ||
			contest.EffectiveState() == entity.ContestRunning ||
			contest.EffectiveState() == entity.ContestEnded
		for _, problem := range problems {
			if !showAll && !problem.VisibleBeforeStart {
				continue
			}
			response.Problems = append(
				response.Problems,
				contract.ContestProblemResponse{
					ContestProblemID:   problem.ContestProblemID,
					ProblemID:          problem.ProblemID,
					ProblemSlug:        problem.ProblemSlug,
					ProblemTitle:       problem.ProblemTitle,
					DisplayOrder:       problem.DisplayOrder,
					Alias:              problem.Alias,
					Points:             problem.Points,
					VisibleBeforeStart: problem.VisibleBeforeStart,
				},
			)
		}
	}

	if userID != uuid.Nil {
		registration, err := service.regRepo.GetProjection(
			ctx,
			contest.ID,
			userID,
		)
		if err != nil {
			return nil, err
		}
		registered := registration != nil &&
			registration.Status == entity.RegistrationRegistered
		response.IsRegistered = &registered
		if registration != nil {
			response.Registration = &contract.ContestRegistrationResponse{
				ID:      registration.ID,
				Status:  string(registration.Status),
				Version: registration.Version,
			}
		}
	}
	return response, nil
}

// Create atomically persists the contest aggregate and ordered draft problems.
func (service *contestService) Create(
	ctx context.Context,
	authorID uuid.UUID,
	request *contract.CreateContestRequest,
) (*contract.ContestResponse, error) {
	if request == nil || authorID == uuid.Nil ||
		entity.ValidateContestReason(request.Reason, true) != nil {
		return nil, invalidContestDraftError(entity.ErrContestDraftInvalid)
	}
	contest, problems := buildContestDraft(
		uuid.Nil,
		authorID,
		1,
		entity.ScoringPolicyICPC,
		1,
		1,
		request.ContestDraftRequest,
	)
	if err := entity.ValidateContestDraft(*contest, problems); err != nil {
		return nil, invalidContestDraftError(err)
	}
	err := service.executeDraftMutation(ctx, func(txCtx context.Context) (mutation.Command, error) {
		if err := service.contestRepo.CreateDraft(txCtx, contest, problems); err != nil {
			return mutation.Command{}, err
		}
		after := contestDraftDocument(contest, len(problems))
		return contestDraftCommand(
			contestDraftCreateAction,
			contestDraftCreatedEvent,
			request.Reason,
			contest,
			map[string]any{},
			after,
		), nil
	})
	if err != nil {
		if errors.Is(err, entity.ErrContestDraftProblem) {
			return nil, invalidContestDraftError(err)
		}
		return nil, err
	}
	logger.FromContext(ctx).Info(
		"contest draft created",
		zap.String("contest_id", contest.ID.String()),
		zap.String("author_id", authorID.String()),
		zap.String(
			"registration_mode",
			string(contest.RegistrationMode),
		),
		zap.Int("problem_count", len(problems)),
		zap.Int64("version", contest.Version),
	)
	return ToContestResponse(
		contest,
		0,
		len(problems),
		problems,
	), nil
}

// Update replaces a mutable draft under an optimistic aggregate version.
func (service *contestService) Update(
	ctx context.Context,
	id uuid.UUID,
	request *contract.UpdateContestRequest,
) (*contract.ContestResponse, error) {
	if request == nil || id == uuid.Nil || request.ExpectedVersion < 1 ||
		entity.ValidateContestReason(request.Reason, true) != nil {
		return nil, invalidContestDraftError(entity.ErrContestDraftInvalid)
	}
	var contest *entity.Contest
	var problems []entity.ContestDraftProblem
	err := service.executeDraftMutation(ctx, func(txCtx context.Context) (mutation.Command, error) {
		current, err := service.contestRepo.Get(txCtx, id)
		if err != nil {
			return mutation.Command{}, err
		}
		if current.EffectiveState() != entity.ContestDraft {
			return mutation.Command{}, entity.ErrContestNotDraft
		}
		if current.Version != request.ExpectedVersion {
			return mutation.Command{}, entity.ErrContestVersionConflict
		}
		contest, problems = buildContestDraft(
			id,
			current.AuthorID,
			request.ExpectedVersion,
			current.ScoringPolicy,
			current.ScoringPolicyVersion,
			current.NextSubmissionSequence,
			request.ContestDraftRequest,
		)
		if err := entity.ValidateContestDraft(*contest, problems); err != nil {
			return mutation.Command{}, err
		}
		before := contestDraftDocument(current, -1)
		if err := service.contestRepo.UpdateDraft(txCtx, contest, request.ExpectedVersion, problems); err != nil {
			return mutation.Command{}, err
		}
		return contestDraftCommand(
			contestDraftUpdateAction,
			contestDraftUpdatedEvent,
			request.Reason,
			contest,
			before,
			contestDraftDocument(contest, len(problems)),
		), nil
	})
	if err != nil {
		switch {
		case errors.Is(err, entity.ErrContestNotDraft),
			errors.Is(err, entity.ErrContestVersionConflict):
			return nil, contestDraftConflict(err)
		case errors.Is(err, entity.ErrContestDraftProblem):
			return nil, invalidContestDraftError(err)
		default:
			return nil, err
		}
	}
	readModels, err := service.contestRepo.GetReadModels(
		ctx,
		[]uuid.UUID{id},
		true,
	)
	if err != nil {
		return nil, err
	}
	readModel, exists := readModels[id]
	if !exists {
		return nil, invalidContestReadModel()
	}
	logger.FromContext(ctx).Info(
		"contest draft updated",
		zap.String("contest_id", id.String()),
		zap.Int("problem_count", len(problems)),
		zap.Int64("version", contest.Version),
	)
	return ToContestResponse(
		contest,
		readModel.ParticipantCount,
		len(readModel.DraftProblems),
		readModel.DraftProblems,
	), nil
}

// Delete removes only a draft. Published contests must use an audited
// lifecycle cancellation command.
func (service *contestService) Delete(
	ctx context.Context,
	id uuid.UUID,
	reason string,
) error {
	if id == uuid.Nil || entity.ValidateContestReason(reason, true) != nil {
		return invalidContestDraftError(entity.ErrContestDraftInvalid)
	}
	err := service.executeDraftMutation(ctx, func(txCtx context.Context) (mutation.Command, error) {
		current, err := service.contestRepo.Get(txCtx, id)
		if err != nil {
			return mutation.Command{}, err
		}
		if current.EffectiveState() != entity.ContestDraft {
			return mutation.Command{}, entity.ErrContestNotDraft
		}
		before := contestDraftDocument(current, -1)
		if err := service.contestRepo.DeleteDraft(txCtx, id); err != nil {
			return mutation.Command{}, err
		}
		deleted := map[string]any{
			"id":      id.String(),
			"state":   "deleted",
			"version": current.Version + 1,
		}
		commandContest := *current
		commandContest.Version++
		return contestDraftCommand(
			contestDraftDeleteAction,
			contestDraftDeletedEvent,
			reason,
			&commandContest,
			before,
			deleted,
		), nil
	})
	if err != nil {
		if errors.Is(err, entity.ErrContestNotDraft) {
			return contestDraftConflict(err)
		}
		return err
	}
	logger.FromContext(ctx).Info(
		"contest draft deleted",
		zap.String("contest_id", id.String()),
	)
	return nil
}

func (service *contestService) executeDraftMutation(
	ctx context.Context,
	mutate func(context.Context) (mutation.Command, error),
) error {
	if service == nil || service.contestRepo == nil || mutate == nil {
		return entity.ErrContestDraftInvalid
	}
	if service.journal != nil {
		return service.journal.Execute(ctx, mutate)
	}
	_, err := mutate(ctx)
	return err
}

func contestDraftCommand(
	action string,
	eventType string,
	reason string,
	contest *entity.Contest,
	before map[string]any,
	after map[string]any,
) mutation.Command {
	return mutation.Command{
		Action:           action,
		Resource:         contestDraftResource,
		ResourceID:       contest.ID.String(),
		Reason:           reason,
		Before:           before,
		After:            after,
		EventType:        eventType,
		AggregateType:    contestDraftAggregate,
		AggregateID:      contest.ID,
		AggregateVersion: contest.Version,
		Payload:          after,
	}
}

func contestDraftDocument(contest *entity.Contest, problemCount int) map[string]any {
	if contest == nil {
		return map[string]any{}
	}
	document := map[string]any{
		"id":                       contest.ID.String(),
		"version":                  contest.Version,
		"state":                    string(contest.EffectiveState()),
		"author_id":                contest.AuthorID.String(),
		"start_time":               contest.StartTime.UTC().Format(time.RFC3339Nano),
		"end_time":                 contest.EndTime.UTC().Format(time.RFC3339Nano),
		"registration_mode":        string(contest.RegistrationMode),
		"rated":                    contest.Rated,
		"rating_algorithm_version": contest.RatingAlgorithmVersion,
	}
	if problemCount >= 0 {
		document["problem_count"] = problemCount
	}
	return document
}

func buildContestDraft(
	id uuid.UUID,
	authorID uuid.UUID,
	version int64,
	scoringPolicy entity.ScoringPolicy,
	scoringPolicyVersion int64,
	nextSubmissionSequence int64,
	request contract.ContestDraftRequest,
) (*entity.Contest, []entity.ContestDraftProblem) {
	participantLimit := cloneInt(request.MaxParticipants)
	rated := true
	if request.Rated != nil {
		rated = *request.Rated
	}
	contest := &entity.Contest{
		ID:               id,
		Title:            strings.TrimSpace(request.Title),
		Description:      strings.TrimSpace(request.Description),
		StartTime:        request.StartTime.UTC(),
		EndTime:          request.EndTime.UTC(),
		Status:           string(entity.ContestDraft),
		State:            entity.ContestDraft,
		Version:          version,
		AuthorID:         authorID,
		ParticipantLimit: participantLimit,
		RegistrationMode: entity.RegistrationMode(
			request.RegistrationMode,
		),
		RegistrationOpensAt:        utcPointer(request.RegistrationOpensAt),
		RegistrationClosesAt:       utcPointer(request.RegistrationClosesAt),
		ScoringPolicy:              scoringPolicy,
		ScoringPolicyVersion:       scoringPolicyVersion,
		StandingsProjectionVersion: 1,
		FreezeStandingsAt:          utcPointer(request.FreezeStandingsAt),
		Rated:                      rated,
		RatingAlgorithmVersion:     RatingAlgorithmVersion,
		NextSubmissionSequence:     nextSubmissionSequence,
	}
	problems := make(
		[]entity.ContestDraftProblem,
		len(request.Problems),
	)
	for index, problem := range request.Problems {
		problems[index] = entity.ContestDraftProblem{
			ProblemID:          problem.ProblemID,
			DisplayOrder:       index + 1,
			Alias:              strings.TrimSpace(problem.Alias),
			Points:             cloneInt(problem.Points),
			VisibleBeforeStart: problem.VisibleBeforeStart,
		}
	}
	return contest, problems
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func invalidContestDraftError(cause error) *apperr.AppError {
	return apperr.New(
		apperr.CodeValidationFailed,
		"contest draft is invalid",
		cause,
	)
}

func contestDraftConflict(cause error) *apperr.AppError {
	return apperr.New(
		apperr.CodeConflict,
		"contest draft changed or is no longer editable",
		cause,
	)
}

func invalidContestReadModel() *apperr.AppError {
	return apperr.New(
		apperr.CodeInternalServer,
		"contest query returned an invalid result",
		nil,
	)
}

func invalidContestProblemReadModel(cause error) *apperr.AppError {
	return apperr.New(
		apperr.CodeInternalServer,
		"contest problem query returned an invalid result",
		cause,
	)
}
