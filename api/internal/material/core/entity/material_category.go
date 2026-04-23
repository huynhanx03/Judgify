package entity

import "time"

// MaterialCategory represents a category for learning materials.
type MaterialCategory struct {
	ID           int
	Name         string
	Description  string
	ArticleCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
