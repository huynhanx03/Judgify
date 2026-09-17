package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/authctx"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

const casbinRBACModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`

var authorizationAggregateID = uuid.MustParse("00000000-0000-5000-8000-000000000001")

type transactionCompletionRegistrar interface {
	RegisterAfterCompletion(context.Context, func(context.Context, bool) error) error
}

type authorizationView struct {
	revision              int64
	rolePolicies          map[string][]dto.PolicyRule
	userRoles             map[uuid.UUID][]string
	effectiveCapabilities map[uuid.UUID][]dto.CapabilityGrant
}

type authorizationService struct {
	authorizationReconcilerState
	repository   ports.AuthorizationPolicyRepository
	txManager    tx.Manager
	completion   transactionCompletionRegistrar
	audit        audit.Appender
	outbox       eventing.OutboxAppender
	runtime      *authcasbin.Runtime
	view         atomic.Pointer[authorizationView]
	mutationGate sync.Mutex
	log          *zap.Logger
}

type authorizationMutation struct {
	expectedRevision *int64
	actor            *authentication.Principal
	reason           string
	action           string
	resource         string
	resourceID       string
	commandID        uuid.UUID
	requestHash      [sha256.Size]byte
	// systemInitialUser is set only by the server-owned registration flow. A
	// nil actor never receives a general authorization mutation capability.
	systemInitialUser *uuid.UUID
	apply             func(context.Context, []authcasbin.PolicyRule) (map[string]any, map[string]any, error)
}

// NewAuthorizationService loads one complete, migration-owned Ent policy
// snapshot. Persistence mutations remain in Judgify's transaction; Casbin is
// immutable in memory and never performs AutoSave or runtime schema DDL.
func NewAuthorizationService(
	ctx context.Context,
	repository ports.AuthorizationPolicyRepository,
	txManager tx.Manager,
	auditAppender audit.Appender,
	outboxAppender eventing.OutboxAppender,
	log *zap.Logger,
) (*authorizationService, error) {
	if ctx == nil || repository == nil || txManager == nil || auditAppender == nil ||
		outboxAppender == nil || log == nil {
		return nil, ErrInvalidAuthorizationMutation
	}
	completion, ok := txManager.(transactionCompletionRegistrar)
	if !ok {
		return nil, fmt.Errorf("authorization: transaction completion capability required")
	}
	state, err := repository.State(ctx)
	if err != nil {
		return nil, fmt.Errorf("load authorization state: %w", err)
	}
	rules, err := repository.LoadRules(ctx, casbinRBACModel)
	if err != nil {
		return nil, fmt.Errorf("load authorization rules: %w", err)
	}
	snapshot := authcasbin.Snapshot{
		Revision: state.DesiredRevision,
		Model:    casbinRBACModel,
		Rules:    cloneAuthorizationRules(rules),
	}
	if err := validateAuthorizationSnapshot(ctx, repository, snapshot, nil); err != nil {
		return nil, fmt.Errorf("validate persisted authorization snapshot: %w", err)
	}
	runtime, err := authcasbin.NewRuntime(snapshot)
	if err != nil {
		return nil, fmt.Errorf("initialize authorization runtime: %w", err)
	}
	service := &authorizationService{
		authorizationReconcilerState: authorizationReconcilerState{
			reconcilerStop: make(chan struct{}),
		},
		repository: repository,
		txManager:  txManager,
		completion: completion,
		audit:      auditAppender,
		outbox:     outboxAppender,
		runtime:    runtime,
		log:        log,
	}
	service.view.Store(newAuthorizationView(snapshot.Revision, snapshot.Rules))
	return service, nil
}

func (service *authorizationService) Authorize(
	ctx context.Context,
	request authorization.Request,
) (authorization.Decision, error) {
	if service == nil || service.runtime == nil || ctx == nil {
		return authorization.Decision{
			Allowed: false,
			Reason:  authorization.ReasonEvaluationError,
		}, authorization.ErrEvaluation
	}
	if request.Principal.Authenticated {
		userID, err := canonicalUserID(request.Principal)
		if err != nil {
			return authorization.Decision{
				Allowed:  false,
				Reason:   authorization.ReasonInvalidRequest,
				Revision: service.runtime.AppliedRevision(),
			}, authorization.ErrInvalidRequest
		}
		request.Principal.Subject = userSubject(userID)
	}
	return service.runtime.Authorize(ctx, request)
}

func canonicalUserID(principal authentication.Principal) (string, error) {
	value := principal.UserID
	if value == "" {
		value = principal.Subject
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return "", authorization.ErrInvalidRequest
	}
	return value, nil
}

func (service *authorizationService) Healthy() error {
	if service == nil || service.runtime == nil {
		return authcasbin.ErrUnhealthy
	}
	return service.runtime.Healthy()
}

func (service *authorizationService) Catalog(ctx context.Context) (*dto.AuthorizationCatalogResponse, error) {
	view, err := service.currentView(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.AuthorizationCatalogResponse{
		Revision:  view.revision,
		Resources: permissions.DetailedCatalog(),
	}, nil
}

func (service *authorizationService) EffectiveCapabilities(
	ctx context.Context,
	userID uuid.UUID,
) (*dto.EffectiveCapabilitiesResponse, error) {
	if userID == uuid.Nil {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	view, err := service.currentView(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := effectiveCapabilitiesSnapshot(view, userID)
	logger.FromContext(ctx).Debug(
		"effective authorization capabilities resolved",
		zap.String("user_id", userID.String()),
		zap.Int64("authorization_revision", snapshot.Revision),
		zap.Int("capability_resources", len(snapshot.Capabilities)),
	)
	return snapshot, nil
}

func (service *authorizationService) GetRolePolicies(
	ctx context.Context,
	roleKey string,
) (*dto.RolePolicySnapshot, error) {
	if !validRoleKey(roleKey) {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	view, err := service.currentView(ctx)
	if err != nil {
		return nil, err
	}
	return rolePolicySnapshot(view, roleKey), nil
}

func (service *authorizationService) PreviewRolePolicies(
	ctx context.Context,
	roleKey string,
	expectedRevision int64,
	rules []dto.PolicyRule,
) (*dto.RolePolicyPreview, error) {
	actor, err := captureMutationActor(ctx, true)
	if err != nil {
		return nil, err
	}
	if !validRoleKey(roleKey) || expectedRevision <= 0 {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	if roleKey == protectedSuperadminRoleKey {
		return nil, conflictAuthorization(
			ErrProtectedAuthorizationRole,
			service.appliedRevision(),
		)
	}
	normalized, err := normalizeRolePolicies(roleKey, rules)
	if err != nil {
		return nil, badAuthorizationRequest(err)
	}
	view, err := service.currentView(ctx)
	if err != nil {
		return nil, err
	}
	if view.revision != expectedRevision {
		return nil, conflictAuthorization(
			ErrAuthorizationRevisionConflict,
			view.revision,
		)
	}
	state, err := service.repository.State(ctx)
	if err != nil {
		return nil, err
	}
	if state.DesiredRevision != expectedRevision {
		return nil, conflictAuthorization(
			ErrAuthorizationRevisionConflict,
			state.DesiredRevision,
		)
	}
	existing, err := service.repository.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	confirmedState, err := service.repository.State(ctx)
	if err != nil {
		return nil, err
	}
	if confirmedState.DesiredRevision != expectedRevision {
		return nil, conflictAuthorization(
			ErrAuthorizationRevisionConflict,
			confirmedState.DesiredRevision,
		)
	}
	candidateRules := replaceRolePolicyRules(existing, roleKey, normalized)
	base := authcasbin.Snapshot{
		Revision: expectedRevision,
		Model:    casbinRBACModel,
		Rules:    cloneAuthorizationRules(existing),
	}
	candidate := authcasbin.Snapshot{
		Revision: expectedRevision + 1,
		Model:    casbinRBACModel,
		Rules:    candidateRules,
	}
	if err := validateAuthorizationSnapshot(ctx, service.repository, candidate, nil); err != nil {
		return nil, mapAuthorizationMutationError(err, expectedRevision)
	}
	if err := validateAuthorizationTransition(ctx, base, candidate, actor, nil); err != nil {
		return nil, mapAuthorizationMutationError(err, expectedRevision)
	}
	before := rolePolicySnapshot(view, roleKey)
	afterRules := append([]dto.PolicyRule(nil), normalized...)
	return &dto.RolePolicyPreview{
		RoleKey:           roleKey,
		ExpectedRevision:  expectedRevision,
		ResultingRevision: expectedRevision + 1,
		Before:            *before,
		After: dto.RolePolicySnapshot{
			Revision: expectedRevision + 1,
			Rules:    afterRules,
		},
		Added:   policyRuleDifference(afterRules, before.Rules),
		Removed: policyRuleDifference(before.Rules, afterRules),
	}, nil
}

func replaceRolePolicyRules(
	existing []authcasbin.PolicyRule,
	roleKey string,
	replacement []dto.PolicyRule,
) []authcasbin.PolicyRule {
	subject := roleSubject(roleKey)
	result := make([]authcasbin.PolicyRule, 0, len(existing)+len(replacement))
	for _, rule := range existing {
		if rule.Type != "p" || rule.V0 != subject {
			result = append(result, rule)
		}
	}
	for _, rule := range replacement {
		result = append(result, authcasbin.PolicyRule{
			Type: "p",
			V0:   subject,
			V1:   rule.Resource,
			V2:   rule.Action,
		})
	}
	return result
}

func replaceUserRoleRules(
	existing []authcasbin.PolicyRule,
	userID uuid.UUID,
	replacement []string,
) []authcasbin.PolicyRule {
	subject := userSubject(userID.String())
	result := make([]authcasbin.PolicyRule, 0, len(existing)+len(replacement))
	for _, rule := range existing {
		if rule.Type != "g" || rule.V0 != subject {
			result = append(result, rule)
		}
	}
	for _, role := range replacement {
		result = append(result, authcasbin.PolicyRule{
			Type: "g",
			V0:   subject,
			V1:   roleSubject(role),
		})
	}
	return result
}

func policyRuleDifference(
	left []dto.PolicyRule,
	right []dto.PolicyRule,
) []dto.PolicyRule {
	rightSet := make(map[string]struct{}, len(right))
	for _, rule := range right {
		rightSet[rule.Resource+"\x00"+rule.Action] = struct{}{}
	}
	difference := make([]dto.PolicyRule, 0, len(left))
	for _, rule := range left {
		if _, found := rightSet[rule.Resource+"\x00"+rule.Action]; !found {
			difference = append(difference, rule)
		}
	}
	return difference
}

func roleKeyDifference(left []string, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, role := range right {
		rightSet[role] = struct{}{}
	}
	difference := make([]string, 0, len(left))
	for _, role := range left {
		if _, found := rightSet[role]; !found {
			difference = append(difference, role)
		}
	}
	return difference
}

func (service *authorizationService) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	snapshot, err := service.GetUserRoleSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), snapshot.Roles...), nil
}

func (service *authorizationService) GetUserRoleSnapshot(
	ctx context.Context,
	userID uuid.UUID,
) (*dto.UserRoleSnapshot, error) {
	if userID == uuid.Nil {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	view, err := service.currentView(ctx)
	if err != nil {
		return nil, err
	}
	return userRoleSnapshot(view, userID), nil
}

// EnsureCanDeactivateUser serializes account deactivation against policy
// writers and rejects removing the final currently authenticating protected
// superadministrator. Callers must already own a physical transaction so the
// authorization-state row remains locked through the user status CAS.
func (service *authorizationService) EnsureCanDeactivateUser(
	ctx context.Context,
	userID uuid.UUID,
) error {
	if service == nil || service.repository == nil || ctx == nil ||
		ctx.Err() != nil || userID == uuid.Nil {
		return badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	state, err := service.repository.LockState(ctx)
	if err != nil {
		return err
	}
	rules, err := service.repository.ListRules(ctx)
	if err != nil {
		return err
	}
	targetSubject := userSubject(userID.String())
	if !hasDirectRole(rules, targetSubject, protectedSuperadminRoleKey) {
		return nil
	}
	directSuperadmins := directRoleUserIDs(rules, protectedSuperadminRoleKey)
	active, err := service.repository.ActiveUserIDs(ctx, directSuperadmins)
	if err != nil {
		return err
	}
	activeCount := 0
	for _, activeUser := range active {
		if activeUser {
			activeCount++
		}
	}
	if active[userID] && activeCount <= 1 {
		return finalSuperadminConflict(state.DesiredRevision)
	}
	return nil
}

func (service *authorizationService) PreviewUserRoles(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision int64,
	roles []string,
) (*dto.UserRolePreview, error) {
	actor, err := captureMutationActor(ctx, true)
	if err != nil {
		return nil, err
	}
	if userID == uuid.Nil || expectedRevision <= 0 || len(roles) == 0 {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	normalized, err := normalizeRoleKeys(roles)
	if err != nil {
		return nil, badAuthorizationRequest(err)
	}
	view, err := service.currentView(ctx)
	if err != nil {
		return nil, err
	}
	if view.revision != expectedRevision {
		return nil, conflictAuthorization(
			ErrAuthorizationRevisionConflict,
			view.revision,
		)
	}
	state, err := service.repository.State(ctx)
	if err != nil {
		return nil, err
	}
	if state.DesiredRevision != expectedRevision {
		return nil, conflictAuthorization(
			ErrAuthorizationRevisionConflict,
			state.DesiredRevision,
		)
	}
	existing, err := service.repository.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	confirmedState, err := service.repository.State(ctx)
	if err != nil {
		return nil, err
	}
	if confirmedState.DesiredRevision != expectedRevision {
		return nil, conflictAuthorization(
			ErrAuthorizationRevisionConflict,
			confirmedState.DesiredRevision,
		)
	}
	candidateRules := replaceUserRoleRules(existing, userID, normalized)
	base := authcasbin.Snapshot{
		Revision: expectedRevision,
		Model:    casbinRBACModel,
		Rules:    cloneAuthorizationRules(existing),
	}
	candidate := authcasbin.Snapshot{
		Revision: expectedRevision + 1,
		Model:    casbinRBACModel,
		Rules:    candidateRules,
	}
	if err := validateAuthorizationSnapshot(ctx, service.repository, candidate, nil); err != nil {
		return nil, mapAuthorizationMutationError(err, expectedRevision)
	}
	if err := validateAuthorizationTransition(ctx, base, candidate, actor, nil); err != nil {
		return nil, mapAuthorizationMutationError(err, expectedRevision)
	}
	before := userRoleSnapshot(view, userID)
	afterRoles := append([]string(nil), normalized...)
	return &dto.UserRolePreview{
		UserID:            userID,
		ExpectedRevision:  expectedRevision,
		ResultingRevision: expectedRevision + 1,
		Before:            *before,
		After: dto.UserRoleSnapshot{
			Revision: expectedRevision + 1,
			Roles:    afterRoles,
		},
		Added:   roleKeyDifference(afterRoles, before.Roles),
		Removed: roleKeyDifference(before.Roles, afterRoles),
	}, nil
}

func (service *authorizationService) SetRolePolicies(
	ctx context.Context,
	roleKey string,
	expectedRevision int64,
	reason string,
	rules []dto.PolicyRule,
) (*dto.RolePolicySnapshot, error) {
	view, err := service.replaceRolePolicies(
		ctx,
		roleKey,
		expectedRevision,
		reason,
		rules,
		uuid.Nil,
		[sha256.Size]byte{},
	)
	if err != nil {
		return nil, err
	}
	return rolePolicySnapshot(view, roleKey), nil
}

func (service *authorizationService) ApplyRolePolicies(
	ctx context.Context,
	roleKey string,
	command ports.ReplaceRolePoliciesCommand,
) (*dto.RolePolicyMutationReceipt, error) {
	if command.CommandID == uuid.Nil || command.ExpectedRevision <= 0 ||
		command.RequestHash == ([sha256.Size]byte{}) {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	if receipt, found, err := service.loadRolePolicyMutationReceipt(
		ctx,
		roleKey,
		command,
		true,
	); err != nil || found {
		return receipt, err
	}
	_, err := service.replaceRolePolicies(
		ctx,
		roleKey,
		command.ExpectedRevision,
		command.Reason,
		command.Rules,
		command.CommandID,
		command.RequestHash,
	)
	if err != nil {
		if receipt, found, receiptErr := service.loadRolePolicyMutationReceipt(
			ctx,
			roleKey,
			command,
			true,
		); receiptErr != nil || found {
			return receipt, receiptErr
		}
		return nil, err
	}
	receipt, found, err := service.loadRolePolicyMutationReceipt(
		ctx,
		roleKey,
		command,
		false,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, conflictAuthorization(
			ErrAuthorizationCommandConflict,
			command.ExpectedRevision+1,
		)
	}
	return receipt, nil
}

func (service *authorizationService) replaceRolePolicies(
	ctx context.Context,
	roleKey string,
	expectedRevision int64,
	reason string,
	rules []dto.PolicyRule,
	commandID uuid.UUID,
	requestHash [sha256.Size]byte,
) (*authorizationView, error) {
	actor, err := captureMutationActor(ctx, true)
	if err != nil {
		return nil, err
	}
	if !validRoleKey(roleKey) {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	if roleKey == protectedSuperadminRoleKey {
		return nil, conflictAuthorization(ErrProtectedAuthorizationRole, service.appliedRevision())
	}
	normalized, err := normalizeRolePolicies(roleKey, rules)
	if err != nil {
		return nil, badAuthorizationRequest(err)
	}
	reason, err = normalizeReason(reason)
	if err != nil || expectedRevision <= 0 {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	expected := expectedRevision
	view, err := service.mutate(ctx, authorizationMutation{
		expectedRevision: &expected,
		actor:            actor,
		reason:           reason,
		action:           "authorization.role_policies.replace",
		resource:         "authorization.role",
		resourceID:       roleKey,
		commandID:        commandID,
		requestHash:      requestHash,
		apply: func(txCtx context.Context, existing []authcasbin.PolicyRule) (map[string]any, map[string]any, error) {
			before := policyDTOsForRole(existing, roleKey)
			persisted := make([]authcasbin.PolicyRule, 0, len(normalized))
			for _, rule := range normalized {
				persisted = append(persisted, authcasbin.PolicyRule{
					Type: "p",
					V0:   roleSubject(roleKey),
					V1:   rule.Resource,
					V2:   rule.Action,
				})
			}
			if err := service.repository.ReplaceRolePolicies(txCtx, roleSubject(roleKey), persisted); err != nil {
				return nil, nil, fmt.Errorf("replace role policies: %w", err)
			}
			return policyAuditDocument(expected, before), policyAuditDocument(expected+1, normalized), nil
		},
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

type authorizationRolePolicyReceiptPayload struct {
	Action            string `json:"action"`
	Resource          string `json:"resource"`
	TargetID          string `json:"target_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ResultingRevision int64  `json:"resulting_revision"`
	AuditID           string `json:"audit_id"`
	CommandID         string `json:"command_id"`
	RequestHash       string `json:"request_hash"`
	Result            struct {
		Revision int64            `json:"revision"`
		Rules    []dto.PolicyRule `json:"rules"`
	} `json:"result"`
}

