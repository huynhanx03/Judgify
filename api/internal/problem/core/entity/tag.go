package entity

import "time"

// TagElement holds element data loaded via tag edges.
type TagElement struct {
	ID   int
	Name string
	Code string
}

// Tag represents a problem classification tag.
type Tag struct {
	ID        int          `json:"id"`
	Name      string       `json:"name"`
	Elements  []TagElement `json:"-"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// ElementIDs returns a slice of element IDs for backward compatibility.
func (t *Tag) ElementIDs() []int {
	ids := make([]int, len(t.Elements))
	for i, e := range t.Elements {
		ids[i] = e.ID
	}
	return ids
}
