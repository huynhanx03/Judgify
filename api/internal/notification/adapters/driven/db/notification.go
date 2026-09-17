package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	entity "github.com/huynhanx03/judgify/internal/notification/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const notificationColumns = `
	id, user_id, type, category, content_kind, title_key, body_key, params,
	campaign_revision_id, action_path, source_event_id, version, created_at,
	read_at, archived_at, expires_at`

type Repository struct {
	client *internalent.EntClient
	newID  func() (uuid.UUID, error)
}

func NewRepository(client *internalent.EntClient) (*Repository, error) {
	if client == nil || client.Client() == nil {
		return nil, ports.ErrInvalidRequest
	}
	return &Repository{client: client, newID: uuid.NewV7}, nil
}

var _ ports.Repository = (*Repository)(nil)

func (repository *Repository) Create(
	ctx context.Context,
	command ports.CreateCommand,
) (*entity.Notification, entity.InboxState, bool, error) {
	if repository == nil || repository.client == nil || ctx == nil {
		return nil, entity.InboxState{}, false, ports.ErrInvalidRequest
	}
	existing, found, err := repository.findBySource(ctx, command.UserID, command.SourceEventID, command.Type)
	if err != nil {
		return nil, entity.InboxState{}, false, err
	}
	if found {
		inbox, unreadErr := repository.Unread(ctx, command.UserID, command.OccurredAt)
		return &existing, inbox, false, unreadErr
	}

	deliver, err := repository.inAppEnabled(ctx, command.UserID, command.Category)
	if err != nil {
		return nil, entity.InboxState{}, false, err
	}
	if !deliver {
		inbox, unreadErr := repository.Unread(ctx, command.UserID, command.OccurredAt)
		return nil, inbox, false, unreadErr
	}

	id, err := repository.newID()
	if err != nil || id == uuid.Nil {
		return nil, entity.InboxState{}, false, fmt.Errorf("create notification identity: %w", err)
	}
	contentKind := contract.ContentKindCatalog
	if command.CampaignRevisionID != nil {
		contentKind = contract.ContentKindCampaign
	}
	value := entity.Notification{
		ID: id, UserID: command.UserID, Type: command.Type,
		Category: command.Category, ContentKind: contentKind,
		TitleKey: command.TitleKey, BodyKey: command.BodyKey,
		Params:             append([]byte(nil), command.Params...),
		CampaignRevisionID: cloneUUID(command.CampaignRevisionID),
		ActionPath:         cloneString(command.ActionPath),
		SourceEventID:      command.SourceEventID, Version: 1,
		CreatedAt: command.OccurredAt.UTC(), ExpiresAt: cloneTime(command.ExpiresAt),
	}
	if err := value.Validate(); err != nil {
		return nil, entity.InboxState{}, false, ports.ErrInvalidRequest
	}

	client := repository.client.DB(ctx)
	rows, err := client.Notification.QueryContext(ctx, `
		INSERT INTO notifications (
			id, user_id, type, category, content_kind, title_key, body_key, params,
			campaign_revision_id, action_path, source_event_id, version, created_at,
			read_at, archived_at, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1,$12,NULL,NULL,$13)
		ON CONFLICT (user_id, source_event_id, type) DO NOTHING
		RETURNING `+notificationColumns,
		value.ID, value.UserID, value.Type, value.Category, value.ContentKind,
		value.TitleKey, value.BodyKey, nullableJSON(value.Params),
		value.CampaignRevisionID, value.ActionPath, value.SourceEventID,
		value.CreatedAt, value.ExpiresAt,
	)
	if err != nil {
		return nil, entity.InboxState{}, false, fmt.Errorf("insert notification: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		created, scanErr := scanNotification(rows)
		if scanErr != nil {
			return nil, entity.InboxState{}, false, fmt.Errorf("scan inserted notification: %w", scanErr)
		}
		if rows.Next() {
			return nil, entity.InboxState{}, false, fmt.Errorf("insert notification returned multiple rows")
		}
		if err := rows.Err(); err != nil {
			return nil, entity.InboxState{}, false, fmt.Errorf("read inserted notification: %w", err)
		}
		if err := rows.Close(); err != nil {
			return nil, entity.InboxState{}, false, fmt.Errorf("close inserted notification result: %w", err)
		}
		inbox, err := repository.lockInbox(ctx, command.UserID, command.OccurredAt)
		if err != nil {
			return nil, entity.InboxState{}, false, err
		}
		inbox.Version++
		inbox.UnreadCount++
		inbox.UpdatedAt = command.OccurredAt.UTC()
		if err := repository.saveInbox(ctx, inbox); err != nil {
			return nil, entity.InboxState{}, false, err
		}
		return &created, inbox, true, nil
	}
	if err := rows.Err(); err != nil {
		return nil, entity.InboxState{}, false, fmt.Errorf("insert notification: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, entity.InboxState{}, false, fmt.Errorf("close notification idempotency result: %w", err)
	}
	duplicate, found, err := repository.findBySource(ctx, command.UserID, command.SourceEventID, command.Type)
	if err != nil {
		return nil, entity.InboxState{}, false, err
	}
	if !found {
		return nil, entity.InboxState{}, false, fmt.Errorf("notification idempotency conflict without canonical row")
	}
	inbox, err := repository.Unread(ctx, command.UserID, command.OccurredAt)
	return &duplicate, inbox, false, err
}

func (repository *Repository) List(
	ctx context.Context,
	query ports.ListQuery,
) ([]entity.Notification, *string, entity.InboxState, error) {
	if repository == nil || repository.client == nil || ctx == nil || query.UserID == uuid.Nil ||
		query.Limit < 1 || query.Limit > contract.MaximumPageSize || query.Now.IsZero() {
		return nil, nil, entity.InboxState{}, ports.ErrInvalidRequest
	}
	var cursor *notificationCursor
	if query.Cursor != "" {
		decoded, err := decodeNotificationCursor(query.Cursor)
		if err != nil {
			return nil, nil, entity.InboxState{}, err
		}
		cursor = &decoded
	}

	statement := `SELECT ` + notificationColumns + `, (
			SELECT revision.title
			FROM notification_campaign_revisions revision
			WHERE revision.id = notifications.campaign_revision_id
		) AS campaign_title FROM notifications
		WHERE user_id = $1 AND (expires_at IS NULL OR expires_at > $2)`
	arguments := []any{query.UserID, query.Now.UTC()}
	if !query.IncludeArchived {
		statement += ` AND archived_at IS NULL`
	}
	if cursor != nil {
		statement += fmt.Sprintf(
			` AND (created_at < $%d OR (created_at = $%d AND id < $%d))`,
			len(arguments)+1,
			len(arguments)+1,
			len(arguments)+2,
		)
		arguments = append(arguments, cursor.createdAt, cursor.id)
	}
	statement += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, len(arguments)+1)
	arguments = append(arguments, query.Limit+1)

	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, nil, entity.InboxState{}, fmt.Errorf("list notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]entity.Notification, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanNotificationWithCampaignTitle(rows)
		if scanErr != nil {
			return nil, nil, entity.InboxState{}, fmt.Errorf("scan notification: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, entity.InboxState{}, fmt.Errorf("list notifications: %w", err)
	}
	var next *string
	if len(items) > query.Limit {
		items = items[:query.Limit]
		last := items[len(items)-1]
		encoded, err := encodeNotificationCursor(last.CreatedAt, last.ID)
		if err != nil {
			return nil, nil, entity.InboxState{}, err
		}
		next = &encoded
	}
	inbox, err := repository.Unread(ctx, query.UserID, query.Now)
	if err != nil {
		return nil, nil, entity.InboxState{}, err
	}
	return items, next, inbox, nil
}

func (repository *Repository) CampaignContent(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
	now time.Time,
) (entity.CampaignContent, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		userID == uuid.Nil || notificationID == uuid.Nil || now.IsZero() {
		return entity.CampaignContent{}, ports.ErrInvalidRequest
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		SELECT
			notification.id,
			revision.id,
			revision.title,
			artifact.sanitized_html,
			notification.action_path
		FROM notifications notification
		JOIN notification_campaign_revisions revision
			ON revision.id = notification.campaign_revision_id
		JOIN content_render_artifacts artifact
			ON artifact.id = revision.body_render_artifact_id
			AND artifact.render_source_id = revision.body_render_source_id
			AND artifact.render_source_kind = revision.body_render_source_kind
			AND artifact.source_checksum = revision.normalized_source_checksum
			AND artifact.status = 'ready'
		WHERE notification.id = $1
			AND notification.user_id = $2
			AND notification.content_kind = 'campaign'
			AND notification.archived_at IS NULL
			AND (notification.expires_at IS NULL OR notification.expires_at > $3)`,
		notificationID,
		userID,
		now.UTC(),
	)
	if err != nil {
		return entity.CampaignContent{}, fmt.Errorf(
			"query notification campaign content: %w",
			err,
		)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.CampaignContent{}, fmt.Errorf(
				"query notification campaign content: %w",
				err,
			)
		}
		return entity.CampaignContent{}, ports.ErrNotFound
	}
	var content entity.CampaignContent
	var actionPath sql.NullString
	if err := rows.Scan(
		&content.NotificationID,
		&content.RevisionID,
		&content.Title,
		&content.SanitizedHTML,
		&actionPath,
	); err != nil {
		return entity.CampaignContent{}, fmt.Errorf(
			"scan notification campaign content: %w",
			err,
		)
	}
	if rows.Next() {
		return entity.CampaignContent{}, fmt.Errorf(
			"notification campaign content returned multiple rows",
		)
	}
	if err := rows.Err(); err != nil {
		return entity.CampaignContent{}, fmt.Errorf(
			"read notification campaign content: %w",
			err,
		)
	}
	content.ActionPath = nullableString(actionPath)
	if err := content.Validate(); err != nil {
		return entity.CampaignContent{}, fmt.Errorf(
			"validate notification campaign content: %w",
			err,
		)
	}
	return content, nil
}

