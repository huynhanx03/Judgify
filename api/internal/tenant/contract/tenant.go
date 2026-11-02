package contract

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/tenant/domain"
)

type TenantView struct {
	Tenant           domain.Tenant
	Role             string
	NextNameChangeAt *time.Time
	CanChangeName    bool
}

type InvitationView struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	TenantName string
	Username   string
	ExpiresAt  time.Time
}

type MemberView struct {
	UserID   uuid.UUID
	Username string
	Role     string
}

type Repository interface {
	FindForUser(ctx context.Context, userID uuid.UUID) ([]TenantView, error)
	CreateForOwner(ctx context.Context, ownerID uuid.UUID, name, slug string) (TenantView, error)
	RenameAsOwner(ctx context.Context, tenantID, ownerID uuid.UUID, name string, now time.Time) (TenantView, error)
	InviteAsOwner(ctx context.Context, tenantID, ownerID uuid.UUID, username string, now time.Time) (InvitationView, error)
	FindPendingInvitationsForUser(ctx context.Context, userID uuid.UUID, now time.Time) ([]InvitationView, error)
	AcceptInvitation(ctx context.Context, invitationID, userID uuid.UUID, now time.Time) (TenantView, error)
	LeaveAsMember(ctx context.Context, tenantID, userID uuid.UUID, now time.Time) error
	RemoveMemberAsOwner(ctx context.Context, tenantID, ownerID, memberID uuid.UUID, now time.Time) error
	FindMembersForOwner(ctx context.Context, tenantID, ownerID uuid.UUID) ([]MemberView, error)
	TransferOwnership(ctx context.Context, tenantID, ownerID, nextOwnerID uuid.UUID) error
}
