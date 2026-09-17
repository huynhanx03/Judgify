package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NotificationInboxState is the single serialization fence per account. The
// primary key is stored as user_id so every inbox mutation takes the same row
// lock before changing unread count or the monotonic client version.
type NotificationInboxState struct{ ent.Schema }

func (NotificationInboxState) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).StorageKey("user_id").Immutable(),
		field.Int64("version").Min(0).Default(0),
		field.Int64("unread_count").Min(0).Default(0),
		field.Time("updated_at"),
	}
}

func (NotificationInboxState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("updated_at", "id"),
	}
}