func (repository *Repository) Unread(
	ctx context.Context,
	userID uuid.UUID,
	now time.Time,
) (entity.InboxState, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil || now.IsZero() {
		return entity.InboxState{}, ports.ErrInvalidRequest
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		SELECT
			COALESCE(state.version, 0),
			COUNT(notification.id),
			COALESCE(state.updated_at, $2)
		FROM (SELECT $1::uuid AS user_id) owner
		LEFT JOIN notification_inbox_states state ON state.user_id = owner.user_id
		LEFT JOIN notifications notification
			ON notification.user_id = owner.user_id
			AND notification.read_at IS NULL
			AND notification.archived_at IS NULL
			AND (notification.expires_at IS NULL OR notification.expires_at > $2)
		GROUP BY state.version, state.updated_at`, userID, now.UTC())
	if err != nil {
		return entity.InboxState{}, fmt.Errorf("query notification unread count: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.InboxState{}, fmt.Errorf("query notification unread count: %w", sql.ErrNoRows)
	}
	state := entity.InboxState{UserID: userID}
	if err := rows.Scan(&state.Version, &state.UnreadCount, &state.UpdatedAt); err != nil {
		return entity.InboxState{}, fmt.Errorf("scan notification unread count: %w", err)
	}
	if rows.Next() {
		return entity.InboxState{}, fmt.Errorf("query notification unread count returned multiple rows")
	}
	if err := rows.Err(); err != nil {
		return entity.InboxState{}, fmt.Errorf("query notification unread count: %w", err)
	}
	return state, nil
}

func (repository *Repository) MarkRead(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
	now time.Time,
) (ports.MutationResult, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil ||
		notificationID == uuid.Nil || now.IsZero() {
		return ports.MutationResult{}, ports.ErrInvalidRequest
	}
	inbox, err := repository.lockInbox(ctx, userID, now)
	if err != nil {
		return ports.MutationResult{}, err
	}
	value, err := repository.lockOwned(ctx, userID, notificationID, now)
	if err != nil {
		return ports.MutationResult{}, err
	}
	if value.ReadAt != nil {
		inbox.UnreadCount, err = repository.canonicalUnread(ctx, userID, now)
		return ports.MutationResult{Notification: &value, Inbox: inbox}, err
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		UPDATE notifications
		SET read_at = $3, version = version + 1
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL
		RETURNING `+notificationColumns, notificationID, userID, now.UTC())
	if err != nil {
		return ports.MutationResult{}, fmt.Errorf("mark notification read: %w", err)
	}
	updated, err := oneNotification(rows, "mark notification read")
	if err != nil {
		return ports.MutationResult{}, err
	}
	inbox.Version++
	inbox.UnreadCount, err = repository.canonicalUnread(ctx, userID, now)
	if err != nil {
		return ports.MutationResult{}, err
	}
	inbox.UpdatedAt = now.UTC()
	if err := repository.saveInbox(ctx, inbox); err != nil {
		return ports.MutationResult{}, err
	}
	return ports.MutationResult{Notification: &updated, Inbox: inbox, Changed: true}, nil
}

