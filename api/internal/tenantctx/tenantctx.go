// Package tenantctx carries tenant scopes validated by authenticated request
// middleware. A missing scope is never interpreted as access to all tenants.
package tenantctx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
)

var ErrInvalidScope = errors.New("tenant context: invalid scope")

type ScopeKind uint8

const (
	ScopeTenant ScopeKind = iota + 1
	ScopeAll
)

// Scope contains only tenant IDs already authorized for the current principal.
type Scope struct {
	kind      ScopeKind
	tenantIDs []uuid.UUID
	sqlBound  *atomic.Bool
}

func Tenant(id uuid.UUID) (Scope, error) {
	if id == uuid.Nil {
		return Scope{}, ErrInvalidScope
	}
	return Scope{kind: ScopeTenant, tenantIDs: []uuid.UUID{id}}, nil
}

// RequireTenant returns only a concrete tenant scope. System workers use it
// when capturing the tenant identity into durable operation payloads.
func RequireTenant(ctx context.Context) (uuid.UUID, error) {
	if ctx == nil {
		return uuid.Nil, ErrInvalidScope
	}
	scope, ok := FromContext(ctx)
	if !ok || !scope.IsTenant() {
		return uuid.Nil, ErrInvalidScope
	}
	id, ok := scope.TenantID()
	if !ok {
		return uuid.Nil, ErrInvalidScope
	}
	return id, nil
}

// WithTenantID replaces any inherited worker scope with the concrete tenant
// carried by one durable tenant-owned operation.
func WithTenantID(ctx context.Context, id uuid.UUID) (context.Context, error) {
	scope, err := Tenant(id)
	if err != nil || ctx == nil {
		return nil, ErrInvalidScope
	}
	return WithContext(ctx, scope), nil
}

func All(ids []uuid.UUID) (Scope, error) {
	if len(ids) == 0 {
		return Scope{}, ErrInvalidScope
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	canonical := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return Scope{}, ErrInvalidScope
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		canonical = append(canonical, id)
	}
	return Scope{kind: ScopeAll, tenantIDs: canonical}, nil
}

func (s Scope) Kind() ScopeKind { return s.kind }
func (s Scope) IsTenant() bool  { return s.kind == ScopeTenant }
func (s Scope) IsAll() bool     { return s.kind == ScopeAll }

// TenantID returns an ID only for a single-tenant scope.
func (s Scope) TenantID() (uuid.UUID, bool) {
	if s.kind != ScopeTenant || len(s.tenantIDs) != 1 {
		return uuid.Nil, false
	}
	return s.tenantIDs[0], true
}

// TenantIDs returns a copy so callers cannot mutate validated scope state.
func (s Scope) TenantIDs() []uuid.UUID { return append([]uuid.UUID(nil), s.tenantIDs...) }

// PostgreSQLUUIDArray returns a safely quoted UUID array literal. UUID values
// are canonicalized by the constructors, so the result contains no SQL input.
func (s Scope) PostgreSQLUUIDArray() (string, error) {
	if len(s.tenantIDs) == 0 || (!s.IsTenant() && !s.IsAll()) {
		return "", ErrInvalidScope
	}
	values := make([]string, len(s.tenantIDs))
	for i, id := range s.tenantIDs {
		if id == uuid.Nil {
			return "", ErrInvalidScope
		}
		values[i] = `"` + id.String() + `"`
	}
	return "{" + strings.Join(values, ",") + "}", nil
}

// BindSQLScope sets the current transaction's RLS scope for database/sql
// transactions owned outside Ent's UnitOfWork.
func BindSQLScope(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}) error {
	scope, ok := FromContext(ctx)
	if !ok || executor == nil {
		return ErrInvalidScope
	}
	value, err := scope.PostgreSQLUUIDArray()
	if err != nil {
		return err
	}
	if _, err := executor.ExecContext(ctx, "SELECT pg_catalog.set_config('app.tenant_ids', $1, true)", value); err != nil {
		return fmt.Errorf("set PostgreSQL tenant scope: %w", err)
	}
	return nil
}

// BindPrincipalSQL binds one authenticated account to transaction-local
// policies for account-owned tenant memberships and invitations.
func BindPrincipalSQL(ctx context.Context, userID uuid.UUID, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}) error {
	if userID == uuid.Nil || executor == nil || ctx == nil {
		return ErrInvalidScope
	}
	if _, err := executor.ExecContext(ctx,
		"SELECT pg_catalog.set_config('app.principal_user_id', $1, true)", userID.String()); err != nil {
		return fmt.Errorf("set PostgreSQL principal scope: %w", err)
	}
	return nil
}

