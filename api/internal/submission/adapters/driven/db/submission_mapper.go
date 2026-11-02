package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedsubmission "github.com/huynhanx03/judgify/internal/ent/generate/submission"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

// ToSubmissionEntity converts Ent Submission model to domain entity.
func ToSubmissionEntity(m *generate.Submission) *entity.Submission {
	if m == nil {
		return nil
	}
	return &entity.Submission{
		ID:                          m.ID,
		ProblemID:                   m.ProblemID,
		UserID:                      m.UserID,
		ContestID:                   m.ContestID,
		ContestProblemID:            m.ContestProblemID,
		ContestScoringPolicyVersion: m.ContestScoringPolicyVersion,
		SubmittedAt:                 m.SubmittedAt,
		ContestElapsedMilliseconds:  m.ContestElapsedMs,
		ContestSubmissionSequence:   m.ContestSubmissionSequence,
		Language:                    string(m.Language),
		RuntimeKey:                  m.RuntimeKey,
		SourceCode:                  m.SourceCode,
		SourceSHA256:                copySubmissionHash(m.SourceSha256),
		Version:                     m.Version,
		LatestGeneration:            m.LatestGeneration,
		LatestGenerationPhase:       entity.Phase(m.LatestGenerationPhase),
		LatestGenerationOutcome:     optionalSubmissionOutcome(m.LatestGenerationOutcome),
		ActiveJudgementID:           m.ActiveJudgementID,
		ActiveJudgementGeneration:   m.ActiveJudgementGeneration,
		IdempotencyKey:              m.IdempotencyKey,
		RequestHash:                 copySubmissionHash(m.RequestHash),
		OriginCID:                   m.OriginCid,
		LatestGenerationQueuedAt:    m.LatestGenerationQueuedAt,
		LatestGenerationStartedAt:   m.LatestGenerationStartedAt,
		LatestGenerationCompletedAt: m.LatestGenerationCompletedAt,
		Status:                      string(m.Status),
		JudgedAt:                    m.JudgedAt,
		VerdictVersion:              m.VerdictVersion,
		PassedCount:                 m.PassedCount,
		TotalCount:                  m.TotalCount,
		TimeMs:                      m.TimeMs,
		MemoryKb:                    m.MemoryKB,
		ErrorMessage:                m.ErrorMessage,
		CreatedAt:                   m.CreatedAt,
		UpdatedAt:                   m.UpdatedAt,
	}
}

func copySubmissionHash(value []byte) [32]byte {
	var checksum [32]byte
	copy(checksum[:], value)
	return checksum
}

func optionalSubmissionOutcome(value *generatedsubmission.LatestGenerationOutcome) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
