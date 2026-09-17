package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// ToUserEntity converts Ent User model to domain entity.
func ToUserEntity(m *generate.User) *entity.User {
	if m == nil {
		return nil
	}
	u := &entity.User{
		ID:                         m.ID,
		Username:                   m.Username,
		SecurityStatus:             entity.UserSecurityStatus(m.Status),
		SecurityVersion:            uint64(m.SecurityVersion),
		SuspendedAt:                m.SuspendedAt,
		ProfileVisibility:          entity.ProfileVisibility(m.ProfileVisibility),
		ProfileVisibilityVersion:   m.ProfileVisibilityVersion,
		ProfileVisibilityUpdatedAt: m.ProfileVisibilityUpdatedAt,
		CreatedAt:                  m.CreatedAt,
		UpdatedAt:                  m.UpdatedAt,
	}
	if m.SuspensionReason != nil {
		u.SuspensionReason = *m.SuspensionReason
	}
	return u
}

// ToUserModel converts domain entity to Ent User model.
func ToUserModel(e *entity.User) *generate.User {
	if e == nil {
		return nil
	}
	m := &generate.User{
		ID:                         e.ID,
		Username:                   e.Username,
		Status:                     user.Status(e.SecurityStatus),
		SecurityVersion:            int64(e.SecurityVersion),
		SuspendedAt:                e.SuspendedAt,
		ProfileVisibility:          user.ProfileVisibility(e.ProfileVisibility),
		ProfileVisibilityVersion:   e.ProfileVisibilityVersion,
		ProfileVisibilityUpdatedAt: e.ProfileVisibilityUpdatedAt,
		CreatedAt:                  e.CreatedAt,
		UpdatedAt:                  e.UpdatedAt,
	}
	if e.SuspensionReason != "" {
		m.SuspensionReason = &e.SuspensionReason
	}
	return m
}
