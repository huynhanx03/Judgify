package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"

	identityconstant "github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

const (
	protectedSuperadminRoleKey = identityconstant.ProtectedSuperadminRoleKey
	maxRolePolicyRules         = 256
	maxAuthorizationReason     = 500
)

var (
	ErrInvalidAuthorizationMutation            = errors.New("authorization: invalid mutation")
	ErrAuthorizationRevisionConflict           = errors.New("authorization: revision conflict")
	ErrAuthorizationCommandConflict            = errors.New("authorization: command identity conflict")
	ErrProtectedAuthorizationRole              = errors.New("authorization: protected role")
	ErrFinalSuperadmin                         = errors.New("authorization: final active superadmin required")
	ErrAuthorizationSelfLockout                = errors.New("authorization: actor self-lockout")
	ErrAuthorizationPrivilegeEscalation        = errors.New("authorization: mutation exceeds actor authority")
	ErrUnauthorizedSystemAuthorizationMutation = errors.New("authorization: unauthorized system mutation")
)

func normalizeRolePolicies(roleKey string, rules []dto.PolicyRule) ([]dto.PolicyRule, error) {
	if strings.TrimSpace(roleKey) != roleKey || !validRoleKey(roleKey) || len(rules) > maxRolePolicyRules {
		return nil, ErrInvalidAuthorizationMutation
	}

	normalized := make([]dto.PolicyRule, 0, len(rules))
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		resource := strings.TrimSpace(rule.Resource)
		action := strings.TrimSpace(rule.Action)
		wildcard := resource == "*" || action == "*"
		if wildcard {
			if roleKey != protectedSuperadminRoleKey || resource != "*" || action != "*" {
				return nil, ErrInvalidAuthorizationMutation
			}
		} else if !permissions.Contains(resource, action) {
			return nil, ErrInvalidAuthorizationMutation
		}

		key := resource + "\x00" + action
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrInvalidAuthorizationMutation
		}
		seen[key] = struct{}{}
		normalized = append(normalized, dto.PolicyRule{
			RoleKey:  roleKey,
			Resource: resource,
			Action:   action,
		})
	}
	slices.SortFunc(normalized, comparePolicyDTO)
	return normalized, nil
}

func normalizeRoleKeys(roles []string) ([]string, error) {
	if len(roles) > 64 {
		return nil, ErrInvalidAuthorizationMutation
	}
	normalized := make([]string, 0, len(roles))
	seen := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if !validRoleKey(role) {
			return nil, ErrInvalidAuthorizationMutation
		}
		if _, duplicate := seen[role]; duplicate {
			return nil, ErrInvalidAuthorizationMutation
		}
		seen[role] = struct{}{}
		normalized = append(normalized, role)
	}
	slices.Sort(normalized)
	return normalized, nil
}

func normalizeReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > maxAuthorizationReason || !utf8.ValidString(reason) {
		return "", ErrInvalidAuthorizationMutation
	}
	for _, character := range reason {
		if unicode.IsControl(character) {
			return "", ErrInvalidAuthorizationMutation
		}
	}
	return reason, nil
}

func validRoleKey(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' ||
			index > 0 && character >= '0' && character <= '9' ||
			index > 0 && (character == '_' || character == '-') {
			continue
		}
		return false
	}
	return true
}

func comparePolicyDTO(left, right dto.PolicyRule) int {
	if compared := strings.Compare(left.Resource, right.Resource); compared != 0 {
		return compared
	}
	return strings.Compare(left.Action, right.Action)
}

