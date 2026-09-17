package usecase

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
	"go.uber.org/zap"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

func TestAuthorizationMutationPublishesOneAuditedRevision(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	before := fixture.authorize(t, fixture.adminPrincipal, permissions.ResourceKeyRole, permissions.PermissionScopeRead)
	if !before.Allowed || before.Revision != 1 {
		t.Fatalf("initial decision = %#v", before)
	}

	snapshot, err := fixture.service.SetRolePolicies(
		fixture.adminContext,
		"teacher",
		1,
		"Teachers now manage role metadata",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeUpdate}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != 2 || len(snapshot.Rules) != 1 ||
		snapshot.Rules[0].Action != permissions.PermissionScopeUpdate {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if fixture.repository.state.DesiredRevision != 2 || fixture.service.Healthy() != nil {
		t.Fatalf("revision state = %#v, health = %v", fixture.repository.state, fixture.service.Healthy())
	}
	if len(fixture.audit.entries) != 1 || fixture.audit.entries[0].Reason == "" {
		t.Fatalf("audit entries = %#v", fixture.audit.entries)
	}

	teacher := fixture.teacherPrincipal
	if decision := fixture.authorize(t, teacher, permissions.ResourceKeyRole, permissions.PermissionScopeRead); decision.Allowed {
		t.Fatalf("old teacher grant remained active: %#v", decision)
	}
	if decision := fixture.authorize(t, teacher, permissions.ResourceKeyRole, permissions.PermissionScopeUpdate); !decision.Allowed || decision.Revision != 2 {
		t.Fatalf("new teacher grant not active: %#v", decision)
	}
}

func TestAuthorizationMutationRetainsSuspendedUserRoleReferences(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)
	fixture.repository.activeUsers[fixture.teacherID] = false
	fixture.repository.eligibleUsers = map[uuid.UUID]bool{
		fixture.adminID:   true,
		fixture.teacherID: true,
		fixture.managerID: true,
	}

	_, err := fixture.service.SetRolePolicies(
		fixture.adminContext,
		"teacher",
		1,
		"Keep suspended teacher assignments durable",
		[]dto.PolicyRule{{
			Resource: permissions.ResourceKeyRole,
			Action:   permissions.PermissionScopeRead,
		}},
	)
	if err != nil {
		t.Fatalf("mutation rejected authorization-eligible suspended user: %v", err)
	}
}

func TestAuthorizationStartupNeverInitializesMissingPolicyState(t *testing.T) {
	t.Parallel()
	adminID := uuid.New()
	repository := &authorizationMemoryRepository{
		rules: []authcasbin.PolicyRule{
			{Type: "g", V0: userSubject(adminID.String()), V1: roleSubject(protectedSuperadminRoleKey)},
			{Type: "p", V0: roleSubject(protectedSuperadminRoleKey), V1: "*", V2: "*"},
		},
		activeRoles: map[string]bool{protectedSuperadminRoleKey: true},
		activeUsers: map[uuid.UUID]bool{adminID: true},
	}
	manager := &authorizationTestTransaction{repository: repository}
	service, err := NewAuthorizationService(
		context.Background(),
		repository,
		manager,
		&authorizationAuditAppender{},
		&authorizationOutboxAppender{},
		zap.NewNop(),
	)
	if err == nil || service != nil {
		t.Fatalf("startup without bootstrap = service %#v, error %v", service, err)
	}
	if repository.ensureCalls != 0 || repository.state.DesiredRevision != 0 {
		t.Fatalf("startup mutated authorization state: calls=%d state=%#v", repository.ensureCalls, repository.state)
	}
}

func TestAuthorizationMutationRejectsStaleRevisionWithoutOverwrite(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	_, err := fixture.service.SetRolePolicies(
		fixture.adminContext,
		"teacher",
		1,
		"First administrator update",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeRead}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.service.SetRolePolicies(
		fixture.adminContext,
		"teacher",
		1,
		"Stale administrator update",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeDelete}},
	)
	var appError *apperr.AppError
	if !errors.As(err, &appError) || appError.Code != apperr.CodeConflict {
		t.Fatalf("stale mutation error = %v", err)
	}
	if fixture.repository.state.DesiredRevision != 2 || len(fixture.audit.entries) != 1 {
		t.Fatalf("stale mutation changed persistence: revision=%d audit=%d", fixture.repository.state.DesiredRevision, len(fixture.audit.entries))
	}
	snapshot, err := fixture.service.GetRolePolicies(context.Background(), "teacher")
	if err != nil || snapshot.Revision != 2 || snapshot.Rules[0].Action != permissions.PermissionScopeRead {
		t.Fatalf("persisted snapshot = %#v, %v", snapshot, err)
	}
}

func TestAuthorizationProtectsRootRoleAndFinalActiveSuperadmin(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	_, err := fixture.service.SetRolePolicies(
		fixture.adminContext,
		protectedSuperadminRoleKey,
		1,
		"Attempt to alter protected role",
		nil,
	)
	assertConflict(t, err)
	if err := fixture.service.DeleteRole(fixture.adminContext, protectedSuperadminRoleKey); err == nil {
		t.Fatal("protected role deletion succeeded")
	}

	_, err = fixture.service.SetUserRolesAtRevision(
		fixture.adminContext,
		fixture.adminID,
		1,
		"Remove the final administrator",
		[]string{"teacher"},
	)
	assertConflict(t, err)
	if fixture.repository.state.DesiredRevision != 1 || fixture.service.Healthy() != nil {
		t.Fatalf("rejected mutation changed fence: %#v, health=%v", fixture.repository.state, fixture.service.Healthy())
	}
	roles, err := fixture.service.GetUserRoles(context.Background(), fixture.adminID)
	if err != nil || !slices.Contains(roles, protectedSuperadminRoleKey) {
		t.Fatalf("root membership after rejection = %#v, %v", roles, err)
	}
}

