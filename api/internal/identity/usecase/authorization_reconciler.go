package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
	"go.uber.org/zap"
)

const authorizationReconcileInterval = 5 * time.Second

// Name implements lifecycle.Component. The service itself owns no goroutine;
// the shared lifecycle invokes Run and controls cancellation.
func (*authorizationService) Name() string { return "authorization-reconciler" }

func (service *authorizationService) Run(ctx context.Context) error {
	if service == nil || service.runtime == nil || service.repository == nil || service.txManager == nil {
		return fmt.Errorf("authorization reconciler: invalid dependencies")
	}
	if ctx == nil {
		return fmt.Errorf("authorization reconciler: nil context")
	}
	if !service.reconcilerRunning.CompareAndSwap(false, true) {
		return fmt.Errorf("authorization reconciler: already running")
	}
	defer service.reconcilerRunning.Store(false)
	ctx = commonlogger.WithContext(ctx, service.log)

	ticker := time.NewTicker(authorizationReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-service.reconcilerStop:
			return nil
		case <-ticker.C:
			reconcileCtx, cancel := context.WithTimeout(
				correlation.EnsureContext(ctx),
				authorizationReconcileInterval/2,
			)
			err := service.ReconcileNow(reconcileCtx)
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				commonlogger.FromContext(reconcileCtx).Warn(
					"authorization reconciliation failed",
					zap.Int64("authorization_desired_revision", service.runtime.DesiredRevision()),
					zap.Int64("authorization_applied_revision", service.runtime.AppliedRevision()),
					zap.Error(err),
				)
			}
		}
	}
}

func (service *authorizationService) Shutdown(context.Context) error {
	if service == nil {
		return nil
	}
	service.reconcilerStopOnce.Do(func() { close(service.reconcilerStop) })
	return nil
}

// ReconcileNow repairs a committed desired/applied gap after a process crash
// or failed post-commit publication. The singleton state row is locked while
// reading so an in-flight writer cannot be mistaken for committed state.
func (service *authorizationService) ReconcileNow(ctx context.Context) error {
	if service == nil || service.runtime == nil || ctx == nil {
		return fmt.Errorf("authorization reconciler: invalid dependencies")
	}
	// A reconciler is also a runtime writer. Serialize it with policy mutations
	// through their physical transaction completion so it never observes or
	// publishes an in-flight desired-revision fence.
	service.mutationGate.Lock()
	defer service.mutationGate.Unlock()
	ctx = correlation.EnsureContext(ctx)
	ctx = commonlogger.WithContext(ctx, service.log)
	var snapshot authcasbin.Snapshot
	err := service.txManager.DoInTx(ctx, func(txCtx context.Context) error {
		state, err := service.repository.LockState(txCtx)
		if err != nil {
			return fmt.Errorf("lock authorization state: %w", err)
		}
		applied := service.runtime.AppliedRevision()
		if state.DesiredRevision < applied {
			service.runtime.SetDesiredRevision(state.DesiredRevision)
			return fmt.Errorf(
				"authorization persisted revision %d is behind applied revision %d",
				state.DesiredRevision,
				applied,
			)
		}
		service.runtime.SetDesiredRevision(state.DesiredRevision)
		if state.DesiredRevision == applied {
			return nil
		}
		rules, err := service.repository.ListRules(txCtx)
		if err != nil {
			return fmt.Errorf("load authorization reconciliation snapshot: %w", err)
		}
		snapshot = authcasbin.Snapshot{
			Revision: state.DesiredRevision,
			Model:    casbinRBACModel,
			Rules:    cloneAuthorizationRules(rules),
		}
		return validateAuthorizationSnapshot(txCtx, service.repository, snapshot, nil)
	})
	if err != nil || snapshot.Revision == 0 {
		return err
	}
	prepared, err := service.runtime.Prepare(ctx, snapshot)
	if err != nil {
		if errors.Is(err, authcasbin.ErrStaleRevision) &&
			service.runtime.AppliedRevision() >= snapshot.Revision {
			return nil
		}
		return fmt.Errorf("prepare reconciled authorization snapshot: %w", err)
	}
	if err := service.runtime.Publish(prepared); err != nil {
		if errors.Is(err, authcasbin.ErrStaleRevision) &&
			service.runtime.AppliedRevision() >= snapshot.Revision {
			return nil
		}
		return fmt.Errorf("publish reconciled authorization snapshot: %w", err)
	}
	service.storeViewIfNewer(newAuthorizationView(snapshot.Revision, snapshot.Rules))
	commonlogger.FromContext(ctx).Info(
		"authorization snapshot reconciled",
		zap.Int64("authorization_revision", snapshot.Revision),
	)
	return nil
}

// Compile-time fields kept beside lifecycle methods to make ownership clear.
// They are embedded in authorizationService below through this helper type's
// explicit declaration in authorization.go.
type authorizationReconcilerState struct {
	reconcilerRunning  atomic.Bool
	reconcilerStop     chan struct{}
	reconcilerStopOnce sync.Once
}