func (repository *Repository) MarkAllRead(
	ctx context.Context,
	userID uuid.UUID,
	now time.Time,
) (ports.MutationResult, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil || now.IsZero() {
		return ports.MutationResult{}, ports.ErrInvalidRequest
	}
	inbox, err := repository.lockInbox(ctx, userID, now)
	if err != nil {
		return ports.MutationResult{}, err
	}
	result, err := repository.client.DB(ctx).Notification.ExecContext(ctx, `
		UPDATE notifications
		SET read_at = $2, version = version + 1
		WHERE user_id = $1 AND read_at IS NULL AND archived_at IS NULL
			AND (expires_at IS NULL OR expires_at > $2)`, userID, now.UTC())
	if err != nil {
		return ports.MutationResult{}, fmt.Errorf("mark all notifications read: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ports.MutationResult{}, fmt.Errorf("count marked notifications: %w", err)
	}
	if changed == 0 {
		inbox.UnreadCount, err = repository.canonicalUnread(ctx, userID, now)
		return ports.MutationResult{Inbox: inbox}, err
	}
	inbox.Version++
	inbox.UnreadCount = 0
	inbox.UpdatedAt = now.UTC()
	if err := repository.saveInbox(ctx, inbox); err != nil {
		return ports.MutationResult{}, err
	}
	return ports.MutationResult{Inbox: inbox, Changed: true}, nil
}

func (repository *Repository) Archive(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
	now time.Time,
) (ports.MutationResult, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil ||
		notificationID == uuid.Nil || now.IsZero() {
		return ports.MutationResult{}, ports.ErrInvalidRequest
	}
	inbox, err := repository.lockInbox(ctx, userID, now)
	if err != nil {
		return ports.MutationResult{}, err
	}
	value, err := repository.lockOwned(ctx, userID, notificationID, now)
	if err != nil {
		return ports.MutationResult{}, err
	}
	if value.ArchivedAt != nil {
		inbox.UnreadCount, err = repository.canonicalUnread(ctx, userID, now)
		return ports.MutationResult{Notification: &value, Inbox: inbox}, err
	}
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		UPDATE notifications
		SET read_at = COALESCE(read_at, $3), archived_at = $3, version = version + 1
		WHERE id = $1 AND user_id = $2 AND archived_at IS NULL
		RETURNING `+notificationColumns, notificationID, userID, now.UTC())
	if err != nil {
		return ports.MutationResult{}, fmt.Errorf("archive notification: %w", err)
	}
	updated, err := oneNotification(rows, "archive notification")
	if err != nil {
		return ports.MutationResult{}, err
	}
	inbox.Version++
	inbox.UnreadCount, err = repository.canonicalUnread(ctx, userID, now)
	if err != nil {
		return ports.MutationResult{}, err
	}
	inbox.UpdatedAt = now.UTC()
	if err := repository.saveInbox(ctx, inbox); err != nil {
		return ports.MutationResult{}, err
	}
	return ports.MutationResult{Notification: &updated, Inbox: inbox, Changed: true}, nil
}

func (repository *Repository) Preferences(ctx context.Context, userID uuid.UUID) ([]entity.Preference, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil {
		return nil, ports.ErrInvalidRequest
	}
	rows, err := repository.client.DB(ctx).NotificationPreference.QueryContext(ctx, `
		SELECT category, in_app, version, updated_at
		FROM notification_preferences
		WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification preferences: %w", err)
	}
	defer func() { _ = rows.Close() }()
	stored := make(map[string]entity.Preference, len(contract.Categories()))
	for rows.Next() {
		value := entity.Preference{UserID: userID}
		if err := rows.Scan(&value.Category, &value.InApp, &value.Version, &value.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan notification preference: %w", err)
		}
		stored[value.Category] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list notification preferences: %w", err)
	}
	result := make([]entity.Preference, 0, len(contract.Categories()))
	for _, category := range contract.Categories() {
		value, exists := stored[category]
		if !exists {
			value = entity.Preference{
				UserID: userID, Category: category, InApp: true,
				Version: 0, UpdatedAt: time.Unix(0, 0).UTC(),
			}
		}
		result = append(result, value)
	}
	return result, nil
}

