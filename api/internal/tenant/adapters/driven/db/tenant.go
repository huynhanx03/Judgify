package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	cultivationdb "github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/casbinrule"
	"github.com/huynhanx03/judgify/internal/ent/generate/tenant"
	"github.com/huynhanx03/judgify/internal/ent/generate/tenantinvitation"
	"github.com/huynhanx03/judgify/internal/ent/generate/tenantmembership"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	identityports "github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/tenant/contract"
	"github.com/huynhanx03/judgify/internal/tenant/domain"
	"github.com/huynhanx03/judgify/internal/tenantctx"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

var ErrTenantOwnerRequired = errors.New("tenant: owner role required")

type TenantRepository struct{ database *ent.EntClient }

func NewTenantRepository(database *ent.EntClient) *TenantRepository {
	return &TenantRepository{database: database}
}

func (r *TenantRepository) FindForUser(ctx context.Context, userID uuid.UUID) ([]contract.TenantView, error) {
	views := make([]contract.TenantView, 0)
	err := r.database.WithPrincipalRead(ctx, userID, func(client *generate.Client) error {
		memberships, err := client.TenantMembership.Query().
			Where(tenantmembership.UserIDEQ(userID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).
			WithTenant().All(ctx)
		if err != nil {
			return fmt.Errorf("list tenant memberships: %w", err)
		}
		views = make([]contract.TenantView, 0, len(memberships))
		for _, membership := range memberships {
			if membership.Edges.Tenant == nil {
				return fmt.Errorf("tenant membership is missing tenant edge")
			}
			entity := membership.Edges.Tenant
			role := "member"
			isOwner, err := client.CasbinRule.Query().
				Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(userID)), casbinrule.V1EQ("role:owner"), casbinrule.V2EQ(tenantDomain(entity.ID))).
				Exist(ctx)
			if err != nil {
				return fmt.Errorf("read tenant role: %w", err)
			}
			if isOwner {
				role = "owner"
			}
			views = append(views, toView(entity, role, time.Now().UTC()))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return views, nil
}

func (r *TenantRepository) CreateForOwner(ctx context.Context, ownerID uuid.UUID, name, slug string) (view contract.TenantView, err error) {
	err = r.database.DoInTx(ctx, func(txctx context.Context) error {
		id := uuid.New()
		created, createErr := r.database.DB(txctx).Tenant.Create().SetID(id).SetName(name).SetSlug(slug).Save(txctx)
		if createErr != nil {
			return fmt.Errorf("create tenant: %w", createErr)
		}
		scope, scopeErr := tenantctx.Tenant(id)
		if scopeErr != nil {
			return fmt.Errorf("create new tenant scope: %w", scopeErr)
		}
		tenantCtx := tenantctx.WithContext(txctx, scope)
		client := r.database.DB(tenantCtx)
		if createErr = cultivationdb.InstallDefaultRewardRule(tenantCtx, r.database, id); createErr != nil {
			return fmt.Errorf("create tenant default reward rule: %w", createErr)
		}
		if _, createErr = client.TenantMembership.Create().
			SetTenantID(id).SetUserID(ownerID).SetJoinedAt(time.Now().UTC()).Save(tenantCtx); createErr != nil {
			return fmt.Errorf("create owner membership: %w", createErr)
		}
		profileSchemaID := uuid.New()
		if createErr = client.ProfileSchemaState.Create().
			SetID(profileSchemaID).
			SetKey(identityports.ProfileSchemaStateKey).
			SetRevision(1).
			Exec(tenantCtx); createErr != nil {
			return fmt.Errorf("create tenant profile schema state: %w", createErr)
		}
		rankingStateID := uuid.New()
		if createErr = client.CultivationRankingState.Create().
			SetID(rankingStateID).
			SetKey("global").
			SetRevision(0).
			Exec(tenantCtx); createErr != nil {
			return fmt.Errorf("create tenant cultivation ranking state: %w", createErr)
		}
		for _, role := range []struct{ key, name string }{{"owner", "Tông chủ"}, {"member", "Đệ tử"}} {
			if _, createErr = client.TenantRole.Create().SetTenantID(id).SetKey(role.key).SetName(role.name).SetIsSystem(true).Save(tenantCtx); createErr != nil {
				return fmt.Errorf("create tenant role: %w", createErr)
			}
		}
		domainID := tenantDomain(id)
		if _, createErr = client.CasbinRule.Create().
			SetPtype("g").SetV0(userSubject(ownerID)).SetV1("role:owner").SetV2(domainID).Save(tenantCtx); createErr != nil {
			return fmt.Errorf("assign tenant owner role: %w", createErr)
		}
		for _, resource := range permissions.Catalog() {
			for _, action := range resource.Actions {
				if _, createErr = client.CasbinRule.Create().
					SetPtype("p").SetV0("role:owner").SetV1(domainID).SetV2(resource.Key).SetV3(action).Save(tenantCtx); createErr != nil {
					return fmt.Errorf("seed tenant owner policy: %w", createErr)
				}
				if action == permissions.PermissionScopeRead {
					if _, createErr = client.CasbinRule.Create().
						SetPtype("p").SetV0("role:member").SetV1(domainID).SetV2(resource.Key).SetV3(action).Save(tenantCtx); createErr != nil {
						return fmt.Errorf("seed tenant member policy: %w", createErr)
					}
				}
			}
		}
		tenantAuditID, auditErr := audit.NewRepository().Append(tenantCtx, audit.Entry{
			ActorUserID: &ownerID,
			Action:      "tenant.create",
			Resource:    "tenant",
			ResourceID:  id.String(),
			Reason:      "tenant created by owner",
			Before:      map[string]any{},
			After:       map[string]any{"tenant_id": id.String(), "name": name, "slug": slug},
		})
		if auditErr != nil {
			return fmt.Errorf("audit tenant creation: %w", auditErr)
		}
		if createErr = advanceTenantAuthorization(tenantCtx, client, ownerID, id, "owner", tenantAuditID); createErr != nil {
			return fmt.Errorf("publish tenant authorization revision: %w", createErr)
		}
		view = toView(created, "owner", time.Now().UTC())
		return nil
	})
	return view, err
}

func (r *TenantRepository) RenameAsOwner(ctx context.Context, tenantID, ownerID uuid.UUID, name string, now time.Time) (view contract.TenantView, err error) {
	err = r.database.DoInTx(ctx, func(txctx context.Context) error {
		current, queryErr := r.database.DB(txctx).Tenant.Query().Where(tenant.IDEQ(tenantID), tenant.StatusEQ(tenant.StatusActive)).Only(txctx)
		if queryErr != nil {
			return fmt.Errorf("load tenant: %w", queryErr)
		}
		isOwner, queryErr := r.database.DB(txctx).CasbinRule.Query().
			Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(ownerID)), casbinrule.V1EQ("role:owner"), casbinrule.V2EQ(tenantDomain(tenantID))).
			Exist(txctx)
		if queryErr != nil {
			return fmt.Errorf("check tenant owner: %w", queryErr)
		}
		if !isOwner {
			return ErrTenantOwnerRequired
		}
		isActiveMember, queryErr := r.database.DB(txctx).TenantMembership.Query().
			Where(tenantmembership.TenantIDEQ(tenantID), tenantmembership.UserIDEQ(ownerID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).
			Exist(txctx)
		if queryErr != nil {
			return fmt.Errorf("check tenant membership: %w", queryErr)
		}
		if !isActiveMember {
			return ErrTenantOwnerRequired
		}
		if current.Name == name {
			view = toView(current, "owner", now)
			return nil
		}
		if !domain.CanRename(now, current.NameChangedAt) {
			return domain.ErrRenameCooldown
		}
		updated, updateErr := r.database.DB(txctx).Tenant.UpdateOneID(tenantID).
			Where(tenant.Or(tenant.NameChangedAtIsNil(), tenant.NameChangedAtLTE(now.Add(-domain.RenameCooldown)))).
			SetName(name).SetNameChangedAt(now).Save(txctx)
		if updateErr != nil {
			if generate.IsNotFound(updateErr) {
				return domain.ErrRenameCooldown
			}
			return fmt.Errorf("rename tenant (cooldown active or concurrent update): %w", updateErr)
		}
		if _, updateErr = audit.NewRepository().Append(txctx, audit.Entry{
			ActorUserID: &ownerID,
			Action:      "tenant.rename",
			Resource:    "tenant",
			ResourceID:  tenantID.String(),
			Reason:      "tenant owner renamed community",
			Before:      map[string]any{"name": current.Name},
			After:       map[string]any{"name": name, "next_name_change_at": now.Add(domain.RenameCooldown).Format(time.RFC3339Nano)},
		}); updateErr != nil {
			return fmt.Errorf("audit tenant rename: %w", updateErr)
		}
		view = toView(updated, "owner", now)
		return nil
	})
	return view, err
}

func toView(entity *generate.Tenant, role string, now time.Time) contract.TenantView {
	var next *time.Time
	if entity.NameChangedAt != nil {
		value := entity.NameChangedAt.Add(domain.RenameCooldown)
		next = &value
	}
	return contract.TenantView{
		Tenant: domain.Tenant{ID: entity.ID, Name: entity.Name, Slug: entity.Slug, Status: string(entity.Status), NameChangedAt: entity.NameChangedAt},
		Role:   role, NextNameChangeAt: next, CanChangeName: domain.CanRename(now, entity.NameChangedAt),
	}
}

func (r *TenantRepository) InviteAsOwner(ctx context.Context, tenantID, ownerID uuid.UUID, username string, now time.Time) (view contract.InvitationView, err error) {
	err = r.database.DoInTx(ctx, func(txctx context.Context) error {
		client := r.database.DB(txctx)
		if err := requireOwner(txctx, client, tenantID, ownerID); err != nil {
			return err
		}
		invitee, err := client.User.Query().Where(user.UsernameEQ(username), user.StatusEQ(user.StatusActive)).Only(txctx)
		if err != nil {
			return fmt.Errorf("load invitation account: %w", err)
		}
		if invitee.ID == ownerID {
			return fmt.Errorf("cannot invite self")
		}
		active, err := client.TenantMembership.Query().Where(tenantmembership.TenantIDEQ(tenantID), tenantmembership.UserIDEQ(invitee.ID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).Exist(txctx)
		if err != nil {
			return fmt.Errorf("check existing membership: %w", err)
		}
		if active {
			return fmt.Errorf("account is already a member")
		}
		pending, err := client.TenantInvitation.Query().Where(tenantinvitation.TenantIDEQ(tenantID), tenantinvitation.InvitedUserIDEQ(invitee.ID), tenantinvitation.StatusEQ(tenantinvitation.StatusPending), tenantinvitation.ExpiresAtGT(now)).Exist(txctx)
		if err != nil {
			return fmt.Errorf("check pending invitation: %w", err)
		}
		if pending {
			return fmt.Errorf("account already has a pending invitation")
		}
		expiresAt := now.Add(7 * 24 * time.Hour)
		invitation, err := client.TenantInvitation.Create().SetTenantID(tenantID).SetInvitedUserID(invitee.ID).SetInvitedBy(ownerID).SetExpiresAt(expiresAt).Save(txctx)
		if err != nil {
			return fmt.Errorf("create tenant invitation: %w", err)
		}
		if _, err := audit.NewRepository().Append(txctx, audit.Entry{ActorUserID: &ownerID, Action: "tenant.invitation.create", Resource: "tenant_invitation", ResourceID: invitation.ID.String(), Reason: "tenant owner invited an existing account", Before: map[string]any{}, After: map[string]any{"tenant_id": tenantID.String(), "invited_user_id": invitee.ID.String(), "expires_at": expiresAt.Format(time.RFC3339Nano)}}); err != nil {
			return fmt.Errorf("audit tenant invitation: %w", err)
		}
		tenantEntity, err := client.Tenant.Get(txctx, tenantID)
		if err != nil {
			return fmt.Errorf("load invitation tenant: %w", err)
		}
		view = contract.InvitationView{ID: invitation.ID, TenantID: tenantID, TenantName: tenantEntity.Name, Username: invitee.Username, ExpiresAt: expiresAt}
		return nil
	})
	return view, err
}

func (r *TenantRepository) FindPendingInvitationsForUser(ctx context.Context, userID uuid.UUID, now time.Time) ([]contract.InvitationView, error) {
	views := make([]contract.InvitationView, 0)
	err := r.database.WithPrincipalRead(ctx, userID, func(client *generate.Client) error {
		items, err := client.TenantInvitation.Query().Where(tenantinvitation.InvitedUserIDEQ(userID), tenantinvitation.StatusEQ(tenantinvitation.StatusPending), tenantinvitation.ExpiresAtGT(now)).WithTenant().WithInvitee().All(ctx)
		if err != nil {
			return fmt.Errorf("list pending tenant invitations: %w", err)
		}
		views = make([]contract.InvitationView, 0, len(items))
		for _, item := range items {
			if item.Edges.Tenant == nil || item.Edges.Invitee == nil {
				return fmt.Errorf("tenant invitation has missing edge")
			}
			views = append(views, contract.InvitationView{ID: item.ID, TenantID: item.TenantID, TenantName: item.Edges.Tenant.Name, Username: item.Edges.Invitee.Username, ExpiresAt: item.ExpiresAt})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return views, nil
}

func (r *TenantRepository) AcceptInvitation(ctx context.Context, invitationID, userID uuid.UUID, now time.Time) (view contract.TenantView, err error) {
	err = r.database.DoInTx(ctx, func(txctx context.Context) error {
		client := r.database.DB(txctx)
		lockedUser, err := client.User.Query().Where(user.IDEQ(userID)).ForUpdate().Only(txctx)
		if err != nil {
			return fmt.Errorf("lock account for invitation acceptance: %w", err)
		}
		if lockedUser.Status != user.StatusActive {
			return fmt.Errorf("invited account is not active")
		}
		invitation, err := client.TenantInvitation.Query().Where(tenantinvitation.IDEQ(invitationID), tenantinvitation.InvitedUserIDEQ(userID), tenantinvitation.StatusEQ(tenantinvitation.StatusPending), tenantinvitation.ExpiresAtGT(now)).WithTenant().Only(txctx)
		if err != nil {
			return fmt.Errorf("load pending tenant invitation: %w", err)
		}
		if invitation.Edges.Tenant == nil || invitation.Edges.Tenant.Status != tenant.StatusActive {
			return fmt.Errorf("invitation tenant is unavailable")
		}
		invitationScope, err := tenantctx.Tenant(invitation.TenantID)
		if err != nil {
			return fmt.Errorf("create invitation tenant scope: %w", err)
		}
		txctx = tenantctx.WithContext(txctx, invitationScope)
		client = r.database.DB(txctx)
		membership, err := client.TenantMembership.Query().Where(tenantmembership.TenantIDEQ(invitation.TenantID), tenantmembership.UserIDEQ(userID)).Only(txctx)
		if err != nil && !generate.IsNotFound(err) {
			return fmt.Errorf("load tenant membership: %w", err)
		}
		if generate.IsNotFound(err) {
			if _, err = client.TenantMembership.Create().SetTenantID(invitation.TenantID).SetUserID(userID).SetJoinedAt(now).Save(txctx); err != nil {
				return fmt.Errorf("create tenant membership: %w", err)
			}
		} else if membership.Status != tenantmembership.StatusActive {
			if _, err = client.TenantMembership.UpdateOneID(membership.ID).SetStatus(tenantmembership.StatusActive).SetJoinedAt(now).ClearLeftAt().Save(txctx); err != nil {
				return fmt.Errorf("reactivate tenant membership: %w", err)
			}
		}
		if err = client.CasbinRule.Create().SetPtype("g").SetV0(userSubject(userID)).SetV1("role:member").SetV2(tenantDomain(invitation.TenantID)).Exec(txctx); err != nil {
			return fmt.Errorf("assign tenant member role: %w", err)
		}
		if _, err = client.TenantInvitation.UpdateOneID(invitation.ID).Where(tenantinvitation.StatusEQ(tenantinvitation.StatusPending), tenantinvitation.ExpiresAtGT(now)).SetStatus(tenantinvitation.StatusAccepted).SetRespondedAt(now).Save(txctx); err != nil {
			return fmt.Errorf("accept tenant invitation: %w", err)
		}
		auditID, err := audit.NewRepository().Append(txctx, audit.Entry{ActorUserID: &userID, Action: "tenant.membership.join", Resource: "tenant", ResourceID: invitation.TenantID.String(), Reason: "accepted tenant invitation", Before: map[string]any{}, After: map[string]any{"tenant_id": invitation.TenantID.String(), "user_id": userID.String(), "role": "member"}})
		if err != nil {
			return fmt.Errorf("audit tenant membership: %w", err)
		}
		if err = advanceTenantAuthorization(txctx, client, userID, invitation.TenantID, "member", auditID); err != nil {
			return fmt.Errorf("publish tenant authorization revision: %w", err)
		}
		view = toView(invitation.Edges.Tenant, "member", now)
		return nil
	})
	return view, err
}

func requireOwner(ctx context.Context, client *generate.Client, tenantID, ownerID uuid.UUID) error {
	active, err := client.Tenant.Query().Where(tenant.IDEQ(tenantID), tenant.StatusEQ(tenant.StatusActive)).Exist(ctx)
	if err != nil {
		return fmt.Errorf("check active tenant: %w", err)
	}
	if !active {
		return ErrTenantOwnerRequired
	}
	isOwner, err := client.CasbinRule.Query().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(ownerID)), casbinrule.V1EQ("role:owner"), casbinrule.V2EQ(tenantDomain(tenantID))).Exist(ctx)
	if err != nil {
		return fmt.Errorf("check tenant owner: %w", err)
	}
	if !isOwner {
		return ErrTenantOwnerRequired
	}
	activeMember, err := client.TenantMembership.Query().Where(
		tenantmembership.TenantIDEQ(tenantID),
		tenantmembership.UserIDEQ(ownerID),
		tenantmembership.StatusEQ(tenantmembership.StatusActive),
	).Exist(ctx)
	if err != nil {
		return fmt.Errorf("check active tenant owner membership: %w", err)
	}
	if !activeMember {
		return ErrTenantOwnerRequired
	}
	return nil
}

func (r *TenantRepository) LeaveAsMember(ctx context.Context, tenantID, userID uuid.UUID, now time.Time) error {
	return r.database.DoInTx(ctx, func(txctx context.Context) error {
		client := r.database.DB(txctx)
		if _, err := client.User.Query().Where(user.IDEQ(userID)).ForUpdate().Only(txctx); err != nil {
			return fmt.Errorf("lock account for tenant leave: %w", err)
		}
		membership, err := client.TenantMembership.Query().Where(tenantmembership.TenantIDEQ(tenantID), tenantmembership.UserIDEQ(userID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).Only(txctx)
		if err != nil {
			return domain.ErrMemberNotFound
		}
		isOwner, err := client.CasbinRule.Query().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(userID)), casbinrule.V1EQ("role:owner"), casbinrule.V2EQ(tenantDomain(tenantID))).Exist(txctx)
		if err != nil {
			return fmt.Errorf("check owner role before leaving: %w", err)
		}
		if isOwner {
			return domain.ErrOwnerCannotLeave
		}
		activeCount, err := client.TenantMembership.Query().Where(tenantmembership.UserIDEQ(userID), tenantmembership.StatusEQ(tenantmembership.StatusActive), tenantmembership.HasTenantWith(tenant.StatusEQ(tenant.StatusActive))).Count(txctx)
		if err != nil {
			return fmt.Errorf("count active account memberships: %w", err)
		}
		if activeCount <= 1 {
			return domain.ErrLastMembership
		}
		if _, err := client.TenantMembership.UpdateOneID(membership.ID).SetStatus(tenantmembership.StatusLeft).SetLeftAt(now).Save(txctx); err != nil {
			return fmt.Errorf("leave tenant membership: %w", err)
		}
		if _, err := client.CasbinRule.Delete().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(userID)), casbinrule.V2EQ(tenantDomain(tenantID))).Exec(txctx); err != nil {
			return fmt.Errorf("remove tenant role assignment: %w", err)
		}
		auditID, err := audit.NewRepository().Append(txctx, audit.Entry{ActorUserID: &userID, Action: "tenant.membership.leave", Resource: "tenant", ResourceID: tenantID.String(), Reason: "member left tenant", Before: map[string]any{"tenant_id": tenantID.String(), "user_id": userID.String(), "role": "member"}, After: map[string]any{"status": "left"}})
		if err != nil {
			return fmt.Errorf("audit tenant membership leave: %w", err)
		}
		return advanceTenantAuthorization(txctx, client, userID, tenantID, "removed", auditID)
	})
}

