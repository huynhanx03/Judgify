package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	entity "github.com/huynhanx03/judgify/internal/notification/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const campaignColumns = `
	campaign.id,
	CASE
		WHEN control_operation.status = 'cancelled' THEN 'cancelled'
		WHEN control_operation.status = 'failed' THEN 'failed'
		ELSE campaign.status
	END,
	campaign.current_draft_revision_id,
	campaign.scheduled_revision_id,
	campaign.scheduled_render_source_id,
	campaign.scheduled_render_source_kind,
	campaign.scheduled_normalized_source_checksum,
	campaign.scheduled_render_artifact_id,
	campaign.operation_id,
	campaign.version,
	campaign.scheduled_for,
	campaign.last_failure_code,
	campaign.created_by,
	campaign.scheduled_by,
	campaign.cancelled_by,
	campaign.schedule_reason,
	campaign.cancel_reason,
	campaign.created_at,
	campaign.updated_at,
	campaign.scheduled_at,
	campaign.started_at,
	campaign.completed_at,
	campaign.cancelled_at`

const campaignRevisionColumns = `
	revision.id,
	revision.campaign_id,
	revision.revision_number,
	revision.title,
	revision.body_markdown,
	revision.body_render_source_id,
	revision.body_render_source_kind,
	revision.body_render_artifact_id,
	artifact.sanitized_html,
	revision.action_path,
	revision.audience_kind,
	revision.audience_role_id,
	revision.normalized_source_checksum,
	revision.author_id,
	revision.created_at,
	COALESCE((
		SELECT jsonb_agg(explicit_user.user_id ORDER BY explicit_user.user_id)
		FROM notification_campaign_explicit_users explicit_user
		WHERE explicit_user.campaign_revision_id = revision.id
	), '[]'::jsonb)`

const campaignCountsColumns = `
	COALESCE(recipient_counts.total_count, 0),
	COALESCE(recipient_counts.pending_count, 0),
	COALESCE(recipient_counts.created_count, 0),
	COALESCE(recipient_counts.skipped_count, 0),
	COALESCE(recipient_counts.failed_count, 0)`

const campaignViewJoins = `
	LEFT JOIN operations control_operation
		ON control_operation.id = campaign.operation_id
	JOIN notification_campaign_revisions revision
		ON revision.id = CASE
			WHEN campaign.status = 'draft' THEN campaign.current_draft_revision_id
			ELSE campaign.scheduled_revision_id
		END
	JOIN content_render_artifacts artifact
		ON artifact.id = revision.body_render_artifact_id
		AND artifact.render_source_id = revision.body_render_source_id
		AND artifact.render_source_kind = revision.body_render_source_kind
		AND artifact.source_checksum = revision.normalized_source_checksum
		AND artifact.status = 'ready'
	LEFT JOIN LATERAL (
		SELECT
			COUNT(*) AS total_count,
			COUNT(*) FILTER (WHERE status = 'pending') AS pending_count,
			COUNT(*) FILTER (WHERE status = 'created') AS created_count,
			COUNT(*) FILTER (WHERE status = 'skipped') AS skipped_count,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed_count
		FROM notification_campaign_recipients recipient
		WHERE recipient.campaign_id = campaign.id
	) recipient_counts ON TRUE`

type CampaignRepository struct {
	client *internalent.EntClient
	newID  func() (uuid.UUID, error)
}

func NewCampaignRepository(client *internalent.EntClient) (*CampaignRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, ports.ErrInvalidRequest
	}
	return &CampaignRepository{client: client, newID: uuid.NewV7}, nil
}

var _ ports.CampaignRepository = (*CampaignRepository)(nil)

func (repository *CampaignRepository) Create(
	ctx context.Context,
	campaign entity.Campaign,
	revision entity.CampaignRevision,
) (entity.CampaignView, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		campaign.Validate() != nil || revision.Validate() != nil ||
		campaign.Status != contract.CampaignStatusDraft ||
		campaign.CurrentDraftRevisionID != revision.ID ||
		campaign.ID != revision.CampaignID {
		return entity.CampaignView{}, ports.ErrInvalidRequest
	}
	executor := repository.client.DB(ctx).Notification
	if _, err := executor.ExecContext(ctx, `
		INSERT INTO notification_campaigns (
			id, status, current_draft_revision_id, version, created_by,
			created_at, updated_at
		) VALUES ($1,'draft',$2,1,$3,$4,$4)`,
		campaign.ID, revision.ID, campaign.CreatedBy, campaign.CreatedAt.UTC(),
	); err != nil {
		return entity.CampaignView{}, fmt.Errorf("insert notification campaign: %w", err)
	}
	if err := repository.insertRevision(ctx, revision); err != nil {
		return entity.CampaignView{}, err
	}
	return repository.Get(ctx, campaign.ID)
}

func (repository *CampaignRepository) Lock(
	ctx context.Context,
	campaignID uuid.UUID,
) (entity.CampaignView, error) {
	if repository == nil || repository.client == nil || ctx == nil || campaignID == uuid.Nil {
		return entity.CampaignView{}, ports.ErrInvalidRequest
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		SELECT id FROM notification_campaigns WHERE id = $1 FOR UPDATE`,
		campaignID,
	)
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("lock notification campaign: %w", err)
	}
	if !rows.Next() {
		readErr := rows.Err()
		_ = rows.Close()
		if readErr != nil {
			return entity.CampaignView{}, fmt.Errorf("lock notification campaign: %w", readErr)
		}
		return entity.CampaignView{}, ports.ErrCampaignNotFound
	}
	var lockedID uuid.UUID
	if err := rows.Scan(&lockedID); err != nil {
		_ = rows.Close()
		return entity.CampaignView{}, fmt.Errorf("scan locked notification campaign: %w", err)
	}
	if rows.Next() {
		_ = rows.Close()
		return entity.CampaignView{}, fmt.Errorf("lock notification campaign returned multiple rows")
	}
	if err := rows.Close(); err != nil {
		return entity.CampaignView{}, fmt.Errorf("close notification campaign lock: %w", err)
	}
	return repository.Get(ctx, lockedID)
}

