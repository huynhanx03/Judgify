package domain

import (
	"time"

	"github.com/google/uuid"
)

// TagElement holds element data loaded via tag edges.
type TagElement struct {
	ID   uuid.UUID
	Name string
	Code string
}

// Tag represents a problem classification tag.
type Tag struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Elements  []TagElement `json:"-"`
	Version   int64        `json:"version"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// ElementIDs projects the loaded element relation into persistence identifiers.
func (t *Tag) ElementIDs() []uuid.UUID {
	ids := make([]uuid.UUID, len(t.Elements))
	for i, e := range t.Elements {
		ids[i] = e.ID
	}
	return ids
}