func (r *TenantRepository) RemoveMemberAsOwner(ctx context.Context, tenantID, ownerID, memberID uuid.UUID, now time.Time) error {
	return r.database.DoInTx(ctx, func(txctx context.Context) error {
		client := r.database.DB(txctx)
		if err := requireOwner(txctx, client, tenantID, ownerID); err != nil {
			return err
		}
		if _, err := client.User.Query().Where(user.IDEQ(memberID)).ForUpdate().Only(txctx); err != nil {
			return fmt.Errorf("lock account for member removal: %w", err)
		}
		if memberID == ownerID {
			return domain.ErrOwnerCannotLeave
		}
		membership, err := client.TenantMembership.Query().Where(tenantmembership.TenantIDEQ(tenantID), tenantmembership.UserIDEQ(memberID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).Only(txctx)
		if err != nil {
			return domain.ErrMemberNotFound
		}
		isOwner, err := client.CasbinRule.Query().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(memberID)), casbinrule.V1EQ("role:owner"), casbinrule.V2EQ(tenantDomain(tenantID))).Exist(txctx)
		if err != nil {
			return fmt.Errorf("check member owner role: %w", err)
		}
		if isOwner {
			return domain.ErrOwnerCannotLeave
		}
		activeCount, err := client.TenantMembership.Query().Where(tenantmembership.UserIDEQ(memberID), tenantmembership.StatusEQ(tenantmembership.StatusActive), tenantmembership.HasTenantWith(tenant.StatusEQ(tenant.StatusActive))).Count(txctx)
		if err != nil {
			return fmt.Errorf("count active member tenants: %w", err)
		}
		if activeCount <= 1 {
			return domain.ErrLastMembership
		}
		if _, err := client.TenantMembership.UpdateOneID(membership.ID).SetStatus(tenantmembership.StatusLeft).SetLeftAt(now).Save(txctx); err != nil {
			return fmt.Errorf("remove tenant membership: %w", err)
		}
		if _, err := client.CasbinRule.Delete().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(memberID)), casbinrule.V2EQ(tenantDomain(tenantID))).Exec(txctx); err != nil {
			return fmt.Errorf("remove member role assignment: %w", err)
		}
		auditID, err := audit.NewRepository().Append(txctx, audit.Entry{ActorUserID: &ownerID, Action: "tenant.membership.remove", Resource: "tenant", ResourceID: tenantID.String(), Reason: "tenant owner removed member", Before: map[string]any{"tenant_id": tenantID.String(), "user_id": memberID.String(), "role": "member"}, After: map[string]any{"status": "left"}})
		if err != nil {
			return fmt.Errorf("audit member removal: %w", err)
		}
		return advanceTenantAuthorization(txctx, client, memberID, tenantID, "removed", auditID)
	})
}

