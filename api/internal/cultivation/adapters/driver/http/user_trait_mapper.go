package http

import (
	"fmt"
	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
)

func toUserTraitID(request *userTraitIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse user trait id: %w", err)
	}
	return id, nil
}
func toUserTraitResponse(result *contract.UserTraitResponse) *userTraitResponse {
	if result == nil {
		return nil
	}
	return &userTraitResponse{ID: result.ID.String(), UserID: result.UserID.String(), TraitID: result.TraitID.String()}
}
func toUserTraitPage(result *d.Paginated[*contract.UserTraitResponse]) *d.Paginated[*userTraitResponse] {
	if result == nil {
		return nil
	}
	page := &d.Paginated[*userTraitResponse]{Pagination: result.Pagination}
	if result.Records != nil {
		values := make([]*userTraitResponse, len(*result.Records))
		for index, value := range *result.Records {
			values[index] = toUserTraitResponse(value)
		}
		page.Records = &values
	}
	return page
}