func TestAuthorizationRollbackRestoresDesiredFence(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)
	fixture.audit.failure = errors.New("audit unavailable")

	_, err := fixture.service.SetRolePolicies(
		fixture.adminContext,
		"teacher",
		1,
		"This transaction must roll back",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeDelete}},
	)
	if err == nil {
		t.Fatal("mutation with failed audit committed")
	}
	if fixture.repository.state.DesiredRevision != 1 || fixture.service.runtime.DesiredRevision() != 1 || fixture.service.Healthy() != nil {
		t.Fatalf("rollback fence state: db=%d desired=%d health=%v", fixture.repository.state.DesiredRevision, fixture.service.runtime.DesiredRevision(), fixture.service.Healthy())
	}
	decision := fixture.authorize(t, fixture.teacherPrincipal, permissions.ResourceKeyRole, permissions.PermissionScopeRead)
	if !decision.Allowed || decision.Revision != 1 {
		t.Fatalf("rollback changed live policy: %#v", decision)
	}
}

func TestAuthorizationWriterGateSpansPhysicalRollbackCompletion(t *testing.T) {
	t.Parallel()
	fixture := newDeferredAuthorizationFixture(t)
	rollbackErr := errors.New("outer transaction rejected")

	aDone := make(chan error, 1)
	go func() {
		aDone <- fixture.manager.DoInTx(
			withAuthorizationTransactionName(context.Background(), "a"),
			func(txCtx context.Context) error {
				_, err := fixture.service.SetRolePolicies(
					authentication.WithPrincipal(txCtx, fixture.adminPrincipal),
					"teacher",
					1,
					"Mutation rolled back by its outer unit of work",
					[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeUpdate}},
				)
				if err != nil {
					return err
				}
				return rollbackErr
			},
		)
	}()
	<-fixture.manager.atCompletion["a"]

	bDone := make(chan error, 1)
	bAttempted := make(chan struct{})
	go func() {
		close(bAttempted)
		_, err := fixture.service.SetRolePolicies(
			authentication.WithPrincipal(
				withAuthorizationTransactionName(context.Background(), "b"),
				fixture.adminPrincipal,
			),
			"teacher",
			1,
			"Mutation committed after the rollback",
			[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeDelete}},
		)
		bDone <- err
	}()
	<-bAttempted

	select {
	case <-fixture.manager.atCompletion["b"]:
		t.Fatal("writer B entered its transaction before writer A completed rollback publication")
	case <-time.After(50 * time.Millisecond):
	}

	close(fixture.manager.releaseCompletion["a"])
	if err := <-aDone; !errors.Is(err, rollbackErr) {
		t.Fatalf("writer A error = %v", err)
	}
	<-fixture.manager.atCompletion["b"]
	if fixture.service.runtime.DesiredRevision() != 2 || fixture.service.runtime.AppliedRevision() != 1 {
		t.Fatalf(
			"writer B in-flight fence: desired=%d applied=%d",
			fixture.service.runtime.DesiredRevision(),
			fixture.service.runtime.AppliedRevision(),
		)
	}
	close(fixture.manager.releaseCompletion["b"])
	if err := <-bDone; err != nil {
		t.Fatal(err)
	}
	if fixture.service.runtime.DesiredRevision() != 2 ||
		fixture.service.runtime.AppliedRevision() != 2 ||
		fixture.service.Healthy() != nil {
		t.Fatalf(
			"final fence: desired=%d applied=%d health=%v",
			fixture.service.runtime.DesiredRevision(),
			fixture.service.runtime.AppliedRevision(),
			fixture.service.Healthy(),
		)
	}
}

func TestAuthorizationReconcilerWaitsForWriterCompletion(t *testing.T) {
	t.Parallel()
	fixture := newDeferredAuthorizationFixture(t)
	rollbackErr := errors.New("outer transaction rejected")

	aDone := make(chan error, 1)
	go func() {
		aDone <- fixture.manager.DoInTx(
			withAuthorizationTransactionName(context.Background(), "a"),
			func(txCtx context.Context) error {
				_, err := fixture.service.SetRolePolicies(
					authentication.WithPrincipal(txCtx, fixture.adminPrincipal),
					"teacher",
					1,
					"Mutation rolled back before reconciliation",
					[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeUpdate}},
				)
				if err != nil {
					return err
				}
				return rollbackErr
			},
		)
	}()
	<-fixture.manager.atCompletion["a"]

	reconcileDone := make(chan error, 1)
	reconcileAttempted := make(chan struct{})
	go func() {
		close(reconcileAttempted)
		reconcileDone <- fixture.service.ReconcileNow(
			withAuthorizationTransactionName(context.Background(), "b"),
		)
	}()
	<-reconcileAttempted
	select {
	case <-fixture.manager.atCompletion["b"]:
		t.Fatal("reconciler entered persistence while a writer awaited rollback completion")
	case <-time.After(50 * time.Millisecond):
	}

	close(fixture.manager.releaseCompletion["a"])
	if err := <-aDone; !errors.Is(err, rollbackErr) {
		t.Fatalf("writer error = %v", err)
	}
	<-fixture.manager.atCompletion["b"]
	close(fixture.manager.releaseCompletion["b"])
	if err := <-reconcileDone; err != nil {
		t.Fatal(err)
	}
	if fixture.service.Healthy() != nil {
		t.Fatalf("runtime unhealthy after serialized reconciliation: %v", fixture.service.Healthy())
	}
}

