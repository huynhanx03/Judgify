package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/notification/domain"
)

type rowScanner interface {
	Scan(...any) error
}

func scanNotification(row rowScanner) (entity.Notification, error) {
	return scanNotificationRecord(row, false)
}

func scanNotificationWithCampaignTitle(
	row rowScanner,
) (entity.Notification, error) {
	return scanNotificationRecord(row, true)
}

func scanNotificationRecord(
	row rowScanner,
	includeCampaignTitle bool,
) (entity.Notification, error) {
	var value entity.Notification
	var titleKey sql.NullString
	var bodyKey sql.NullString
	var params []byte
	var campaignRevisionID uuid.NullUUID
	var actionPath sql.NullString
	var readAt sql.NullTime
	var archivedAt sql.NullTime
	var expiresAt sql.NullTime
	var campaignTitle sql.NullString
	destinations := []any{
		&value.ID,
		&value.UserID,
		&value.Type,
		&value.Category,
		&value.ContentKind,
		&titleKey,
		&bodyKey,
		&params,
		&campaignRevisionID,
		&actionPath,
		&value.SourceEventID,
		&value.Version,
		&value.CreatedAt,
		&readAt,
		&archivedAt,
		&expiresAt,
	}
	if includeCampaignTitle {
		destinations = append(destinations, &campaignTitle)
	}
	err := row.Scan(destinations...)
	if err != nil {
		return entity.Notification{}, err
	}
	value.TitleKey = nullableString(titleKey)
	value.BodyKey = nullableString(bodyKey)
	value.Params = append(json.RawMessage(nil), params...)
	value.CampaignRevisionID = nullableUUID(campaignRevisionID)
	value.CampaignTitle = nullableString(campaignTitle)
	value.ActionPath = nullableString(actionPath)
	value.ReadAt = nullableTime(readAt)
	value.ArchivedAt = nullableTime(archivedAt)
	value.ExpiresAt = nullableTime(expiresAt)
	return value, nil
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func nullableUUID(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	copy := value.UUID
	return &copy
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time.UTC()
	return &copy
}
