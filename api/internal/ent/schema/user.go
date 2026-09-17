package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").
			Unique().
			NotEmpty().
			MaxLen(50),
		field.Enum("status").
			Values(userStatusInvited, userStatusActive, userStatusSuspended, userStatusDeleted).
			Default(userStatusActive),
		field.Int64("security_version").
			Min(1).
			Default(1),
		// Public profile discovery is explicitly opt-in. Keeping the privacy
		// state and its optimistic version on the account makes the read gate a
		// single-row, fail-closed decision instead of an inferred convention.
		field.Enum("profile_visibility").
			Values("private", "public").
			Default("private"),
		field.Int64("profile_visibility_version").
			Min(1).
			Default(1),
		field.Time("profile_visibility_updated_at").
			Default(time.Now),
		field.Time("suspended_at").
			Optional().
			Nillable(),
		field.String("suspension_reason").
			MaxLen(1024).
			Optional().
			Nillable().
			Sensitive(),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("credentials", Credential.Type),
		edge.To("attributes", UserAttributeValue.Type),
		edge.To("federated_identities", FederatedIdentity.Type),
		edge.To("auth_sessions", AuthSession.Type),
		edge.To("recovery_contacts", UserRecoveryContact.Type),
		edge.To("recovery_contact_commands", RecoveryContactCommandReceipt.Type),
		edge.To("password_reset_tokens", PasswordResetToken.Type),
		edge.To("session_issuance_receipts", AuthSessionIssuanceReceipt.Type),
		edge.To("consumed_trait_offers", OnboardingTraitOffer.Type),
		edge.To("session_commands", AuthSessionCommandReceipt.Type),
		edge.To("oauth_completion_tickets", OAuthCompletionTicket.Type),
		edge.To("authorization_updates", AuthorizationState.Type),
		edge.To("problems", Problem.Type),
		edge.To("submissions", Submission.Type),
		edge.To("user_traits", UserTrait.Type),
		edge.To("reward_profile_revisions", UserRewardProfileRevision.Type),
		edge.To("user_element_exps", UserElementExp.Type),
		edge.To("user_stats", UserStats.Type),
		edge.To("cultivation_ranking_projection", CultivationRankingProjection.Type),
		edge.To("solved_problems", UserSolvedProblem.Type),
		edge.To("difficulty_stats", UserDifficultyStats.Type),
		edge.To("tag_stats", UserTagStats.Type),
		edge.To("contests", Contest.Type),
		edge.To("contest_registrations", ContestRegistration.Type),
		edge.To("contest_standings", ContestStanding.Type),
		edge.To("rating_histories", RatingHistory.Type),
		edge.To("contest_rating_results", ContestRatingResult.Type),
		edge.To("contest_user_ratings", ContestUserRating.Type),
		edge.To("contest_scoring_attempts", ContestScoringAttempt.Type),
		edge.To("contest_problem_results", ContestProblemResult.Type),
		edge.To("materials", Material.Type),
		edge.To("material_revisions", MaterialRevision.Type),
		edge.To("notifications", Notification.Type),
		edge.To("notification_preferences", NotificationPreference.Type),
		edge.To("created_notification_campaigns", NotificationCampaign.Type),
		edge.To("notification_campaign_revisions", NotificationCampaignRevision.Type),
		edge.To("explicit_notification_campaign_revisions", NotificationCampaignExplicitUser.Type),
		edge.To("notification_campaign_deliveries", NotificationCampaignRecipient.Type),
		edge.To("contest_content_revisions", ContestContentRevision.Type),
		edge.To("contest_clarifications", ContestClarification.Type),
		edge.To("contest_content_commands", ContestContentCommand.Type),
	}
}

// Annotations prevent a future repository from persisting an internally
// contradictory account state.
func (User) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"user_status_closed":                       "status IN ('invited', 'active', 'suspended', 'deleted')",
			"user_profile_visibility_closed":           "profile_visibility IN ('private', 'public')",
			"user_profile_visibility_version_positive": "profile_visibility_version >= 1",
			"user_suspension_state":                    "(status = 'suspended' AND suspended_at IS NOT NULL AND suspension_reason IS NOT NULL) OR (status <> 'suspended' AND suspended_at IS NULL AND suspension_reason IS NULL)",
		}),
	}
}
