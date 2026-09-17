package db

import (
	"time"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToContestEntity converts Ent Contest model to domain entity.
func ToContestEntity(m *generate.Contest) *entity.Contest {
	if m == nil {
		return nil
	}
	limit := contestParticipantLimit(m.MaxParticipants)
	legacyLimit := 0
	if limit != nil {
		legacyLimit = *limit
	}
	return &entity.Contest{
		ID:                         m.ID,
		Title:                      m.Title,
		Description:                m.Description,
		StartTime:                  m.StartTime.UTC(),
		EndTime:                    m.EndTime.UTC(),
		Status:                     string(m.Status),
		State:                      entity.ContestState(m.Status),
		Version:                    m.Version,
		AuthorID:                   m.AuthorID,
		MaxParticipants:            legacyLimit,
		ParticipantLimit:           limit,
		RegistrationMode:           entity.RegistrationMode(m.RegistrationMode),
		RegistrationOpensAt:        utcTimePointer(m.RegistrationOpensAt),
		RegistrationClosesAt:       utcTimePointer(m.RegistrationClosesAt),
		ScoringPolicy:              entity.ScoringPolicy(m.ScoringPolicy),
		ScoringPolicyVersion:       m.ScoringPolicyVersion,
		StandingsProjectionVersion: m.StandingsProjectionVersion,
		FreezeStandingsAt:          utcTimePointer(m.FreezeStandingsAt),
		Rated:                      m.Rated,
		RatingAlgorithmVersion:     m.RatingAlgorithmVersion,
		NextSubmissionSequence:     m.NextSubmissionSequence,
		PublishedAt:                utcTimePointer(m.PublishedAt),
		StartedAt:                  utcTimePointer(m.StartedAt),
		EndedAt:                    utcTimePointer(m.EndedAt),
		CancelledAt:                utcTimePointer(m.CancelledAt),
		RatingCompletedAt:          utcTimePointer(m.RatingCompletedAt),
		CreatedAt:                  m.CreatedAt.UTC(),
		UpdatedAt:                  m.UpdatedAt.UTC(),
	}
}

func contestParticipantLimit(value any) *int {
	switch typed := value.(type) {
	case int:
		if typed <= 0 {
			return nil
		}
		copy := typed
		return &copy
	case *int:
		if typed == nil {
			return nil
		}
		copy := *typed
		return &copy
	default:
		return nil
	}
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}
