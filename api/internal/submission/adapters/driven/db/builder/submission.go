package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/submission"
	"github.com/huynhanx03/judgify/internal/submission/core/entity"
)

// BuildCreateSubmission builds the create mutation for Submission.
func BuildCreateSubmission(ctx context.Context, e *entity.Submission) *generate.SubmissionCreate {
	b := global.EntClient.DB(ctx).Submission.Create().
		SetProblemID(e.ProblemID).
		SetUserID(e.UserID).
		SetLanguage(submission.Language(e.Language)).
		SetSourceCode(e.SourceCode).
		SetStatus(submission.Status(e.Status)).
		SetPassedCount(e.PassedCount).
		SetTotalCount(e.TotalCount)

	if e.TimeMs != nil {
		b.SetTimeMs(*e.TimeMs)
	}
	if e.MemoryKb != nil {
		b.SetMemoryKB(*e.MemoryKb)
	}
	if e.ErrorMessage != nil {
		b.SetErrorMessage(*e.ErrorMessage)
	}
	if e.ContestID != nil {
		b.SetContestID(*e.ContestID)
	}
	return b
}

// BuildUpdateSubmission builds the update mutation for Submission.
func BuildUpdateSubmission(ctx context.Context, e *entity.Submission) *generate.SubmissionUpdateOne {
	b := global.EntClient.DB(ctx).Submission.UpdateOneID(e.ID).
		SetStatus(submission.Status(e.Status)).
		SetPassedCount(e.PassedCount).
		SetTotalCount(e.TotalCount)

	if e.TimeMs != nil {
		b.SetTimeMs(*e.TimeMs)
	} else {
		b.ClearTimeMs()
	}
	if e.MemoryKb != nil {
		b.SetMemoryKB(*e.MemoryKb)
	} else {
		b.ClearMemoryKB()
	}
	if e.ErrorMessage != nil {
		b.SetErrorMessage(*e.ErrorMessage)
	} else {
		b.ClearErrorMessage()
	}
	return b
}