func validateAuthorizationSnapshot(
	ctx context.Context,
	repository ports.AuthorizationPolicyRepository,
	snapshot authcasbin.Snapshot,
	actor *authentication.Principal,
) error {
	if ctx == nil || repository == nil || snapshot.Revision <= 0 {
		return ErrInvalidAuthorizationMutation
	}
	roles := make(map[string]struct{})
	users := make(map[uuid.UUID]struct{})
	roleGraph := make(map[string][]string)
	superadminUsers := make(map[uuid.UUID]struct{})
	hasSuperadminWildcard := false

	for _, rule := range snapshot.Rules {
		if rule.V3 != "" || rule.V4 != "" || rule.V5 != "" {
			return ErrInvalidAuthorizationMutation
		}
		switch rule.Type {
		case "p":
			if rule.V2 == "" {
				return ErrInvalidAuthorizationMutation
			}
			roleKey, ok := parseRoleSubject(rule.V0)
			if !ok || rule.V1 == "" {
				return ErrInvalidAuthorizationMutation
			}
			roles[roleKey] = struct{}{}
			if rule.V1 == "*" || rule.V2 == "*" {
				if roleKey != protectedSuperadminRoleKey || rule.V1 != "*" || rule.V2 != "*" {
					return ErrInvalidAuthorizationMutation
				}
				hasSuperadminWildcard = true
			} else if !permissions.Contains(rule.V1, rule.V2) {
				return ErrInvalidAuthorizationMutation
			}
		case "g":
			if rule.V2 != "" {
				return ErrInvalidAuthorizationMutation
			}
			parentKey, ok := parseRoleSubject(rule.V1)
			if !ok {
				return ErrInvalidAuthorizationMutation
			}
			roles[parentKey] = struct{}{}
			if childKey, roleChild := parseRoleSubject(rule.V0); roleChild {
				if parentKey == protectedSuperadminRoleKey {
					return ErrInvalidAuthorizationMutation
				}
				roles[childKey] = struct{}{}
				roleGraph[childKey] = append(roleGraph[childKey], parentKey)
				continue
			}
			userID, ok := parseUserSubject(rule.V0)
			if !ok {
				return ErrInvalidAuthorizationMutation
			}
			users[userID] = struct{}{}
			if parentKey == protectedSuperadminRoleKey {
				superadminUsers[userID] = struct{}{}
			}
		default:
			return ErrInvalidAuthorizationMutation
		}
	}
	if !hasSuperadminWildcard ||
		hasRoleCycle(roleGraph) ||
		roleGraphExceedsMaxParentEdges(
			roleGraph,
			authcasbin.DefaultMaxRoleHierarchyDepth-1,
		) {
		return ErrInvalidAuthorizationMutation
	}

	roleKeys := mapKeys(roles)
	activeRoles, err := repository.ActiveRoleKeys(ctx, roleKeys)
	if err != nil {
		return fmt.Errorf("validate active authorization roles: %w", err)
	}
	for _, key := range roleKeys {
		if !activeRoles[key] {
			return ErrInvalidAuthorizationMutation
		}
	}

	userIDs := uuidMapKeys(users)
	eligibleUsers, err := repository.AuthorizationEligibleUserIDs(ctx, userIDs)
	if err != nil {
		return fmt.Errorf("validate authorization-eligible users: %w", err)
	}
	for _, userID := range userIDs {
		if !eligibleUsers[userID] {
			return ErrInvalidAuthorizationMutation
		}
	}
	activeUsers, err := repository.ActiveUserIDs(ctx, uuidMapKeys(superadminUsers))
	if err != nil {
		return fmt.Errorf("validate active authorization superadmins: %w", err)
	}
	activeSuperadmins := 0
	for userID := range superadminUsers {
		if activeUsers[userID] {
			activeSuperadmins++
		}
	}
	if activeSuperadmins == 0 {
		return ErrFinalSuperadmin
	}

	_, err = authcasbin.NewRuntime(snapshot)
	if err != nil {
		return fmt.Errorf("validate casbin snapshot: %w", err)
	}
	return nil
}

