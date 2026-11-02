package db

import (
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
)

// BuildCreateContest builds a complete version-one draft without relying on a
// process-global Ent client.
func BuildCreateContest(
	client *generate.Client,
	value *entity.Contest,
) *generate.ContestCreate {
	return client.Contest.Create().
		SetTitle(value.Title).
		SetDescription(value.Description).
		SetStartTime(value.StartTime).
		SetEndTime(value.EndTime).
		SetStatus(contest.Status(value.EffectiveState())).
		SetVersion(value.Version).
		SetAuthorID(value.AuthorID).
		SetRegistrationMode(
			contest.RegistrationMode(value.RegistrationMode),
		).
		SetNillableRegistrationOpensAt(value.RegistrationOpensAt).
		SetNillableRegistrationClosesAt(value.RegistrationClosesAt).
		SetNillableMaxParticipants(value.ParticipantLimit).
		SetNextSubmissionSequence(value.NextSubmissionSequence).
		SetScoringPolicy(contest.ScoringPolicy(value.ScoringPolicy)).
		SetScoringPolicyVersion(value.ScoringPolicyVersion).
		SetStandingsProjectionVersion(value.StandingsProjectionVersion).
		SetRated(value.Rated).
		SetRatingAlgorithmVersion(value.RatingAlgorithmVersion).
		SetNillableFreezeStandingsAt(value.FreezeStandingsAt)
}

// BuildUpdateContest builds the complete mutable policy replacement. Callers
// own the row lock and optimistic version fence.
func BuildUpdateContest(
	client *generate.Client,
	value *entity.Contest,
) *generate.ContestUpdateOne {
	update := client.Contest.UpdateOneID(value.ID).
		Where(
			contest.StatusEQ(contest.StatusDraft),
			contest.VersionEQ(value.Version-1),
		).
		SetTitle(value.Title).
		SetDescription(value.Description).
		SetStartTime(value.StartTime).
		SetEndTime(value.EndTime).
		SetVersion(value.Version).
		SetRegistrationMode(
			contest.RegistrationMode(value.RegistrationMode),
		).
		SetScoringPolicy(contest.ScoringPolicy(value.ScoringPolicy)).
		SetScoringPolicyVersion(value.ScoringPolicyVersion).
		SetRated(value.Rated).
		SetRatingAlgorithmVersion(value.RatingAlgorithmVersion)

	if value.RegistrationOpensAt == nil {
		update.ClearRegistrationOpensAt()
	} else {
		update.SetRegistrationOpensAt(*value.RegistrationOpensAt)
	}
	if value.RegistrationClosesAt == nil {
		update.ClearRegistrationClosesAt()
	} else {
		update.SetRegistrationClosesAt(*value.RegistrationClosesAt)
	}
	if value.ParticipantLimit == nil {
		update.ClearMaxParticipants()
	} else {
		update.SetMaxParticipants(*value.ParticipantLimit)
	}
	if value.FreezeStandingsAt == nil {
		update.ClearFreezeStandingsAt()
	} else {
		update.SetFreezeStandingsAt(*value.FreezeStandingsAt)
	}
	return update
}
