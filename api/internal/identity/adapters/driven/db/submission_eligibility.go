package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

type SubmissionEligibility struct{ client *internalent.EntClient }

func NewSubmissionEligibility(
	client *internalent.EntClient,
) (*SubmissionEligibility, error) {
	if client == nil {
		return nil, intake.ErrPrincipalIneligible
	}
	return &SubmissionEligibility{client: client}, nil
}

func (eligibility *SubmissionEligibility) RequireEligible(ctx context.Context, userID uuid.UUID) error {
	if eligibility == nil || eligibility.client == nil || ctx == nil || userID == uuid.Nil {
		return intake.ErrPrincipalIneligible
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	rows, err := eligibility.client.DB(ctx).User.QueryContext(ctx, `SELECT status, deleted_at FROM users WHERE id = $1 FOR SHARE`, userID)
	if err != nil {
		return fmt.Errorf("lock submission principal: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fmt.Errorf("lock submission principal: %w", err)
		}
		return intake.ErrPrincipalIneligible
	}
	var status string
	var deletedAt *time.Time
	if err := rows.Scan(&status, &deletedAt); err != nil {
		return fmt.Errorf("scan submission principal: %w", err)
	}
	if rows.Next() {
		return fmt.Errorf("lock submission principal: duplicate user identity")
	}
	if status != "active" || deletedAt != nil {
		return intake.ErrPrincipalIneligible
	}
	return rows.Err()
}

var _ intake.IdentityEligibility = (*SubmissionEligibility)(nil)
