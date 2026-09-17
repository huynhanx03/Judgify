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

// NotificationPreference stores only the launch in-app channel. New delivery
// channels require their own versioned design instead of speculative columns.
type NotificationPreference struct{ ent.Schema }

func (NotificationPreference) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}}
}

func (NotificationPreference) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Enum("category").Values(contract.Categories()...).Immutable(),
		field.Bool("in_app").Default(true),
		field.Int64("version").Min(1).Default(1),
		field.Time("updated_at"),
	}
}

func (NotificationPreference) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("notification_preferences").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (NotificationPreference) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "category").Unique(),
	}
}

func (NotificationPreference) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"notification_preference_required_categories": "category NOT IN ('security', 'authorization', 'operation') OR in_app",
			"notification_preference_positive_version":    "version >= 1",
		}),
	}
}
