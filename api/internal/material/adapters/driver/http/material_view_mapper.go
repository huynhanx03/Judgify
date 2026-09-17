package http

import (
	"fmt"

	"github.com/google/uuid"
)

func toMaterialViewID(request *RecordMaterialViewRequest) (uuid.UUID, error) {
	return parseMaterialID(request.ID)
}

func toMaterialID(request *materialIDRequest) (uuid.UUID, error) {
	return parseMaterialID(request.ID)
}

func parseMaterialID(rawID string) (uuid.UUID, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse material id: %w", err)
	}
	return id, nil
}
