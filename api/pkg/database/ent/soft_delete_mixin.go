package ent

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type (
	softDeleteKey struct{}

	// SoftDeleteQuery is implemented by all generated query types.
	SoftDeleteQuery interface {
		WhereP(...func(*sql.Selector))
	}
)

// SoftDeleteMixin adds deleted_at/deleted_by fields and auto-filters soft-deleted rows.
type SoftDeleteMixin struct {
	mixin.Schema
}

// Fields adds soft-delete tracking columns.
func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time(SoftDeleteAtColumnName).
			Optional().
			Nillable(),
		field.Int(SoftDeleteByColumnName).
			Optional().
			Nillable(),
	}
}

// Hooks returns a no-op hook to satisfy the generated runtime init expectations.
// Actual soft delete is handled at the repository layer via UpdateOneID().SetDeletedAt().
func (SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		func(next ent.Mutator) ent.Mutator {
			return next
		},
	}
}

// Interceptors auto-applies WHERE deleted_at IS NULL to all queries.
func (d SoftDeleteMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		ent.TraverseFunc(func(ctx context.Context, q ent.Query) error {
			if IsSkipSoftDelete(ctx) {
				return nil
			}
			if query, ok := q.(SoftDeleteQuery); ok {
				d.P(query)
			}
			return nil
		}),
	}
}

// P appends the deleted_at IS NULL predicate.
func (d SoftDeleteMixin) P(w SoftDeleteQuery) {
	w.WhereP(
		sql.FieldIsNull(d.Fields()[0].Descriptor().Name),
	)
}

// SkipSoftDelete returns a context that bypasses soft-delete filtering.
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, softDeleteKey{}, true)
}

// IsSkipSoftDelete checks if soft delete should be skipped.
func IsSkipSoftDelete(ctx context.Context) bool {
	skip, _ := ctx.Value(softDeleteKey{}).(bool)
	return skip
}
