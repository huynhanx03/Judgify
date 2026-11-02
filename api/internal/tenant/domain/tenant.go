package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidTenantName = errors.New("tenant: invalid name")
	ErrRenameCooldown    = errors.New("tenant: rename cooldown active")
	ErrLastMembership    = errors.New("tenant: account must remain in one active tenant")
	ErrOwnerCannotLeave  = errors.New("tenant: owner must transfer ownership before leaving")
	ErrMemberNotFound    = errors.New("tenant: active membership not found")
)

const (
	CommunityName  = "Judgify Community"
	CommunitySlug  = "judgify-community"
	RenameCooldown = 365 * 24 * time.Hour
)

type Tenant struct {
	ID            uuid.UUID
	Name          string
	Slug          string
	Status        string
	NameChangedAt *time.Time
}

var CommunityID = uuid.MustParse("00000000-0000-5000-8000-000000000001")

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 120 {
		return "", ErrInvalidTenantName
	}
	return name, nil
}

func NextRenameAt(changedAt *time.Time) *time.Time {
	if changedAt == nil {
		return nil
	}
	next := changedAt.Add(RenameCooldown)
	return &next
}

func CanRename(now time.Time, changedAt *time.Time) bool {
	return changedAt == nil || !now.Before(changedAt.Add(RenameCooldown))
}
