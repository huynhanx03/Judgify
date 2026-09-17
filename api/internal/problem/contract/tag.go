package contract

import "github.com/google/uuid"

// CreateTagCommand is the application input for creating a tag.
type CreateTagCommand struct {
	Name       string
	ElementIDs []uuid.UUID
}

// UpdateTagCommand is the application input for updating a tag.
type UpdateTagCommand struct {
	Name       *string
	ElementIDs *[]uuid.UUID
}

// TagElementResult is the application projection of an element nested in a tag.
type TagElementResult struct {
	ID   uuid.UUID
	Name string
	Code string
}

// TagResult is the application projection of a tag.
type TagResult struct {
	ID       uuid.UUID
	Name     string
	Elements []*TagElementResult
	Version  int64
}
