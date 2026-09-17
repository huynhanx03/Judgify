package domain

import (
	"time"

	"github.com/google/uuid"
)

// MaterialCategory represents a category for learning materials.
type MaterialCategory struct {
	ID           uuid.UUID
	Name         string
	Description  string
	ArticleCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
