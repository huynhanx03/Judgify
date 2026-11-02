// Package authorizationadmin coordinates reviewed authorization commands.
// Route capabilities remain the HTTP layer's responsibility; this workflow
// binds one semantic preview to the exact live actor, session and command.
package authorizationadmin

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
	identityentity "github.com/huynhanx03/judgify/internal/identity/domain"
	identityports "github.com/huynhanx03/judgify/internal/ports"
)

const (
	rolePolicyAction     = "authorization.role_policies.replace"
	userRoleAction       = "authorization.user_roles.replace"
	confirmationResource = "authorization"
)

var ErrInvalidWorkflow = errors.New("authorization admin: invalid workflow")

type Config struct {
	RecentAuthenticationTTL time.Duration
}

type RolePolicyRequest struct {
	RoleID            uuid.UUID
	CommandID         uuid.UUID
	ExpectedRevision  int64
	Reason            string
	Rules             []identitydto.PolicyRule
	ConfirmationToken string
	ActorID           uuid.UUID
	SessionID         uuid.UUID
}

type UserRoleRequest struct {
	UserID            uuid.UUID
	CommandID         uuid.UUID
	ExpectedRevision  int64
	Reason            string
	Roles             []string
	ConfirmationToken string
	ActorID           uuid.UUID
	SessionID         uuid.UUID
}

type roleReader interface {
	Get(context.Context, uuid.UUID) (*identitydto.RoleResponse, error)
}

type userReader interface {
	Get(context.Context, uuid.UUID) (*identityentity.User, error)
}

type authorizationCommands interface {
	PreviewRolePolicies(context.Context, string, int64, []identitydto.PolicyRule) (*identitydto.RolePolicyPreview, error)
	ApplyRolePolicies(context.Context, string, identityports.ReplaceRolePoliciesCommand) (*identitydto.RolePolicyMutationReceipt, error)
	PreviewUserRoles(context.Context, uuid.UUID, int64, []string) (*identitydto.UserRolePreview, error)
	ApplyUserRoles(context.Context, uuid.UUID, identityports.ReplaceUserRolesCommand) (*identitydto.UserRoleMutationReceipt, error)
}

type confirmationProtector interface {
	IssueWithClaims(admincommand.ConfirmationExpectation) (string, admincommand.ConfirmationClaims, error)
	Verify(string, admincommand.ConfirmationExpectation) (admincommand.ConfirmationClaims, error)
}

type Workflow struct {
	roles         roleReader
	users         userReader
	authorization authorizationCommands
	confirmations confirmationProtector
	config        Config
	now           func() time.Time
}

func New(
	roles roleReader,
	users userReader,
	authorization authorizationCommands,
	confirmations confirmationProtector,
	config Config,
) (*Workflow, error) {
	if roles == nil || users == nil || authorization == nil || confirmations == nil ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, ErrInvalidWorkflow
	}
	return &Workflow{
		roles:         roles,
		users:         users,
		authorization: authorization,
		confirmations: confirmations,
		config:        config,
		now:           func() time.Time { return time.Now().UTC() },
	}, nil
}

func (workflow *Workflow) PreviewRolePolicies(
	ctx context.Context,
	request RolePolicyRequest,
) (*identitydto.RolePolicyPreviewResponse, error) {
	request, expectation, err := workflow.prepareRolePolicyRequest(ctx, request, false)
	if err != nil {
		return nil, err
	}
	role, err := workflow.roles.Get(ctx, request.RoleID)
	if err != nil {
		return nil, err
	}
	if !validRoleTarget(role, request.RoleID) {
		return nil, ErrInvalidWorkflow
	}
	preview, err := workflow.authorization.PreviewRolePolicies(
		ctx,
		role.Key,
		request.ExpectedRevision,
		request.Rules,
	)
	if err != nil {
		return nil, err
	}
	if !validRolePolicyPreview(preview, role.Key, request) {
		return nil, ErrInvalidWorkflow
	}
	token, claims, err := workflow.confirmations.IssueWithClaims(expectation)
	if err != nil {
		return nil, err
	}
	return &identitydto.RolePolicyPreviewResponse{
		Preview:           *preview,
		ConfirmationToken: token,
		ExpiresAt:         claims.ExpiresAt.UTC(),
	}, nil
}

