package http

import (
	"fmt"
	"github.com/google/uuid"
)

func toMaterialRevisionIDs(request *GetMaterialRevisionRequest) (uuid.UUID, uuid.UUID, error) {
	materialID, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse material id: %w", err)
	}
	revisionID, err := uuid.Parse(request.RevisionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse material revision id: %w", err)
	}
	return materialID, revisionID, nil
}
