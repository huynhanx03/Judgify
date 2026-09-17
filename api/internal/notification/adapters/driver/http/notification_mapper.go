package http

import (
	"fmt"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
)

func toNotificationID(request *notificationIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse notification id: %w", err)
	}
	return id, nil
}

func toNotificationListQuery(request *notificationListRequest) *contract.ListRequest {
	return &contract.ListRequest{
		Cursor: request.Cursor, Limit: request.Limit, IncludeArchived: request.IncludeArchived,
	}
}

func toUpdateNotificationPreferenceRequest(request *updateNotificationPreferenceRequest) *contract.UpdatePreferenceRequest {
	return &contract.UpdatePreferenceRequest{
		Category: request.Category, InApp: request.InApp, ExpectedVersion: request.ExpectedVersion,
	}
}

func toUnreadCountResponse(result *contract.UnreadCountResponse) *unreadCountResponse {
	if result == nil {
		return nil
	}
	return &unreadCountResponse{Count: result.Count, Version: result.Version}
}
func toPreferenceResponse(result *contract.PreferenceResponse) *preferenceResponse {
	if result == nil {
		return nil
	}
	return &preferenceResponse{Category: result.Category, InApp: result.InApp, Required: result.Required, Version: result.Version}
}
func toPreferenceResponses(results []contract.PreferenceResponse) []preferenceResponse {
	values := make([]preferenceResponse, len(results))
	for index := range results {
		values[index] = *toPreferenceResponse(&results[index])
	}
	return values
}
func toCampaignContentResponse(result *contract.CampaignContentResponse) *campaignContentResponse {
	if result == nil {
		return nil
	}
	return &campaignContentResponse{NotificationID: result.NotificationID.String(), RevisionID: result.RevisionID.String(), Title: result.Title, SanitizedHTML: result.SanitizedHTML, ActionPath: result.ActionPath}
}
func toNotificationResponse(result *contract.NotificationResponse) *notificationResponse {
	if result == nil {
		return nil
	}
	return &notificationResponse{ID: result.ID.String(), Type: result.Type, Category: result.Category, ContentKind: result.ContentKind, TitleKey: result.TitleKey, BodyKey: result.BodyKey, Params: result.Params, CampaignRevisionID: toNotificationUUIDPointer(result.CampaignRevisionID), CampaignTitle: result.CampaignTitle, ActionPath: result.ActionPath, Version: result.Version, CreatedAt: result.CreatedAt, ReadAt: result.ReadAt, ArchivedAt: result.ArchivedAt, ExpiresAt: result.ExpiresAt}
}
func toNotificationUUIDPointer(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}
func toNotificationListResponse(result *contract.ListResponse) *notificationListResponse {
	if result == nil {
		return nil
	}
	items := make([]*notificationResponse, len(result.Items))
	for i := range result.Items {
		items[i] = toNotificationResponse(&result.Items[i])
	}
	return &notificationListResponse{Items: items, NextCursor: result.NextCursor, UnreadCount: result.UnreadCount, InboxVersion: result.InboxVersion}
}
func toNotificationMutationResponse(result *contract.MutationResponse) *notificationMutationResponse {
	if result == nil {
		return nil
	}
	return &notificationMutationResponse{Notification: toNotificationResponse(result.Notification), UnreadCount: result.UnreadCount, InboxVersion: result.InboxVersion, Changed: result.Changed}
}
