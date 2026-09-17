// Package contract owns the closed, transport-independent notification
// vocabulary. Persistence and clients may store these keys, but they cannot
// invent delivery categories or bypass category policy.
package contract

import (
	"errors"
	"regexp"
	"slices"
)

const (
	CategorySecurity      = "security"
	CategorySubmission    = "submission"
	CategoryContest       = "contest"
	CategoryAuthorization = "authorization"
	CategoryMaterial      = "material"
	CategoryOperation     = "operation"
	CategoryCampaign      = "campaign"

	ContentKindCatalog  = "catalog"
	ContentKindCampaign = "campaign"

	EventCreated  = "notification.created.v1"
	EventRead     = "notification.read.v1"
	EventArchived = "notification.archived.v1"

	NotificationAggregateType      = "notification"
	NotificationInboxAggregateType = "notification_inbox"
)

// Product-owned catalog keys are shared by durable producers and the client
// presentation catalog. They are identifiers, never user-authored copy.
const (
	MessageKeySubmissionJudgedTitle      = "notification.submission.judged.title"
	MessageKeySubmissionJudgedBody       = "notification.submission.judged.body"
	MessageKeySubmissionSystemErrorTitle = "notification.submission.system_error.title"
	MessageKeySubmissionSystemErrorBody  = "notification.submission.system_error.body"
	MessageKeySecurityAccountTitle       = "notification.security.account.title"
	MessageKeySecurityAccountBody        = "notification.security.account.body"
	MessageKeySecuritySessionTitle       = "notification.security.session.title"
	MessageKeySecuritySessionBody        = "notification.security.session.body"
	MessageKeyContestRatingTitle         = "notification.contest.rating.title"
	MessageKeyContestRatingBody          = "notification.contest.rating.body"
)

const (
	MaximumTypeBytes       = 128
	MaximumMessageKeyBytes = 128
	MaximumParamsBytes     = 16 << 10
	MaximumActionPathBytes = 512
	MaximumPageSize        = 100
	DefaultPageSize        = 30
)

var (
	ErrInvalidNotification = errors.New("notification: invalid value")
	typePattern            = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}(\.[a-z][a-z0-9_]{0,31}){1,5}\.v[1-9][0-9]{0,8}$`)
	messageKeyPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}(\.[a-z][a-z0-9_]{0,31}){1,7}$`)
	categories             = []string{
		CategoryAuthorization,
		CategoryCampaign,
		CategoryContest,
		CategoryMaterial,
		CategoryOperation,
		CategorySecurity,
		CategorySubmission,
	}
)

// Categories returns a stable copy suitable for API presentation.
func Categories() []string { return append([]string(nil), categories...) }

func ValidCategory(value string) bool { return slices.Contains(categories, value) }

func RequiredCategory(value string) bool {
	return value == CategorySecurity || value == CategoryAuthorization || value == CategoryOperation
}

func ValidContentKind(value string) bool {
	return value == ContentKindCatalog || value == ContentKindCampaign
}

func ValidType(value string) bool {
	return len(value) <= MaximumTypeBytes && typePattern.MatchString(value)
}

func ValidMessageKey(value string) bool {
	return len(value) <= MaximumMessageKeyBytes && messageKeyPattern.MatchString(value)
}
