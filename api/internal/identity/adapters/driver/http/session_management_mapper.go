package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toSessionListRequest(query *sessionListQuery) *dto.SessionListRequest {
	if query == nil {
		return nil
	}
	return &dto.SessionListRequest{Cursor: query.Cursor, Limit: query.Limit}
}

func toSessionListResponse(value *dto.SessionListResponse) *sessionListResponse {
	if value == nil {
		return nil
	}
	result := &sessionListResponse{Items: make([]sessionResponse, len(value.Items)), NextCursor: value.NextCursor}
	for index, item := range value.Items {
		result.Items[index] = sessionResponse{
			ID: item.ID.String(), Current: item.Current, Status: item.Status,
			CreatedAt: item.CreatedAt, LastSeenAt: item.LastSeenAt,
			LastReauthenticatedAt: item.LastReauthenticatedAt, IdleExpiresAt: item.IdleExpiresAt,
			AbsoluteExpiresAt: item.AbsoluteExpiresAt, RevokedAt: item.RevokedAt, UserAgent: item.UserAgent,
		}
	}
	return result
}
