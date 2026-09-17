package schema

import (
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

// NotificationCampaignRevision is immutable authored content plus one closed
// audience selector. Exact content-render tuple constraints are installed by
// the PostgreSQL desired-schema extension during final integration.
type NotificationCampaignRevision struct{ ent.Schema }

func (NotificationCampaignRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (NotificationCampaignRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("campaign_id", uuid.UUID{}).Immutable(),
		field.Int64("revision_number").Positive().Immutable(),
		field.String("title").NotEmpty().MaxLen(contract.MaximumCampaignTitleBytes).Immutable(),
		field.Text("body_markdown").Immutable().Sensitive(),
		field.UUID("body_render_source_id", uuid.UUID{}).Immutable(),
		field.Enum("body_render_source_kind").
			Values(rendercontract.SourceKindNotificationCampaignBody).
			Immutable(),
		field.UUID("body_render_artifact_id", uuid.UUID{}).Immutable(),
		field.String("action_path").MaxLen(MaxNotificationActionPathBytes).Optional().Nillable().Immutable(),
		field.Enum("audience_kind").Values(contract.AudienceKinds()...).Immutable(),
		field.UUID("audience_role_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Bytes("normalized_source_checksum").NotEmpty().Immutable().Sensitive(),
		field.UUID("author_id", uuid.UUID{}).Immutable(),
		field.Time("created_at").Immutable(),
	}
}

func (NotificationCampaignRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("campaign", NotificationCampaign.Type).
			Ref("revisions").
			Field("campaign_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("author", User.Type).
			Ref("notification_campaign_revisions").
			Field("author_id").
			Unique().
			Required().
			Immutable(),
		edge.To("explicit_users", NotificationCampaignExplicitUser.Type).
			Immutable(),
		edge.To("notifications", Notification.Type).
			Immutable(),
		edge.To("recipients", NotificationCampaignRecipient.Type).
			Immutable(),
	}
}

func (NotificationCampaignRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("campaign_id", "revision_number").Unique(),
		index.Fields("id", "campaign_id", "body_render_source_id", "body_render_source_kind", "normalized_source_checksum").Unique(),
		index.Fields("id", "body_render_source_id", "body_render_source_kind", "normalized_source_checksum").Unique(),
		index.Fields("body_render_artifact_id", "body_render_source_id", "body_render_source_kind", "normalized_source_checksum").Unique(),
	}
}

func (NotificationCampaignRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"notification_campaign_revision_body_bound": fmt.Sprintf(
				"octet_length(body_markdown) BETWEEN 1 AND %d",
				contract.MaximumCampaignBodyBytes,
			),
			"notification_campaign_revision_checksum_size": "octet_length(normalized_source_checksum) = 32",
			"notification_campaign_revision_audience_shape": `(
				audience_kind = 'role' AND audience_role_id IS NOT NULL
			) OR (
				audience_kind IN ('explicit_users', 'all_active')
				AND audience_role_id IS NULL
			)`,
			"notification_campaign_revision_source_kind": "body_render_source_kind = 'notification_campaign_body'",
		}),
	}
}

const MaxNotificationActionPathBytes = contract.MaximumActionPathBytes
