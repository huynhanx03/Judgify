package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateRegistration builds the create mutation for ContestRegistration.
func BuildCreateRegistration(ctx context.Context, contestID, userID int) *generate.ContestRegistrationCreate {
	return global.EntClient.DB(ctx).ContestRegistration.Create().
		SetContestID(contestID).
		SetUserID(userID)
}