func (repository *CampaignRepository) Get(
	ctx context.Context,
	campaignID uuid.UUID,
) (entity.CampaignView, error) {
	if repository == nil || repository.client == nil || ctx == nil || campaignID == uuid.Nil {
		return entity.CampaignView{}, ports.ErrInvalidRequest
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(
		ctx,
		`SELECT `+campaignColumns+`,`+campaignRevisionColumns+`,`+campaignCountsColumns+`
		 FROM notification_campaigns campaign
		 `+campaignViewJoins+`
		 WHERE campaign.id = $1`,
		campaignID,
	)
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("get notification campaign: %w", err)
	}
	return oneCampaignView(rows, "get notification campaign")
}

func (repository *CampaignRepository) List(
	ctx context.Context,
	query ports.CampaignListQuery,
) (ports.CampaignPage, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		query.Limit < 1 || query.Limit > contract.MaximumCampaignPageSize ||
		(query.Status != "" && !contract.ValidCampaignStatus(query.Status)) {
		return ports.CampaignPage{}, ports.ErrInvalidRequest
	}
	statement := `SELECT ` + campaignColumns + `,` + campaignRevisionColumns + `,` + campaignCountsColumns + `
		FROM notification_campaigns campaign
		` + campaignViewJoins + `
		WHERE ($1 = '' OR CASE
			WHEN control_operation.status = 'cancelled' THEN 'cancelled'
			WHEN control_operation.status = 'failed' THEN 'failed'
			ELSE campaign.status
		END = $1)`
	arguments := []any{query.Status}
	if query.Cursor != nil {
		if query.Cursor.CreatedAt.IsZero() || query.Cursor.ID == uuid.Nil {
			return ports.CampaignPage{}, ports.ErrInvalidCursor
		}
		statement += ` AND (
			campaign.created_at < $2 OR
			(campaign.created_at = $2 AND campaign.id < $3)
		)`
		arguments = append(arguments, query.Cursor.CreatedAt.UTC(), query.Cursor.ID)
	}
	statement += fmt.Sprintf(
		` ORDER BY campaign.created_at DESC, campaign.id DESC LIMIT $%d`,
		len(arguments)+1,
	)
	arguments = append(arguments, query.Limit+1)
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return ports.CampaignPage{}, fmt.Errorf("list notification campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]entity.CampaignView, 0, query.Limit+1)
	for rows.Next() {
		value, scanErr := scanCampaignView(rows)
		if scanErr != nil {
			return ports.CampaignPage{}, fmt.Errorf("scan notification campaign: %w", scanErr)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return ports.CampaignPage{}, fmt.Errorf("list notification campaigns: %w", err)
	}
	var next *ports.CampaignCursor
	if len(items) > query.Limit {
		items = items[:query.Limit]
		last := items[len(items)-1].Campaign
		next = &ports.CampaignCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return ports.CampaignPage{Items: items, Next: next}, nil
}

func (repository *CampaignRepository) AddRevision(
	ctx context.Context,
	revision entity.CampaignRevision,
	expectedVersion int64,
	updatedAt time.Time,
) (entity.CampaignView, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		revision.Validate() != nil || expectedVersion < 1 || updatedAt.IsZero() {
		return entity.CampaignView{}, ports.ErrInvalidRequest
	}
	if err := repository.insertRevision(ctx, revision); err != nil {
		return entity.CampaignView{}, err
	}
	result, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		UPDATE notification_campaigns
		SET current_draft_revision_id = $2,
			version = version + 1,
			updated_at = $4
		WHERE id = $1 AND status = 'draft' AND version = $3`,
		revision.CampaignID, revision.ID, expectedVersion, updatedAt.UTC(),
	)
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("advance campaign draft revision: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("count campaign draft revision update: %w", err)
	}
	if changed != 1 {
		return entity.CampaignView{}, ports.ErrCampaignConflict
	}
	return repository.Get(ctx, revision.CampaignID)
}

func (repository *CampaignRepository) StoreConfirmation(
	ctx context.Context,
	confirmation entity.CampaignConfirmation,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		confirmation.Validate() != nil || confirmation.ConsumedAt != nil {
		return ports.ErrInvalidRequest
	}
	_, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		INSERT INTO notification_campaign_confirmations (
			id, nonce_hash, actor_id, session_id, campaign_id,
			campaign_revision_id, render_source_id, render_source_kind,
			normalized_source_checksum, render_artifact_id,
			audience_fingerprint, expected_campaign_version, scheduled_for,
			dry_run_estimated_at, dry_run_recipient_count, issued_at, expires_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)`,
		confirmation.ID, confirmation.NonceHash[:], confirmation.ActorID,
		confirmation.SessionID, confirmation.CampaignID,
		confirmation.CampaignRevisionID, confirmation.RenderSourceID,
		confirmation.RenderSourceKind, confirmation.NormalizedSourceChecksum[:],
		confirmation.RenderArtifactID, confirmation.AudienceFingerprint[:],
		confirmation.ExpectedCampaignVersion, confirmation.ScheduledFor,
		confirmation.DryRunEstimatedAt.UTC(), confirmation.DryRunRecipientCount,
		confirmation.IssuedAt.UTC(), confirmation.ExpiresAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("store campaign confirmation: %w", err)
	}
	return nil
}

func (repository *CampaignRepository) LockConfirmation(
	ctx context.Context,
	nonceHash [sha256.Size]byte,
) (entity.CampaignConfirmation, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		nonceHash == ([sha256.Size]byte{}) {
		return entity.CampaignConfirmation{}, ports.ErrInvalidRequest
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		SELECT
			id, nonce_hash, actor_id, session_id, campaign_id,
			campaign_revision_id, render_source_id, render_source_kind,
			normalized_source_checksum, render_artifact_id,
			audience_fingerprint, expected_campaign_version, scheduled_for,
			dry_run_estimated_at, dry_run_recipient_count, issued_at, expires_at,
			consumed_at, consumed_by_idempotency_key, schedule_request_hash,
			operation_id
		FROM notification_campaign_confirmations
		WHERE nonce_hash = $1
		FOR UPDATE`, nonceHash[:])
	if err != nil {
		return entity.CampaignConfirmation{}, fmt.Errorf("lock campaign confirmation: %w", err)
	}
	return oneCampaignConfirmation(rows)
}

func (repository *CampaignRepository) FreezeSchedule(
	ctx context.Context,
	freeze ports.ScheduleFreeze,
	members []entity.AudienceMember,
) (entity.CampaignView, error) {
	if repository == nil || repository.client == nil || repository.newID == nil ||
		ctx == nil || freeze.CampaignID == uuid.Nil || freeze.OperationID == uuid.Nil ||
		freeze.ActorID == uuid.Nil || freeze.ExpectedVersion < 1 ||
		!contract.ValidCampaignReason(freeze.Reason) || freeze.At.IsZero() ||
		freeze.Revision.Validate() != nil || freeze.Revision.CampaignID != freeze.CampaignID {
		return entity.CampaignView{}, ports.ErrInvalidRequest
	}
	executor := repository.client.DB(ctx).Notification
	result, err := executor.ExecContext(ctx, `
		UPDATE notification_campaigns
		SET status = 'scheduled',
			scheduled_revision_id = $2,
			scheduled_render_source_id = $3,
			scheduled_render_source_kind = $4,
			scheduled_normalized_source_checksum = $5,
			scheduled_render_artifact_id = $6,
			operation_id = $7,
			scheduled_for = $8,
			scheduled_by = $9,
			schedule_reason = $10,
			scheduled_at = $11,
			version = version + 1,
			updated_at = $11
		WHERE id = $1 AND status = 'draft'
			AND current_draft_revision_id = $2
			AND version = $12`,
		freeze.CampaignID, freeze.Revision.ID,
		freeze.Revision.BodyRenderSourceID,
		freeze.Revision.BodyRenderSourceKind,
		freeze.Revision.NormalizedSourceChecksum[:],
		freeze.Revision.BodyRenderArtifactID,
		freeze.OperationID, freeze.ScheduledFor, freeze.ActorID,
		freeze.Reason, freeze.At.UTC(), freeze.ExpectedVersion,
	)
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("freeze notification campaign: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("count frozen notification campaign: %w", err)
	}
	if changed != 1 {
		return entity.CampaignView{}, ports.ErrCampaignConflict
	}
	if len(members) > 0 {
		type frozenRecipient struct {
			ID      uuid.UUID `json:"id"`
			UserID  uuid.UUID `json:"user_id"`
			EventID uuid.UUID `json:"event_id"`
		}
		frozen := make([]frozenRecipient, len(members))
		for index, member := range members {
			if member.Validate() != nil {
				return entity.CampaignView{}, ports.ErrInvalidRequest
			}
			recipientID, idErr := repository.newID()
			if idErr != nil || recipientID == uuid.Nil {
				return entity.CampaignView{}, fmt.Errorf("create campaign recipient identity: %w", idErr)
			}
			sourceEventID, idErr := repository.newID()
			if idErr != nil || sourceEventID == uuid.Nil {
				return entity.CampaignView{}, fmt.Errorf("create campaign delivery identity: %w", idErr)
			}
			frozen[index] = frozenRecipient{
				ID:      recipientID,
				UserID:  member.UserID,
				EventID: sourceEventID,
			}
		}
		encoded, encodeErr := json.Marshal(frozen)
		if encodeErr != nil {
			return entity.CampaignView{}, fmt.Errorf(
				"encode campaign recipient snapshot: %w", encodeErr,
			)
		}
		_, err = executor.ExecContext(ctx, `
			INSERT INTO notification_campaign_recipients (
				id, campaign_id, user_id, campaign_revision_id,
				source_event_id, status, attempt_count, created_at
			)
			SELECT value.id, $1, value.user_id, $2, value.event_id,
				'pending', 0, $3
			FROM jsonb_to_recordset($4::jsonb)
				AS value(id uuid, user_id uuid, event_id uuid)`,
			freeze.CampaignID, freeze.Revision.ID, freeze.At.UTC(),
			encoded,
		)
		if err != nil {
			return entity.CampaignView{}, fmt.Errorf("snapshot campaign recipients: %w", err)
		}
	}
	return repository.Get(ctx, freeze.CampaignID)
}

func (repository *CampaignRepository) ConsumeConfirmation(
	ctx context.Context,
	consumption ports.ConfirmationConsumption,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		consumption.NonceHash == ([sha256.Size]byte{}) ||
		consumption.IdempotencyKey == uuid.Nil ||
		consumption.RequestHash == ([sha256.Size]byte{}) ||
		consumption.OperationID == uuid.Nil || consumption.ConsumedAt.IsZero() {
		return ports.ErrInvalidRequest
	}
	result, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		UPDATE notification_campaign_confirmations
		SET consumed_at = $2,
			consumed_by_idempotency_key = $3,
			schedule_request_hash = $4,
			operation_id = $5
		WHERE nonce_hash = $1 AND consumed_at IS NULL`,
		consumption.NonceHash[:], consumption.ConsumedAt.UTC(),
		consumption.IdempotencyKey, consumption.RequestHash[:],
		consumption.OperationID,
	)
	if err != nil {
		return fmt.Errorf("consume campaign confirmation: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count campaign confirmation consumption: %w", err)
	}
	if changed != 1 {
		return ports.ErrCampaignConfirmationInvalid
	}
	return nil
}

func (repository *CampaignRepository) MarkCancelled(
	ctx context.Context,
	cancellation ports.CampaignCancellation,
) (entity.CampaignView, bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		cancellation.CampaignID == uuid.Nil ||
		cancellation.OperationID == uuid.Nil ||
		cancellation.ExpectedVersion < 1 || cancellation.ActorID == uuid.Nil ||
		!contract.ValidCampaignReason(cancellation.Reason) ||
		cancellation.At.IsZero() {
		return entity.CampaignView{}, false, ports.ErrInvalidRequest
	}
	result, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		UPDATE notification_campaigns
		SET status = 'cancelled',
			cancelled_by = $4,
			cancel_reason = $5,
			cancelled_at = $6,
			version = version + 1,
			updated_at = $6
		WHERE id = $1 AND operation_id = $2 AND version = $3
			AND status IN ('scheduled', 'sending')`,
		cancellation.CampaignID,
		cancellation.OperationID,
		cancellation.ExpectedVersion,
		cancellation.ActorID,
		cancellation.Reason,
		cancellation.At.UTC(),
	)
	if err != nil {
		return entity.CampaignView{}, false, fmt.Errorf(
			"cancel notification campaign: %w",
			err,
		)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return entity.CampaignView{}, false, fmt.Errorf(
			"count notification campaign cancellation: %w",
			err,
		)
	}
	view, err := repository.Get(ctx, cancellation.CampaignID)
	if err != nil {
		return entity.CampaignView{}, false, err
	}
	if changed == 1 {
		return view, true, nil
	}
	if changed != 0 || view.Campaign.Status != contract.CampaignStatusCancelled ||
		view.Campaign.OperationID == nil ||
		*view.Campaign.OperationID != cancellation.OperationID ||
		view.Campaign.Version != cancellation.ExpectedVersion+1 ||
		view.Campaign.CancelledBy == nil ||
		*view.Campaign.CancelledBy != cancellation.ActorID ||
		view.Campaign.CancelReason == nil ||
		*view.Campaign.CancelReason != cancellation.Reason ||
		view.Campaign.CancelledAt == nil {
		return entity.CampaignView{}, false, ports.ErrCampaignConflict
	}
	return view, false, nil
}

func (repository *CampaignRepository) MarkSending(
	ctx context.Context,
	campaignID uuid.UUID,
	operationID uuid.UUID,
	at time.Time,
) (entity.CampaignView, bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		campaignID == uuid.Nil || operationID == uuid.Nil || at.IsZero() {
		return entity.CampaignView{}, false, ports.ErrInvalidRequest
	}
	result, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		UPDATE notification_campaigns
		SET status = 'sending',
			started_at = COALESCE(started_at, $3),
			version = version + 1,
			updated_at = $3
		WHERE id = $1 AND operation_id = $2 AND status = 'scheduled'`,
		campaignID, operationID, at.UTC(),
	)
	if err != nil {
		return entity.CampaignView{}, false, fmt.Errorf("mark campaign sending: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return entity.CampaignView{}, false, fmt.Errorf("count campaign sending transition: %w", err)
	}
	view, err := repository.Get(ctx, campaignID)
	if err != nil {
		return entity.CampaignView{}, false, err
	}
	if view.Campaign.OperationID == nil || *view.Campaign.OperationID != operationID {
		return entity.CampaignView{}, false, ports.ErrCampaignConflict
	}
	return view, changed == 1, nil
}

func (repository *CampaignRepository) LockDeliveryBatch(
	ctx context.Context,
	campaignID uuid.UUID,
	afterUserID uuid.UUID,
	limit int,
	now time.Time,
) (entity.CampaignView, []entity.CampaignRecipient, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		campaignID == uuid.Nil || limit < 1 || limit > 1000 || now.IsZero() {
		return entity.CampaignView{}, nil, ports.ErrInvalidRequest
	}
	view, err := repository.Lock(ctx, campaignID)
	if err != nil {
		return entity.CampaignView{}, nil, err
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		SELECT
			id, campaign_id, user_id, campaign_revision_id, source_event_id,
			status, notification_id, skip_code, failure_code, attempt_count,
			next_attempt_at, created_at, last_attempt_at, processed_at
		FROM notification_campaign_recipients
		WHERE campaign_id = $1
			AND status = 'pending'
			AND user_id > $2
			AND (next_attempt_at IS NULL OR next_attempt_at <= $3)
		ORDER BY user_id
		LIMIT $4
		FOR UPDATE`, campaignID, afterUserID, now.UTC(), limit)
	if err != nil {
		return entity.CampaignView{}, nil, fmt.Errorf("lock campaign delivery batch: %w", err)
	}
	defer func() { _ = rows.Close() }()
	recipients := make([]entity.CampaignRecipient, 0, limit)
	for rows.Next() {
		recipient, scanErr := scanCampaignRecipient(rows)
		if scanErr != nil {
			return entity.CampaignView{}, nil, fmt.Errorf("scan campaign recipient: %w", scanErr)
		}
		recipients = append(recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		return entity.CampaignView{}, nil, fmt.Errorf("read campaign delivery batch: %w", err)
	}
	return view, recipients, nil
}

func (repository *CampaignRepository) ApplyRecipientResult(
	ctx context.Context,
	campaignID uuid.UUID,
	result ports.RecipientResult,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		campaignID == uuid.Nil || result.UserID == uuid.Nil ||
		!contract.ValidRecipientStatus(result.Status) ||
		result.Status == contract.RecipientStatusPending || result.AttemptedAt.IsZero() {
		return ports.ErrInvalidRequest
	}
	var notificationID any
	var skipCode any
	var failureCode any
	switch result.Status {
	case contract.RecipientStatusCreated:
		if result.NotificationID == nil || *result.NotificationID == uuid.Nil {
			return ports.ErrInvalidRequest
		}
		notificationID = *result.NotificationID
	case contract.RecipientStatusSkipped:
		if !contract.ValidCampaignCode(result.SkipCode) {
			return ports.ErrInvalidRequest
		}
		skipCode = result.SkipCode
	case contract.RecipientStatusFailed:
		if !contract.ValidCampaignCode(result.FailureCode) {
			return ports.ErrInvalidRequest
		}
		failureCode = result.FailureCode
	default:
		return ports.ErrInvalidRequest
	}
	update, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		UPDATE notification_campaign_recipients
		SET status = $3,
			notification_id = $4,
			skip_code = $5,
			failure_code = $6,
			attempt_count = attempt_count + 1,
			next_attempt_at = NULL,
			last_attempt_at = $7,
			processed_at = $7
		WHERE campaign_id = $1 AND user_id = $2 AND status = 'pending'`,
		campaignID, result.UserID, result.Status, notificationID, skipCode,
		failureCode, result.AttemptedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("apply campaign recipient result: %w", err)
	}
	changed, err := update.RowsAffected()
	if err != nil {
		return fmt.Errorf("count campaign recipient result: %w", err)
	}
	if changed != 1 {
		return ports.ErrCampaignConflict
	}
	return nil
}

func (repository *CampaignRepository) MarkTerminal(
	ctx context.Context,
	campaignID uuid.UUID,
	operationID uuid.UUID,
	status string,
	failureCode string,
	at time.Time,
) (entity.CampaignView, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		campaignID == uuid.Nil || operationID == uuid.Nil || at.IsZero() ||
		(status != contract.CampaignStatusCompleted &&
			status != contract.CampaignStatusFailed) ||
		(status == contract.CampaignStatusFailed &&
			!contract.ValidCampaignCode(failureCode)) {
		return entity.CampaignView{}, ports.ErrInvalidRequest
	}
	var code any
	if status == contract.CampaignStatusFailed {
		code = failureCode
	}
	result, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		UPDATE notification_campaigns
		SET status = $3,
			last_failure_code = $4,
			completed_at = $5,
			version = version + 1,
			updated_at = $5
		WHERE id = $1 AND operation_id = $2
			AND status IN ('scheduled', 'sending')`,
		campaignID, operationID, status, code, at.UTC(),
	)
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("mark campaign terminal: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("count campaign terminal transition: %w", err)
	}
	if changed != 1 {
		return entity.CampaignView{}, ports.ErrCampaignConflict
	}
	return repository.Get(ctx, campaignID)
}

func (repository *CampaignRepository) ListRecipients(
	ctx context.Context,
	query ports.RecipientListQuery,
) (ports.RecipientPage, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		query.CampaignID == uuid.Nil || query.Limit < 1 ||
		query.Limit > contract.MaximumCampaignPageSize ||
		(query.Status != "" && !contract.ValidRecipientStatus(query.Status)) {
		return ports.RecipientPage{}, ports.ErrInvalidRequest
	}
	after := uuid.Nil
	if query.Cursor != nil {
		if query.Cursor.UserID == uuid.Nil {
			return ports.RecipientPage{}, ports.ErrInvalidCursor
		}
		after = query.Cursor.UserID
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		SELECT
			id, campaign_id, user_id, campaign_revision_id, source_event_id,
			status, notification_id, skip_code, failure_code, attempt_count,
			next_attempt_at, created_at, last_attempt_at, processed_at
		FROM notification_campaign_recipients
		WHERE campaign_id = $1
			AND ($2 = '' OR status = $2)
			AND user_id > $3
		ORDER BY user_id
		LIMIT $4`,
		query.CampaignID, query.Status, after, query.Limit+1,
	)
	if err != nil {
		return ports.RecipientPage{}, fmt.Errorf("list campaign recipients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]entity.CampaignRecipient, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanCampaignRecipient(rows)
		if scanErr != nil {
			return ports.RecipientPage{}, fmt.Errorf("scan campaign recipient: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ports.RecipientPage{}, fmt.Errorf("read campaign recipients: %w", err)
	}
	var next *ports.RecipientCursor
	if len(items) > query.Limit {
		items = items[:query.Limit]
		next = &ports.RecipientCursor{UserID: items[len(items)-1].UserID}
	}
	return ports.RecipientPage{Items: items, Next: next}, nil
}

func (repository *CampaignRepository) insertRevision(
	ctx context.Context,
	revision entity.CampaignRevision,
) error {
	executor := repository.client.DB(ctx).Notification
	_, err := executor.ExecContext(ctx, `
		INSERT INTO notification_campaign_revisions (
			id, campaign_id, revision_number, title, body_markdown,
			body_render_source_id, body_render_source_kind,
			body_render_artifact_id, action_path, audience_kind,
			audience_role_id, normalized_source_checksum, author_id, created_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
		)`,
		revision.ID, revision.CampaignID, revision.RevisionNumber,
		revision.Title, revision.BodyMarkdown, revision.BodyRenderSourceID,
		revision.BodyRenderSourceKind, revision.BodyRenderArtifactID,
		revision.ActionPath, revision.Audience.Kind, revision.Audience.RoleID,
		revision.NormalizedSourceChecksum[:], revision.AuthorID,
		revision.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert campaign revision: %w", err)
	}
	if revision.Audience.Kind != contract.AudienceExplicitUsers {
		return nil
	}
	type explicitRecipient struct {
		ID     uuid.UUID `json:"id"`
		UserID uuid.UUID `json:"user_id"`
	}
	explicit := make([]explicitRecipient, len(revision.Audience.UserIDs))
	for index, userID := range revision.Audience.UserIDs {
		id, idErr := repository.newID()
		if idErr != nil || id == uuid.Nil {
			return fmt.Errorf("create explicit audience identity: %w", idErr)
		}
		explicit[index] = explicitRecipient{ID: id, UserID: userID}
	}
	encoded, encodeErr := json.Marshal(explicit)
	if encodeErr != nil {
		return fmt.Errorf("encode explicit campaign audience: %w", encodeErr)
	}
	_, err = executor.ExecContext(ctx, `
		INSERT INTO notification_campaign_explicit_users (
			id, campaign_revision_id, user_id, created_at
		)
		SELECT value.id, $1, value.user_id, $2
		FROM jsonb_to_recordset($3::jsonb)
			AS value(id uuid, user_id uuid)`,
		revision.ID, revision.CreatedAt.UTC(), encoded,
	)
	if err != nil {
		return fmt.Errorf("insert explicit campaign audience: %w", err)
	}
	return nil
}

func scanCampaignView(scanner interface{ Scan(...any) error }) (entity.CampaignView, error) {
	var (
		view                 entity.CampaignView
		scheduledRevisionID  uuid.NullUUID
		scheduledSourceID    uuid.NullUUID
		scheduledSourceKind  sql.NullString
		scheduledChecksum    []byte
		scheduledArtifactID  uuid.NullUUID
		operationID          uuid.NullUUID
		scheduledFor         sql.NullTime
		lastFailureCode      sql.NullString
		scheduledBy          uuid.NullUUID
		cancelledBy          uuid.NullUUID
		scheduleReason       sql.NullString
		cancelReason         sql.NullString
		scheduledAt          sql.NullTime
		startedAt            sql.NullTime
		completedAt          sql.NullTime
		cancelledAt          sql.NullTime
		revisionArtifactHTML sql.NullString
		actionPath           sql.NullString
		roleID               uuid.NullUUID
		revisionChecksum     []byte
		explicitUserIDsJSON  []byte
	)
	err := scanner.Scan(
		&view.Campaign.ID,
		&view.Campaign.Status,
		&view.Campaign.CurrentDraftRevisionID,
		&scheduledRevisionID,
		&scheduledSourceID,
		&scheduledSourceKind,
		&scheduledChecksum,
		&scheduledArtifactID,
		&operationID,
		&view.Campaign.Version,
		&scheduledFor,
		&lastFailureCode,
		&view.Campaign.CreatedBy,
		&scheduledBy,
		&cancelledBy,
		&scheduleReason,
		&cancelReason,
		&view.Campaign.CreatedAt,
		&view.Campaign.UpdatedAt,
		&scheduledAt,
		&startedAt,
		&completedAt,
		&cancelledAt,
		&view.Revision.ID,
		&view.Revision.CampaignID,
		&view.Revision.RevisionNumber,
		&view.Revision.Title,
		&view.Revision.BodyMarkdown,
		&view.Revision.BodyRenderSourceID,
		&view.Revision.BodyRenderSourceKind,
		&view.Revision.BodyRenderArtifactID,
		&revisionArtifactHTML,
		&actionPath,
		&view.Revision.Audience.Kind,
		&roleID,
		&revisionChecksum,
		&view.Revision.AuthorID,
		&view.Revision.CreatedAt,
		&explicitUserIDsJSON,
		&view.Counts.Total,
		&view.Counts.Pending,
		&view.Counts.Created,
		&view.Counts.Skipped,
		&view.Counts.Failed,
	)
	if err != nil {
		return entity.CampaignView{}, err
	}
	view.Campaign.ScheduledRevisionID = nullableCampaignUUID(scheduledRevisionID)
	view.Campaign.ScheduledRenderSourceID = nullableCampaignUUID(scheduledSourceID)
	view.Campaign.ScheduledRenderSourceKind = nullableCampaignString(scheduledSourceKind)
	view.Campaign.ScheduledNormalizedSourceChecksum = append([]byte(nil), scheduledChecksum...)
	view.Campaign.ScheduledRenderArtifactID = nullableCampaignUUID(scheduledArtifactID)
	view.Campaign.OperationID = nullableCampaignUUID(operationID)
	view.Campaign.ScheduledFor = nullableCampaignTime(scheduledFor)
	view.Campaign.LastFailureCode = nullableCampaignString(lastFailureCode)
	view.Campaign.ScheduledBy = nullableCampaignUUID(scheduledBy)
	view.Campaign.CancelledBy = nullableCampaignUUID(cancelledBy)
	view.Campaign.ScheduleReason = nullableCampaignString(scheduleReason)
	view.Campaign.CancelReason = nullableCampaignString(cancelReason)
	view.Campaign.ScheduledAt = nullableCampaignTime(scheduledAt)
	view.Campaign.StartedAt = nullableCampaignTime(startedAt)
	view.Campaign.CompletedAt = nullableCampaignTime(completedAt)
	view.Campaign.CancelledAt = nullableCampaignTime(cancelledAt)
	view.Revision.SanitizedHTML = revisionArtifactHTML.String
	view.Revision.ActionPath = nullableCampaignString(actionPath)
	view.Revision.Audience.RoleID = nullableCampaignUUID(roleID)
	if len(revisionChecksum) != sha256.Size {
		return entity.CampaignView{}, contract.ErrInvalidCampaign
	}
	copy(view.Revision.NormalizedSourceChecksum[:], revisionChecksum)
	if view.Revision.Audience.Kind == contract.AudienceExplicitUsers {
		if err := json.Unmarshal(explicitUserIDsJSON, &view.Revision.Audience.UserIDs); err != nil {
			return entity.CampaignView{}, contract.ErrInvalidCampaign
		}
	}
	if view.Campaign.Validate() != nil || view.Revision.Validate() != nil ||
		view.Counts.Validate() != nil {
		return entity.CampaignView{}, contract.ErrInvalidCampaign
	}
	return view, nil
}

func oneCampaignView(rows *sql.Rows, operation string) (entity.CampaignView, error) {
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.CampaignView{}, fmt.Errorf("%s: %w", operation, err)
		}
		return entity.CampaignView{}, ports.ErrCampaignNotFound
	}
	value, err := scanCampaignView(rows)
	if err != nil {
		return entity.CampaignView{}, fmt.Errorf("%s: %w", operation, err)
	}
	if rows.Next() {
		return entity.CampaignView{}, fmt.Errorf("%s returned multiple rows", operation)
	}
	if err := rows.Err(); err != nil {
		return entity.CampaignView{}, fmt.Errorf("%s: %w", operation, err)
	}
	return value, nil
}

func oneCampaignConfirmation(rows *sql.Rows) (entity.CampaignConfirmation, error) {
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.CampaignConfirmation{}, fmt.Errorf("lock campaign confirmation: %w", err)
		}
		return entity.CampaignConfirmation{}, ports.ErrCampaignConfirmationInvalid
	}
	var (
		value          entity.CampaignConfirmation
		nonceHash      []byte
		sourceChecksum []byte
		fingerprint    []byte
		scheduledFor   sql.NullTime
		consumedAt     sql.NullTime
		idempotencyKey uuid.NullUUID
		requestHash    []byte
		operationID    uuid.NullUUID
	)
	if err := rows.Scan(
		&value.ID, &nonceHash, &value.ActorID, &value.SessionID,
		&value.CampaignID, &value.CampaignRevisionID, &value.RenderSourceID,
		&value.RenderSourceKind, &sourceChecksum, &value.RenderArtifactID,
		&fingerprint, &value.ExpectedCampaignVersion, &scheduledFor,
		&value.DryRunEstimatedAt, &value.DryRunRecipientCount,
		&value.IssuedAt, &value.ExpiresAt, &consumedAt, &idempotencyKey,
		&requestHash, &operationID,
	); err != nil {
		return entity.CampaignConfirmation{}, fmt.Errorf("scan campaign confirmation: %w", err)
	}
	if rows.Next() {
		return entity.CampaignConfirmation{}, fmt.Errorf("campaign confirmation query returned multiple rows")
	}
	if len(nonceHash) != sha256.Size || len(sourceChecksum) != sha256.Size ||
		len(fingerprint) != sha256.Size {
		return entity.CampaignConfirmation{}, contract.ErrInvalidCampaign
	}
	copy(value.NonceHash[:], nonceHash)
	copy(value.NormalizedSourceChecksum[:], sourceChecksum)
	copy(value.AudienceFingerprint[:], fingerprint)
	value.ScheduledFor = nullableCampaignTime(scheduledFor)
	value.ConsumedAt = nullableCampaignTime(consumedAt)
	value.ConsumedByIdempotencyKey = nullableCampaignUUID(idempotencyKey)
	value.ScheduleRequestHash = append([]byte(nil), requestHash...)
	value.OperationID = nullableCampaignUUID(operationID)
	if value.Validate() != nil {
		return entity.CampaignConfirmation{}, contract.ErrInvalidCampaign
	}
	return value, nil
}

