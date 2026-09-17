// Package debuglevel owns the security-sensitive application workflow for a
// temporary process debug window. Transport authentication is not enough:
// this workflow repeats capability and recent-reauthentication checks at the
// point where the requested level is known, then writes an immutable audit
// record before applying the process-local override.
package debuglevel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"
	"github.com/huynhanx03/go-common/pkg/security/authorization"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

var (
	ErrInvalidCommand               = errors.New("debug level: invalid command")
	ErrPermissionDenied             = errors.New("debug level: permission denied")
	ErrRecentAuthenticationRequired = errors.New("debug level: recent authentication required")
	ErrAuditFailed                  = errors.New("debug level: audit failed")
)

const (
	maxReasonBytes    = 1024
	maxComponentBytes = 64
)

// Command contains only reviewed control-plane input. IdempotencyKey binds a
// replay to the same process-local lease in go-common's DebugWindow.
type Command struct {
	Component      string
	Level          string
	Duration       time.Duration
	Reason         string
	IdempotencyKey uuid.UUID
}

type Config struct {
	Component               string
	MaximumDuration         time.Duration
	RecentAuthenticationTTL time.Duration
}

type RecentAuthentication interface {
	RequireRecentReauthentication(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		time.Time,
	) error
}

type Window interface {
	Activate(context.Context, commonmetrics.DebugRequest) (commonmetrics.DebugLease, error)
	Revoke(context.Context, string) error
	Active() (commonmetrics.DebugLease, bool)
}

type Workflow struct {
	authorizer authorization.Authorizer
	recent     RecentAuthentication
	unit       platformtx.UnitOfWork
	audit      audit.Appender
	window     Window
	config     Config
	now        func() time.Time
	controlMu  sync.Mutex
}

