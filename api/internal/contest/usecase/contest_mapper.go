package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
)

// ToContestResponse converts the aggregate and its batched read model to the
// browser-safe API projection.
func ToContestResponse(
	contest *entity.Contest,
	participantCount int,
	problemCount int,
	draftProblems []entity.ContestDraftProblem,
) *contract.ContestResponse {
	if contest == nil {
		return nil
	}
	response := &contract.ContestResponse{
		ID:                         contest.ID,
		Version:                    contest.Version,
		Title:                      contest.Title,
		Description:                contest.Description,
		StartTime:                  contest.StartTime,
		EndTime:                    contest.EndTime,
		Status:                     string(contest.EffectiveState()),
		AuthorID:                   contest.AuthorID,
		RegistrationMode:           string(contest.RegistrationMode),
		RegistrationOpensAt:        contest.RegistrationOpensAt,
		RegistrationClosesAt:       contest.RegistrationClosesAt,
		MaxParticipants:            contest.ParticipantLimit,
		ParticipantCount:           participantCount,
		ProblemCount:               problemCount,
		ScoringPolicy:              string(contest.ScoringPolicy),
		ScoringPolicyVersion:       contest.ScoringPolicyVersion,
		StandingsProjectionVersion: contest.StandingsProjectionVersion,
		FreezeStandingsAt:          contest.FreezeStandingsAt,
		Rated:                      contest.Rated,
		RatingAlgorithmVersion:     contest.RatingAlgorithmVersion,
		PublishedAt:                contest.PublishedAt,
		StartedAt:                  contest.StartedAt,
		EndedAt:                    contest.EndedAt,
		CancelledAt:                contest.CancelledAt,
		RatingCompletedAt:          contest.RatingCompletedAt,
		CreatedAt:                  contest.CreatedAt,
		UpdatedAt:                  contest.UpdatedAt,
	}
	if draftProblems != nil {
		response.DraftProblems = make(
			[]contract.ContestDraftProblemResponse,
			len(draftProblems),
		)
		for index, problem := range draftProblems {
			response.DraftProblems[index] = contract.ContestDraftProblemResponse{
				ProblemID:          problem.ProblemID,
				ProblemSlug:        problem.ProblemSlug,
				ProblemTitle:       problem.ProblemTitle,
				DisplayOrder:       problem.DisplayOrder,
				Alias:              problem.Alias,
				Points:             problem.Points,
				VisibleBeforeStart: problem.VisibleBeforeStart,
			}
		}
	}
	return response
}