func TestAuthorizationReconcilesCommittedRevisionGap(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)
	fixture.repository.mu.Lock()
	fixture.repository.state.DesiredRevision = 2
	for index := range fixture.repository.rules {
		if fixture.repository.rules[index].Type == "p" && fixture.repository.rules[index].V0 == roleSubject("teacher") {
			fixture.repository.rules[index].V2 = permissions.PermissionScopeUpdate
		}
	}
	fixture.repository.mu.Unlock()
	fixture.service.runtime.SetDesiredRevision(2)
	if fixture.service.Healthy() == nil {
		t.Fatal("revision gap reported healthy")
	}
	if err := fixture.service.ReconcileNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fixture.service.Healthy() != nil {
		t.Fatalf("reconciled runtime unhealthy: %v", fixture.service.Healthy())
	}
	decision := fixture.authorize(t, fixture.teacherPrincipal, permissions.ResourceKeyRole, permissions.PermissionScopeUpdate)
	if !decision.Allowed || decision.Revision != 2 {
		t.Fatalf("reconciled decision = %#v", decision)
	}
}

func TestAuthorizationConcurrentWritersUseRevisionCAS(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, action := range []string{permissions.PermissionScopeUpdate, permissions.PermissionScopeDelete} {
		action := action
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := fixture.service.SetRolePolicies(
				fixture.adminContext,
				"teacher",
				1,
				"Concurrent administrator update",
				[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: action}},
			)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		var appError *apperr.AppError
		if errors.As(err, &appError) && appError.Code == apperr.CodeConflict {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent error: %v", err)
	}
	if successes != 1 || conflicts != 1 || fixture.repository.state.DesiredRevision != 2 {
		t.Fatalf("success=%d conflict=%d revision=%d", successes, conflicts, fixture.repository.state.DesiredRevision)
	}
}

func TestAuthorizationAdminMutationNeverDowngradesMissingActorToSystem(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)
	mutation := func(ctx context.Context) error {
		_, err := fixture.service.SetRolePolicies(
			ctx,
			"teacher",
			1,
			"Administrator update",
			[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeUpdate}},
		)
		return err
	}
	for _, ctx := range []context.Context{
		context.Background(),
		authentication.WithPrincipal(context.Background(), func() authentication.Principal {
			principal := fixture.adminPrincipal
			principal.AuthenticatedUntil = time.Now().Add(-time.Second)
			return principal
		}()),
	} {
		var appError *apperr.AppError
		if err := mutation(ctx); !errors.As(err, &appError) || appError.Code != apperr.CodeUnauthorized {
			t.Fatalf("mutation actor error = %v", err)
		}
	}
	if fixture.repository.state.DesiredRevision != 1 || len(fixture.audit.entries) != 0 {
		t.Fatalf("invalid actor mutated state: revision=%d audit=%d", fixture.repository.state.DesiredRevision, len(fixture.audit.entries))
	}
}

func TestAuthorizationValidationRejectsUnknownCapabilitiesAndCycles(t *testing.T) {
	t.Parallel()
	if _, err := normalizeRolePolicies("teacher", []dto.PolicyRule{{Resource: "invented", Action: "read"}}); !errors.Is(err, ErrInvalidAuthorizationMutation) {
		t.Fatalf("unknown capability error = %v", err)
	}
	if _, err := normalizeRolePolicies("teacher", []dto.PolicyRule{{Resource: "*", Action: "*"}}); !errors.Is(err, ErrInvalidAuthorizationMutation) {
		t.Fatalf("unprotected wildcard error = %v", err)
	}
	if !hasRoleCycle(map[string][]string{"teacher": {"student"}, "student": {"teacher"}}) {
		t.Fatal("role cycle was accepted")
	}
}

func TestAuthorizationRoleHierarchyMatchesCasbinDepth(t *testing.T) {
	t.Parallel()

	within := make(map[string][]string)
	for depth := 0; depth < authcasbin.DefaultMaxRoleHierarchyDepth-1; depth++ {
		within[fmt.Sprintf("role-%d", depth)] = []string{
			fmt.Sprintf("role-%d", depth+1),
		}
	}
	if roleGraphExceedsMaxParentEdges(
		within,
		authcasbin.DefaultMaxRoleHierarchyDepth-1,
	) {
		t.Fatal("role hierarchy at Casbin boundary was rejected")
	}

	beyond := maps.Clone(within)
	beyond[fmt.Sprintf("role-%d", authcasbin.DefaultMaxRoleHierarchyDepth-1)] =
		[]string{fmt.Sprintf("role-%d", authcasbin.DefaultMaxRoleHierarchyDepth)}
	if !roleGraphExceedsMaxParentEdges(
		beyond,
		authcasbin.DefaultMaxRoleHierarchyDepth-1,
	) {
		t.Fatal("role hierarchy beyond Casbin boundary was accepted")
	}
}

