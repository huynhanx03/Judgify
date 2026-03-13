package ent

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	pkgEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/hook"
)

// softDeleteMutation is satisfied by all generated mutation types that support soft delete.
type softDeleteMutation interface {
	SetOp(generate.Op)
	Client() *generate.Client
	SetDeletedAt(time.Time)
	WhereP(...func(*sql.Selector))
}

// registerSoftDeleteHook registers a hook converts delete operation to update operation with deleted_at.
func registerSoftDeleteHook(client *generate.Client) {
	client.Use(
		hook.On(
			func(next generate.Mutator) generate.Mutator {
				return generate.MutateFunc(func(ctx context.Context, m generate.Mutation) (generate.Value, error) {
					if pkgEnt.IsSkipSoftDelete(ctx) {
						return next.Mutate(ctx, m)
					}

					mx, ok := m.(softDeleteMutation)
					if !ok {
						return nil, fmt.Errorf("unexpected mutation type %T for soft delete", m)
					}

					mx.WhereP(sql.FieldIsNull(pkgEnt.SoftDeleteAtColumnName))
					mx.SetOp(generate.OpUpdate)
					mx.SetDeletedAt(time.Now().UTC())
					return mx.Client().Mutate(ctx, m)
				})
			},
			generate.OpDelete|generate.OpDeleteOne,
		),
	)
}
