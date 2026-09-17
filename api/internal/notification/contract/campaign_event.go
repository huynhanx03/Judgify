package contract

import "github.com/google/uuid"

// CampaignLifecycleEvent is the closed, non-sensitive outbox projection for
// campaign draft and status transitions. The outbox aggregate ID/version must
// equal CampaignID/Version when a dispatcher validates the envelope.
type CampaignLifecycleEvent struct {
	CampaignID  uuid.UUID  `json:"campaign_id"`
	Status      string     `json:"status"`
	Version     int64      `json:"version"`
	OperationID *uuid.UUID `json:"operation_id,omitempty"`
}

func (event CampaignLifecycleEvent) Validate() error {
	if event.CampaignID == uuid.Nil || event.Version < 1 ||
		!ValidCampaignStatus(event.Status) ||
		(event.OperationID != nil && *event.OperationID == uuid.Nil) {
		return ErrInvalidCampaign
	}
	if event.Status == CampaignStatusDraft && event.OperationID != nil {
		return ErrInvalidCampaign
	}
	return nil
}