func TestAuthorizationDelegatedManagerCannotGrantCapabilitiesItDoesNotHold(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	_, err := fixture.service.SetRolePolicies(
		fixture.managerContext,
		"teacher",
		1,
		"Attempt to grant a stronger problem capability",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyProblem, Action: permissions.PermissionScopeUpdate}},
	)
	var appError *apperr.AppError
	if !errors.As(err, &appError) || appError.Code != apperr.CodeForbidden {
		t.Fatalf("escalation error = %v, want forbidden", err)
	}
	if fixture.repository.state.DesiredRevision != 1 || len(fixture.audit.entries) != 0 {
		t.Fatalf("escalation mutated state: revision=%d audit=%d", fixture.repository.state.DesiredRevision, len(fixture.audit.entries))
	}
}

func TestAuthorizationDelegatedManagerCanGrantCapabilitiesItAlreadyHolds(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	snapshot, err := fixture.service.SetRolePolicies(
		fixture.managerContext,
		"teacher",
		1,
		"Delegate existing problem read capability",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyProblem, Action: permissions.PermissionScopeRead}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != 2 || len(snapshot.Rules) != 1 || snapshot.Rules[0].Resource != permissions.ResourceKeyProblem {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestAuthorizationRequiresManageCapabilityAtServiceBoundary(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	_, err := fixture.service.SetRolePolicies(
		authentication.WithPrincipal(context.Background(), fixture.teacherPrincipal),
		"student",
		1,
		"Attempt without authorization manage",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyRole, Action: permissions.PermissionScopeRead}},
	)
	var appError *apperr.AppError
	if !errors.As(err, &appError) || appError.Code != apperr.CodeForbidden {
		t.Fatalf("service-boundary error = %v, want forbidden", err)
	}
}

func TestAuthorizationDelegatedManagerCannotChangeSuperadminMembership(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	for _, target := range []uuid.UUID{fixture.managerID, fixture.teacherID} {
		_, err := fixture.service.SetUserRolesAtRevision(
			fixture.managerContext,
			target,
			1,
			"Attempt to grant protected membership",
			[]string{protectedSuperadminRoleKey},
		)
		var appError *apperr.AppError
		if !errors.As(err, &appError) || appError.Code != apperr.CodeForbidden {
			t.Fatalf("target %s error = %v, want forbidden", target, err)
		}
	}
	if fixture.repository.state.DesiredRevision != 1 || len(fixture.audit.entries) != 0 {
		t.Fatalf("protected membership mutated state: revision=%d audit=%d", fixture.repository.state.DesiredRevision, len(fixture.audit.entries))
	}
}

func TestAuthorizationDirectSuperadminCanGrantAndRevokeNonFinalMembership(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	granted, err := fixture.service.SetUserRolesAtRevision(
		fixture.adminContext,
		fixture.teacherID,
		1,
		"Grant break glass membership",
		[]string{protectedSuperadminRoleKey},
	)
	if err != nil {
		t.Fatal(err)
	}
	if granted.Revision != 2 || !slices.Contains(granted.Roles, protectedSuperadminRoleKey) {
		t.Fatalf("granted snapshot = %#v", granted)
	}

	revoked, err := fixture.service.SetUserRolesAtRevision(
		fixture.adminContext,
		fixture.teacherID,
		2,
		"Return account to teacher role",
		[]string{"teacher"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Revision != 3 || slices.Contains(revoked.Roles, protectedSuperadminRoleKey) {
		t.Fatalf("revoked snapshot = %#v", revoked)
	}
}

func TestAuthorizationSystemPathOnlyInitializesDefaultRole(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)
	newUserID := uuid.New()
	fixture.repository.activeUsers[newUserID] = true

	if err := fixture.service.SetUserRoles(context.Background(), newUserID, []string{defaultRoleKey}); err != nil {
		t.Fatalf("default initialization failed: %v", err)
	}
	if err := fixture.service.SetUserRoles(context.Background(), newUserID, []string{"teacher"}); err == nil {
		t.Fatal("system path replaced an existing assignment")
	} else {
		var appError *apperr.AppError
		if !errors.As(err, &appError) || appError.Code != apperr.CodeForbidden {
			t.Fatalf("replacement error = %v, want forbidden", err)
		}
	}

	privilegedUserID := uuid.New()
	fixture.repository.activeUsers[privilegedUserID] = true
	if err := fixture.service.SetUserRoles(context.Background(), privilegedUserID, []string{"manager"}); err == nil {
		t.Fatal("system path assigned a non-default role")
	} else {
		var appError *apperr.AppError
		if !errors.As(err, &appError) || appError.Code != apperr.CodeForbidden {
			t.Fatalf("privileged assignment error = %v, want forbidden", err)
		}
	}
}

func TestAuthorizationRejectsDelegatedManagerSelfLockout(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	_, err := fixture.service.SetRolePolicies(
		fixture.managerContext,
		"manager",
		1,
		"Attempt to remove own authorization manage",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyProblem, Action: permissions.PermissionScopeRead}},
	)
	assertConflict(t, err)
	if fixture.repository.state.DesiredRevision != 1 {
		t.Fatalf("self-lockout changed revision to %d", fixture.repository.state.DesiredRevision)
	}
}

func TestAuthorizationRejectsWhitespaceProtectedRoleAlias(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	_, err := fixture.service.SetRolePolicies(
		fixture.adminContext,
		" "+protectedSuperadminRoleKey+" ",
		1,
		"Attempt whitespace alias",
		nil,
	)
	var appError *apperr.AppError
	if !errors.As(err, &appError) || appError.Code != apperr.CodeBadRequest {
		t.Fatalf("whitespace alias error = %v, want bad request", err)
	}
}

