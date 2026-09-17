// Package contract owns stable, transport-independent Material integration
// event names and wire payloads.
package contract

import (
	"encoding/json"

	"github.com/google/uuid"
)

const (
	AggregateMaterial = "material"

	EventDraftCreatedV1    = "material.draft_created.v1"
	EventDraftSavedV1      = "material.draft_saved.v1"
	EventReviewSubmittedV1 = "material.review_submitted.v1"
	EventReviewReturnedV1  = "material.review_returned.v1"
	EventPublishedV1       = "material.published.v1"
	EventRolledBackV1      = "material.rolled_back.v1"
	EventArchivedV1        = "material.archived.v1"
	EventSlugChangedV1     = "material.slug_changed.v1"
)

// ChangedV1 is a bounded editorial fact. After is event-specific immutable
// evidence; it never includes Markdown or rendered HTML. Rollback receipts
// additionally carry a request hash and a redacted response snapshot.
type ChangedV1 struct {
	MaterialID  uuid.UUID       `json:"material_id"`
	Version     int64           `json:"version"`
	After       json.RawMessage `json:"after"`
	RequestHash string          `json:"request_hash,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}
