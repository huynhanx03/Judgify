package eventing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event is a versioned integration event. Metadata is deliberately closed:
// arbitrary transport attributes are not accepted at the domain boundary.
type Event struct {
	ID               uuid.UUID
	Type             string
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	CorrelationID    string
	CausationID      *uuid.UUID
	Payload          json.RawMessage
	OccurredAt       time.Time
}