func (repository *Repository) UpdatePreference(
	ctx context.Context,
	userID uuid.UUID,
	category string,
	inApp bool,
	expectedVersion int64,
	now time.Time,
) (entity.Preference, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil ||
		!contract.ValidCategory(category) || expectedVersion < 0 || now.IsZero() ||
		(contract.RequiredCategory(category) && !inApp) {
		return entity.Preference{}, ports.ErrInvalidRequest
	}
	client := repository.client.DB(ctx)
	rows, err := client.NotificationPreference.QueryContext(ctx, `
		SELECT category, in_app, version, updated_at
		FROM notification_preferences
		WHERE user_id = $1 AND category = $2
		FOR UPDATE`, userID, category)
	if err != nil {
		return entity.Preference{}, fmt.Errorf("lock notification preference: %w", err)
	}
	var existing entity.Preference
	existing.UserID = userID
	found := rows.Next()
	if found {
		err = rows.Scan(&existing.Category, &existing.InApp, &existing.Version, &existing.UpdatedAt)
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return entity.Preference{}, fmt.Errorf("scan notification preference: %w", err)
	}
	if found {
		if existing.Version != expectedVersion {
			return entity.Preference{}, ports.ErrPreferenceVersionConflict
		}
		if existing.InApp == inApp {
			return existing, nil
		}
		rows, err = client.NotificationPreference.QueryContext(ctx, `
			UPDATE notification_preferences
			SET in_app = $3, version = version + 1, updated_at = $4
			WHERE user_id = $1 AND category = $2 AND version = $5
			RETURNING category, in_app, version, updated_at`,
			userID, category, inApp, now.UTC(), expectedVersion,
		)
		if err != nil {
			return entity.Preference{}, fmt.Errorf("update notification preference: %w", err)
		}
		return scanOnePreference(rows, userID)
	}
	if expectedVersion != 0 {
		return entity.Preference{}, ports.ErrPreferenceVersionConflict
	}
	id, err := repository.newID()
	if err != nil || id == uuid.Nil {
		return entity.Preference{}, fmt.Errorf("create notification preference identity: %w", err)
	}
	rows, err = client.NotificationPreference.QueryContext(ctx, `
		INSERT INTO notification_preferences (id, user_id, category, in_app, version, updated_at)
		VALUES ($1,$2,$3,$4,1,$5)
		ON CONFLICT (user_id, category) DO NOTHING
		RETURNING category, in_app, version, updated_at`, id, userID, category, inApp, now.UTC())
	if err != nil {
		return entity.Preference{}, fmt.Errorf("create notification preference: %w", err)
	}
	value, err := scanOnePreference(rows, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Preference{}, ports.ErrPreferenceVersionConflict
	}
	return value, err
}