func (r *TenantRepository) FindMembersForOwner(ctx context.Context, tenantID, ownerID uuid.UUID) ([]contract.MemberView, error) {
	client := r.database.DB(ctx)
	if err := requireOwner(ctx, client, tenantID, ownerID); err != nil {
		return nil, err
	}
	memberships, err := client.TenantMembership.Query().Where(tenantmembership.TenantIDEQ(tenantID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).WithUser().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tenant members: %w", err)
	}
	views := make([]contract.MemberView, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Edges.User == nil {
			return nil, fmt.Errorf("tenant membership has missing user edge")
		}
		role := "member"
		isOwner, err := client.CasbinRule.Query().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(membership.UserID)), casbinrule.V1EQ("role:owner"), casbinrule.V2EQ(tenantDomain(tenantID))).Exist(ctx)
		if err != nil {
			return nil, fmt.Errorf("read member role: %w", err)
		}
		if isOwner {
			role = "owner"
		}
		views = append(views, contract.MemberView{UserID: membership.UserID, Username: membership.Edges.User.Username, Role: role})
	}
	return views, nil
}

func (r *TenantRepository) TransferOwnership(ctx context.Context, tenantID, ownerID, nextOwnerID uuid.UUID) error {
	return r.database.DoInTx(ctx, func(txctx context.Context) error {
		client := r.database.DB(txctx)
		if _, err := client.Tenant.Query().Where(tenant.IDEQ(tenantID)).ForUpdate().Only(txctx); err != nil {
			return fmt.Errorf("lock tenant for ownership transfer: %w", err)
		}
		firstUserID, secondUserID := ownerID, nextOwnerID
		if secondUserID.String() < firstUserID.String() {
			firstUserID, secondUserID = secondUserID, firstUserID
		}
		for _, userID := range []uuid.UUID{firstUserID, secondUserID} {
			if _, err := client.User.Query().Where(user.IDEQ(userID)).ForUpdate().Only(txctx); err != nil {
				return fmt.Errorf("lock ownership transfer account: %w", err)
			}
		}
		if err := requireOwner(txctx, client, tenantID, ownerID); err != nil {
			return err
		}
		if ownerID == nextOwnerID {
			return domain.ErrOwnerCannotLeave
		}
		activeMember, err := client.TenantMembership.Query().Where(tenantmembership.TenantIDEQ(tenantID), tenantmembership.UserIDEQ(nextOwnerID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).Exist(txctx)
		if err != nil {
			return fmt.Errorf("check next owner membership: %w", err)
		}
		if !activeMember {
			return domain.ErrMemberNotFound
		}
		isAlreadyOwner, err := client.CasbinRule.Query().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(nextOwnerID)), casbinrule.V1EQ("role:owner"), casbinrule.V2EQ(tenantDomain(tenantID))).Exist(txctx)
		if err != nil {
			return fmt.Errorf("check next owner role: %w", err)
		}
		if isAlreadyOwner {
			return domain.ErrOwnerCannotLeave
		}
		domainID := tenantDomain(tenantID)
		if _, err := client.CasbinRule.Delete().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(ownerID)), casbinrule.V2EQ(domainID)).Exec(txctx); err != nil {
			return fmt.Errorf("remove current owner grants: %w", err)
		}
		if _, err := client.CasbinRule.Delete().Where(casbinrule.PtypeEQ("g"), casbinrule.V0EQ(userSubject(nextOwnerID)), casbinrule.V2EQ(domainID)).Exec(txctx); err != nil {
			return fmt.Errorf("replace next owner grants: %w", err)
		}
		for _, assignment := range []struct {
			userID uuid.UUID
			role   string
		}{{ownerID, "member"}, {nextOwnerID, "owner"}} {
			if err := client.CasbinRule.Create().SetPtype("g").SetV0(userSubject(assignment.userID)).SetV1("role:" + assignment.role).SetV2(domainID).Exec(txctx); err != nil {
				return fmt.Errorf("assign transferred tenant role: %w", err)
			}
		}
		auditID, err := audit.NewRepository().Append(txctx, audit.Entry{ActorUserID: &ownerID, Action: "tenant.owner.transfer", Resource: "tenant", ResourceID: tenantID.String(), Reason: "tenant owner transferred ownership", Before: map[string]any{"owner_user_id": ownerID.String()}, After: map[string]any{"owner_user_id": nextOwnerID.String()}})
		if err != nil {
			return fmt.Errorf("audit tenant ownership transfer: %w", err)
		}
		if err := advanceTenantAuthorization(txctx, client, ownerID, tenantID, "member", auditID); err != nil {
			return fmt.Errorf("publish former owner authorization revision: %w", err)
		}
		if err := advanceTenantAuthorization(txctx, client, nextOwnerID, tenantID, "owner", auditID); err != nil {
			return fmt.Errorf("publish next owner authorization revision: %w", err)
		}
		return nil
	})
}

func userSubject(id uuid.UUID) string  { return "user:" + id.String() }
func tenantDomain(id uuid.UUID) string { return "tenant:" + id.String() }
