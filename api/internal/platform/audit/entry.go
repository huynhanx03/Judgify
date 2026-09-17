package audit

import (
	"time"

	"github.com/google/uuid"
)

// Entry is the redaction input for one immutable audit record. Before and
// After must contain bounded JSON-shaped values; the repository clones,
// redacts, and canonicalizes them before persistence.
type Entry struct {
	ID             uuid.UUID
	ActorUserID    *uuid.UUID
	ActorSessionID *uuid.UUID
	Action         string
	Resource       string
	ResourceID     string
	Reason         string
	Before         map[string]any
	After          map[string]any
	CorrelationID  string
	OccurredAt     time.Time
}
