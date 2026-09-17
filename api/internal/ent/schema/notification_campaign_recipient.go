package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
)

// NotificationCampaignRecipient is the immutable schedule membership plus
// retry-safe delivery state for one account.
type NotificationCampaignRecipient struct{ ent.Schema }

func (NotificationCampaignRecipient) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}}
}

func (NotificationCampaignRecipient) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("campaign_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("campaign_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("source_event_id", uuid.UUID{}).Immutable(),
		field.Enum("status").Values(contract.RecipientStatuses()...).Default(contract.RecipientStatusPending),
		field.UUID("notification_id", uuid.UUID{}).Optional().Nillable(),
		field.String("skip_code").MaxLen(contract.MaximumCampaignFailureCodeBytes).Optional().Nillable(),
		field.String("failure_code").MaxLen(contract.MaximumCampaignFailureCodeBytes).Optional().Nillable(),
		field.Uint32("attempt_count").Default(0),
		field.Time("next_attempt_at").Optional().Nillable(),
		field.Time("created_at").Immutable(),
		field.Time("last_attempt_at").Optional().Nillable(),
		field.Time("processed_at").Optional().Nillable(),
	}
}

func (NotificationCampaignRecipient) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("campaign", NotificationCampaign.Type).
			Ref("recipients").
			Field("campaign_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("user", User.Type).
			Ref("notification_campaign_deliveries").
			Field("user_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("revision", NotificationCampaignRevision.Type).
			Ref("recipients").
			Field("campaign_revision_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("notification", Notification.Type).
			Ref("campaign_delivery").
			Field("notification_id").
			Unique().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (NotificationCampaignRecipient) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("campaign_id", "user_id").Unique(),
		index.Fields("source_event_id").Unique(),
		index.Fields("campaign_id", "status", "user_id"),
		index.Fields("campaign_id", "status", "next_attempt_at", "user_id"),
		index.Fields("campaign_revision_id", "user_id"),
	}
}

func (NotificationCampaignRecipient) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"notification_campaign_recipient_result_shape": `(
				status = 'pending'
				AND notification_id IS NULL
				AND skip_code IS NULL
				AND failure_code IS NULL
				AND processed_at IS NULL
			) OR (
				status = 'created'
				AND notification_id IS NOT NULL
				AND skip_code IS NULL
				AND failure_code IS NULL
				AND processed_at IS NOT NULL
			) OR (
				status = 'skipped'
				AND notification_id IS NULL
				AND skip_code IS NOT NULL
				AND failure_code IS NULL
				AND processed_at IS NOT NULL
			) OR (
				status = 'failed'
				AND notification_id IS NULL
				AND skip_code IS NULL
				AND failure_code IS NOT NULL
				AND processed_at IS NOT NULL
			)`,
			"notification_campaign_recipient_retry_shape": "status = 'pending' OR next_attempt_at IS NULL",
		}),
	}
}
