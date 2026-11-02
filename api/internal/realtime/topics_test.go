package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	"github.com/stretchr/testify/require"
)

type participantAccessStub struct {
	registered bool
	err        error
}

type publicAccessStub struct {
	visible bool
	err     error
}

func (stub publicAccessStub) IsPubliclyVisible(
	context.Context,
	uuid.UUID,
) (bool, error) {
	return stub.visible, stub.err
}

func (stub participantAccessStub) IsRegistered(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) (bool, error) {
	return stub.registered, stub.err
}

type topicStaffAuthorizerStub struct {
	allowed bool
	err     error
}

func (stub topicStaffAuthorizerStub) Authorize(
	context.Context,
	authorization.Request,
) (authorization.Decision, error) {
	return authorization.Decision{Allowed: stub.allowed}, stub.err
}

func TestTopicAuthorizationIsClosedAndOwnershipAware(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	otherID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000003")
	userTopic, err := UserSubmissionsTopic(userID)
	require.NoError(t, err)
	contestTopic, err := ContestStandingsTopic(contestID)
	require.NoError(t, err)
	authorizationTopic := AuthorizationCapabilitiesTopic()
	lifecycleTopic := ContestLifecycleTopic()
	principal := authentication.Principal{
		Authenticated: true, Subject: userID.String(), UserID: userID.String(),
		AuthenticatedUntil: time.Now().Add(time.Minute),
	}

	require.ErrorIs(t, AuthorizeTopic(context.Background(), authentication.Anonymous(), contestTopic), ErrTopicForbidden)
	require.NoError(t, AuthorizeTopic(context.Background(), authentication.Anonymous(), lifecycleTopic))
	require.NoError(t, AuthorizeTopic(context.Background(), principal, userTopic))
	require.NoError(t, AuthorizeTopic(context.Background(), principal, authorizationTopic))
	require.ErrorIs(t, AuthorizeTopic(context.Background(), authentication.Anonymous(), userTopic), ErrTopicForbidden)
	require.ErrorIs(t, AuthorizeTopic(context.Background(), authentication.Anonymous(), authorizationTopic), ErrTopicForbidden)
	malformedPrincipal := principal
	malformedPrincipal.Subject = "different-subject"
	require.ErrorIs(t, AuthorizeTopic(context.Background(), malformedPrincipal, authorizationTopic), ErrTopicForbidden)
	otherTopic, err := UserSubmissionsTopic(otherID)
	require.NoError(t, err)
	require.ErrorIs(t, AuthorizeTopic(context.Background(), principal, otherTopic), ErrTopicForbidden)
}

func TestTopicParserRejectsEveryOpenEndedShape(t *testing.T) {
	values := []string{
		"", "contest", "contest:00000000-0000-7000-8000-000000000003",
		"contest:00000000-0000-7000-8000-000000000003:submissions",
		"user:00000000-0000-7000-8000-000000000001:standings",
		"user:00000000-0000-7000-8000-000000000001:submissions:extra",
		"user:00000000-0000-0000-0000-000000000000:submissions",
		"user:00000000-0000-7000-8000-000000000001:submissions ",
		"USER:00000000-0000-7000-8000-000000000001:submissions",
		"authorization", "authorization:capability", "authorization:capabilities:extra",
		"Authorization:capabilities", "authorization:capabilities ",
	}
	for _, value := range values {
		_, err := ParseTopic(value)
		require.ErrorIs(t, err, ErrTopicForbidden, value)
	}
}

func TestAuthorizationTopicHasClosedFixedGrammar(t *testing.T) {
	topic, err := ParseTopic(AuthorizationCapabilitiesTopic())
	require.NoError(t, err)
	require.Equal(t, TopicAuthorizationCapabilities, topic.Kind)
	require.Equal(t, uuid.Nil, topic.ID)
}

func TestContestAudienceAuthorizationIsFailClosed(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	otherID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000003")
	principal := authentication.Principal{
		Authenticated:      true,
		Subject:            userID.String(),
		UserID:             userID.String(),
		AuthenticatedUntil: time.Now().Add(time.Minute),
	}
	authorizer, err := NewTopicAuthorizer(
		participantAccessStub{registered: true},
		publicAccessStub{visible: true},
		topicStaffAuthorizerStub{allowed: true},
	)
	require.NoError(t, err)

	publicTopic, err := ContestPublicTopic(contestID)
	require.NoError(t, err)
	participantTopic, err := ContestParticipantsTopic(contestID)
	require.NoError(t, err)
	staffTopic, err := ContestStaffTopic(contestID)
	require.NoError(t, err)
	privateTopic, err := ContestPrivateUserTopic(contestID, userID)
	require.NoError(t, err)
	otherPrivateTopic, err := ContestPrivateUserTopic(contestID, otherID)
	require.NoError(t, err)

	require.NoError(t, authorizer.Authorize(
		context.Background(),
		authentication.Anonymous(),
		publicTopic,
	))
	require.NoError(t, authorizer.Authorize(context.Background(), principal, participantTopic))
	require.NoError(t, authorizer.Authorize(context.Background(), principal, staffTopic))
	require.NoError(t, authorizer.Authorize(context.Background(), principal, privateTopic))
	require.ErrorIs(
		t,
		authorizer.Authorize(context.Background(), principal, otherPrivateTopic),
		ErrTopicForbidden,
	)

	deniedParticipants, err := NewTopicAuthorizer(
		participantAccessStub{},
		publicAccessStub{visible: true},
		topicStaffAuthorizerStub{allowed: true},
	)
	require.NoError(t, err)
	require.ErrorIs(
		t,
		deniedParticipants.Authorize(context.Background(), principal, participantTopic),
		ErrTopicForbidden,
	)

	deniedStaff, err := NewTopicAuthorizer(
		participantAccessStub{registered: true},
		publicAccessStub{visible: true},
		topicStaffAuthorizerStub{},
	)
	require.NoError(t, err)
	require.ErrorIs(
		t,
		deniedStaff.Authorize(context.Background(), principal, staffTopic),
		ErrTopicForbidden,
	)

	privateContest, err := NewTopicAuthorizer(
		participantAccessStub{registered: true},
		publicAccessStub{},
		topicStaffAuthorizerStub{allowed: true},
	)
	require.NoError(t, err)
	require.ErrorIs(
		t,
		privateContest.Authorize(
			context.Background(),
			authentication.Anonymous(),
			publicTopic,
		),
		ErrTopicForbidden,
	)
}