func (workflow *Workflow) ApplyRolePolicies(
	ctx context.Context,
	request RolePolicyRequest,
) (*identitydto.RolePolicyMutationReceipt, error) {
	request, expectation, err := workflow.prepareRolePolicyRequest(ctx, request, true)
	if err != nil {
		return nil, err
	}
	if err := admincommand.RequireRecentAuthentication(
		ctx,
		request.ActorID,
		request.SessionID,
		workflow.now().UTC(),
		workflow.config.RecentAuthenticationTTL,
	); err != nil {
		return nil, err
	}
	if _, err := workflow.confirmations.Verify(
		request.ConfirmationToken,
		expectation,
	); err != nil {
		return nil, err
	}
	role, err := workflow.roles.Get(ctx, request.RoleID)
	if err != nil {
		return nil, err
	}
	if !validRoleTarget(role, request.RoleID) {
		return nil, ErrInvalidWorkflow
	}
	receipt, err := workflow.authorization.ApplyRolePolicies(
		ctx,
		role.Key,
		identityports.ReplaceRolePoliciesCommand{
			CommandID:        request.CommandID,
			ExpectedRevision: request.ExpectedRevision,
			Reason:           request.Reason,
			RequestHash:      expectation.RequestHash,
			Rules:            request.Rules,
		},
	)
	if err != nil {
		return nil, err
	}
	if !validRolePolicyReceipt(receipt, role.Key, request) {
		return nil, ErrInvalidWorkflow
	}
	return receipt, nil
}

func (workflow *Workflow) PreviewUserRoles(
	ctx context.Context,
	request UserRoleRequest,
) (*identitydto.UserRolePreviewResponse, error) {
	request, expectation, err := workflow.prepareUserRoleRequest(
		ctx,
		request,
		false,
	)
	if err != nil {
		return nil, err
	}
	user, err := workflow.users.Get(ctx, request.UserID)
	if err != nil {
		return nil, err
	}
	if !validUserTarget(user, request.UserID) {
		return nil, ErrInvalidWorkflow
	}
	preview, err := workflow.authorization.PreviewUserRoles(
		ctx,
		request.UserID,
		request.ExpectedRevision,
		request.Roles,
	)
	if err != nil {
		return nil, err
	}
	if !validUserRolePreview(preview, request) {
		return nil, ErrInvalidWorkflow
	}
	token, claims, err := workflow.confirmations.IssueWithClaims(expectation)
	if err != nil {
		return nil, err
	}
	return &identitydto.UserRolePreviewResponse{
		Preview:           *preview,
		ConfirmationToken: token,
		ExpiresAt:         claims.ExpiresAt.UTC(),
	}, nil
}

func (workflow *Workflow) ApplyUserRoles(
	ctx context.Context,
	request UserRoleRequest,
) (*identitydto.UserRoleMutationReceipt, error) {
	request, expectation, err := workflow.prepareUserRoleRequest(
		ctx,
		request,
		true,
	)
	if err != nil {
		return nil, err
	}
	if err := admincommand.RequireRecentAuthentication(
		ctx,
		request.ActorID,
		request.SessionID,
		workflow.now().UTC(),
		workflow.config.RecentAuthenticationTTL,
	); err != nil {
		return nil, err
	}
	if _, err := workflow.confirmations.Verify(
		request.ConfirmationToken,
		expectation,
	); err != nil {
		return nil, err
	}
	user, err := workflow.users.Get(ctx, request.UserID)
	if err != nil {
		return nil, err
	}
	if !validUserTarget(user, request.UserID) {
		return nil, ErrInvalidWorkflow
	}
	receipt, err := workflow.authorization.ApplyUserRoles(
		ctx,
		request.UserID,
		identityports.ReplaceUserRolesCommand{
			CommandID:        request.CommandID,
			ExpectedRevision: request.ExpectedRevision,
			Reason:           request.Reason,
			RequestHash:      expectation.RequestHash,
			Roles:            request.Roles,
		},
	)
	if err != nil {
		return nil, err
	}
	if !validUserRoleReceipt(receipt, request) {
		return nil, ErrInvalidWorkflow
	}
	return receipt, nil
}

