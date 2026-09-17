package realtime

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

const (
	contestTopicPrefix              = "contest"
	lifecycleTopicSuffix            = "lifecycle"
	standingsTopicSuffix            = "standings"
	publicTopicSuffix               = "public"
	participantsTopicSuffix         = "participants"
	staffTopicSuffix                = "staff"
	privateUserTopicSegment         = "user"
	userTopicPrefix                 = "user"
	submissionsTopicSuffix          = "submissions"
	authorizationTopicPrefix        = "authorization"
	authorizationCapabilitiesSuffix = "capabilities"
	notificationTopicPrefix         = "notifications"
)

type TopicKind uint8

const (
	TopicContestStandings TopicKind = iota + 1
	TopicUserSubmissions
	TopicAuthorizationCapabilities
	TopicNotifications
	TopicContestPublic
	TopicContestParticipants
	TopicContestStaff
	TopicContestPrivateUser
	TopicContestLifecycle
)

type Topic struct {
	Kind    TopicKind
	ID      uuid.UUID
	OwnerID uuid.UUID
}

// ContestParticipantAccess is the narrow, domain-owned subscription check.
// Implementations must return true only for an active registration.
type ContestParticipantAccess interface {
	IsRegistered(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

// ContestPublicAccess mirrors the canonical public REST visibility predicate.
// Per-contest anonymous topics must not reveal draft or unpublished contests.
type ContestPublicAccess interface {
	IsPubliclyVisible(context.Context, uuid.UUID) (bool, error)
}

type TopicAuthorizer struct {
	participants ContestParticipantAccess
	public       ContestPublicAccess
	staff        authorization.Authorizer
}

func NewTopicAuthorizer(
	participants ContestParticipantAccess,
	public ContestPublicAccess,
	staff authorization.Authorizer,
) (*TopicAuthorizer, error) {
	if participants == nil || public == nil || staff == nil {
		return nil, ErrInvalidConfig
	}
	return &TopicAuthorizer{
		participants: participants,
		public:       public,
		staff:        staff,
	}, nil
}

func ContestStandingsTopic(contestID uuid.UUID) (string, error) {
	if contestID == uuid.Nil {
		return "", ErrInvalidConfig
	}
	return contestTopicPrefix + ":" + contestID.String() + ":" + standingsTopicSuffix, nil
}

// ContestLifecycleTopic is the public discovery/list invalidation stream.
// Per-contest public streams receive the same lifecycle event as well.
func ContestLifecycleTopic() string {
	return contestTopicPrefix + ":" + lifecycleTopicSuffix
}

func ContestPublicTopic(contestID uuid.UUID) (string, error) {
	return contestAudienceTopic(contestID, publicTopicSuffix)
}

func ContestParticipantsTopic(contestID uuid.UUID) (string, error) {
	return contestAudienceTopic(contestID, participantsTopicSuffix)
}

func ContestStaffTopic(contestID uuid.UUID) (string, error) {
	return contestAudienceTopic(contestID, staffTopicSuffix)
}

func ContestPrivateUserTopic(
	contestID uuid.UUID,
	userID uuid.UUID,
) (string, error) {
	if contestID == uuid.Nil || userID == uuid.Nil {
		return "", ErrInvalidConfig
	}
	return contestTopicPrefix + ":" + contestID.String() + ":" +
		privateUserTopicSegment + ":" + userID.String(), nil
}

func contestAudienceTopic(contestID uuid.UUID, audience string) (string, error) {
	if contestID == uuid.Nil {
		return "", ErrInvalidConfig
	}
	return contestTopicPrefix + ":" + contestID.String() + ":" + audience, nil
}

func UserSubmissionsTopic(userID uuid.UUID) (string, error) {
	if userID == uuid.Nil {
		return "", ErrInvalidConfig
	}
	return userTopicPrefix + ":" + userID.String() + ":" + submissionsTopicSuffix, nil
}

// AuthorizationCapabilitiesTopic is a single authenticated, tenant-wide
// invalidation stream. Its payload never includes policy or identity data.
func AuthorizationCapabilitiesTopic() string {
	return authorizationTopicPrefix + ":" + authorizationCapabilitiesSuffix
}

// NotificationTopic is private to exactly one authenticated account.
func NotificationTopic(userID uuid.UUID) (string, error) {
	if userID == uuid.Nil {
		return "", ErrInvalidConfig
	}
	return notificationTopicPrefix + ":" + userID.String(), nil
}

func ParseTopic(value string) (Topic, error) {
	if value == AuthorizationCapabilitiesTopic() {
		return Topic{Kind: TopicAuthorizationCapabilities}, nil
	}
	if value == ContestLifecycleTopic() {
		return Topic{Kind: TopicContestLifecycle}, nil
	}
	parts := strings.Split(value, ":")
	if len(parts) == 2 && parts[0] == notificationTopicPrefix {
		id, err := uuid.Parse(parts[1])
		if err != nil || id == uuid.Nil || id.String() != parts[1] {
			return Topic{}, ErrTopicForbidden
		}
		return Topic{Kind: TopicNotifications, ID: id}, nil
	}
	if len(parts) == 4 && parts[0] == contestTopicPrefix &&
		parts[2] == privateUserTopicSegment {
		contestID, err := canonicalTopicID(parts[1])
		if err != nil {
			return Topic{}, ErrTopicForbidden
		}
		userID, err := canonicalTopicID(parts[3])
		if err != nil {
			return Topic{}, ErrTopicForbidden
		}
		return Topic{
			Kind:    TopicContestPrivateUser,
			ID:      contestID,
			OwnerID: userID,
		}, nil
	}
	if len(parts) != 3 {
		return Topic{}, ErrTopicForbidden
	}
	id, err := canonicalTopicID(parts[1])
	if err != nil {
		return Topic{}, ErrTopicForbidden
	}
	switch {
	case parts[0] == contestTopicPrefix && parts[2] == standingsTopicSuffix:
		return Topic{Kind: TopicContestStandings, ID: id}, nil
	case parts[0] == contestTopicPrefix && parts[2] == publicTopicSuffix:
		return Topic{Kind: TopicContestPublic, ID: id}, nil
	case parts[0] == contestTopicPrefix && parts[2] == participantsTopicSuffix:
		return Topic{Kind: TopicContestParticipants, ID: id}, nil
	case parts[0] == contestTopicPrefix && parts[2] == staffTopicSuffix:
		return Topic{Kind: TopicContestStaff, ID: id}, nil
	case parts[0] == userTopicPrefix && parts[2] == submissionsTopicSuffix:
		return Topic{Kind: TopicUserSubmissions, ID: id}, nil
	default:
		return Topic{}, ErrTopicForbidden
	}
}

// AuthorizeTopic preserves the dependency-free policy used by focused tests.
// It intentionally denies per-contest public topics because their publication
// state requires a database check. Production always uses TopicAuthorizer.
func AuthorizeTopic(
	ctx context.Context,
	principal authentication.Principal,
	value string,
) error {
	return (&TopicAuthorizer{}).Authorize(ctx, principal, value)
}

func (authorizer *TopicAuthorizer) Authorize(
	ctx context.Context,
	principal authentication.Principal,
	value string,
) error {
	if ctx == nil || principal.Validate(time.Now().UTC()) != nil {
		return ErrTopicForbidden
	}
	topic, err := ParseTopic(value)
	if err != nil {
		return ErrTopicForbidden
	}
	if topic.Kind == TopicContestLifecycle {
		return nil
	}
	if topic.Kind == TopicContestStandings || topic.Kind == TopicContestPublic {
		if authorizer == nil || authorizer.public == nil {
			return ErrTopicForbidden
		}
		visible, err := authorizer.public.IsPubliclyVisible(ctx, topic.ID)
		if err != nil || !visible {
			return ErrTopicForbidden
		}
		return nil
	}
	if !principal.Authenticated {
		return ErrTopicForbidden
	}
	userID, err := uuid.Parse(principal.UserID)
	if err != nil || userID == uuid.Nil || userID.String() != principal.UserID ||
		principal.Subject != principal.UserID {
		return ErrTopicForbidden
	}
	if topic.Kind == TopicAuthorizationCapabilities {
		return nil
	}
	switch topic.Kind {
	case TopicUserSubmissions, TopicNotifications:
		if userID != topic.ID {
			return ErrTopicForbidden
		}
	case TopicContestPrivateUser:
		if userID != topic.OwnerID {
			return ErrTopicForbidden
		}
	case TopicContestParticipants:
		if authorizer == nil || authorizer.participants == nil {
			return ErrTopicForbidden
		}
		registered, err := authorizer.participants.IsRegistered(
			ctx,
			topic.ID,
			userID,
		)
		if err != nil || !registered {
			return ErrTopicForbidden
		}
	case TopicContestStaff:
		if authorizer == nil || authorizer.staff == nil {
			return ErrTopicForbidden
		}
		decision, err := authorizer.staff.Authorize(ctx, authorization.Request{
			Principal: principal,
			Resource:  permissions.ResourceKeyContest,
			Action:    permissions.PermissionScopeRead,
		})
		if err != nil || !decision.Allowed {
			return ErrTopicForbidden
		}
	default:
		return ErrTopicForbidden
	}
	return nil
}

func canonicalTopicID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, ErrTopicForbidden
	}
	return id, nil
}
