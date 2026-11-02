package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/constant"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	ratingSettlementRepoName = "ContestRatingSettlement"
	maxPendingRatingBatch    = 1000
)

var (
	ErrInvalidRatingSettlementRepository = errors.New("contest rating settlement repository: invalid dependency or command")
	ErrRatingSettlementStateChanged      = errors.New("contest rating settlement repository: guarded completion matched no contest")
)

// RatingSettlementRepository persists the durable settlement fence. Commands
// deliberately join a caller-owned transaction so the marker can never commit
// independently from rating history and user rating updates.
type RatingSettlementRepository struct {
	client *dbent.EntClient
}

func NewRatingSettlementRepository(client *dbent.EntClient) ports.RatingSettlementRepository {
	return &RatingSettlementRepository{client: client}
}

func (repository *RatingSettlementRepository) FindPending(
	ctx context.Context,
	limit int,
) ([]uuid.UUID, error) {
	if repository == nil || repository.client == nil || ctx == nil || limit <= 0 || limit > maxPendingRatingBatch {
		return nil, ErrInvalidRatingSettlementRepository
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ids, err := repository.client.DB(ctx).Contest.Query().
		Where(
			contest.StatusEQ(contest.Status(constant.ContestStatusEnded)),
			contest.RatedEQ(true),
			contest.RatingCompletedAtIsNil(),
		).
		Order(generate.Asc(contest.FieldEndTime), generate.Asc(contest.FieldID)).
		Limit(limit).
		IDs(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, ratingSettlementRepoName)
	}
	return ids, nil
}

func (repository *RatingSettlementRepository) LockPending(
	ctx context.Context,
	contestID uuid.UUID,
) (bool, error) {
	if err := repository.validateCommand(ctx, contestID); err != nil {
		return false, err
	}

	_, err := repository.client.DB(ctx).Contest.Query().
		Where(
			contest.IDEQ(contestID),
			contest.StatusEQ(contest.Status(constant.ContestStatusEnded)),
			contest.RatedEQ(true),
			contest.RatingCompletedAtIsNil(),
		).
		Select(contest.FieldID).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		OnlyID(ctx)
	if generate.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, commonent.MapEntError(err, ratingSettlementRepoName)
	}
	return true, nil
}

func (repository *RatingSettlementRepository) Complete(
	ctx context.Context,
	contestID uuid.UUID,
	completedAt time.Time,
) error {
	if err := repository.validateCommand(ctx, contestID); err != nil {
		return err
	}
	if completedAt.IsZero() {
		return ErrInvalidRatingSettlementRepository
	}

	updated, err := repository.client.DB(ctx).Contest.Update().
		Where(
			contest.IDEQ(contestID),
			contest.StatusEQ(contest.Status(constant.ContestStatusEnded)),
			contest.RatingCompletedAtIsNil(),
		).
		SetRatingCompletedAt(completedAt.UTC()).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, ratingSettlementRepoName)
	}
	if updated != 1 {
		return ErrRatingSettlementStateChanged
	}
	return nil
}

func (repository *RatingSettlementRepository) validateCommand(ctx context.Context, contestID uuid.UUID) error {
	if repository == nil || repository.client == nil || ctx == nil || contestID == uuid.Nil {
		return ErrInvalidRatingSettlementRepository
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return errors.Join(platformtx.ErrTransactionRequired, fmt.Errorf("%s: transaction required", ratingSettlementRepoName))
	}
	return nil
}

var _ ports.RatingSettlementRepository = (*RatingSettlementRepository)(nil)