func TestAuthorizationPolicyAuditDocumentIsJSONShaped(t *testing.T) {
	t.Parallel()
	document := policyAuditDocument(4, []dto.PolicyRule{{
		RoleKey:  "teacher",
		Resource: permissions.ResourceKeyProblem,
		Action:   permissions.PermissionScopeUpdate,
	}})
	rules, ok := document["rules"].([]map[string]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("audit rules = %#v", document["rules"])
	}
	if rules[0]["resource"] != permissions.ResourceKeyProblem ||
		rules[0]["action"] != permissions.PermissionScopeUpdate {
		t.Fatalf("audit rule = %#v", rules[0])
	}
	if _, leaked := rules[0]["role_key"]; leaked {
		t.Fatal("redundant role key leaked into bounded audit payload")
	}
}

func TestAuthorizationViewIsSharedAndSnapshotsCloneOnlyIndexedResults(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	first, err := fixture.service.currentView(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.currentView(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("currentView cloned the complete policy snapshot")
	}

	roleSnapshot := rolePolicySnapshot(first, "teacher")
	roleSnapshot.Rules[0].Resource = "mutated"
	userSnapshot := userRoleSnapshot(first, fixture.teacherID)
	userSnapshot.Roles[0] = "mutated"

	roleAgain := rolePolicySnapshot(first, "teacher")
	userAgain := userRoleSnapshot(first, fixture.teacherID)
	if roleAgain.Rules[0].Resource == "mutated" || userAgain.Roles[0] == "mutated" {
		t.Fatal("indexed authorization view leaked mutable storage")
	}
}

func TestEffectiveCapabilitiesExpandWildcardWithoutPolicyLeakage(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	snapshot, err := fixture.service.EffectiveCapabilities(context.Background(), fixture.adminID)
	if err != nil {
		t.Fatal(err)
	}
	catalog := permissions.Catalog()
	if snapshot.Revision != 1 || len(snapshot.Capabilities) != len(catalog) {
		t.Fatalf("effective snapshot = %#v, catalog size = %d", snapshot, len(catalog))
	}
	for index, capability := range snapshot.Capabilities {
		if capability.Resource != catalog[index].Key ||
			!slices.Equal(capability.Actions, catalog[index].Actions) {
			t.Fatalf("capability[%d] = %#v, catalog = %#v", index, capability, catalog[index])
		}
		if capability.Resource == "*" || slices.Contains(capability.Actions, "*") {
			t.Fatalf("wildcard leaked in capability %#v", capability)
		}
	}
}

func TestEffectiveCapabilitiesUnionRolesAndFilterUnknownPolicies(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	view := newAuthorizationView(7, []authcasbin.PolicyRule{
		{Type: "g", V0: userSubject(userID.String()), V1: roleSubject("reader")},
		{Type: "g", V0: userSubject(userID.String()), V1: roleSubject("editor")},
		{Type: "p", V0: roleSubject("reader"), V1: permissions.ResourceKeyProblem, V2: permissions.PermissionScopeRead},
		{Type: "p", V0: roleSubject("editor"), V1: permissions.ResourceKeyProblem, V2: permissions.PermissionScopeRead},
		{Type: "p", V0: roleSubject("editor"), V1: permissions.ResourceKeyProblem, V2: permissions.PermissionScopeUpdate},
		{Type: "p", V0: roleSubject("editor"), V1: "internal_runtime", V2: "restart"},
	})

	snapshot := effectiveCapabilitiesSnapshot(view, userID)
	if snapshot.Revision != 7 || len(snapshot.Capabilities) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	want := dto.CapabilityGrant{
		Resource: permissions.ResourceKeyProblem,
		Actions:  []string{permissions.PermissionScopeRead, permissions.PermissionScopeUpdate},
	}
	if snapshot.Capabilities[0].Resource != want.Resource ||
		!slices.Equal(snapshot.Capabilities[0].Actions, want.Actions) {
		t.Fatalf("capability = %#v, want %#v", snapshot.Capabilities[0], want)
	}
	snapshot.Capabilities[0].Actions[0] = "mutated"
	again := effectiveCapabilitiesSnapshot(view, userID)
	if again.Capabilities[0].Actions[0] == "mutated" {
		t.Fatal("effective capability response leaked immutable view storage")
	}
}

func TestEffectiveCapabilitiesIncludeInheritedRoleGrants(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	view := newAuthorizationView(8, []authcasbin.PolicyRule{
		{Type: "g", V0: userSubject(userID.String()), V1: roleSubject("junior")},
		{Type: "g", V0: roleSubject("junior"), V1: roleSubject("editor")},
		{Type: "g", V0: roleSubject("editor"), V1: roleSubject("reader")},
		{Type: "p", V0: roleSubject("junior"), V1: permissions.ResourceKeyProblem, V2: permissions.PermissionScopeRead},
		{Type: "p", V0: roleSubject("editor"), V1: permissions.ResourceKeyProblem, V2: permissions.PermissionScopeUpdate},
		{Type: "p", V0: roleSubject("reader"), V1: permissions.ResourceKeyTag, V2: permissions.PermissionScopeRead},
	})

	snapshot := effectiveCapabilitiesSnapshot(view, userID)
	want := []dto.CapabilityGrant{
		{
			Resource: permissions.ResourceKeyProblem,
			Actions: []string{
				permissions.PermissionScopeRead,
				permissions.PermissionScopeUpdate,
			},
		},
		{
			Resource: permissions.ResourceKeyTag,
			Actions:  []string{permissions.PermissionScopeRead},
		},
	}
	if snapshot.Revision != 8 || !reflect.DeepEqual(snapshot.Capabilities, want) {
		t.Fatalf("snapshot = %#v, want revision 8 capabilities %#v", snapshot, want)
	}
}

func TestEffectiveCapabilitiesSwapWithAuthorizationRevision(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)

	before, err := fixture.service.EffectiveCapabilities(context.Background(), fixture.teacherID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Revision != 1 || len(before.Capabilities) != 1 ||
		!slices.Equal(before.Capabilities[0].Actions, []string{permissions.PermissionScopeRead}) {
		t.Fatalf("before = %#v", before)
	}

	_, err = fixture.service.SetRolePolicies(
		fixture.adminContext,
		"teacher",
		1,
		"Teacher capability changed",
		[]dto.PolicyRule{{Resource: permissions.ResourceKeyProblem, Action: permissions.PermissionScopeUpdate}},
	)
	if err != nil {
		t.Fatal(err)
	}
	after, err := fixture.service.EffectiveCapabilities(context.Background(), fixture.teacherID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != 2 || len(after.Capabilities) != 1 ||
		after.Capabilities[0].Resource != permissions.ResourceKeyProblem ||
		!slices.Equal(after.Capabilities[0].Actions, []string{permissions.PermissionScopeUpdate}) {
		t.Fatalf("after = %#v", after)
	}
}

func TestEffectiveCapabilitiesFailClosedForUnavailableView(t *testing.T) {
	t.Parallel()
	fixture := newAuthorizationFixture(t)
	fixture.service.view.Store(newAuthorizationView(99, nil))

	if _, err := fixture.service.EffectiveCapabilities(context.Background(), fixture.teacherID); err == nil {
		t.Fatal("revision-mismatched capability view was served")
	}
}

type authorizationFixture struct {
	service          *authorizationService
	repository       *authorizationMemoryRepository
	audit            *authorizationAuditAppender
	adminID          uuid.UUID
	teacherID        uuid.UUID
	managerID        uuid.UUID
	adminPrincipal   authentication.Principal
	teacherPrincipal authentication.Principal
	managerPrincipal authentication.Principal
	adminContext     context.Context
	managerContext   context.Context
}

type deferredAuthorizationFixture struct {
	service        *authorizationService
	manager        *deferredAuthorizationTransaction
	adminPrincipal authentication.Principal
}

func newAuthorizationFixture(t *testing.T) *authorizationFixture {
	t.Helper()
	adminID := uuid.New()
	teacherID := uuid.New()
	managerID := uuid.New()
	repository := &authorizationMemoryRepository{
		state: ports.AuthorizationState{DesiredRevision: 1},
		rules: []authcasbin.PolicyRule{
			{Type: "g", V0: userSubject(adminID.String()), V1: roleSubject(protectedSuperadminRoleKey)},
			{Type: "g", V0: userSubject(teacherID.String()), V1: roleSubject("teacher")},
			{Type: "g", V0: userSubject(managerID.String()), V1: roleSubject("manager")},
			{Type: "p", V0: roleSubject(protectedSuperadminRoleKey), V1: "*", V2: "*"},
			{Type: "p", V0: roleSubject("teacher"), V1: permissions.ResourceKeyRole, V2: permissions.PermissionScopeRead},
			{Type: "p", V0: roleSubject("manager"), V1: permissions.ResourceKeyAuthorization, V2: permissions.PermissionScopeManage},
			{Type: "p", V0: roleSubject("manager"), V1: permissions.ResourceKeyProblem, V2: permissions.PermissionScopeRead},
		},
		activeRoles: map[string]bool{
			protectedSuperadminRoleKey: true,
			"teacher":                  true,
			"student":                  true,
			"manager":                  true,
		},
		activeUsers: map[uuid.UUID]bool{adminID: true, teacherID: true, managerID: true},
	}
	manager := &authorizationTestTransaction{repository: repository}
	auditAppender := &authorizationAuditAppender{}
	service, err := NewAuthorizationService(context.Background(), repository, manager, auditAppender, &authorizationOutboxAppender{}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	adminPrincipal := authorizationPrincipal(adminID)
	managerPrincipal := authorizationPrincipal(managerID)
	return &authorizationFixture{
		service:          service,
		repository:       repository,
		audit:            auditAppender,
		adminID:          adminID,
		teacherID:        teacherID,
		managerID:        managerID,
		adminPrincipal:   adminPrincipal,
		teacherPrincipal: authorizationPrincipal(teacherID),
		managerPrincipal: managerPrincipal,
		adminContext:     authentication.WithPrincipal(context.Background(), adminPrincipal),
		managerContext:   authentication.WithPrincipal(context.Background(), managerPrincipal),
	}
}

func newDeferredAuthorizationFixture(t *testing.T) *deferredAuthorizationFixture {
	t.Helper()
	adminID := uuid.New()
	teacherID := uuid.New()
	repository := &authorizationMemoryRepository{
		state: ports.AuthorizationState{DesiredRevision: 1},
		rules: []authcasbin.PolicyRule{
			{Type: "g", V0: userSubject(adminID.String()), V1: roleSubject(protectedSuperadminRoleKey)},
			{Type: "g", V0: userSubject(teacherID.String()), V1: roleSubject("teacher")},
			{Type: "p", V0: roleSubject(protectedSuperadminRoleKey), V1: "*", V2: "*"},
			{Type: "p", V0: roleSubject("teacher"), V1: permissions.ResourceKeyRole, V2: permissions.PermissionScopeRead},
		},
		activeRoles: map[string]bool{
			protectedSuperadminRoleKey: true,
			"teacher":                  true,
		},
		activeUsers: map[uuid.UUID]bool{adminID: true, teacherID: true},
	}
	manager := newDeferredAuthorizationTransaction(repository, "a", "b")
	service, err := NewAuthorizationService(
		context.Background(),
		repository,
		manager,
		&authorizationAuditAppender{},
		&authorizationOutboxAppender{},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return &deferredAuthorizationFixture{
		service:        service,
		manager:        manager,
		adminPrincipal: authorizationPrincipal(adminID),
	}
}

func authorizationPrincipal(userID uuid.UUID) authentication.Principal {
	now := time.Now().UTC()
	return authentication.Principal{
		Authenticated:      true,
		Subject:            userID.String(),
		UserID:             userID.String(),
		SessionID:          uuid.NewString(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    now.Add(-time.Minute),
		SecurityVersion:    1,
		AuthenticatedUntil: now.Add(time.Hour),
	}
}

func (fixture *authorizationFixture) authorize(
	t *testing.T,
	principal authentication.Principal,
	resource string,
	action string,
) authorization.Decision {
	t.Helper()
	decision, err := fixture.service.Authorize(context.Background(), authorization.Request{
		Principal: principal,
		Resource:  resource,
		Action:    action,
	})
	if err != nil {
		t.Fatal(err)
	}
	return decision
}

func assertConflict(t *testing.T, err error) {
	t.Helper()
	var appError *apperr.AppError
	if !errors.As(err, &appError) || appError.Code != apperr.CodeConflict {
		t.Fatalf("error = %v, want conflict", err)
	}
}

type authorizationTestTransaction struct {
	transactionMu sync.Mutex
	repository    *authorizationMemoryRepository
	callbacks     []func(context.Context, bool) error
}

type authorizationTransactionNameKey struct{}
type deferredAuthorizationScopeKey struct{}

type deferredAuthorizationScope struct {
	callbacks []func(context.Context, bool) error
}

type deferredAuthorizationTransaction struct {
	transactionMu     sync.Mutex
	repository        *authorizationMemoryRepository
	atCompletion      map[string]chan struct{}
	releaseCompletion map[string]chan struct{}
}

func newDeferredAuthorizationTransaction(
	repository *authorizationMemoryRepository,
	names ...string,
) *deferredAuthorizationTransaction {
	manager := &deferredAuthorizationTransaction{
		repository:        repository,
		atCompletion:      make(map[string]chan struct{}, len(names)),
		releaseCompletion: make(map[string]chan struct{}, len(names)),
	}
	for _, name := range names {
		manager.atCompletion[name] = make(chan struct{})
		manager.releaseCompletion[name] = make(chan struct{})
	}
	return manager
}

func withAuthorizationTransactionName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, authorizationTransactionNameKey{}, name)
}

func (manager *deferredAuthorizationTransaction) DoInTx(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	if _, nested := ctx.Value(deferredAuthorizationScopeKey{}).(*deferredAuthorizationScope); nested {
		return operation(ctx)
	}

	manager.transactionMu.Lock()
	state, rules := manager.repository.snapshot()
	scope := &deferredAuthorizationScope{}
	txCtx := context.WithValue(ctx, deferredAuthorizationScopeKey{}, scope)
	err := operation(txCtx)
	committed := err == nil
	if !committed {
		manager.repository.restore(state, rules)
	}
	manager.transactionMu.Unlock()

	name, _ := ctx.Value(authorizationTransactionNameKey{}).(string)
	if at := manager.atCompletion[name]; at != nil {
		close(at)
		<-manager.releaseCompletion[name]
	}
	for _, callback := range scope.callbacks {
		if callbackErr := callback(ctx, committed); callbackErr != nil && err == nil {
			err = callbackErr
		}
	}
	return err
}

func (manager *deferredAuthorizationTransaction) RegisterAfterCompletion(
	ctx context.Context,
	callback func(context.Context, bool) error,
) error {
	scope, ok := ctx.Value(deferredAuthorizationScopeKey{}).(*deferredAuthorizationScope)
	if !ok || scope == nil {
		return errors.New("test transaction scope required")
	}
	scope.callbacks = append(scope.callbacks, callback)
	return nil
}

func (manager *authorizationTestTransaction) DoInTx(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	manager.transactionMu.Lock()
	state, rules := manager.repository.snapshot()
	manager.callbacks = nil
	err := operation(ctx)
	committed := err == nil
	if !committed {
		manager.repository.restore(state, rules)
	}
	callbacks := append([]func(context.Context, bool) error(nil), manager.callbacks...)
	manager.callbacks = nil
	manager.transactionMu.Unlock()
	for _, callback := range callbacks {
		if callbackErr := callback(ctx, committed); callbackErr != nil && err == nil {
			err = callbackErr
		}
	}
	return err
}

func (manager *authorizationTestTransaction) RegisterAfterCompletion(
	_ context.Context,
	callback func(context.Context, bool) error,
) error {
	manager.callbacks = append(manager.callbacks, callback)
	return nil
}

type authorizationMemoryRepository struct {
	mu            sync.Mutex
	state         ports.AuthorizationState
	rules         []authcasbin.PolicyRule
	activeRoles   map[string]bool
	activeUsers   map[uuid.UUID]bool
	eligibleUsers map[uuid.UUID]bool
	ensureCalls   int
}

func (repository *authorizationMemoryRepository) EnsureState(_ context.Context, revision int64) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.ensureCalls++
	if repository.state.DesiredRevision == 0 {
		repository.state.DesiredRevision = revision
	}
	return nil
}

func (repository *authorizationMemoryRepository) State(context.Context) (ports.AuthorizationState, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.state, nil
}

func (repository *authorizationMemoryRepository) LoadRules(
	ctx context.Context,
	_ string,
) ([]authcasbin.PolicyRule, error) {
	return repository.ListRules(ctx)
}

func (repository *authorizationMemoryRepository) LockState(ctx context.Context) (ports.AuthorizationState, error) {
	return repository.State(ctx)
}

func (repository *authorizationMemoryRepository) ListRules(context.Context) ([]authcasbin.PolicyRule, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return append([]authcasbin.PolicyRule(nil), repository.rules...), nil
}

func (repository *authorizationMemoryRepository) ReplaceRolePolicies(
	_ context.Context,
	subject string,
	rules []authcasbin.PolicyRule,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	kept := repository.rules[:0]
	for _, rule := range repository.rules {
		if rule.Type != "p" || rule.V0 != subject {
			kept = append(kept, rule)
		}
	}
	repository.rules = append(kept, rules...)
	return nil
}

func (repository *authorizationMemoryRepository) ReplaceUserRoles(
	_ context.Context,
	subject string,
	roles []string,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	kept := repository.rules[:0]
	for _, rule := range repository.rules {
		if rule.Type != "g" || rule.V0 != subject {
			kept = append(kept, rule)
		}
	}
	for _, role := range roles {
		kept = append(kept, authcasbin.PolicyRule{Type: "g", V0: subject, V1: role})
	}
	repository.rules = kept
	return nil
}

func (repository *authorizationMemoryRepository) DeleteRoleReferences(_ context.Context, subject string) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	kept := repository.rules[:0]
	for _, rule := range repository.rules {
		if (rule.Type == "p" && rule.V0 == subject) ||
			(rule.Type == "g" && (rule.V0 == subject || rule.V1 == subject)) {
			continue
		}
		kept = append(kept, rule)
	}
	repository.rules = kept
	return nil
}

func (repository *authorizationMemoryRepository) DeleteUserReferences(_ context.Context, subject string) error {
	return repository.ReplaceUserRoles(context.Background(), subject, nil)
}

func (repository *authorizationMemoryRepository) AdvanceRevision(
	_ context.Context,
	expected int64,
	next int64,
	updatedBy *uuid.UUID,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.state.DesiredRevision != expected {
		return ports.ErrAuthorizationRevisionConflict
	}
	repository.state.DesiredRevision = next
	repository.state.UpdatedBy = updatedBy
	return nil
}

func (repository *authorizationMemoryRepository) ActiveRoleKeys(
	_ context.Context,
	keys []string,
) (map[string]bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make(map[string]bool, len(keys))
	for _, key := range keys {
		result[key] = repository.activeRoles[key]
	}
	return result, nil
}

func (repository *authorizationMemoryRepository) ActiveUserIDs(
	_ context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		result[id] = repository.activeUsers[id]
	}
	return result, nil
}