func New(
	authorizer authorization.Authorizer,
	recent RecentAuthentication,
	unit platformtx.UnitOfWork,
	appender audit.Appender,
	window Window,
	config Config,
) (*Workflow, error) {
	if authorizer == nil || recent == nil || unit == nil || appender == nil || window == nil ||
		!validToken(config.Component, maxComponentBytes) ||
		config.MaximumDuration <= 0 || config.RecentAuthenticationTTL <= 0 {
		return nil, ErrInvalidCommand
	}
	return &Workflow{
		authorizer: authorizer, recent: recent, unit: unit, audit: appender,
		window: window, config: config, now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// Activate authorizes and audits one bounded debug window. The database
// transaction contains no logger mutation; the override is applied only after
// the audit record commits. If level application is ambiguous, DebugWindow
// either restores baseline or keeps the degraded lease visible and retryable.
func (workflow *Workflow) Activate(
	ctx context.Context,
	command Command,
) (commonmetrics.DebugLease, bool, error) {
	if workflow == nil || ctx == nil || !validCommand(command, workflow.config.MaximumDuration) {
		return commonmetrics.DebugLease{}, false, ErrInvalidCommand
	}
	if command.Component != workflow.config.Component {
		return commonmetrics.DebugLease{}, false, ErrInvalidCommand
	}
	workflow.controlMu.Lock()
	defer workflow.controlMu.Unlock()
	actorID, sessionID, err := workflow.authorizedActor(
		ctx,
		permissions.PermissionScopeUpdate,
	)
	if err != nil {
		return commonmetrics.DebugLease{}, false, err
	}

	now := workflow.now().UTC()
	err = workflow.unit.Within(ctx, func(txCtx context.Context) error {
		if recentErr := workflow.recent.RequireRecentReauthentication(
			txCtx,
			actorID,
			sessionID,
			now.Add(-workflow.config.RecentAuthenticationTTL),
		); recentErr != nil {
			return errors.Join(ErrRecentAuthenticationRequired, recentErr)
		}
		auditID, idErr := uuid.NewV7()
		if idErr != nil {
			return idErr
		}
		_, appendErr := workflow.audit.Append(txCtx, audit.Entry{
			ID:             auditID,
			ActorUserID:    &actorID,
			ActorSessionID: &sessionID,
			Action:         "operations.debug.request",
			Resource:       "process_log_level",
			ResourceID:     command.Component,
			Reason:         command.Reason,
			Before: map[string]any{
				"temporary": false,
			},
			After: map[string]any{
				"temporary":        true,
				"level":            command.Level,
				"duration_seconds": int64(command.Duration / time.Second),
				"idempotency_key":  command.IdempotencyKey.String(),
			},
			OccurredAt: now,
		})
		if appendErr != nil {
			return errors.Join(ErrAuditFailed, appendErr)
		}
		return nil
	})
	if err != nil {
		return commonmetrics.DebugLease{}, false, err
	}

	_, err = workflow.window.Activate(ctx, commonmetrics.DebugRequest{
		Key: command.IdempotencyKey.String(), Component: command.Component,
		Level: command.Level, Duration: command.Duration, Reason: command.Reason,
	})
	if err != nil {
		// Revoke is idempotent and also handles a partial LevelControl failure.
		return commonmetrics.DebugLease{}, false, errors.Join(err, workflow.window.Revoke(
			context.WithoutCancel(ctx),
			command.IdempotencyKey.String(),
		))
	}
	// Activate may have idempotently replayed a completed lease that has since
	// expired or been superseded. Project the effective current state instead
	// of inferring active=true from a successful replay.
	lease, active := workflow.window.Active()
	return lease, active, nil
}

// Active returns the current process-local lease after a fresh authorization
// decision. The reason is deliberately absent from DebugLease.
func (workflow *Workflow) Active(
	ctx context.Context,
) (commonmetrics.DebugLease, bool, error) {
	if workflow == nil || ctx == nil {
		return commonmetrics.DebugLease{}, false, ErrInvalidCommand
	}
	principal, ok := authctx.Principal(ctx)
	if !ok {
		return commonmetrics.DebugLease{}, false, ErrPermissionDenied
	}
	decision, err := workflow.authorizer.Authorize(ctx, authorization.Request{
		Principal: principal,
		Resource:  permissions.ResourceKeyObservability,
		Action:    permissions.PermissionScopeRead,
	})
	if err != nil {
		return commonmetrics.DebugLease{}, false, err
	}
	if !decision.Allowed {
		return commonmetrics.DebugLease{}, false, ErrPermissionDenied
	}
	lease, active := workflow.window.Active()
	return lease, active, nil
}

// Deactivate audits and revokes the current lease. Repeating a request after
// the lease is gone is a successful no-op and does not append duplicate audit.
func (workflow *Workflow) Deactivate(
	ctx context.Context,
	key uuid.UUID,
	reason string,
) (commonmetrics.DebugLease, bool, error) {
	if workflow == nil || ctx == nil || key == uuid.Nil ||
		len(strings.TrimSpace(reason)) == 0 ||
		len(reason) > maxReasonBytes {
		return commonmetrics.DebugLease{}, false, ErrInvalidCommand
	}
	workflow.controlMu.Lock()
	defer workflow.controlMu.Unlock()
	actorID, sessionID, err := workflow.authorizedActor(
		ctx,
		permissions.PermissionScopeUpdate,
	)
	if err != nil {
		return commonmetrics.DebugLease{}, false, err
	}
	lease, active := workflow.window.Active()
	if !active || lease.Key != key.String() {
		return commonmetrics.DebugLease{}, false, nil
	}
	now := workflow.now().UTC()
	err = workflow.unit.Within(ctx, func(txCtx context.Context) error {
		if recentErr := workflow.recent.RequireRecentReauthentication(
			txCtx,
			actorID,
			sessionID,
			now.Add(-workflow.config.RecentAuthenticationTTL),
		); recentErr != nil {
			return errors.Join(ErrRecentAuthenticationRequired, recentErr)
		}
		auditID, idErr := uuid.NewV7()
		if idErr != nil {
			return idErr
		}
		_, appendErr := workflow.audit.Append(txCtx, audit.Entry{
			ID:             auditID,
			ActorUserID:    &actorID,
			ActorSessionID: &sessionID,
			Action:         "operations.debug.revoke",
			Resource:       "process_log_level",
			ResourceID:     workflow.config.Component,
			Reason:         reason,
			Before: map[string]any{
				"temporary":  true,
				"level":      lease.Level,
				"expires_at": lease.ExpiresAt.UTC(),
			},
			After: map[string]any{
				"temporary": false,
			},
			OccurredAt: now,
		})
		if appendErr != nil {
			return errors.Join(ErrAuditFailed, appendErr)
		}
		return nil
	})
	if err != nil {
		return commonmetrics.DebugLease{}, false, err
	}
	if err := workflow.window.Revoke(ctx, key.String()); err != nil {
		return commonmetrics.DebugLease{}, false, err
	}
	return lease, true, nil
}

func (workflow *Workflow) authorizedActor(
	ctx context.Context,
	action string,
) (uuid.UUID, uuid.UUID, error) {
	principal, ok := authctx.Principal(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, ErrPermissionDenied
	}
	actorID, ok := authctx.UserID(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, ErrPermissionDenied
	}
	sessionID, ok := authctx.SessionID(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, ErrPermissionDenied
	}
	decision, err := workflow.authorizer.Authorize(ctx, authorization.Request{
		Principal: principal,
		Resource:  permissions.ResourceKeyObservability,
		Action:    action,
	})
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !decision.Allowed {
		return uuid.Nil, uuid.Nil, ErrPermissionDenied
	}
	return actorID, sessionID, nil
}

func validCommand(command Command, maximum time.Duration) bool {
	return command.IdempotencyKey != uuid.Nil &&
		command.Level == "debug" &&
		validToken(command.Component, maxComponentBytes) &&
		len(strings.TrimSpace(command.Reason)) > 0 &&
		len(command.Reason) <= maxReasonBytes &&
		command.Duration > 0 &&
		command.Duration <= maximum &&
		command.Duration%time.Second == 0
}

func validToken(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func (command Command) String() string {
	return fmt.Sprintf("debug %s for %s", command.Component, command.Duration)
}
