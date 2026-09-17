package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestregistration"
	"github.com/huynhanx03/judgify/internal/ports"
)

const registrationRepoName = "ContestRegistration"

var ErrInvalidRegistrationRepository = errors.New("contest registration repository: invalid command or dependency")

// RegistrationRepository implements ports.RegistrationRepository.
type RegistrationRepository struct {
	client *dbEnt.EntClient
}

// NewRegistrationRepository constructs a RegistrationRepository.
func NewRegistrationRepository(client *dbEnt.EntClient) ports.RegistrationRepository {
	return &RegistrationRepository{client: client}
}

// Register serializes every admission for a contest on its contest row. The
// lifecycle check, duplicate check, capacity check, and insert therefore see a
// single authoritative database state and commit as one operation.
func (r *RegistrationRepository) Register(ctx context.Context, contestID, userID uuid.UUID) error {
	if r == nil || r.client == nil || ctx == nil || contestID == uuid.Nil || userID == uuid.Nil {
		return ErrInvalidRegistrationRepository
	}
	return r.withinRegistrationWindow(ctx, contestID, func(
		txCtx context.Context,
		client *generate.Client,
		record *generate.Contest,
		now time.Time,
	) error {
		exists, err := client.ContestRegistration.Query().
			Where(
				contestregistration.ContestIDEQ(contestID),
				contestregistration.UserIDEQ(userID),
			).
			Exist(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, registrationRepoName)
		}
		if exists {
			return entity.ErrContestAlreadyRegistered
		}

		if limit := legacyContestParticipantLimit(record.MaxParticipants); limit != nil {
			count, err := client.ContestRegistration.Query().
				Where(contestregistration.ContestIDEQ(contestID)).
				Count(txCtx)
			if err != nil {
				return commonEnt.MapEntError(err, registrationRepoName)
			}
			if count >= *limit {
				return entity.ErrContestRegistrationFull
			}
		}

		_, err = client.ContestRegistration.Create().
			SetContestID(contestID).
			SetUserID(userID).
			SetStatus(contestregistration.StatusRegistered).
			SetSource(contestregistration.SourceExplicit).
			SetVersion(1).
			SetOfficialSubmissionCount(0).
			SetRegisteredAt(now).
			Save(txCtx)
		if err != nil {
			mapped := commonEnt.MapEntError(err, registrationRepoName)
			if mapped.Code == apperr.CodeConflict {
				return entity.ErrContestAlreadyRegistered
			}
			return mapped
		}
		return nil
	})
}

// Unregister shares the same contest-row lock as Register. A lifecycle
// transition and an unregister command cannot both succeed across the start
// boundary, even when the status orchestrator is slightly behind the DB clock.
func (r *RegistrationRepository) Unregister(ctx context.Context, contestID, userID uuid.UUID) error {
	if r == nil || r.client == nil || ctx == nil || contestID == uuid.Nil || userID == uuid.Nil {
		return ErrInvalidRegistrationRepository
	}
	return r.withinRegistrationWindow(ctx, contestID, func(
		txCtx context.Context,
		client *generate.Client,
		_ *generate.Contest,
		_ time.Time,
	) error {
		deleted, err := client.ContestRegistration.Delete().
			Where(
				contestregistration.ContestIDEQ(contestID),
				contestregistration.UserIDEQ(userID),
			).
			Exec(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, registrationRepoName)
		}
		if deleted == 0 {
			return entity.ErrContestRegistrationNotFound
		}
		return nil
	})
}

func (r *RegistrationRepository) withinRegistrationWindow(
	ctx context.Context,
	contestID uuid.UUID,
	operation func(
		context.Context,
		*generate.Client,
		*generate.Contest,
		time.Time,
	) error,
) error {
	err := r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		record, now, err := lockRegistrationContest(txCtx, client, contestID)
		if err != nil {
			return err
		}
		if err := validateRegistrationWindow(record, now); err != nil {
			return err
		}
		return operation(txCtx, client, record, now)
	})
	return mapRegistrationTransactionError(err)
}

func lockRegistrationContest(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
) (*generate.Contest, time.Time, error) {
	record, err := client.Contest.Query().
		Where(contest.IDEQ(contestID)).
		Select(
			contest.FieldID,
			contest.FieldStatus,
			contest.FieldStartTime,
			contest.FieldEndTime,
			contest.FieldMaxParticipants,
		).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if generate.IsNotFound(err) {
		return nil, time.Time{}, entity.ErrContestNotFound
	}
	if err != nil {
		return nil, time.Time{}, commonEnt.MapEntError(err, contestRepoName)
	}
	now, err := registrationDatabaseClock(ctx, client)
	if err != nil {
		return nil, time.Time{}, err
	}
	return record, now, nil
}