func (repository *authorizationMemoryRepository) AuthorizationEligibleUserIDs(
	_ context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	eligible := repository.eligibleUsers
	if eligible == nil {
		eligible = repository.activeUsers
	}
	result := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		result[id] = eligible[id]
	}
	return result, nil
}

func (repository *authorizationMemoryRepository) snapshot() (ports.AuthorizationState, []authcasbin.PolicyRule) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.state, append([]authcasbin.PolicyRule(nil), repository.rules...)
}

func (repository *authorizationMemoryRepository) restore(state ports.AuthorizationState, rules []authcasbin.PolicyRule) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.state = state
	repository.rules = append([]authcasbin.PolicyRule(nil), rules...)
}

func (repository *authorizationMemoryRepository) LoadCommandReceipt(
	_ context.Context,
	_ uuid.UUID,
) (ports.AuthorizationCommandReceipt, bool, error) {
	return ports.AuthorizationCommandReceipt{}, false, nil
}

type authorizationAuditAppender struct {
	mu      sync.Mutex
	entries []audit.Entry
	failure error
}

type authorizationOutboxAppender struct {
	mu      sync.Mutex
	events  []eventing.Event
	failure error
}

func (appender *authorizationOutboxAppender) Append(_ context.Context, events ...eventing.Event) error {
	appender.mu.Lock()
	defer appender.mu.Unlock()
	if appender.failure != nil {
		return appender.failure
	}
	appender.events = append(appender.events, events...)
	return nil
}

func (appender *authorizationAuditAppender) Append(_ context.Context, entry audit.Entry) (uuid.UUID, error) {
	appender.mu.Lock()
	defer appender.mu.Unlock()
	if appender.failure != nil {
		return uuid.Nil, appender.failure
	}
	appender.entries = append(appender.entries, entry)
	return uuid.New(), nil
}

var _ ports.AuthorizationPolicyRepository = (*authorizationMemoryRepository)(nil)
