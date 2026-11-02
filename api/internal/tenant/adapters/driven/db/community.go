package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authorizationstate"
	"github.com/huynhanx03/judgify/internal/ent/generate/tenant"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/tenant/domain"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

// BootstrapCommunity initializes the baseline tenant membership and scoped
// roles for the configured administrator. It is called inside bootstrap's
// existing transaction after the global account has been created.
func BootstrapCommunity(ctx context.Context, client *generate.Client, ownerID uuid.UUID) error {
	community, err := client.Tenant.Get(ctx, domain.CommunityID)
	if generate.IsNotFound(err) {
		community, err = client.Tenant.Create().
			SetID(domain.CommunityID).
			SetName(domain.CommunityName).
			SetSlug(domain.CommunitySlug).
			SetStatus(tenant.StatusActive).
			Save(ctx)
	}
	if err != nil {
		return fmt.Errorf("ensure community tenant: %w", err)
	}
	if community.Status != tenant.StatusActive {
		return fmt.Errorf("bootstrap community tenant is missing or inactive")
	}
	if err := createCommunityRoleDefinitions(ctx, client); err != nil {
		return err
	}
	if err := createMembership(ctx, client, domain.CommunityID, ownerID, time.Now().UTC()); err != nil {
		return fmt.Errorf("create community owner membership: %w", err)
	}
	if err := createRoleAssignment(ctx, client, ownerID, domain.CommunityID, "owner"); err != nil {
		return err
	}
	return seedRolePolicies(ctx, client, domain.CommunityID)
}

func createCommunityRoleDefinitions(ctx context.Context, client *generate.Client) error {
	for _, role := range []struct{ key, name string }{{"owner", "Tông chủ"}, {"member", "Đệ tử"}} {
		if _, err := client.TenantRole.Create().SetTenantID(domain.CommunityID).
			SetKey(role.key).SetName(role.name).SetIsSystem(true).Save(ctx); err != nil {
			return fmt.Errorf("create community role %s: %w", role.key, err)
		}
	}
	return nil
}

func createMembership(ctx context.Context, client *generate.Client, tenantID, userID uuid.UUID, joinedAt time.Time) error {
	return client.TenantMembership.Create().SetTenantID(tenantID).SetUserID(userID).SetJoinedAt(joinedAt).Exec(ctx)
}

func createRoleAssignment(ctx context.Context, client *generate.Client, userID, tenantID uuid.UUID, role string) error {
	if err := client.CasbinRule.Create().SetPtype("g").SetV0(userSubject(userID)).
		SetV1("role:" + role).SetV2(tenantDomain(tenantID)).Exec(ctx); err != nil {
		return fmt.Errorf("assign community %s role: %w", role, err)
	}
	return nil
}

func seedRolePolicies(ctx context.Context, client *generate.Client, tenantID uuid.UUID) error {
	domainID := tenantDomain(tenantID)
	for _, resource := range permissions.Catalog() {
		for _, action := range resource.Actions {
			if err := client.CasbinRule.Create().SetPtype("p").SetV0("role:owner").
				SetV1(domainID).SetV2(resource.Key).SetV3(action).Exec(ctx); err != nil {
				return fmt.Errorf("seed community owner policy: %w", err)
			}
			if action == permissions.PermissionScopeRead {
				if err := client.CasbinRule.Create().SetPtype("p").SetV0("role:member").
					SetV1(domainID).SetV2(resource.Key).SetV3(action).Exec(ctx); err != nil {
					return fmt.Errorf("seed community member policy: %w", err)
				}
			}
		}
	}
	return nil
}

func (r *TenantRepository) JoinCommunity(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return fmt.Errorf("join community: invalid account")
	}
	if err := createMembership(ctx, r.database.DB(ctx), domain.CommunityID, userID, time.Now().UTC()); err != nil {
		return fmt.Errorf("join community membership: %w", err)
	}
	if err := createRoleAssignment(ctx, r.database.DB(ctx), userID, domain.CommunityID, "member"); err != nil {
		return err
	}
	auditID, err := audit.NewRepository().Append(ctx, audit.Entry{
		ActorUserID: &userID,
		Action:      "tenant.membership.join",
		Resource:    "tenant",
		ResourceID:  domain.CommunityID.String(),
		Reason:      "new account joined Judgify Community",
		Before:      map[string]any{},
		After:       map[string]any{"tenant_id": domain.CommunityID.String(), "user_id": userID.String(), "role": "member"},
	})
	if err != nil {
		return fmt.Errorf("audit community membership: %w", err)
	}
	return advanceTenantAuthorization(ctx, r.database.DB(ctx), userID, domain.CommunityID, "member", auditID)
}

func advanceTenantAuthorization(ctx context.Context, client *generate.Client, userID, tenantID uuid.UUID, role string, auditID uuid.UUID) error {
	state, err := client.AuthorizationState.Query().Where(authorizationstate.ID("default")).Only(ctx)
	if err != nil {
		return fmt.Errorf("load authorization revision: %w", err)
	}
	nextRevision := state.DesiredRevision + 1
	if _, err := client.AuthorizationState.UpdateOneID(state.ID).
		Where(authorizationstate.DesiredRevisionEQ(state.DesiredRevision)).
		SetDesiredRevision(nextRevision).SetUpdatedBy(userID).Save(ctx); err != nil {
		return fmt.Errorf("advance authorization revision: %w", err)
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("allocate authorization event: %w", err)
	}
	payload, err := json.Marshal(map[string]any{
		"action": "authorization.user_roles.replace", "resource": "authorization.user",
		"target_id": userID.String(), "expected_revision": state.DesiredRevision,
		"resulting_revision": nextRevision, "audit_id": auditID.String(),
		"result": map[string]any{"tenant_id": tenantID.String(), "tenant_role": role},
	})
	if err != nil {
		return fmt.Errorf("encode tenant authorization event: %w", err)
	}
	if err := eventing.NewOutbox().Append(ctx, eventing.Event{
		ID: eventID, Type: "identity.authorization.user_roles_replaced.v1",
		AggregateType:    "authorization",
		AggregateID:      uuid.MustParse("00000000-0000-5000-8000-000000000001"),
		AggregateVersion: nextRevision, Payload: payload, OccurredAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("append tenant authorization event: %w", err)
	}
	return nil
}