// validateAuthorizationTransition applies actor-relative invariants that a
// structurally valid Casbin snapshot cannot express by itself. Middleware is
// only the first guard: every service mutation must independently prove that
// the actor owns authorization:manage and cannot manufacture authority it did
// not have before the transaction.
func validateAuthorizationTransition(
	ctx context.Context,
	before authcasbin.Snapshot,
	after authcasbin.Snapshot,
	actor *authentication.Principal,
	systemInitialUser *uuid.UUID,
) error {
	if ctx == nil || before.Revision <= 0 || after.Revision != before.Revision+1 {
		return ErrInvalidAuthorizationMutation
	}
	if actor == nil {
		return validateSystemInitialRoleAssignment(before, after, systemInitialUser)
	}
	if systemInitialUser != nil {
		return ErrUnauthorizedSystemAuthorizationMutation
	}

	actorUserID, err := canonicalUserID(*actor)
	if err != nil {
		return ErrAuthorizationPrivilegeEscalation
	}
	principal := *actor
	principal.Subject = userSubject(actorUserID)

	beforeRuntime, err := authcasbin.NewRuntime(before)
	if err != nil {
		return fmt.Errorf("compile authorization transition base: %w", err)
	}
	afterRuntime, err := authcasbin.NewRuntime(after)
	if err != nil {
		return fmt.Errorf("compile authorization transition candidate: %w", err)
	}

	allowed, err := runtimeAllows(
		ctx,
		beforeRuntime,
		principal,
		permissions.ResourceKeyAuthorization,
		permissions.PermissionScopeManage,
	)
	if err != nil || !allowed {
		return ErrAuthorizationPrivilegeEscalation
	}

	added, removed := authorizationRuleDelta(before.Rules, after.Rules)
	directSuperadmin := hasDirectRole(before.Rules, principal.Subject, protectedSuperadminRoleKey)
	if touchesProtectedSuperadmin(added) || touchesProtectedSuperadmin(removed) {
		if !directSuperadmin {
			return ErrAuthorizationPrivilegeEscalation
		}
	}

	for _, rule := range added {
		if rule.Type != "p" {
			continue
		}
		if rule.V1 == "*" || rule.V2 == "*" {
			if !directSuperadmin {
				return ErrAuthorizationPrivilegeEscalation
			}
			continue
		}
		allowed, authorizeErr := runtimeAllows(ctx, beforeRuntime, principal, rule.V1, rule.V2)
		if authorizeErr != nil || !allowed {
			return ErrAuthorizationPrivilegeEscalation
		}
	}

	// A role assignment can grant through role inheritance, so comparing only
	// the literal grouping rule is insufficient. Check every newly affected
	// subject against the closed, code-owned capability catalog.
	for _, subject := range subjectsWithAddedGrouping(added) {
		target := transitionPrincipal(subject)
		for _, resource := range permissions.Catalog() {
			for _, action := range resource.Actions {
				wasAllowed, beforeErr := runtimeAllows(ctx, beforeRuntime, target, resource.Key, action)
				isAllowed, afterErr := runtimeAllows(ctx, afterRuntime, target, resource.Key, action)
				if beforeErr != nil || afterErr != nil {
					return ErrAuthorizationPrivilegeEscalation
				}
				if wasAllowed || !isAllowed {
					continue
				}
				actorAllowed, actorErr := runtimeAllows(ctx, beforeRuntime, principal, resource.Key, action)
				if actorErr != nil || !actorAllowed {
					return ErrAuthorizationPrivilegeEscalation
				}
			}
		}
	}

	allowed, err = runtimeAllows(
		ctx,
		afterRuntime,
		principal,
		permissions.ResourceKeyAuthorization,
		permissions.PermissionScopeManage,
	)
	if err != nil || !allowed {
		return ErrAuthorizationSelfLockout
	}
	return nil
}

func validateSystemInitialRoleAssignment(
	before authcasbin.Snapshot,
	after authcasbin.Snapshot,
	userID *uuid.UUID,
) error {
	if userID == nil || *userID == uuid.Nil {
		return ErrUnauthorizedSystemAuthorizationMutation
	}
	subject := userSubject(userID.String())
	if len(roleKeysForSubject(before.Rules, subject)) != 0 {
		return ErrUnauthorizedSystemAuthorizationMutation
	}
	roles := roleKeysForSubject(after.Rules, subject)
	if len(roles) != 1 || roles[0] != defaultRoleKey {
		return ErrUnauthorizedSystemAuthorizationMutation
	}
	added, removed := authorizationRuleDelta(before.Rules, after.Rules)
	if len(removed) != 0 || len(added) != 1 ||
		added[0].Type != "g" || added[0].V0 != subject ||
		added[0].V1 != roleSubject(defaultRoleKey) || added[0].V2 != "" {
		return ErrUnauthorizedSystemAuthorizationMutation
	}
	return nil
}

func authorizationRuleDelta(
	before []authcasbin.PolicyRule,
	after []authcasbin.PolicyRule,
) (added []authcasbin.PolicyRule, removed []authcasbin.PolicyRule) {
	beforeSet := make(map[authcasbin.PolicyRule]struct{}, len(before))
	afterSet := make(map[authcasbin.PolicyRule]struct{}, len(after))
	for _, rule := range before {
		beforeSet[rule] = struct{}{}
	}
	for _, rule := range after {
		afterSet[rule] = struct{}{}
		if _, exists := beforeSet[rule]; !exists {
			added = append(added, rule)
		}
	}
	for _, rule := range before {
		if _, exists := afterSet[rule]; !exists {
			removed = append(removed, rule)
		}
	}
	return added, removed
}

