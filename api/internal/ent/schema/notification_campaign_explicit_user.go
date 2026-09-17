package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// NotificationCampaignExplicitUser freezes the bounded authored selector on
// an immutable revision. Scheduling still snapshots active eligibility.
type NotificationCampaignExplicitUser struct{ ent.Schema }

func (NotificationCampaignExplicitUser) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (NotificationCampaignExplicitUser) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("campaign_revision_id", uuid.UUID{}).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Time("created_at").Immutable(),
	}
}

func (NotificationCampaignExplicitUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("revision", NotificationCampaignRevision.Type).
			Ref("explicit_users").
			Field("campaign_revision_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("user", User.Type).
			Ref("explicit_notification_campaign_revisions").
			Field("user_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (NotificationCampaignExplicitUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("campaign_revision_id", "user_id").Unique(),
		index.Fields("user_id", "campaign_revision_id"),
	}
}
