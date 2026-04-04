package entity

import "time"

// UserTrait represents a user's gacha result (root bone or talent).
type UserTrait struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	TraitID   int       `json:"trait_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
