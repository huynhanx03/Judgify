package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/tenant/contract"
	"github.com/huynhanx03/judgify/internal/tenant/domain"
)

type Tenant struct {
	repository contract.Repository
	now        func() time.Time
}

func NewTenant(repository contract.Repository, now func() time.Time) *Tenant {
	if now == nil {
		now = time.Now
	}
	return &Tenant{repository: repository, now: now}
}

func (u *Tenant) FindForUser(ctx context.Context, userID uuid.UUID) ([]contract.TenantView, error) {
	if userID == uuid.Nil {
		return nil, domain.ErrInvalidTenantName
	}
	return u.repository.FindForUser(ctx, userID)
}

func (u *Tenant) Create(ctx context.Context, ownerID uuid.UUID, name string) (contract.TenantView, error) {
	name, err := domain.ValidateName(name)
	if err != nil || ownerID == uuid.Nil {
		return contract.TenantView{}, domain.ErrInvalidTenantName
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return contract.TenantView{}, err
	}
	return u.repository.CreateForOwner(ctx, ownerID, name, slugBase(name)+"-"+hex.EncodeToString(suffix))
}

func (u *Tenant) Rename(ctx context.Context, tenantID, ownerID uuid.UUID, name string) (contract.TenantView, error) {
	name, err := domain.ValidateName(name)
	if err != nil || tenantID == uuid.Nil || ownerID == uuid.Nil {
		return contract.TenantView{}, domain.ErrInvalidTenantName
	}
	return u.repository.RenameAsOwner(ctx, tenantID, ownerID, name, u.now().UTC())
}

func (u *Tenant) Invite(ctx context.Context, tenantID, ownerID uuid.UUID, username string) (contract.InvitationView, error) {
	username = strings.TrimSpace(username)
	if tenantID == uuid.Nil || ownerID == uuid.Nil || username == "" {
		return contract.InvitationView{}, domain.ErrInvalidTenantName
	}
	return u.repository.InviteAsOwner(ctx, tenantID, ownerID, username, u.now().UTC())
}

func (u *Tenant) FindPendingInvitationsForUser(ctx context.Context, userID uuid.UUID) ([]contract.InvitationView, error) {
	if userID == uuid.Nil {
		return nil, domain.ErrInvalidTenantName
	}
	return u.repository.FindPendingInvitationsForUser(ctx, userID, u.now().UTC())
}

func (u *Tenant) AcceptInvitation(ctx context.Context, invitationID, userID uuid.UUID) (contract.TenantView, error) {
	if invitationID == uuid.Nil || userID == uuid.Nil {
		return contract.TenantView{}, domain.ErrInvalidTenantName
	}
	return u.repository.AcceptInvitation(ctx, invitationID, userID, u.now().UTC())
}

func (u *Tenant) Leave(ctx context.Context, tenantID, userID uuid.UUID) error {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return domain.ErrInvalidTenantName
	}
	return u.repository.LeaveAsMember(ctx, tenantID, userID, u.now().UTC())
}

func (u *Tenant) RemoveMember(ctx context.Context, tenantID, ownerID, memberID uuid.UUID) error {
	if tenantID == uuid.Nil || ownerID == uuid.Nil || memberID == uuid.Nil {
		return domain.ErrInvalidTenantName
	}
	return u.repository.RemoveMemberAsOwner(ctx, tenantID, ownerID, memberID, u.now().UTC())
}

func (u *Tenant) FindMembersForOwner(ctx context.Context, tenantID, ownerID uuid.UUID) ([]contract.MemberView, error) {
	if tenantID == uuid.Nil || ownerID == uuid.Nil {
		return nil, domain.ErrInvalidTenantName
	}
	return u.repository.FindMembersForOwner(ctx, tenantID, ownerID)
}

func (u *Tenant) TransferOwnership(ctx context.Context, tenantID, ownerID, nextOwnerID uuid.UUID) error {
	if tenantID == uuid.Nil || ownerID == uuid.Nil || nextOwnerID == uuid.Nil || ownerID == nextOwnerID {
		return domain.ErrInvalidTenantName
	}
	return u.repository.TransferOwnership(ctx, tenantID, ownerID, nextOwnerID)
}

func slugBase(name string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
			continue
		}
		separator = true
	}
	base := strings.Trim(b.String(), "-")
	if base == "" {
		return "tenant"
	}
	return base
}
