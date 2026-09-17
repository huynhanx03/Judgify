package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// RealtimeCursorState is the singleton PostgreSQL serialization fence for
// every browser-safe replay event. The global cursor advances in the same
// transaction as the complete routed batch, so clients observe one opaque,
// contiguous order across all topics and never a partially projected source
// event.
type RealtimeCursorState struct{ ent.Schema }

func (RealtimeCursorState) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().MaxLen(64).Immutable(),
		field.Int64("last_sequence").Min(0).Default(0),
		field.Time("updated_at").Default(func() time.Time { return time.Now().UTC() }),
	}
}