func (repository *Repository) findBySource(
	ctx context.Context,
	userID uuid.UUID,
	sourceEventID uuid.UUID,
	typeName string,
) (entity.Notification, bool, error) {
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `SELECT `+notificationColumns+`
		FROM notifications WHERE user_id = $1 AND source_event_id = $2 AND type = $3`, userID, sourceEventID, typeName)
	if err != nil {
		return entity.Notification{}, false, fmt.Errorf("query notification idempotency row: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.Notification{}, false, rows.Err()
	}
	value, err := scanNotification(rows)
	if err != nil {
		return entity.Notification{}, false, fmt.Errorf("scan notification idempotency row: %w", err)
	}
	if rows.Next() {
		return entity.Notification{}, false, fmt.Errorf("notification idempotency query returned multiple rows")
	}
	return value, true, rows.Err()
}

func (repository *Repository) inAppEnabled(ctx context.Context, userID uuid.UUID, category string) (bool, error) {
	if contract.RequiredCategory(category) {
		return true, nil
	}
	rows, err := repository.client.DB(ctx).NotificationPreference.QueryContext(ctx, `
		SELECT COALESCE((
			SELECT in_app FROM notification_preferences WHERE user_id = $1 AND category = $2
		), TRUE)`, userID, category)
	if err != nil {
		return false, fmt.Errorf("query notification preference: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return false, fmt.Errorf("query notification preference: %w", sql.ErrNoRows)
	}
	var enabled bool
	if err := rows.Scan(&enabled); err != nil {
		return false, fmt.Errorf("scan notification preference: %w", err)
	}
	return enabled, rows.Err()
}

func (repository *Repository) lockInbox(ctx context.Context, userID uuid.UUID, now time.Time) (entity.InboxState, error) {
	client := repository.client.DB(ctx)
	if _, err := client.NotificationInboxState.ExecContext(ctx, `
		INSERT INTO notification_inbox_states (user_id, version, unread_count, updated_at)
		VALUES ($1,0,0,$2)
		ON CONFLICT (user_id) DO NOTHING`, userID, now.UTC()); err != nil {
		return entity.InboxState{}, fmt.Errorf("initialize notification inbox: %w", err)
	}
	rows, err := client.NotificationInboxState.QueryContext(ctx, `
		SELECT version, unread_count, updated_at
		FROM notification_inbox_states WHERE user_id = $1 FOR UPDATE`, userID)
	if err != nil {
		return entity.InboxState{}, fmt.Errorf("lock notification inbox: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.InboxState{}, fmt.Errorf("lock notification inbox: %w", sql.ErrNoRows)
	}
	state := entity.InboxState{UserID: userID}
	if err := rows.Scan(&state.Version, &state.UnreadCount, &state.UpdatedAt); err != nil {
		return entity.InboxState{}, fmt.Errorf("scan notification inbox: %w", err)
	}
	return state, rows.Err()
}

func (repository *Repository) saveInbox(ctx context.Context, state entity.InboxState) error {
	result, err := repository.client.DB(ctx).NotificationInboxState.ExecContext(ctx, `
		UPDATE notification_inbox_states
		SET version = $2, unread_count = $3, updated_at = $4
		WHERE user_id = $1`, state.UserID, state.Version, state.UnreadCount, state.UpdatedAt.UTC())
	if err != nil {
		return fmt.Errorf("save notification inbox: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count notification inbox update: %w", err)
	}
	if updated != 1 {
		return fmt.Errorf("save notification inbox: expected one row, updated %d", updated)
	}
	return nil
}

func (repository *Repository) lockOwned(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
	now time.Time,
) (entity.Notification, error) {
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `SELECT `+notificationColumns+`
		FROM notifications
		WHERE id = $1 AND user_id = $2 AND (expires_at IS NULL OR expires_at > $3)
		FOR UPDATE`, notificationID, userID, now.UTC())
	if err != nil {
		return entity.Notification{}, fmt.Errorf("lock owned notification: %w", err)
	}
	value, err := oneNotification(rows, "lock owned notification")
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Notification{}, ports.ErrNotFound
	}
	return value, err
}

