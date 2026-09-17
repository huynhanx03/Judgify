package contract

import "github.com/google/uuid"

type CreateElementCommand struct {
	Name        string
	Code        string
	Description string
}

type UpdateElementCommand struct {
	Name        *string
	Code        *string
	Description *string
}

type ElementResponse struct {
	ID          uuid.UUID
	Name        string
	Code        string
	Description string
	Version     int64
}