func (workflow *Workflow) prepareUserRoleRequest(
	ctx context.Context,
	request UserRoleRequest,
	requireConfirmation bool,
) (UserRoleRequest, admincommand.ConfirmationExpectation, error) {
	if workflow == nil || workflow.users == nil || workflow.authorization == nil ||
		workflow.confirmations == nil || ctx == nil || request.UserID == uuid.Nil {
		return UserRoleRequest{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	meta, err := admincommand.NormalizeCommandMeta(
		admincommand.CommandMeta{
			ActorID:           request.ActorID,
			SessionID:         request.SessionID,
			ExpectedVersion:   request.ExpectedRevision,
			IdempotencyKey:    request.CommandID,
			Reason:            request.Reason,
			ConfirmationToken: request.ConfirmationToken,
		},
		admincommand.CommandMetaPolicy{
			RequireIdempotency:  true,
			RequireConfirmation: requireConfirmation,
		},
	)
	if err != nil {
		return UserRoleRequest{}, admincommand.ConfirmationExpectation{}, err
	}
	roles, err := canonicalRoleKeys(request.Roles)
	if err != nil {
		return UserRoleRequest{}, admincommand.ConfirmationExpectation{}, err
	}
	request.Reason = meta.Reason
	request.Roles = roles
	request.ConfirmationToken = meta.ConfirmationToken
	document, err := json.Marshal(struct {
		UserID           uuid.UUID `json:"user_id"`
		CommandID        uuid.UUID `json:"command_id"`
		ExpectedRevision int64     `json:"expected_revision"`
		Reason           string    `json:"reason"`
		Roles            []string  `json:"roles"`
	}{
		UserID:           request.UserID,
		CommandID:        request.CommandID,
		ExpectedRevision: request.ExpectedRevision,
		Reason:           request.Reason,
		Roles:            request.Roles,
	})
	if err != nil {
		return UserRoleRequest{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"authorization.user_roles.replace.v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return UserRoleRequest{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return request, admincommand.ConfirmationExpectation{
		ActorID:         meta.ActorID,
		SessionID:       meta.SessionID,
		IdempotencyKey:  meta.IdempotencyKey,
		Action:          userRoleAction,
		Resource:        confirmationResource,
		TargetID:        request.UserID.String(),
		ExpectedVersion: meta.ExpectedVersion,
		RequestHash:     hash,
	}, nil
}

func (workflow *Workflow) prepareRolePolicyRequest(
	ctx context.Context,
	request RolePolicyRequest,
	requireConfirmation bool,
) (RolePolicyRequest, admincommand.ConfirmationExpectation, error) {
	if workflow == nil || workflow.roles == nil || workflow.authorization == nil ||
		workflow.confirmations == nil || ctx == nil || request.RoleID == uuid.Nil {
		return RolePolicyRequest{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	meta, err := admincommand.NormalizeCommandMeta(
		admincommand.CommandMeta{
			ActorID:           request.ActorID,
			SessionID:         request.SessionID,
			ExpectedVersion:   request.ExpectedRevision,
			IdempotencyKey:    request.CommandID,
			Reason:            request.Reason,
			ConfirmationToken: request.ConfirmationToken,
		},
		admincommand.CommandMetaPolicy{
			RequireIdempotency:  true,
			RequireConfirmation: requireConfirmation,
		},
	)
	if err != nil {
		return RolePolicyRequest{}, admincommand.ConfirmationExpectation{}, err
	}
	rules, err := canonicalPolicyRules(request.Rules)
	if err != nil {
		return RolePolicyRequest{}, admincommand.ConfirmationExpectation{}, err
	}
	request.Reason = meta.Reason
	request.Rules = rules
	request.ConfirmationToken = meta.ConfirmationToken
	document, err := json.Marshal(struct {
		RoleID           uuid.UUID                `json:"role_id"`
		CommandID        uuid.UUID                `json:"command_id"`
		ExpectedRevision int64                    `json:"expected_revision"`
		Reason           string                   `json:"reason"`
		Rules            []identitydto.PolicyRule `json:"rules"`
	}{
		RoleID:           request.RoleID,
		CommandID:        request.CommandID,
		ExpectedRevision: request.ExpectedRevision,
		Reason:           request.Reason,
		Rules:            request.Rules,
	})
	if err != nil {
		return RolePolicyRequest{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	hash, err := admincommand.ConfirmationRequestHash(
		"authorization.role_policies.replace.v1",
		document,
	)
	if err != nil || hash == ([sha256.Size]byte{}) {
		return RolePolicyRequest{}, admincommand.ConfirmationExpectation{}, ErrInvalidWorkflow
	}
	return request, admincommand.ConfirmationExpectation{
		ActorID:         meta.ActorID,
		SessionID:       meta.SessionID,
		IdempotencyKey:  meta.IdempotencyKey,
		Action:          rolePolicyAction,
		Resource:        confirmationResource,
		TargetID:        request.RoleID.String(),
		ExpectedVersion: meta.ExpectedVersion,
		RequestHash:     hash,
	}, nil
}

func canonicalPolicyRules(rules []identitydto.PolicyRule) ([]identitydto.PolicyRule, error) {
	if len(rules) > 256 {
		return nil, ErrInvalidWorkflow
	}
	canonical := make([]identitydto.PolicyRule, len(rules))
	for index, rule := range rules {
		if strings.TrimSpace(rule.Resource) != rule.Resource || rule.Resource == "" ||
			strings.TrimSpace(rule.Action) != rule.Action || rule.Action == "" {
			return nil, ErrInvalidWorkflow
		}
		canonical[index] = identitydto.PolicyRule{
			Resource: rule.Resource,
			Action:   rule.Action,
		}
	}
	slices.SortFunc(canonical, func(left, right identitydto.PolicyRule) int {
		if compared := strings.Compare(left.Resource, right.Resource); compared != 0 {
			return compared
		}
		return strings.Compare(left.Action, right.Action)
	})
	for index := 1; index < len(canonical); index++ {
		if canonical[index-1].Resource == canonical[index].Resource &&
			canonical[index-1].Action == canonical[index].Action {
			return nil, ErrInvalidWorkflow
		}
	}
	return canonical, nil
}

func canonicalRoleKeys(roles []string) ([]string, error) {
	if len(roles) == 0 || len(roles) > 64 {
		return nil, ErrInvalidWorkflow
	}
	canonical := append([]string(nil), roles...)
	for _, role := range canonical {
		if !validCanonicalRoleKey(role) {
			return nil, ErrInvalidWorkflow
		}
	}
	slices.Sort(canonical)
	for index := 1; index < len(canonical); index++ {
		if canonical[index-1] == canonical[index] {
			return nil, ErrInvalidWorkflow
		}
	}
	return canonical, nil
}

func validCanonicalRoleKey(value string) bool {
	if len(value) < 2 || len(value) > 50 || strings.TrimSpace(value) != value ||
		value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		current := value[index]
		if (current < 'a' || current > 'z') &&
			(current < '0' || current > '9') && current != '_' &&
			current != '.' && current != '-' {
			return false
		}
	}
	return true
}

func validRoleTarget(role *identitydto.RoleResponse, roleID uuid.UUID) bool {
	return role != nil && role.ID == roleID && role.Key != "" &&
		role.Status == "active" && role.Version > 0 && !role.IsProtected
}

func validUserTarget(user *identityentity.User, userID uuid.UUID) bool {
	return user != nil && user.ID == userID && user.Username != "" &&
		user.SecurityStatus == identityentity.UserSecurityStatusActive &&
		user.SecurityVersion > 0
}

func validRolePolicyPreview(
	preview *identitydto.RolePolicyPreview,
	roleKey string,
	request RolePolicyRequest,
) bool {
	return preview != nil && preview.RoleKey == roleKey &&
		preview.ExpectedRevision == request.ExpectedRevision &&
		preview.ResultingRevision == request.ExpectedRevision+1 &&
		preview.Before.Revision == request.ExpectedRevision &&
		preview.After.Revision == request.ExpectedRevision+1 &&
		policyRulesEqual(preview.After.Rules, request.Rules) &&
		policyRulesEqual(
			preview.Added,
			policyRuleDifference(preview.After.Rules, preview.Before.Rules),
		) &&
		policyRulesEqual(
			preview.Removed,
			policyRuleDifference(preview.Before.Rules, preview.After.Rules),
		)
}

func validRolePolicyReceipt(
	receipt *identitydto.RolePolicyMutationReceipt,
	roleKey string,
	request RolePolicyRequest,
) bool {
	return receipt != nil && receipt.CommandID == request.CommandID &&
		receipt.EventID == request.CommandID && receipt.RoleKey == roleKey &&
		receipt.Snapshot.Revision == request.ExpectedRevision+1 &&
		!receipt.CommittedAt.IsZero()
}

func validUserRolePreview(
	preview *identitydto.UserRolePreview,
	request UserRoleRequest,
) bool {
	return preview != nil && preview.UserID == request.UserID &&
		preview.ExpectedRevision == request.ExpectedRevision &&
		preview.ResultingRevision == request.ExpectedRevision+1 &&
		preview.Before.Revision == request.ExpectedRevision &&
		preview.After.Revision == request.ExpectedRevision+1 &&
		slices.Equal(preview.After.Roles, request.Roles) &&
		slices.Equal(
			preview.Added,
			roleKeyDifference(preview.After.Roles, preview.Before.Roles),
		) &&
		slices.Equal(
			preview.Removed,
			roleKeyDifference(preview.Before.Roles, preview.After.Roles),
		)
}

func validUserRoleReceipt(
	receipt *identitydto.UserRoleMutationReceipt,
	request UserRoleRequest,
) bool {
	return receipt != nil && receipt.CommandID == request.CommandID &&
		receipt.EventID == request.CommandID && receipt.UserID == request.UserID &&
		receipt.Snapshot.Revision == request.ExpectedRevision+1 &&
		slices.Equal(receipt.Snapshot.Roles, request.Roles) &&
		!receipt.CommittedAt.IsZero()
}

func policyRulesEqual(left, right []identitydto.PolicyRule) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Resource != right[index].Resource ||
			left[index].Action != right[index].Action {
			return false
		}
	}
	return true
}

func policyRuleDifference(
	left, right []identitydto.PolicyRule,
) []identitydto.PolicyRule {
	rightSet := make(map[string]struct{}, len(right))
	for _, rule := range right {
		rightSet[rule.Resource+"\x00"+rule.Action] = struct{}{}
	}
	difference := make([]identitydto.PolicyRule, 0, len(left))
	for _, rule := range left {
		if _, found := rightSet[rule.Resource+"\x00"+rule.Action]; !found {
			difference = append(difference, rule)
		}
	}
	return difference
}

func roleKeyDifference(left, right []string) []string {
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
