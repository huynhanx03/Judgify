package db

import (
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateUserStats builds the create mutation for UserStats.
func BuildCreateUserStats(client *generate.Client, e *entity.UserStats) *generate.UserStatsCreate {
	return client.UserStats.Create().
		SetUserID(e.UserID).
		SetTotalExp(e.TotalExp).
		SetRating(e.Rating)
}
