package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type ContentRepository struct{ client *internalent.EntClient }

func NewContentRepository(client *internalent.EntClient) (*ContentRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	return &ContentRepository{client: client}, nil
}

var _ ports.ContentRepository = (*ContentRepository)(nil)

type contestContentLock struct {
	State            entity.ContestState
	RegistrationMode entity.RegistrationMode
	Now              time.Time
}

func (repository *ContentRepository) lockContest(
	ctx context.Context,
	contestID uuid.UUID,
) (contestContentLock, error) {
	if repository == nil || repository.client == nil || ctx == nil || contestID == uuid.Nil {
		return contestContentLock{}, ports.ErrInvalidContestContentCommand
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return contestContentLock{}, errors.Join(ports.ErrInvalidContestContentCommand, err)
	}
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `
		SELECT status, registration_mode, clock_timestamp()
		FROM contests
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, contestID)
	if err != nil {
		return contestContentLock{}, fmt.Errorf("lock contest for content: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return contestContentLock{}, fmt.Errorf("read contest content lock: %w", err)
		}
		return contestContentLock{}, entity.ErrContestNotFound
	}
	var state, registrationMode string
	var result contestContentLock
	if err := rows.Scan(&state, &registrationMode, &result.Now); err != nil {
		return contestContentLock{}, fmt.Errorf("scan contest content lock: %w", err)
	}
	if rows.Next() {
		return contestContentLock{}, fmt.Errorf("contest content lock returned multiple rows")
	}
	result.State = entity.ContestState(state)
	result.RegistrationMode = entity.RegistrationMode(registrationMode)
	return result, rows.Err()
}

func (repository *ContentRepository) requireRegisteredParticipant(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
) error {
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM contest_registrations
			WHERE contest_id = $1 AND user_id = $2 AND status = 'registered'
		)`, contestID, userID)
	if err != nil {
		return fmt.Errorf("check contest content participant: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var registered bool
	if !rows.Next() {
		return fmt.Errorf("check contest content participant: %w", sql.ErrNoRows)
	}
	if err := rows.Scan(&registered); err != nil {
		return fmt.Errorf("scan contest content participant: %w", err)
	}
	if !registered {
		return entity.ErrContestContentRegistration
	}
	return rows.Err()
}

func insertContentRevision(
	ctx context.Context,
	repository *ContentRepository,
	revision entity.ContentRevision,
) (entity.ContentRevision, error) {
	if err := revision.Validate(); err != nil {
		return entity.ContentRevision{}, err
	}
	_, err := repository.client.DB(ctx).Contest.ExecContext(ctx, `
		INSERT INTO contest_content_revisions (
			id, owner_type, owner_id, content_kind, revision_number, title,
			markdown, author_id, render_source_id, render_source_kind,
			normalized_source_checksum, render_artifact_id, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		revision.ID, revision.OwnerType, revision.OwnerID, revision.Kind,
		revision.RevisionNumber, revision.Title, revision.Markdown,
		revision.AuthorID, revision.RenderSourceID, revision.RenderSourceKind,
		revision.SourceChecksum[:], revision.RenderArtifactID, revision.CreatedAt.UTC(),
	)
	if err != nil {
		return entity.ContentRevision{}, fmt.Errorf("insert contest content revision: %w", err)
	}
	return revision, nil
}

func nextContentRevisionNumber(
	ctx context.Context,
	repository *ContentRepository,
	ownerType entity.ContentOwnerType,
	ownerID uuid.UUID,
	kind entity.ContentKind,
) (int64, error) {
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `
		SELECT COALESCE(MAX(revision_number), 0) + 1
		FROM contest_content_revisions
		WHERE owner_type = $1 AND owner_id = $2 AND content_kind = $3`,
		ownerType, ownerID, kind,
	)
	if err != nil {
		return 0, fmt.Errorf("query next contest content revision: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var next int64
	if !rows.Next() {
		return 0, fmt.Errorf("query next contest content revision: %w", sql.ErrNoRows)
	}
	if err := rows.Scan(&next); err != nil || next < 1 {
		if err == nil {
			err = entity.ErrContestContentInvalid
		}
		return 0, fmt.Errorf("scan next contest content revision: %w", err)
	}
	return next, rows.Err()
}

func validContentListQuery(query ports.ContentListQuery, requireActor bool) bool {
	return query.ContestID != uuid.Nil && query.Limit >= 1 &&
		query.Limit <= entity.MaximumContestContentPageSize &&
		(!requireActor || query.ActorID != uuid.Nil) &&
		(query.After == nil || (!query.After.OccurredAt.IsZero() && query.After.ID != uuid.Nil))
}

func cloneContentTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func cloneContentUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
