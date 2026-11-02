package http

import "github.com/google/uuid"

type loginRequest struct {
	Username       string `json:"username" validate:"required,max=50"`
	Password       string `json:"password" validate:"required,max=256"`
	PreSessionID   uuid.UUID
	IdempotencyKey uuid.UUID
	UserAgent      string
	IPHash         []byte
}