func (service *authorizationService) loadRolePolicyMutationReceipt(
	ctx context.Context,
	roleKey string,
	command ports.ReplaceRolePoliciesCommand,
	replayed bool,
) (*dto.RolePolicyMutationReceipt, bool, error) {
	receipt, found, err := service.repository.LoadCommandReceipt(ctx, command.CommandID)
	if err != nil || !found {
		return nil, found, err
	}
	eventType, eventErr := authorizationMutationEventType(
		"authorization.role_policies.replace",
	)
	if eventErr != nil {
		return nil, false, eventErr
	}
	var payload authorizationRolePolicyReceiptPayload
	normalized, normalizeErr := normalizeRolePolicies(roleKey, command.Rules)
	if normalizeErr != nil {
		return nil, false, badAuthorizationRequest(normalizeErr)
	}
	if receipt.EventID != command.CommandID || receipt.EventType != eventType ||
		receipt.AggregateID != authorizationAggregateID ||
		receipt.AggregateVersion != command.ExpectedRevision+1 ||
		receipt.OccurredAt.IsZero() || json.Unmarshal(receipt.Payload, &payload) != nil ||
		payload.Action != "authorization.role_policies.replace" ||
		payload.Resource != "authorization.role" || payload.TargetID != roleKey ||
		payload.ExpectedRevision != command.ExpectedRevision ||
		payload.ResultingRevision != receipt.AggregateVersion ||
		payload.Result.Revision != receipt.AggregateVersion ||
		!canonicalReceiptUUID(payload.AuditID) ||
		payload.CommandID != command.CommandID.String() ||
		payload.RequestHash != hex.EncodeToString(command.RequestHash[:]) {
		return nil, false, conflictAuthorization(
			ErrAuthorizationCommandConflict,
			receipt.AggregateVersion,
		)
	}
	receiptRules, receiptRulesErr := normalizeRolePolicies(roleKey, payload.Result.Rules)
	if receiptRulesErr != nil || !slices.Equal(normalized, receiptRules) {
		return nil, false, conflictAuthorization(
			ErrAuthorizationCommandConflict,
			receipt.AggregateVersion,
		)
	}
	return &dto.RolePolicyMutationReceipt{
		CommandID: command.CommandID,
		EventID:   receipt.EventID,
		RoleKey:   roleKey,
		Snapshot: dto.RolePolicySnapshot{
			Revision: payload.Result.Revision,
			Rules:    receiptRules,
		},
		CommittedAt:      receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func canonicalReceiptUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func (service *authorizationService) ApplyUserRoles(
	ctx context.Context,
	userID uuid.UUID,
	command ports.ReplaceUserRolesCommand,
) (*dto.UserRoleMutationReceipt, error) {
	if userID == uuid.Nil || command.CommandID == uuid.Nil ||
		command.ExpectedRevision <= 0 || len(command.Roles) == 0 ||
		command.RequestHash == ([sha256.Size]byte{}) {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	if receipt, found, err := service.loadUserRoleMutationReceipt(
		ctx,
		userID,
		command,
		true,
	); err != nil || found {
		return receipt, err
	}
	actor, err := captureMutationActor(ctx, true)
	if err != nil {
		return nil, err
	}
	expected := command.ExpectedRevision
	_, err = service.setUserRoles(
		ctx,
		userID,
		&expected,
		actor,
		nil,
		command.Reason,
		command.Roles,
		command.CommandID,
		command.RequestHash,
	)
	if err != nil {
		if receipt, found, receiptErr := service.loadUserRoleMutationReceipt(
			ctx,
			userID,
			command,
			true,
		); receiptErr != nil || found {
			return receipt, receiptErr
		}
		return nil, err
	}
	receipt, found, err := service.loadUserRoleMutationReceipt(
		ctx,
		userID,
		command,
		false,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, conflictAuthorization(
			ErrAuthorizationCommandConflict,
			command.ExpectedRevision+1,
		)
	}
	return receipt, nil
}

type authorizationUserRoleReceiptPayload struct {
	Action            string `json:"action"`
	Resource          string `json:"resource"`
	TargetID          string `json:"target_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ResultingRevision int64  `json:"resulting_revision"`
	AuditID           string `json:"audit_id"`
	CommandID         string `json:"command_id"`
	RequestHash       string `json:"request_hash"`
	Result            struct {
		Roles []string `json:"roles"`
	} `json:"result"`
}

func (service *authorizationService) loadUserRoleMutationReceipt(
	ctx context.Context,
	userID uuid.UUID,
	command ports.ReplaceUserRolesCommand,
	replayed bool,
) (*dto.UserRoleMutationReceipt, bool, error) {
	receipt, found, err := service.repository.LoadCommandReceipt(
		ctx,
		command.CommandID,
	)
	if err != nil || !found {
		return nil, found, err
	}
	eventType, eventErr := authorizationMutationEventType(
		"authorization.user_roles.replace",
	)
	if eventErr != nil {
		return nil, false, eventErr
	}
	var payload authorizationUserRoleReceiptPayload
	normalized, normalizeErr := normalizeRoleKeys(command.Roles)
	if normalizeErr != nil {
		return nil, false, badAuthorizationRequest(normalizeErr)
	}
	if receipt.EventID != command.CommandID || receipt.EventType != eventType ||
		receipt.AggregateID != authorizationAggregateID ||
		receipt.AggregateVersion != command.ExpectedRevision+1 ||
		receipt.OccurredAt.IsZero() || json.Unmarshal(receipt.Payload, &payload) != nil ||
		payload.Action != "authorization.user_roles.replace" ||
		payload.Resource != "authorization.user" ||
		payload.TargetID != userID.String() ||
		payload.ExpectedRevision != command.ExpectedRevision ||
		payload.ResultingRevision != receipt.AggregateVersion ||
		!canonicalReceiptUUID(payload.AuditID) ||
		payload.CommandID != command.CommandID.String() ||
		payload.RequestHash != hex.EncodeToString(command.RequestHash[:]) {
		return nil, false, conflictAuthorization(
			ErrAuthorizationCommandConflict,
			receipt.AggregateVersion,
		)
	}
	receiptRoles, receiptRolesErr := normalizeRoleKeys(payload.Result.Roles)
	if receiptRolesErr != nil || !slices.Equal(normalized, receiptRoles) {
		return nil, false, conflictAuthorization(
			ErrAuthorizationCommandConflict,
			receipt.AggregateVersion,
		)
	}
	return &dto.UserRoleMutationReceipt{
		CommandID: command.CommandID,
		EventID:   receipt.EventID,
		UserID:    userID,
		Snapshot: dto.UserRoleSnapshot{
			Revision: receipt.AggregateVersion,
			Roles:    receiptRoles,
		},
		CommittedAt:      receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func (service *authorizationService) SetUserRoles(
	ctx context.Context,
	userID uuid.UUID,
	roles []string,
) error {
	actor, actorErr := captureMutationActor(ctx, false)
	if actorErr != nil {
		return actorErr
	}
	var systemInitialUser *uuid.UUID
	if actor == nil {
		id := userID
		systemInitialUser = &id
	}
	_, err := service.setUserRoles(
		ctx,
		userID,
		nil,
		actor,
		systemInitialUser,
		"Identity workflow synchronized user roles",
		roles,
		uuid.Nil,
		[sha256.Size]byte{},
	)
	return err
}

func (service *authorizationService) SetUserRolesAtRevision(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision int64,
	reason string,
	roles []string,
) (*dto.UserRoleSnapshot, error) {
	actor, err := captureMutationActor(ctx, true)
	if err != nil {
		return nil, err
	}
	if expectedRevision <= 0 {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	expected := expectedRevision
	return service.setUserRoles(
		ctx,
		userID,
		&expected,
		actor,
		nil,
		reason,
		roles,
		uuid.Nil,
		[sha256.Size]byte{},
	)
}

func (service *authorizationService) setUserRoles(
	ctx context.Context,
	userID uuid.UUID,
	expectedRevision *int64,
	actor *authentication.Principal,
	systemInitialUser *uuid.UUID,
	reason string,
	roles []string,
	commandID uuid.UUID,
	requestHash [sha256.Size]byte,
) (*dto.UserRoleSnapshot, error) {
	if userID == uuid.Nil {
		return nil, badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	normalized, err := normalizeRoleKeys(roles)
	if err != nil {
		return nil, badAuthorizationRequest(err)
	}
	reason, err = normalizeReason(reason)
	if err != nil {
		return nil, badAuthorizationRequest(err)
	}
	view, err := service.mutate(ctx, authorizationMutation{
		expectedRevision:  expectedRevision,
		actor:             actor,
		systemInitialUser: systemInitialUser,
		reason:            reason,
		action:            "authorization.user_roles.replace",
		resource:          "authorization.user",
		resourceID:        userID.String(),
		commandID:         commandID,
		requestHash:       requestHash,
		apply: func(txCtx context.Context, existing []authcasbin.PolicyRule) (map[string]any, map[string]any, error) {
			before := roleKeysForUser(existing, userID)
			roleSubjects := make([]string, len(normalized))
			for index, role := range normalized {
				roleSubjects[index] = roleSubject(role)
			}
			if err := service.repository.ReplaceUserRoles(txCtx, userSubject(userID.String()), roleSubjects); err != nil {
				return nil, nil, fmt.Errorf("replace user roles: %w", err)
			}
			return userRoleAuditDocument(before), userRoleAuditDocument(normalized), nil
		},
	})
	if err != nil {
		return nil, err
	}
	return userRoleSnapshot(view, userID), nil
}

func (service *authorizationService) DeleteRole(ctx context.Context, roleKey string) error {
	actor, err := captureMutationActor(ctx, true)
	if err != nil {
		return err
	}
	if !validRoleKey(roleKey) {
		return badAuthorizationRequest(ErrInvalidAuthorizationMutation)
	}
	if roleKey == protectedSuperadminRoleKey {
		return conflictAuthorization(ErrProtectedAuthorizationRole, service.appliedRevision())
	}
	_, err = service.mutate(ctx, authorizationMutation{
		actor:      actor,
		reason:     "Role was deleted",
		action:     "authorization.role.delete",
		resource:   "authorization.role",
		resourceID: roleKey,
		apply: func(txCtx context.Context, existing []authcasbin.PolicyRule) (map[string]any, map[string]any, error) {
			before := policyDTOsForRole(existing, roleKey)
			if err := service.repository.DeleteRoleReferences(txCtx, roleSubject(roleKey)); err != nil {
				return nil, nil, fmt.Errorf("delete role references: %w", err)
			}
			return policyAuditDocument(0, before), policyAuditDocument(0, []dto.PolicyRule{}), nil
		},
	})
	return err
}

func (service *authorizationService) mutate(
	ctx context.Context,
	mutation authorizationMutation,
) (*authorizationView, error) {
	if service == nil || service.repository == nil || service.txManager == nil ||
		service.completion == nil || service.audit == nil || service.outbox == nil || service.runtime == nil ||
		ctx == nil || mutation.apply == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authorization unavailable", nil)
	}
	// Hold the process-local writer gate through the physical transaction
	// outcome. This prevents a rolled-back revision number from racing a newer
	// writer that legitimately reuses that number (the desired-revision ABA
	// case). The database row lock remains the cross-process source of truth.
	service.mutationGate.Lock()
	gateHandedToCompletion := false
	defer func() {
		if !gateHandedToCompletion {
			service.mutationGate.Unlock()
		}
	}()
	ctx = correlation.EnsureContext(ctx)
	var committedView *authorizationView
	err := service.txManager.DoInTx(ctx, func(txCtx context.Context) error {
		state, err := service.repository.LockState(txCtx)
		if err != nil {
			return fmt.Errorf("lock authorization state: %w", err)
		}
		if state.DesiredRevision <= 0 {
			return ErrInvalidAuthorizationMutation
		}
		if mutation.expectedRevision != nil && *mutation.expectedRevision != state.DesiredRevision {
			return conflictAuthorization(ErrAuthorizationRevisionConflict, state.DesiredRevision)
		}
		existing, err := service.repository.ListRules(txCtx)
		if err != nil {
			return fmt.Errorf("load authorization mutation base: %w", err)
		}
		before, after, err := mutation.apply(txCtx, existing)
		if err != nil {
			return err
		}
		candidateRules, err := service.repository.ListRules(txCtx)
		if err != nil {
			return fmt.Errorf("load authorization candidate: %w", err)
		}
		nextRevision := state.DesiredRevision + 1
		candidate := authcasbin.Snapshot{
			Revision: nextRevision,
			Model:    casbinRBACModel,
			Rules:    cloneAuthorizationRules(candidateRules),
		}
		if err := validateAuthorizationSnapshot(txCtx, service.repository, candidate, nil); err != nil {
			return mapAuthorizationMutationError(err, state.DesiredRevision)
		}
		base := authcasbin.Snapshot{
			Revision: state.DesiredRevision,
			Model:    casbinRBACModel,
			Rules:    cloneAuthorizationRules(existing),
		}
		if err := validateAuthorizationTransition(
			txCtx,
			base,
			candidate,
			mutation.actor,
			mutation.systemInitialUser,
		); err != nil {
			return mapAuthorizationMutationError(err, state.DesiredRevision)
		}
		prepared, err := service.runtime.Prepare(txCtx, candidate)
		if err != nil {
			return fmt.Errorf("prepare authorization snapshot: %w", err)
		}
		actorUserID, actorSessionID := auditActor(mutation.actor)
		if err := service.repository.AdvanceRevision(
			txCtx,
			state.DesiredRevision,
			nextRevision,
			actorUserID,
		); err != nil {
			if errors.Is(err, ports.ErrAuthorizationRevisionConflict) {
				return conflictAuthorization(ErrAuthorizationRevisionConflict, state.DesiredRevision)
			}
			return fmt.Errorf("advance authorization revision: %w", err)
		}
		occurredAt := time.Now().UTC()
		auditID, err := service.audit.Append(txCtx, audit.Entry{
			ActorUserID:    actorUserID,
			ActorSessionID: actorSessionID,
			Action:         mutation.action,
			Resource:       mutation.resource,
			ResourceID:     mutation.resourceID,
			Reason:         mutation.reason,
			Before:         before,
			After:          after,
			OccurredAt:     occurredAt,
		})
		if err != nil {
			return fmt.Errorf("append authorization audit: %w", err)
		}
		eventID := mutation.commandID
		if eventID == uuid.Nil {
			eventID, err = uuid.NewV7()
			if err != nil {
				return fmt.Errorf("create authorization event identity: %w", err)
			}
		}
		eventType, err := authorizationMutationEventType(mutation.action)
		if err != nil {
			return err
		}
		payload, err := authorizationMutationPayload(
			mutation,
			auditID,
			state.DesiredRevision,
			nextRevision,
			after,
		)
		if err != nil {
			return err
		}
		if err := service.outbox.Append(txCtx, eventing.Event{
			ID:               eventID,
			Type:             eventType,
			AggregateType:    "authorization",
			AggregateID:      authorizationAggregateID,
			AggregateVersion: nextRevision,
			Payload:          payload,
			OccurredAt:       occurredAt,
		}); err != nil {
			return fmt.Errorf("append authorization event: %w", err)
		}
		publishedView := newAuthorizationView(nextRevision, candidate.Rules)
		if err := service.completion.RegisterAfterCompletion(
			txCtx,
			func(callbackCtx context.Context, committed bool) error {
				defer service.mutationGate.Unlock()
				if !committed {
					service.runtime.CompareAndSwapDesiredRevision(
						nextRevision,
						service.runtime.AppliedRevision(),
					)
					return nil
				}
				if err := service.runtime.Publish(prepared); err != nil {
					if errors.Is(err, authcasbin.ErrStaleRevision) &&
						service.runtime.AppliedRevision() >= nextRevision {
						return nil
					}
					return fmt.Errorf("publish authorization revision: %w", err)
				}
				service.storeViewIfNewer(publishedView)
				logger.FromContext(callbackCtx).Info(
					"authorization snapshot published",
					zap.Int64("authorization_revision", nextRevision),
				)
				return nil
			},
		); err != nil {
			return fmt.Errorf("register authorization publication: %w", err)
		}
		gateHandedToCompletion = true
		service.runtime.SetDesiredRevision(nextRevision)
		committedView = publishedView
		return nil
	})
	if err != nil {
		return nil, mapAuthorizationMutationError(err, service.appliedRevision())
	}
	if committedView == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authorization unavailable", nil)
	}
	return committedView, nil
}

func authorizationMutationEventType(action string) (string, error) {
	switch action {
	case "authorization.role_policies.replace":
		return "identity.authorization.role_policies_replaced.v1", nil
	case "authorization.user_roles.replace":
		return "identity.authorization.user_roles_replaced.v1", nil
	case "authorization.role.delete":
		return "identity.authorization.role_deleted.v1", nil
	default:
		return "", ErrInvalidAuthorizationMutation
	}
}

func authorizationMutationPayload(
	mutation authorizationMutation,
	auditID uuid.UUID,
	expectedRevision int64,
	resultingRevision int64,
	result map[string]any,
) (json.RawMessage, error) {
	if auditID == uuid.Nil || expectedRevision <= 0 ||
		resultingRevision != expectedRevision+1 || result == nil {
		return nil, ErrInvalidAuthorizationMutation
	}
	document := map[string]any{
		"action":             mutation.action,
		"resource":           mutation.resource,
		"target_id":          mutation.resourceID,
		"expected_revision":  expectedRevision,
		"resulting_revision": resultingRevision,
		"audit_id":           auditID.String(),
		"result":             result,
	}
	if mutation.commandID != uuid.Nil {
		if mutation.requestHash == ([sha256.Size]byte{}) {
			return nil, ErrInvalidAuthorizationMutation
		}
		document["command_id"] = mutation.commandID.String()
		document["request_hash"] = hex.EncodeToString(mutation.requestHash[:])
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode authorization event: %w", err)
	}
	return payload, nil
}

func (service *authorizationService) currentView(ctx context.Context) (*authorizationView, error) {
	if service == nil || service.runtime == nil || ctx == nil || ctx.Err() != nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authorization unavailable", ctxError(ctx))
	}
	if err := service.runtime.Healthy(); err != nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authorization unavailable", err)
	}
	view := service.view.Load()
	if view == nil || view.revision != service.runtime.AppliedRevision() {
		return nil, apperr.New(apperr.CodeInternalServer, "authorization unavailable", authcasbin.ErrUnhealthy)
	}
	return view, nil
}

func (service *authorizationService) storeViewIfNewer(candidate *authorizationView) {
	if service == nil || candidate == nil || candidate.revision <= 0 {
		return
	}
	for {
		current := service.view.Load()
		if current != nil && current.revision >= candidate.revision {
			return
		}
		if service.view.CompareAndSwap(current, candidate) {
			return
		}
	}
}

func (service *authorizationService) appliedRevision() int64 {
	return service.AppliedRevision()
}

// AppliedRevision and DesiredRevision expose only the process-local policy
// fence for health and low-cardinality metrics. The Casbin model and policy
// rows remain encapsulated.
func (service *authorizationService) AppliedRevision() int64 {
	if service == nil || service.runtime == nil {
		return 0
	}
	return service.runtime.AppliedRevision()
}

func (service *authorizationService) DesiredRevision() int64 {
	if service == nil || service.runtime == nil {
		return 0
	}
	return service.runtime.DesiredRevision()
}

func cloneAuthorizationRules(rules []authcasbin.PolicyRule) []authcasbin.PolicyRule {
	return append([]authcasbin.PolicyRule(nil), rules...)
}

func rolePolicySnapshot(view *authorizationView, roleKey string) *dto.RolePolicySnapshot {
	rules := view.rolePolicies[roleKey]
	return &dto.RolePolicySnapshot{
		Revision: view.revision,
		Rules:    append([]dto.PolicyRule(nil), rules...),
	}
}

func userRoleSnapshot(view *authorizationView, userID uuid.UUID) *dto.UserRoleSnapshot {
	roles := view.userRoles[userID]
	return &dto.UserRoleSnapshot{
		Revision: view.revision,
		Roles:    append([]string(nil), roles...),
	}
}

func newAuthorizationView(revision int64, rules []authcasbin.PolicyRule) *authorizationView {
	view := &authorizationView{
		revision:              revision,
		rolePolicies:          make(map[string][]dto.PolicyRule),
		userRoles:             make(map[uuid.UUID][]string),
		effectiveCapabilities: make(map[uuid.UUID][]dto.CapabilityGrant),
	}
	roleParents := make(map[string][]string)
	for _, rule := range rules {
		switch rule.Type {
		case "p":
			roleKey, ok := parseRoleSubject(rule.V0)
			if !ok {
				continue
			}
			view.rolePolicies[roleKey] = append(view.rolePolicies[roleKey], dto.PolicyRule{
				RoleKey:  roleKey,
				Resource: rule.V1,
				Action:   rule.V2,
			})
		case "g":
			parentRole, parentOK := parseRoleSubject(rule.V1)
			if !parentOK {
				continue
			}
			if childRole, childOK := parseRoleSubject(rule.V0); childOK {
				roleParents[childRole] = append(roleParents[childRole], parentRole)
				continue
			}
			userID, userOK := parseUserSubject(rule.V0)
			if !userOK {
				continue
			}
			view.userRoles[userID] = append(view.userRoles[userID], parentRole)
		}
	}
	for roleKey := range view.rolePolicies {
		slices.SortFunc(view.rolePolicies[roleKey], comparePolicyDTO)
	}
	for userID := range view.userRoles {
		slices.Sort(view.userRoles[userID])
		view.effectiveCapabilities[userID] = catalogCapabilitiesForRoles(
			expandInheritedRoles(view.userRoles[userID], roleParents),
			view.rolePolicies,
		)
	}
	return view
}

func expandInheritedRoles(directRoles []string, roleParents map[string][]string) []string {
	roles := make([]string, 0, len(directRoles))
	visited := make(map[string]struct{}, len(directRoles))
	pending := append([]string(nil), directRoles...)
	for len(pending) != 0 {
		last := len(pending) - 1
		role := pending[last]
		pending = pending[:last]
		if _, exists := visited[role]; exists {
			continue
		}
		visited[role] = struct{}{}
		roles = append(roles, role)
		pending = append(pending, roleParents[role]...)
	}
	slices.Sort(roles)
	return roles
}

func catalogCapabilitiesForRoles(
	roles []string,
	rolePolicies map[string][]dto.PolicyRule,
) []dto.CapabilityGrant {
	capabilities := make([]dto.CapabilityGrant, 0)
	for _, resource := range permissions.Catalog() {
		actions := make([]string, 0, len(resource.Actions))
		for _, action := range resource.Actions {
			if rolesGrantCapability(roles, rolePolicies, resource.Key, action) {
				actions = append(actions, action)
			}
		}
		if len(actions) != 0 {
			capabilities = append(capabilities, dto.CapabilityGrant{
				Resource: resource.Key,
				Actions:  actions,
			})
		}
	}
	return capabilities
}

func rolesGrantCapability(
	roles []string,
	rolePolicies map[string][]dto.PolicyRule,
	resource string,
	action string,
) bool {
	for _, role := range roles {
		for _, policy := range rolePolicies[role] {
			if (policy.Resource == "*" || policy.Resource == resource) &&
				(policy.Action == "*" || policy.Action == action) {
				return true
			}
		}
	}
	return false
}

func effectiveCapabilitiesSnapshot(
	view *authorizationView,
	userID uuid.UUID,
) *dto.EffectiveCapabilitiesResponse {
	capabilities := view.effectiveCapabilities[userID]
	cloned := make([]dto.CapabilityGrant, len(capabilities))
	for index, capability := range capabilities {
		cloned[index] = dto.CapabilityGrant{
			Resource: capability.Resource,
			Actions:  append([]string(nil), capability.Actions...),
		}
	}
	return &dto.EffectiveCapabilitiesResponse{
		Revision:     view.revision,
		Capabilities: cloned,
	}
}

func policyDTOsForRole(rules []authcasbin.PolicyRule, roleKey string) []dto.PolicyRule {
	result := make([]dto.PolicyRule, 0)
	subject := roleSubject(roleKey)
	for _, rule := range rules {
		if rule.Type == "p" && rule.V0 == subject {
			result = append(result, dto.PolicyRule{
				RoleKey:  roleKey,
				Resource: rule.V1,
				Action:   rule.V2,
			})
		}
	}
	slices.SortFunc(result, comparePolicyDTO)
	return result
}

func roleKeysForUser(rules []authcasbin.PolicyRule, userID uuid.UUID) []string {
	result := make([]string, 0)
	subject := userSubject(userID.String())
	for _, rule := range rules {
		if rule.Type == "g" && rule.V0 == subject {
			if role, ok := parseRoleSubject(rule.V1); ok {
				result = append(result, role)
			}
		}
	}
	slices.Sort(result)
	return result
}

func policyAuditDocument(revision int64, rules []dto.PolicyRule) map[string]any {
	auditRules := make([]map[string]any, len(rules))
	for index, rule := range rules {
		auditRules[index] = map[string]any{
			"resource": rule.Resource,
			"action":   rule.Action,
		}
	}
	document := map[string]any{"rules": auditRules}
	if revision > 0 {
		document["revision"] = revision
	}
	return document
}

func userRoleAuditDocument(roles []string) map[string]any {
	return map[string]any{"roles": append([]string(nil), roles...)}
}

func captureMutationActor(ctx context.Context, required bool) (*authentication.Principal, error) {
	if ctx == nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", authorization.ErrInvalidRequest)
	}
	_, carried := authentication.FromContext(ctx)
	principal, valid := authctx.Principal(ctx)
	if !carried && !required {
		return nil, nil
	}
	if !valid {
		return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", authorization.ErrInvalidRequest)
	}
	copy := principal
	return &copy, nil
}

func auditActor(principal *authentication.Principal) (*uuid.UUID, *uuid.UUID) {
	if principal == nil {
		return nil, nil
	}
	userID, userErr := uuid.Parse(principal.UserID)
	sessionID, sessionErr := uuid.Parse(principal.SessionID)
	if userErr != nil || sessionErr != nil || userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, nil
	}
	return &userID, &sessionID
}

func mapAuthorizationMutationError(err error, revision int64) error {
	if err == nil {
		return nil
	}
	var appError *apperr.AppError
	if errors.As(err, &appError) {
		return err
	}
	switch {
	case errors.Is(err, ErrFinalSuperadmin):
		return finalSuperadminConflict(revision)
	case errors.Is(err, ErrAuthorizationRevisionConflict),
		errors.Is(err, ports.ErrAuthorizationRevisionConflict),
		errors.Is(err, ErrAuthorizationCommandConflict),
		errors.Is(err, ErrProtectedAuthorizationRole),
		errors.Is(err, ErrAuthorizationSelfLockout):
		return conflictAuthorization(err, revision)
	case errors.Is(err, ErrAuthorizationPrivilegeEscalation),
		errors.Is(err, ErrUnauthorizedSystemAuthorizationMutation):
		return apperr.New(apperr.CodeForbidden, "authorization change exceeds actor authority", err)
	case errors.Is(err, ErrInvalidAuthorizationMutation),
		errors.Is(err, ports.ErrInvalidAuthorizationPolicy):
		return badAuthorizationRequest(err)
	default:
		return err
	}
}

func badAuthorizationRequest(cause error) error {
	return apperr.New(apperr.CodeBadRequest, "invalid authorization change", cause)
}

func conflictAuthorization(cause error, revision int64) error {
	return apperr.New(apperr.CodeConflict, "authorization changed; reload and try again", cause).
		WithDetails(map[string]any{"current_revision": revision})
}

func finalSuperadminConflict(revision int64) error {
	return apperr.New(
		apperr.CodeConflict,
		"at least one active superadministrator is required",
		ErrFinalSuperadmin,
	).WithDetails(map[string]any{
		"reason":           "final_superadmin_required",
		"current_revision": revision,
	})
}

func ctxError(ctx context.Context) error {
	if ctx == nil {
		return context.Canceled
	}
	return ctx.Err()
}

var (
	_ ports.AuthorizationService       = (*authorizationService)(nil)
	_ ports.AuthorizationCatalogReader = (*authorizationService)(nil)
	_ ports.EffectiveCapabilityReader  = (*authorizationService)(nil)
)