func touchesProtectedSuperadmin(rules []authcasbin.PolicyRule) bool {
	protected := roleSubject(protectedSuperadminRoleKey)
	for _, rule := range rules {
		if rule.Type == "p" && rule.V0 == protected ||
			rule.Type == "g" && (rule.V0 == protected || rule.V1 == protected) {
			return true
		}
	}
	return false
}

func hasDirectRole(rules []authcasbin.PolicyRule, subject string, roleKey string) bool {
	role := roleSubject(roleKey)
	for _, rule := range rules {
		if rule.Type == "g" && rule.V0 == subject && rule.V1 == role && rule.V2 == "" {
			return true
		}
	}
	return false
}

func directRoleUserIDs(
	rules []authcasbin.PolicyRule,
	roleKey string,
) []uuid.UUID {
	role := roleSubject(roleKey)
	seen := make(map[uuid.UUID]struct{})
	for _, rule := range rules {
		if rule.Type != "g" || rule.V1 != role || rule.V2 != "" {
			continue
		}
		userID, ok := parseUserSubject(rule.V0)
		if ok {
			seen[userID] = struct{}{}
		}
	}
	return uuidMapKeys(seen)
}

func subjectsWithAddedGrouping(rules []authcasbin.PolicyRule) []string {
	seen := make(map[string]struct{})
	for _, rule := range rules {
		if rule.Type == "g" {
			seen[rule.V0] = struct{}{}
		}
	}
	return mapKeys(seen)
}

func roleKeysForSubject(rules []authcasbin.PolicyRule, subject string) []string {
	roles := make([]string, 0)
	for _, rule := range rules {
		if rule.Type != "g" || rule.V0 != subject {
			continue
		}
		if role, ok := parseRoleSubject(rule.V1); ok {
			roles = append(roles, role)
		}
	}
	slices.Sort(roles)
	return roles
}

func transitionPrincipal(subject string) authentication.Principal {
	return authentication.Principal{
		Authenticated:      true,
		Subject:            subject,
		AuthenticatedUntil: time.Now().UTC().Add(time.Minute),
	}
}

func runtimeAllows(
	ctx context.Context,
	runtime *authcasbin.Runtime,
	principal authentication.Principal,
	resource string,
	action string,
) (bool, error) {
	decision, err := runtime.Authorize(ctx, authorization.Request{
		Principal: principal,
		Resource:  resource,
		Action:    action,
	})
	return err == nil && decision.Allowed, err
}

func hasRoleCycle(graph map[string][]string) bool {
	const (
		unseen = iota
		visiting
		visited
	)
	state := make(map[string]int, len(graph))
	var visit func(string) bool
	visit = func(node string) bool {
		switch state[node] {
		case visiting:
			return true
		case visited:
			return false
		}
		state[node] = visiting
		for _, parent := range graph[node] {
			if visit(parent) {
				return true
			}
		}
		state[node] = visited
		return false
	}
	for node := range graph {
		if visit(node) {
			return true
		}
	}
	return false
}

// Users consume one Casbin hierarchy edge when they inherit their direct
// role. A role-only graph must therefore reserve that edge so the API
// projection and the runtime enforcer always reach exactly the same parents.
func roleGraphExceedsMaxParentEdges(
	graph map[string][]string,
	maximumEdges int,
) bool {
	if maximumEdges < 0 {
		return len(graph) != 0
	}
	var exceeds func(string, int) bool
	exceeds = func(role string, remainingEdges int) bool {
		for _, parent := range graph[role] {
			if remainingEdges == 0 || exceeds(parent, remainingEdges-1) {
				return true
			}
		}
		return false
	}
	for role := range graph {
		if exceeds(role, maximumEdges) {
			return true
		}
	}
	return false
}

func parseRoleSubject(subject string) (string, bool) {
	if !strings.HasPrefix(subject, "role:") {
		return "", false
	}
	key := strings.TrimPrefix(subject, "role:")
	return key, validRoleKey(key)
}

func parseUserSubject(subject string) (uuid.UUID, bool) {
	if !strings.HasPrefix(subject, "user:") {
		return uuid.Nil, false
	}
	raw := strings.TrimPrefix(subject, "user:")
	id, err := uuid.Parse(raw)
	return id, err == nil && id != uuid.Nil && id.String() == raw
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func uuidMapKeys(values map[uuid.UUID]struct{}) []uuid.UUID {
	keys := make([]uuid.UUID, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right uuid.UUID) int {
		return strings.Compare(left.String(), right.String())
	})
	return keys
}

func userSubject(userID string) string { return "user:" + userID }

func roleSubject(roleKey string) string { return "role:" + roleKey }
