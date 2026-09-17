package schema

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
)

// Notification is the durable, account-owned inbox projection. Its content is
// either a stable frontend catalog key plus bounded parameters or one frozen
// campaign revision; arbitrary rendered HTML is never stored in this row.
type Notification struct{ ent.Schema }

func (Notification) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}}
}

func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.String("type").NotEmpty().MaxLen(contract.MaximumTypeBytes).Immutable(),
		field.Enum("category").Values(contract.Categories()...).Immutable(),
		field.Enum("content_kind").Values(contract.ContentKindCatalog, contract.ContentKindCampaign).Immutable(),
		field.String("title_key").MaxLen(contract.MaximumMessageKeyBytes).Optional().Nillable().Immutable(),
		field.String("body_key").MaxLen(contract.MaximumMessageKeyBytes).Optional().Nillable().Immutable(),
		field.JSON("params", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Optional().
			Immutable(),
		field.UUID("campaign_revision_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("action_path").MaxLen(contract.MaximumActionPathBytes).Optional().Nillable().Immutable(),
		field.UUID("source_event_id", uuid.UUID{}).Immutable(),
		field.Int64("version").Min(1).Default(1),
		field.Time("created_at").Immutable(),
		field.Time("read_at").Optional().Nillable(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable().Immutable(),
	}
}

func (Notification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("notifications").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
		edge.From("campaign_revision", NotificationCampaignRevision.Type).
			Ref("notifications").
			Field("campaign_revision_id").
			Unique().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("campaign_delivery", NotificationCampaignRecipient.Type).
			Unique().
			Immutable(),
	}
}

func (Notification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "source_event_id", "type").Unique(),
		index.Fields("user_id", "created_at", "id"),
		index.Fields("user_id", "archived_at", "created_at", "id"),
		index.Fields("user_id", "read_at", "expires_at"),
		index.Fields("campaign_revision_id", "user_id"),
	}
}

func (Notification) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"notification_content_shape": `(
				content_kind = 'catalog'
				AND title_key IS NOT NULL
				AND body_key IS NOT NULL
				AND params IS NOT NULL
				AND campaign_revision_id IS NULL
			) OR (
				content_kind = 'campaign'
				AND title_key IS NULL
				AND body_key IS NULL
				AND params IS NULL
				AND campaign_revision_id IS NOT NULL
			)`,
			"notification_params_bound": fmt.Sprintf(
				"params IS NULL OR octet_length(params::text) <= %d",
				contract.MaximumParamsBytes,
			),
			"notification_expiry_after_creation": "expires_at IS NULL OR expires_at > created_at",
			"notification_read_before_archive":   "archived_at IS NULL OR read_at IS NOT NULL",
			"notification_positive_version":      "version >= 1",
		}),
	}
}
