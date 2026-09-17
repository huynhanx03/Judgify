package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/ports"
)

func (repository *ContentRepository) CreateAnnouncement(
	ctx context.Context,
	command ports.AnnouncementDraftCommand,
) (ports.AnnouncementView, error) {
	if !validAnnouncementDraftCommand(command, false) {
		return ports.AnnouncementView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	if contest.State == entity.ContestCancelled {
		return ports.AnnouncementView{}, entity.ErrContestContentStateConflict
	}
	if command.ScheduledFor != nil && !command.ScheduledFor.After(contest.Now) {
		return ports.AnnouncementView{}, entity.ErrContestContentSchedulePending
	}
	command.Revision.OwnerType = entity.ContentOwnerAnnouncement
	command.Revision.OwnerID = command.AnnouncementID
	command.Revision.Kind = entity.AnnouncementKind
	command.Revision.RevisionNumber = 1
	command.Revision.CreatedAt = contest.Now.UTC()
	if _, err := insertContentRevision(ctx, repository, command.Revision); err != nil {
		return ports.AnnouncementView{}, err
	}
	pointer := command.Revision.Pointer()
	_, err = repository.client.DB(ctx).Contest.ExecContext(ctx, `
		INSERT INTO contest_announcements (
			id, contest_id, status, audience,
			current_draft_revision_id, current_draft_owner_type, current_draft_content_kind,
			current_draft_render_source_id, current_draft_render_source_kind,
			current_draft_normalized_source_checksum, current_draft_render_artifact_id,
			version, scheduled_for, created_at, updated_at
		) VALUES ($1,$2,'draft',$3,$4,'announcement','announcement_body',$5,$6,$7,$8,1,$9,$10,$10)`,
		command.AnnouncementID, command.ContestID, command.Audience,
		pointer.RevisionID, pointer.RenderSourceID, pointer.RenderSourceKind,
		pointer.SourceChecksum[:], pointer.RenderArtifactID,
		cloneContentTime(command.ScheduledFor), contest.Now.UTC(),
	)
	if err != nil {
		return ports.AnnouncementView{}, fmt.Errorf("insert contest announcement: %w", err)
	}
	return repository.getAdminAnnouncement(ctx, command.ContestID, command.AnnouncementID)
}

func (repository *ContentRepository) EditAnnouncement(
	ctx context.Context,
	command ports.AnnouncementDraftCommand,
) (ports.AnnouncementView, error) {
	if !validAnnouncementDraftCommand(command, true) {
		return ports.AnnouncementView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	if contest.State == entity.ContestCancelled {
		return ports.AnnouncementView{}, entity.ErrContestContentStateConflict
	}
	current, err := repository.lockAnnouncement(ctx, command.ContestID, command.AnnouncementID)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	if current.Status == entity.AnnouncementWithdrawn {
		return ports.AnnouncementView{}, entity.ErrContestContentStateConflict
	}
	if current.Version != command.ExpectedVersion {
		return ports.AnnouncementView{}, entity.ErrContestContentVersionConflict
	}
	if command.ScheduledFor != nil && !command.ScheduledFor.After(contest.Now) {
		return ports.AnnouncementView{}, entity.ErrContestContentSchedulePending
	}
	next, err := nextContentRevisionNumber(ctx, repository, entity.ContentOwnerAnnouncement, command.AnnouncementID, entity.AnnouncementKind)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	command.Revision.OwnerType = entity.ContentOwnerAnnouncement
	command.Revision.OwnerID = command.AnnouncementID
	command.Revision.Kind = entity.AnnouncementKind
	command.Revision.RevisionNumber = next
	command.Revision.CreatedAt = contest.Now.UTC()
	if _, err := insertContentRevision(ctx, repository, command.Revision); err != nil {
		return ports.AnnouncementView{}, err
	}
	pointer := command.Revision.Pointer()
	result, err := repository.client.DB(ctx).Contest.ExecContext(ctx, `
		UPDATE contest_announcements
		SET audience = $3,
			current_draft_revision_id = $4,
			current_draft_owner_type = 'announcement',
			current_draft_content_kind = 'announcement_body',
			current_draft_render_source_id = $5,
			current_draft_render_source_kind = $6,
			current_draft_normalized_source_checksum = $7,
			current_draft_render_artifact_id = $8,
			version = version + 1, scheduled_for = $9, updated_at = $10
		WHERE id = $1 AND contest_id = $2 AND version = $11`,
		command.AnnouncementID, command.ContestID, command.Audience,
		pointer.RevisionID, pointer.RenderSourceID, pointer.RenderSourceKind,
		pointer.SourceChecksum[:], pointer.RenderArtifactID,
		cloneContentTime(command.ScheduledFor), contest.Now.UTC(), command.ExpectedVersion,
	)
	if err != nil {
		return ports.AnnouncementView{}, fmt.Errorf("update contest announcement draft: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return ports.AnnouncementView{}, entity.ErrContestContentVersionConflict
	}
	return repository.getAdminAnnouncement(ctx, command.ContestID, command.AnnouncementID)
}

func (repository *ContentRepository) PublishAnnouncement(
	ctx context.Context,
	command ports.AnnouncementStateCommand,
) (ports.AnnouncementView, error) {
	if !validAnnouncementStateCommand(command) {
		return ports.AnnouncementView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	if contest.State == entity.ContestDraft || contest.State == entity.ContestCancelled {
		return ports.AnnouncementView{}, entity.ErrContestContentStateConflict
	}
	current, err := repository.lockAnnouncement(ctx, command.ContestID, command.AnnouncementID)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	if current.Status == entity.AnnouncementWithdrawn || current.Version != command.ExpectedVersion {
		if current.Version != command.ExpectedVersion {
			return ports.AnnouncementView{}, entity.ErrContestContentVersionConflict
		}
		return ports.AnnouncementView{}, entity.ErrContestContentStateConflict
	}
	if current.ScheduledFor != nil && contest.Now.Before(*current.ScheduledFor) {
		return ports.AnnouncementView{}, entity.ErrContestContentSchedulePending
	}
	if err := repository.requireReadyArtifact(ctx, current.CurrentDraft); err != nil {
		return ports.AnnouncementView{}, err
	}
	pointer := current.CurrentDraft
	result, err := repository.client.DB(ctx).Contest.ExecContext(ctx, `
		UPDATE contest_announcements
		SET status = 'published',
			published_revision_id = $3, published_owner_type = 'announcement',
			published_content_kind = 'announcement_body', published_render_source_id = $4,
			published_render_source_kind = $5, published_normalized_source_checksum = $6,
			published_render_artifact_id = $7, version = version + 1,
			published_at = $8, withdrawn_at = NULL, scheduled_for = NULL,
			updated_at = $8
		WHERE id = $1 AND contest_id = $2 AND version = $9`,
		command.AnnouncementID, command.ContestID, pointer.RevisionID,
		pointer.RenderSourceID, pointer.RenderSourceKind, pointer.SourceChecksum[:],
		pointer.RenderArtifactID, contest.Now.UTC(), command.ExpectedVersion,
	)
	if err != nil {
		return ports.AnnouncementView{}, fmt.Errorf("publish contest announcement: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return ports.AnnouncementView{}, entity.ErrContestContentVersionConflict
	}
	return repository.getPublishedAnnouncement(ctx, command.ContestID, command.AnnouncementID)
}

func (repository *ContentRepository) WithdrawAnnouncement(
	ctx context.Context,
	command ports.AnnouncementStateCommand,
) (ports.AnnouncementView, error) {
	if !validAnnouncementStateCommand(command) || command.System {
		return ports.AnnouncementView{}, ports.ErrInvalidContestContentCommand
	}
	contest, err := repository.lockContest(ctx, command.ContestID)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	current, err := repository.lockAnnouncement(ctx, command.ContestID, command.AnnouncementID)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	if current.Version != command.ExpectedVersion {
		return ports.AnnouncementView{}, entity.ErrContestContentVersionConflict
	}
	if current.Status == entity.AnnouncementWithdrawn {
		return ports.AnnouncementView{}, entity.ErrContestContentStateConflict
	}
	result, err := repository.client.DB(ctx).Contest.ExecContext(ctx, `
		UPDATE contest_announcements
		SET status = 'withdrawn', version = version + 1,
			withdrawn_at = $3, updated_at = $3
		WHERE id = $1 AND contest_id = $2 AND version = $4`,
		command.AnnouncementID, command.ContestID, contest.Now.UTC(), command.ExpectedVersion,
	)
	if err != nil {
		return ports.AnnouncementView{}, fmt.Errorf("withdraw contest announcement: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return ports.AnnouncementView{}, entity.ErrContestContentVersionConflict
	}
	return repository.getAdminAnnouncement(ctx, command.ContestID, command.AnnouncementID)
}

func (repository *ContentRepository) ListPublicAnnouncements(
	ctx context.Context,
	query ports.ContentListQuery,
) ([]ports.AnnouncementView, *ports.ContentCursor, error) {
	return repository.listAnnouncements(ctx, query, "public", false)
}

func (repository *ContentRepository) ListParticipantAnnouncements(
	ctx context.Context,
	query ports.ContentListQuery,
) ([]ports.AnnouncementView, *ports.ContentCursor, error) {
	if !validContentListQuery(query, true) {
		return nil, nil, ports.ErrInvalidContestContentCommand
	}
	if err := repository.requireRegisteredParticipant(ctx, query.ContestID, query.ActorID); err != nil {
		return nil, nil, err
	}
	return repository.listAnnouncements(ctx, query, "participant", false)
}

func (repository *ContentRepository) ListAdminAnnouncements(
	ctx context.Context,
	query ports.ContentListQuery,
) ([]ports.AnnouncementView, *ports.ContentCursor, error) {
	return repository.listAnnouncements(ctx, query, "admin", true)
}

func (repository *ContentRepository) FindDueAnnouncements(ctx context.Context, limit int) ([]ports.DueAnnouncement, error) {
	if repository == nil || repository.client == nil || ctx == nil || limit < 1 || limit > entity.MaximumContestContentPageSize {
		return nil, ports.ErrInvalidContestContentCommand
	}
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `
		SELECT announcement.contest_id, announcement.id, announcement.version
		FROM contest_announcements announcement
		JOIN contests contest ON contest.id = announcement.contest_id AND contest.deleted_at IS NULL
		WHERE announcement.status <> 'withdrawn'
			AND announcement.scheduled_for IS NOT NULL
			AND announcement.scheduled_for <= clock_timestamp()
			AND contest.status IN ('upcoming','running','ended')
		ORDER BY announcement.scheduled_for, announcement.id
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("find due contest announcements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]ports.DueAnnouncement, 0, limit)
	for rows.Next() {
		var item ports.DueAnnouncement
		if err := rows.Scan(&item.ContestID, &item.AnnouncementID, &item.Version); err != nil ||
			item.ContestID == uuid.Nil || item.AnnouncementID == uuid.Nil || item.Version < 1 {
			if err == nil {
				err = entity.ErrContestContentInvalid
			}
			return nil, fmt.Errorf("scan due contest announcement: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *ContentRepository) listAnnouncements(
	ctx context.Context,
	query ports.ContentListQuery,
	visibility string,
	currentDraft bool,
) ([]ports.AnnouncementView, *ports.ContentCursor, error) {
	if repository == nil || repository.client == nil || ctx == nil || !validContentListQuery(query, visibility == "participant") {
		return nil, nil, ports.ErrInvalidContestContentCommand
	}
	prefix := "published"
	timeColumn := "announcement.published_at"
	where := []string{"announcement.contest_id = $1"}
	arguments := []any{query.ContestID}
	switch visibility {
	case "public":
		where = append(where, "announcement.status = 'published'", "announcement.audience = 'public'", "contest.status IN ('upcoming','running','ended')")
	case "participant":
		where = append(where, "announcement.status = 'published'", "announcement.audience IN ('public','participants')", "contest.status IN ('upcoming','running','ended')")
	case "admin":
		prefix = "current_draft"
		timeColumn = "announcement.updated_at"
		if query.Status != "" {
			arguments = append(arguments, query.Status)
			where = append(where, fmt.Sprintf("announcement.status = $%d", len(arguments)))
		}
	default:
		return nil, nil, ports.ErrInvalidContestContentCommand
	}
	if query.After != nil {
		arguments = append(arguments, query.After.OccurredAt.UTC(), query.After.ID)
		where = append(where, fmt.Sprintf("(%s < $%d OR (%s = $%d AND announcement.id < $%d))", timeColumn, len(arguments)-1, timeColumn, len(arguments)-1, len(arguments)))
	}
	arguments = append(arguments, query.Limit+1)
	statement := announcementSelect(prefix) + `
		FROM contest_announcements announcement
		JOIN contests contest ON contest.id = announcement.contest_id AND contest.deleted_at IS NULL
		JOIN contest_content_revisions revision ON revision.id = announcement.` + prefix + `_revision_id
		JOIN content_render_artifacts artifact ON artifact.id = announcement.` + prefix + `_render_artifact_id
		WHERE ` + strings.Join(where, " AND ") + `
			AND artifact.status = 'ready'
			AND artifact.sanitizer_policy_version = $` + fmt.Sprint(len(arguments)+1) + `
		ORDER BY ` + timeColumn + ` DESC, announcement.id DESC
		LIMIT $` + fmt.Sprint(len(arguments))
	arguments = append(arguments, contentrender.CurrentSanitizerPolicyVersion)
	// The sanitizer value follows the already-appended LIMIT argument, matching
	// the explicit parameter position embedded in the statement above.
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, nil, fmt.Errorf("list contest announcements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	views := make([]ports.AnnouncementView, 0, query.Limit+1)
	for rows.Next() {
		view, err := scanAnnouncementView(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan contest announcement: %w", err)
		}
		views = append(views, view)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("list contest announcements: %w", err)
	}
	var next *ports.ContentCursor
	if len(views) > query.Limit {
		views = views[:query.Limit]
		last := views[len(views)-1].Announcement
		occurredAt := last.UpdatedAt
		if !currentDraft && last.PublishedAt != nil {
			occurredAt = *last.PublishedAt
		}
		next = &ports.ContentCursor{OccurredAt: occurredAt, ID: last.ID}
	}
	return views, next, nil
}

func (repository *ContentRepository) getAdminAnnouncement(ctx context.Context, contestID, announcementID uuid.UUID) (ports.AnnouncementView, error) {
	return repository.getAnnouncement(ctx, contestID, announcementID, "current_draft")
}

func (repository *ContentRepository) GetAdminAnnouncement(ctx context.Context, contestID, announcementID uuid.UUID) (ports.AnnouncementView, error) {
	return repository.getAdminAnnouncement(ctx, contestID, announcementID)
}

func (repository *ContentRepository) getPublishedAnnouncement(ctx context.Context, contestID, announcementID uuid.UUID) (ports.AnnouncementView, error) {
	return repository.getAnnouncement(ctx, contestID, announcementID, "published")
}

func (repository *ContentRepository) getAnnouncement(ctx context.Context, contestID, announcementID uuid.UUID, prefix string) (ports.AnnouncementView, error) {
	if prefix != "current_draft" && prefix != "published" {
		return ports.AnnouncementView{}, ports.ErrInvalidContestContentCommand
	}
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, announcementSelect(prefix)+`
		FROM contest_announcements announcement
		JOIN contest_content_revisions revision ON revision.id = announcement.`+prefix+`_revision_id
		JOIN content_render_artifacts artifact ON artifact.id = announcement.`+prefix+`_render_artifact_id
		WHERE announcement.id = $1 AND announcement.contest_id = $2
			AND artifact.status = 'ready'
			AND artifact.sanitizer_policy_version = $3`,
		announcementID, contestID, contentrender.CurrentSanitizerPolicyVersion,
	)
	if err != nil {
		return ports.AnnouncementView{}, fmt.Errorf("get contest announcement: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return ports.AnnouncementView{}, fmt.Errorf("get contest announcement: %w", err)
		}
		return ports.AnnouncementView{}, entity.ErrContestContentNotFound
	}
	view, err := scanAnnouncementView(rows)
	if err != nil {
		return ports.AnnouncementView{}, fmt.Errorf("scan contest announcement: %w", err)
	}
	if rows.Next() {
		return ports.AnnouncementView{}, fmt.Errorf("contest announcement query returned multiple rows")
	}
	return view, rows.Err()
}

func announcementSelect(prefix string) string {
	return `SELECT
		announcement.id, announcement.contest_id, announcement.status, announcement.audience,
		announcement.current_draft_revision_id, announcement.current_draft_content_kind,
		announcement.current_draft_render_source_id, announcement.current_draft_render_source_kind,
		announcement.current_draft_normalized_source_checksum, announcement.current_draft_render_artifact_id,
		announcement.published_revision_id, announcement.published_content_kind,
		announcement.published_render_source_id, announcement.published_render_source_kind,
		announcement.published_normalized_source_checksum, announcement.published_render_artifact_id,
		announcement.version, announcement.scheduled_for, announcement.published_at,
		announcement.withdrawn_at, announcement.created_at, announcement.updated_at,
		revision.id, revision.owner_type, revision.owner_id, revision.content_kind,
		revision.revision_number, revision.title, revision.markdown, revision.author_id,
		revision.render_source_id, revision.render_source_kind,
		revision.normalized_source_checksum, revision.render_artifact_id, revision.created_at,
		artifact.sanitized_html`
}

func (repository *ContentRepository) lockAnnouncement(ctx context.Context, contestID, announcementID uuid.UUID) (entity.Announcement, error) {
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `SELECT
		id, contest_id, status, audience,
		current_draft_revision_id, current_draft_content_kind,
		current_draft_render_source_id, current_draft_render_source_kind,
		current_draft_normalized_source_checksum, current_draft_render_artifact_id,
		published_revision_id, published_content_kind, published_render_source_id,
		published_render_source_kind, published_normalized_source_checksum,
		published_render_artifact_id, version, scheduled_for, published_at,
		withdrawn_at, created_at, updated_at
		FROM contest_announcements
		WHERE id = $1 AND contest_id = $2
		FOR UPDATE`, announcementID, contestID)
	if err != nil {
		return entity.Announcement{}, fmt.Errorf("lock contest announcement: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.Announcement{}, entity.ErrContestContentNotFound
	}
	announcement, err := scanAnnouncement(rows)
	if err != nil {
		return entity.Announcement{}, fmt.Errorf("scan locked contest announcement: %w", err)
	}
	return announcement, rows.Err()
}

func (repository *ContentRepository) requireReadyArtifact(ctx context.Context, pointer entity.RenderPointer) error {
	rows, err := repository.client.DB(ctx).Contest.QueryContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM content_render_artifacts
			WHERE id = $1 AND render_source_id = $2 AND render_source_kind = $3
				AND source_checksum = $4 AND status = 'ready'
				AND sanitizer_policy_version = $5
		)`, pointer.RenderArtifactID, pointer.RenderSourceID, pointer.RenderSourceKind,
		pointer.SourceChecksum[:], contentrender.CurrentSanitizerPolicyVersion,
	)
	if err != nil {
		return fmt.Errorf("verify contest content render artifact: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ready bool
	if !rows.Next() {
		return fmt.Errorf("verify contest content render artifact: %w", sql.ErrNoRows)
	}
	if err := rows.Scan(&ready); err != nil {
		return fmt.Errorf("scan contest content render artifact: %w", err)
	}
	if !ready {
		return entity.ErrContestContentNotReady
	}
	return rows.Err()
}

func validAnnouncementDraftCommand(command ports.AnnouncementDraftCommand, edit bool) bool {
	return command.ContestID != uuid.Nil && command.AnnouncementID != uuid.Nil &&
		command.Audience.Valid() && command.Revision.ID != uuid.Nil &&
		command.Revision.AuthorID != uuid.Nil && command.ActorUserID != uuid.Nil &&
		command.CommandID != uuid.Nil && entity.ValidateContestContentReason(command.Reason, true) == nil &&
		(!edit || command.ExpectedVersion > 0) &&
		(command.ActorSessionID == nil || *command.ActorSessionID != uuid.Nil) &&
		(command.ScheduledFor == nil || !command.ScheduledFor.IsZero())
}

func validAnnouncementStateCommand(command ports.AnnouncementStateCommand) bool {
	return command.ContestID != uuid.Nil && command.AnnouncementID != uuid.Nil &&
		command.ExpectedVersion > 0 && command.CommandID != uuid.Nil &&
		((command.System && command.ActorUserID == uuid.Nil && command.ActorSessionID == nil) ||
			(!command.System && command.ActorUserID != uuid.Nil && command.ActorSessionID != nil && *command.ActorSessionID != uuid.Nil)) &&
		entity.ValidateContestContentReason(command.Reason, true) == nil
}

type announcementRowScanner interface{ Scan(...any) error }

func scanAnnouncement(row announcementRowScanner) (entity.Announcement, error) {
	var value entity.Announcement
	var status, audience, draftKind, draftSourceKind string
	var draftChecksum []byte
	var publishedRevision, publishedSource, publishedArtifact uuid.NullUUID
	var publishedKind, publishedSourceKind sql.NullString
	var publishedChecksum []byte
	var scheduled, publishedAt, withdrawnAt sql.NullTime
	err := row.Scan(
		&value.ID, &value.ContestID, &status, &audience,
		&value.CurrentDraft.RevisionID, &draftKind, &value.CurrentDraft.RenderSourceID,
		&draftSourceKind, &draftChecksum, &value.CurrentDraft.RenderArtifactID,
		&publishedRevision, &publishedKind, &publishedSource, &publishedSourceKind,
		&publishedChecksum, &publishedArtifact, &value.Version, &scheduled,
		&publishedAt, &withdrawnAt, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return entity.Announcement{}, err
	}
	value.Status = entity.AnnouncementStatus(status)
	value.Audience = entity.AnnouncementAudience(audience)
	value.CurrentDraft.ContentKind = entity.ContentKind(draftKind)
	value.CurrentDraft.RenderSourceKind = draftSourceKind
	if len(draftChecksum) != 32 {
		return entity.Announcement{}, entity.ErrContestContentInvalid
	}
	copy(value.CurrentDraft.SourceChecksum[:], draftChecksum)
	value.ScheduledFor = nullableContentTime(scheduled)
	value.PublishedAt = nullableContentTime(publishedAt)
	value.WithdrawnAt = nullableContentTime(withdrawnAt)
	if publishedRevision.Valid || publishedSource.Valid || publishedArtifact.Valid || publishedKind.Valid || publishedSourceKind.Valid || len(publishedChecksum) > 0 {
		if !publishedRevision.Valid || !publishedSource.Valid || !publishedArtifact.Valid || !publishedKind.Valid || !publishedSourceKind.Valid || len(publishedChecksum) != 32 {
			return entity.Announcement{}, entity.ErrContestContentInvalid
		}
		pointer := entity.RenderPointer{
			RevisionID: publishedRevision.UUID, ContentKind: entity.ContentKind(publishedKind.String),
			RenderSourceID: publishedSource.UUID, RenderSourceKind: publishedSourceKind.String,
			RenderArtifactID: publishedArtifact.UUID,
		}
		copy(pointer.SourceChecksum[:], publishedChecksum)
		value.Published = &pointer
	}
	if err := value.Validate(); err != nil {
		return entity.Announcement{}, err
	}
	return value, nil
}

func scanAnnouncementView(row announcementRowScanner) (ports.AnnouncementView, error) {
	// Scan the aggregate first using a temporary collector because database/sql
	// requires all destinations in one call.
	var value entity.Announcement
	var status, audience, draftKind, draftSourceKind string
	var draftChecksum []byte
	var publishedRevision, publishedSource, publishedArtifact uuid.NullUUID
	var publishedKind, publishedSourceKind sql.NullString
	var publishedChecksum []byte
	var scheduled, publishedAt, withdrawnAt sql.NullTime
	var revision entity.ContentRevision
	var ownerType, contentKind, revisionSourceKind string
	var title sql.NullString
	var revisionChecksum []byte
	var html sql.NullString
	err := row.Scan(
		&value.ID, &value.ContestID, &status, &audience,
		&value.CurrentDraft.RevisionID, &draftKind, &value.CurrentDraft.RenderSourceID,
		&draftSourceKind, &draftChecksum, &value.CurrentDraft.RenderArtifactID,
		&publishedRevision, &publishedKind, &publishedSource, &publishedSourceKind,
		&publishedChecksum, &publishedArtifact, &value.Version, &scheduled,
		&publishedAt, &withdrawnAt, &value.CreatedAt, &value.UpdatedAt,
		&revision.ID, &ownerType, &revision.OwnerID, &contentKind,
		&revision.RevisionNumber, &title, &revision.Markdown, &revision.AuthorID,
		&revision.RenderSourceID, &revisionSourceKind, &revisionChecksum,
		&revision.RenderArtifactID, &revision.CreatedAt, &html,
	)
	if err != nil {
		return ports.AnnouncementView{}, err
	}
	value.Status, value.Audience = entity.AnnouncementStatus(status), entity.AnnouncementAudience(audience)
	value.CurrentDraft.ContentKind, value.CurrentDraft.RenderSourceKind = entity.ContentKind(draftKind), draftSourceKind
	if len(draftChecksum) != 32 {
		return ports.AnnouncementView{}, entity.ErrContestContentInvalid
	}
	copy(value.CurrentDraft.SourceChecksum[:], draftChecksum)
	value.ScheduledFor, value.PublishedAt, value.WithdrawnAt = nullableContentTime(scheduled), nullableContentTime(publishedAt), nullableContentTime(withdrawnAt)
	if publishedRevision.Valid || publishedSource.Valid || publishedArtifact.Valid || publishedKind.Valid || publishedSourceKind.Valid || len(publishedChecksum) > 0 {
		if !publishedRevision.Valid || !publishedSource.Valid || !publishedArtifact.Valid || !publishedKind.Valid || !publishedSourceKind.Valid || len(publishedChecksum) != 32 {
			return ports.AnnouncementView{}, entity.ErrContestContentInvalid
		}
		pointer := entity.RenderPointer{RevisionID: publishedRevision.UUID, ContentKind: entity.ContentKind(publishedKind.String), RenderSourceID: publishedSource.UUID, RenderSourceKind: publishedSourceKind.String, RenderArtifactID: publishedArtifact.UUID}
		copy(pointer.SourceChecksum[:], publishedChecksum)
		value.Published = &pointer
	}
	revision.OwnerType, revision.Kind, revision.RenderSourceKind = entity.ContentOwnerType(ownerType), entity.ContentKind(contentKind), revisionSourceKind
	if title.Valid {
		revision.Title = &title.String
	}
	if len(revisionChecksum) != 32 || !html.Valid {
		return ports.AnnouncementView{}, entity.ErrContestContentInvalid
	}
	copy(revision.SourceChecksum[:], revisionChecksum)
	if err := value.Validate(); err != nil {
		return ports.AnnouncementView{}, err
	}
	if err := revision.Validate(); err != nil {
		return ports.AnnouncementView{}, err
	}
	return ports.AnnouncementView{Announcement: value, Revision: revision, SanitizedHTML: html.String}, nil
}

func nullableContentTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time.UTC()
	return &copy
}
