package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const MaterialViewRecordedV1 = "material.view_recorded.v1"

type DailyViewReceipt struct {
	ID         uuid.UUID
	MaterialID uuid.UUID
	ViewDay    time.Time
	Inserted   bool
}

type DailyViewRecorded struct {
	ReceiptID  uuid.UUID `json:"receipt_id"`
	MaterialID uuid.UUID `json:"material_id"`
	ViewDay    time.Time `json:"view_day"`
}

type MaterialCommandReceipt struct {
	EventID       uuid.UUID
	EventType     string
	AggregateType string
	AggregateID   uuid.UUID
	Version       int64
	Payload       json.RawMessage
	OccurredAt    time.Time
}