func (repository *Repository) canonicalUnread(ctx context.Context, userID uuid.UUID, now time.Time) (int64, error) {
	rows, err := repository.client.DB(ctx).Notification.QueryContext(ctx, `
		SELECT COUNT(*) FROM notifications
		WHERE user_id = $1 AND read_at IS NULL AND archived_at IS NULL
			AND (expires_at IS NULL OR expires_at > $2)`, userID, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, fmt.Errorf("count unread notifications: %w", sql.ErrNoRows)
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return 0, fmt.Errorf("scan unread notification count: %w", err)
	}
	return count, rows.Err()
}

func oneNotification(rows *sql.Rows, operation string) (entity.Notification, error) {
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.Notification{}, fmt.Errorf("%s: %w", operation, err)
		}
		return entity.Notification{}, sql.ErrNoRows
	}
	value, err := scanNotification(rows)
	if err != nil {
		return entity.Notification{}, fmt.Errorf("%s: %w", operation, err)
	}
	if rows.Next() {
		return entity.Notification{}, fmt.Errorf("%s returned multiple rows", operation)
	}
	return value, rows.Err()
}

func scanOnePreference(rows *sql.Rows, userID uuid.UUID) (entity.Preference, error) {
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.Preference{}, err
		}
		return entity.Preference{}, sql.ErrNoRows
	}
	value := entity.Preference{UserID: userID}
	if err := rows.Scan(&value.Category, &value.InApp, &value.Version, &value.UpdatedAt); err != nil {
		return entity.Preference{}, fmt.Errorf("scan notification preference: %w", err)
	}
	if rows.Next() {
		return entity.Preference{}, fmt.Errorf("notification preference mutation returned multiple rows")
	}
	return value, rows.Err()
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
