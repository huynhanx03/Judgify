package contract

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	CampaignStatusDraft     = "draft"
	CampaignStatusScheduled = "scheduled"
	CampaignStatusSending   = "sending"
	CampaignStatusCompleted = "completed"
	CampaignStatusFailed    = "failed"
	CampaignStatusCancelled = "cancelled"

	AudienceExplicitUsers = "explicit_users"
	AudienceRole          = "role"
	AudienceAllActive     = "all_active"

	RecipientStatusPending = "pending"
	RecipientStatusCreated = "created"
	RecipientStatusSkipped = "skipped"
	RecipientStatusFailed  = "failed"

	CampaignOperationKind = "notification.campaign.delivery"
	EventCampaignStatus   = "notification.campaign.status_changed.v1"
	EventCampaignDraft    = "notification.campaign.draft_changed.v1"
	CampaignAggregateType = "notification_campaign"

	MaximumCampaignTitleBytes        = 160
	MaximumCampaignBodyBytes         = 64 << 10
	MaximumCampaignReasonBytes       = 1024
	MaximumCampaignFailureCodeBytes  = 64
	MaximumCampaignSampleSize        = 20
	MaximumCampaignPageSize          = 100
	DefaultCampaignPageSize          = 25
	MaximumExplicitRecipients        = 5000
	DefaultMaximumCampaignRecipients = 50_000
	DefaultCampaignBatchSize         = 100
	DefaultRecipientAttemptLimit     = 5
	MaximumCampaignScheduleAhead     = 365 * 24 * time.Hour
)

var (
	ErrInvalidCampaign = errors.New("notification campaign: invalid value")

	campaignStatuses = []string{
		CampaignStatusCancelled,
		CampaignStatusCompleted,
		CampaignStatusDraft,
		CampaignStatusFailed,
		CampaignStatusScheduled,
		CampaignStatusSending,
	}
	audienceKinds = []string{
		AudienceAllActive,
		AudienceExplicitUsers,
		AudienceRole,
	}
	recipientStatuses = []string{
		RecipientStatusCreated,
		RecipientStatusFailed,
		RecipientStatusPending,
		RecipientStatusSkipped,
	}
	campaignCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

func CampaignStatuses() []string  { return append([]string(nil), campaignStatuses...) }
func AudienceKinds() []string     { return append([]string(nil), audienceKinds...) }
func RecipientStatuses() []string { return append([]string(nil), recipientStatuses...) }

func ValidCampaignStatus(value string) bool  { return slices.Contains(campaignStatuses, value) }
func ValidAudienceKind(value string) bool    { return slices.Contains(audienceKinds, value) }
func ValidRecipientStatus(value string) bool { return slices.Contains(recipientStatuses, value) }

func ValidCampaignTitle(value string) bool {
	return validCampaignText(value, MaximumCampaignTitleBytes)
}

func ValidCampaignBody(value string) bool {
	return validCampaignText(value, MaximumCampaignBodyBytes)
}

func ValidCampaignReason(value string) bool {
	return validCampaignText(value, MaximumCampaignReasonBytes)
}

func ValidCampaignCode(value string) bool {
	return len(value) <= MaximumCampaignFailureCodeBytes &&
		campaignCodePattern.MatchString(value)
}

func validCampaignText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value
}