// BindAllSQLScope resolves the authoritative tenant IDs from the registry and
// binds them to a transaction used for fixed aggregate infrastructure reads.
// It intentionally rejects an empty registry so callers cannot confuse an
// unavailable tenant catalog with an empty result set.
func BindAllSQLScope(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) error {
	if ctx == nil || executor == nil {
		return ErrInvalidScope
	}
	rows, err := executor.QueryContext(ctx, `SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return fmt.Errorf("load tenant registry for aggregate scope: %w", err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan tenant registry for aggregate scope: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate tenant registry for aggregate scope: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close tenant registry aggregate scope: %w", err)
	}
	if len(ids) == 0 {
		return ErrInvalidScope
	}
	scope, err := All(ids)
	if err != nil {
		return err
	}
	return BindSQLScope(WithContext(ctx, scope), executor)
}

// WithAllSQLScope temporarily widens a trusted platform query to the current
// tenant registry, then restores the transaction's previous tenant setting.
// The caller must already own the transaction and keep the callback read-only.
func WithAllSQLScope(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, fn func() error) (result error) {
	if ctx == nil || executor == nil || fn == nil {
		return ErrInvalidScope
	}
	rows, err := executor.QueryContext(ctx, `SELECT pg_catalog.current_setting('app.tenant_ids', true)`)
	if err != nil {
		return fmt.Errorf("read previous tenant scope: %w", err)
	}
	var previous sql.NullString
	if !rows.Next() {
		err := rows.Err()
		_ = rows.Close()
		if err != nil {
			return fmt.Errorf("read previous tenant scope: %w", err)
		}
		return ErrInvalidScope
	}
	if err := rows.Scan(&previous); err != nil {
		_ = rows.Close()
		return fmt.Errorf("scan previous tenant scope: %w", err)
	}
	if rows.Next() || rows.Err() != nil {
		err := rows.Err()
		_ = rows.Close()
		if err != nil {
			return fmt.Errorf("read previous tenant scope: %w", err)
		}
		return ErrInvalidScope
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close previous tenant scope: %w", err)
	}
	if err := BindAllSQLScope(ctx, executor); err != nil {
		return err
	}
	defer func() {
		previousScope := ""
		if previous.Valid {
			previousScope = previous.String
		}
		if _, err := executor.ExecContext(ctx,
			"SELECT pg_catalog.set_config('app.tenant_ids', $1, true)", previousScope); err != nil {
			result = errors.Join(result, fmt.Errorf("restore tenant scope: %w", err))
		}
	}()
	return fn()
}

// BindOperationWorkerSQL binds the runner's narrowly scoped control-plane
// capability to the current transaction. It never changes tenant IDs when a
// concrete tenant scope is already present.
func BindOperationWorkerSQL(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}) error {
	if !IsOperationWorker(ctx) || executor == nil {
		return ErrInvalidScope
	}
	if _, scoped := FromContext(ctx); scoped {
		if _, err := executor.ExecContext(ctx,
			"SELECT pg_catalog.set_config('app.operation_worker', 'off', true)"); err != nil {
			return fmt.Errorf("disable PostgreSQL operation worker scope: %w", err)
		}
		return nil
	}
	if _, err := executor.ExecContext(ctx,
		"SELECT pg_catalog.set_config('app.tenant_ids', '{}', true)"); err != nil {
		return fmt.Errorf("clear PostgreSQL tenant scope for operation worker: %w", err)
	}
	if _, err := executor.ExecContext(ctx,
		"SELECT pg_catalog.set_config('app.operation_worker', 'on', true)"); err != nil {
		return fmt.Errorf("bind PostgreSQL operation worker scope: %w", err)
	}
	return nil
}

type contextKey struct{}
type operationWorkerContextKey struct{}

func WithContext(ctx context.Context, scope Scope) context.Context {
	scope.tenantIDs = append([]uuid.UUID(nil), scope.tenantIDs...)
	scope.sqlBound = &atomic.Bool{}
	return context.WithValue(ctx, contextKey{}, scope)
}

// WithoutScope shadows any inherited worker or request scope. Durable global
// events must not accidentally run with the worker's All-memberships scope.
func WithoutScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, Scope{})
}

// WithOperationWorker grants the durable operation runner access to operation
// control-plane tables only. It does not widen app.tenant_ids, so tenant-owned
// business rows remain protected by their concrete tenant scope.
func WithOperationWorker(ctx context.Context) context.Context {
	return context.WithValue(ctx, operationWorkerContextKey{}, true)
}

// IsOperationWorker reports whether trusted internal operation-runner code
// marked this context for control-plane SQL access.
func IsOperationWorker(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(operationWorkerContextKey{}).(bool)
	return value
}

// SQLBound reports whether this request scope has been installed in its
// current database transaction with transaction-local PostgreSQL state.
func (s Scope) SQLBound() bool { return s.sqlBound != nil && s.sqlBound.Load() }

// MarkSQLBound records successful transaction-local scope installation.
func (s Scope) MarkSQLBound() {
	if s.sqlBound != nil {
		s.sqlBound.Store(true)
	}
}

func FromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(contextKey{}).(Scope)
	return scope, ok && (scope.IsTenant() || scope.IsAll()) && len(scope.tenantIDs) > 0
}