func registrationDatabaseClock(ctx context.Context, client *generate.Client) (time.Time, error) {
	rows, err := client.Contest.QueryContext(ctx, "SELECT clock_timestamp()")
	if err != nil {
		return time.Time{}, commonEnt.MapEntError(err, registrationRepoName)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return time.Time{}, commonEnt.MapEntError(err, registrationRepoName)
		}
		return time.Time{}, apperr.New(apperr.CodeDatabaseError, "failed to read database clock", nil)
	}
	var now time.Time
	if err := rows.Scan(&now); err != nil {
		return time.Time{}, commonEnt.MapEntError(err, registrationRepoName)
	}
	return now.UTC(), nil
}

func validateRegistrationWindow(record *generate.Contest, now time.Time) error {
	if record == nil || now.IsZero() || record.StartTime.IsZero() ||
		record.EndTime.IsZero() || !record.StartTime.Before(record.EndTime) {
		return entity.ErrContestRegistrationNotOpen
	}
	if record.Status == contest.StatusRunning || record.Status == contest.StatusEnded || !now.Before(record.StartTime) {
		return entity.ErrContestRegistrationStarted
	}
	if record.Status != contest.StatusUpcoming {
		return entity.ErrContestRegistrationNotOpen
	}
	return nil
}

func legacyContestParticipantLimit(value any) *int {
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

func mapRegistrationTransactionError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, ErrInvalidRegistrationRepository) || errors.Is(err, entity.ErrContestNotFound) ||
		errors.Is(err, entity.ErrContestRegistrationNotOpen) || errors.Is(err, entity.ErrContestRegistrationStarted) ||
		errors.Is(err, entity.ErrContestAlreadyRegistered) || errors.Is(err, entity.ErrContestRegistrationFull) ||
		errors.Is(err, entity.ErrContestRegistrationNotFound) {
		return err
	}
	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return commonEnt.MapEntError(fmt.Errorf("registration transaction: %w", err), registrationRepoName)
}

func (r *RegistrationRepository) Exists(ctx context.Context, contestID, userID uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).ContestRegistration.Query().
		Where(
			contestregistration.ContestIDEQ(contestID),
			contestregistration.UserIDEQ(userID),
			contestregistration.StatusEQ(
				contestregistration.StatusRegistered,
			),
		).
		Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, registrationRepoName)
	}
	return exists, nil
}

func (r *RegistrationRepository) GetProjection(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
) (*ports.RegistrationProjection, error) {
	if r == nil || r.client == nil || ctx == nil ||
		contestID == uuid.Nil || userID == uuid.Nil {
		return nil, ErrInvalidRegistrationRepository
	}
	rows, err := r.client.DB(ctx).ContestRegistration.QueryContext(ctx, `
SELECT id, status, version
FROM contest_registrations
WHERE contest_id = $1 AND user_id = $2`,
		contestID,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"read contest registration projection: %w",
			err,
		)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf(
				"read contest registration projection: %w",
				err,
			)
		}
		return nil, nil
	}
	var result ports.RegistrationProjection
	var status string
	if err := rows.Scan(&result.ID, &status, &result.Version); err != nil {
		return nil, fmt.Errorf(
			"scan contest registration projection: %w",
			err,
		)
	}
	if rows.Next() || result.ID == uuid.Nil || result.Version < 1 {
		return nil, ErrInvalidRegistrationRepository
	}
	result.Status = entity.RegistrationStatus(status)
	if !result.Status.Valid() {
		return nil, ErrInvalidRegistrationRepository
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"read contest registration projection: %w",
			err,
		)
	}
	return &result, nil
}

func (r *RegistrationRepository) CountByContest(ctx context.Context, contestID uuid.UUID) (int, error) {
	count, err := r.client.DB(ctx).ContestRegistration.Query().
		Where(
			contestregistration.ContestIDEQ(contestID),
			contestregistration.StatusEQ(
				contestregistration.StatusRegistered,
			),
		).
		Count(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, registrationRepoName)
	}
	return count, nil
}

func (r *RegistrationRepository) FindByContest(ctx context.Context, contestID uuid.UUID) ([]*entity.ContestRegistration, error) {
	records, err := r.client.DB(ctx).ContestRegistration.Query().
		Where(contestregistration.ContestIDEQ(contestID)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, registrationRepoName)
	}

	entities := make([]*entity.ContestRegistration, len(records))
	for i, record := range records {
		entities[i] = ToRegistrationEntity(record)
	}
	return entities, nil
}
