package domain

import (
	"bytes"
	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
)

type Notification struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	Type               string
	Category           string
	ContentKind        string
	TitleKey           *string
	BodyKey            *string
	Params             json.RawMessage
	CampaignRevisionID *uuid.UUID
	CampaignTitle      *string
	ActionPath         *string
	SourceEventID      uuid.UUID
	Version            int64
	CreatedAt          time.Time
	ReadAt             *time.Time
	ArchivedAt         *time.Time
	ExpiresAt          *time.Time
}

// CampaignContent is the immutable rich-content projection for one owned
// campaign notification. It is loaded on demand so inbox pages stay bounded.
type CampaignContent struct {
	NotificationID uuid.UUID
	RevisionID     uuid.UUID
	Title          string
	SanitizedHTML  string
	ActionPath     *string
}

func (content CampaignContent) Validate() error {
	if content.NotificationID == uuid.Nil || content.RevisionID == uuid.Nil ||
		!contract.ValidCampaignTitle(content.Title) ||
		content.SanitizedHTML == "" || !utf8.ValidString(content.SanitizedHTML) {
		return contract.ErrInvalidNotification
	}
	if content.ActionPath != nil &&
		contract.ValidateActionPath(*content.ActionPath) != nil {
		return contract.ErrInvalidNotification
	}
	return nil
}

type Preference struct {
	UserID    uuid.UUID
	Category  string
	InApp     bool
	Version   int64
	UpdatedAt time.Time
}

type InboxState struct {
	UserID      uuid.UUID
	Version     int64
	UnreadCount int64
	UpdatedAt   time.Time
}

func (preference Preference) Validate() error {
	if preference.UserID == uuid.Nil || !contract.ValidCategory(preference.Category) ||
		preference.Version < 0 || preference.UpdatedAt.IsZero() ||
		(contract.RequiredCategory(preference.Category) && !preference.InApp) {
		return contract.ErrInvalidNotification
	}
	return nil
}

func (state InboxState) Validate() error {
	if state.UserID == uuid.Nil || state.Version < 0 || state.UnreadCount < 0 || state.UpdatedAt.IsZero() {
		return contract.ErrInvalidNotification
	}
	return nil
}

func (notification Notification) Validate() error {
	if notification.ID == uuid.Nil || notification.UserID == uuid.Nil ||
		notification.SourceEventID == uuid.Nil || notification.Version <= 0 ||
		!contract.ValidType(notification.Type) || !contract.ValidCategory(notification.Category) ||
		!contract.ValidContentKind(notification.ContentKind) || notification.CreatedAt.IsZero() {
		return contract.ErrInvalidNotification
	}
	if notification.ActionPath != nil && contract.ValidateActionPath(*notification.ActionPath) != nil {
		return contract.ErrInvalidNotification
	}
	if notification.ExpiresAt != nil && !notification.ExpiresAt.After(notification.CreatedAt) {
		return contract.ErrInvalidNotification
	}
	switch notification.ContentKind {
	case contract.ContentKindCatalog:
		if notification.CampaignRevisionID != nil || notification.TitleKey == nil ||
			notification.BodyKey == nil || !contract.ValidMessageKey(*notification.TitleKey) ||
			!contract.ValidMessageKey(*notification.BodyKey) ||
			notification.CampaignTitle != nil ||
			validateParams(notification.Params) != nil {
			return contract.ErrInvalidNotification
		}
	case contract.ContentKindCampaign:
		if notification.CampaignRevisionID == nil || *notification.CampaignRevisionID == uuid.Nil ||
			notification.TitleKey != nil || notification.BodyKey != nil || len(notification.Params) != 0 {
			return contract.ErrInvalidNotification
		}
		if notification.CampaignTitle != nil &&
			!contract.ValidCampaignTitle(*notification.CampaignTitle) {
			return contract.ErrInvalidNotification
		}
	default:
		return contract.ErrInvalidNotification
	}
	return nil
}

func validateParams(value json.RawMessage) error {
	if len(value) == 0 || len(value) > contract.MaximumParamsBytes || !utf8.Valid(value) {
		return contract.ErrInvalidNotification
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil || decoder.More() {
		return contract.ErrInvalidNotification
	}
	var trailing any
	if decoder.Decode(&trailing) == nil {
		return contract.ErrInvalidNotification
	}
	return nil
}