func scanCampaignRecipient(scanner interface{ Scan(...any) error }) (entity.CampaignRecipient, error) {
	var (
		value          entity.CampaignRecipient
		notificationID uuid.NullUUID
		skipCode       sql.NullString
		failureCode    sql.NullString
		nextAttemptAt  sql.NullTime
		lastAttemptAt  sql.NullTime
		processedAt    sql.NullTime
	)
	if err := scanner.Scan(
		&value.ID, &value.CampaignID, &value.UserID,
		&value.CampaignRevisionID, &value.SourceEventID, &value.Status,
		&notificationID, &skipCode, &failureCode, &value.AttemptCount,
		&nextAttemptAt, &value.CreatedAt, &lastAttemptAt, &processedAt,
	); err != nil {
		return entity.CampaignRecipient{}, err
	}
	value.NotificationID = nullableCampaignUUID(notificationID)
	value.SkipCode = nullableCampaignString(skipCode)
	value.FailureCode = nullableCampaignString(failureCode)
	value.NextAttemptAt = nullableCampaignTime(nextAttemptAt)
	value.LastAttemptAt = nullableCampaignTime(lastAttemptAt)
	value.ProcessedAt = nullableCampaignTime(processedAt)
	if value.Validate() != nil {
		return entity.CampaignRecipient{}, contract.ErrInvalidCampaign
	}
	return value, nil
}

func nullableCampaignUUID(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid || value.UUID == uuid.Nil {
		return nil
	}
	result := value.UUID
	return &result
}

func nullableCampaignString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := strings.Clone(value.String)
	return &result
}

func nullableCampaignTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
