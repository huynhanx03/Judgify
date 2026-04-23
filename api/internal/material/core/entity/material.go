package entity

import (
	"time"

	problemEntity "github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// Material represents a learning material article.
type Material struct {
	ID                int
	Title             string
	Description       string
	Content           string
	DifficultyID      int
	Difficulty        *problemEntity.ProblemDifficulty
	AuthorID          int
	CategoryID        int
	Category          *MaterialCategory
	Status            string // draft, published
	Visibility        string // public, group
	GroupID           *int
	ViewCount         int
	EstimatedReadTime int
	Tags              []problemEntity.Tag
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
