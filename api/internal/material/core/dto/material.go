package dto

import (
	"time"

	problemDto "github.com/huynhanx03/judgify/internal/problem/core/dto"
)

// MaterialResponse represents material article data in API response.
type MaterialResponse struct {
	ID                int                       `json:"id"`
	Title             string                    `json:"title"`
	Description       string                    `json:"description"`
	Content           string                    `json:"content,omitempty"`
	Difficulty        *problemDto.DifficultyResponse `json:"difficulty"`
	Category          *MaterialCategoryResponse `json:"category"`
	Tags              []*problemDto.TagResponse `json:"tags"`
	AuthorID          int                       `json:"author_id"`
	Status            string                    `json:"status"`
	Visibility        string                    `json:"visibility"`
	ViewCount         int                       `json:"view_count"`
	EstimatedReadTime int                       `json:"estimated_read_time"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

// CreateMaterialRequest represents request to create a material article.
type CreateMaterialRequest struct {
	Title        string `json:"title" validate:"required,min=1,max=255"`
	Description  string `json:"description" validate:"omitempty,max=1000"`
	Content      string `json:"content"`
	DifficultyID int    `json:"difficulty_id" validate:"required"`
	CategoryID   int    `json:"category_id" validate:"required"`
	TagIDs       []int  `json:"tag_ids"`
	Status       string `json:"status" validate:"omitempty,oneof=draft published"`
	Visibility   string `json:"visibility" validate:"omitempty,oneof=public group"`
}

// UpdateMaterialRequest represents request to update a material article.
type UpdateMaterialRequest struct {
	ID           int     `json:"-" uri:"id"`
	Title        *string `json:"title" validate:"omitempty,min=1,max=255"`
	Description  *string `json:"description" validate:"omitempty,max=1000"`
	Content      *string `json:"content"`
	DifficultyID *int    `json:"difficulty_id"`
	CategoryID   *int    `json:"category_id"`
	TagIDs       *[]int  `json:"tag_ids"`
	Status       *string `json:"status" validate:"omitempty,oneof=draft published"`
	Visibility   *string `json:"visibility" validate:"omitempty,oneof=public group"`
}

// GetMaterialRequest represents request to get a material by ID.
type GetMaterialRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteMaterialRequest represents request to delete a material.
type DeleteMaterialRequest struct {
	ID int `uri:"id" validate:"required"`
}
