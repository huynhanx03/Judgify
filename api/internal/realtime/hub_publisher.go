package realtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
)

type HubPublisher struct {
	hub        *commonwebsocket.Hub
	projection ReplayProjection
}

func NewHubPublisher(
	hub *commonwebsocket.Hub,
	projection ReplayProjection,
) (*HubPublisher, error) {
	if hub == nil || projection == nil {
		return nil, ErrInvalidConfig
	}
	return &HubPublisher{hub: hub, projection: projection}, nil
}

func (publisher *HubPublisher) PublishMessage(
	ctx context.Context,
	topic string,
	message commonwebsocket.Message,
) error {
	return publisher.publishRouted(ctx, []RoutedMessage{{
		Topic:   topic,
		Message: message,
	}})
}

func (publisher *HubPublisher) PublishAuthorization(
	ctx context.Context,
	change AuthorizationRevisionChange,
) error {
	if publisher == nil || publisher.hub == nil || ctx == nil {
		return ErrInvalidConfig
	}
	routed, err := MessageForAuthorizationChange(change)
	if err != nil {
		return err
	}
	return publisher.publishRouted(ctx, []RoutedMessage{routed})
}

func (publisher *HubPublisher) PublishContest(
	ctx context.Context,
	change ContestChange,
) error {
	if publisher == nil || publisher.hub == nil || ctx == nil {
		return ErrInvalidConfig
	}
	messages, err := MessagesForContestChange(change)
	if err != nil {
		return err
	}
	return publisher.publishRouted(ctx, messages)
}

func (publisher *HubPublisher) RevokeAllAuthenticated(
	ctx context.Context,
) (int, error) {
	return publisher.revoke(ctx, commonwebsocket.ConnectionSelector{
		All:            true,
		PrincipalClass: commonwebsocket.PrincipalAuthenticated,
	})
}

func (publisher *HubPublisher) RevokeSubject(
	ctx context.Context,
	userID uuid.UUID,
) (int, error) {
	if userID == uuid.Nil {
		return 0, ErrInvalidConfig
	}
	return publisher.revoke(ctx, commonwebsocket.ConnectionSelector{
		PrincipalClass: commonwebsocket.PrincipalAuthenticated,
		Subjects:       []string{userID.String()},
	})
}

func (publisher *HubPublisher) RevokeSession(
	ctx context.Context,
	sessionID uuid.UUID,
) (int, error) {
	if sessionID == uuid.Nil {
		return 0, ErrInvalidConfig
	}
	return publisher.revoke(ctx, commonwebsocket.ConnectionSelector{
		PrincipalClass: commonwebsocket.PrincipalAuthenticated,
		SessionIDs:     []string{sessionID.String()},
	})
}

func (publisher *HubPublisher) revoke(
	ctx context.Context,
	selector commonwebsocket.ConnectionSelector,
) (int, error) {
	if publisher == nil || publisher.hub == nil || ctx == nil {
		return 0, ErrInvalidConfig
	}
	result, err := publisher.hub.CloseConnections(
		ctx,
		selector,
		commonwebsocket.CloseOptions{
			Code:   commonwebsocket.CloseAuthenticationGone,
			Reason: "authentication state changed",
		},
	)
	if err != nil {
		return 0, fmt.Errorf("revoke realtime connections: %w", err)
	}
	return result.MarkedClosing, nil
}

func (publisher *HubPublisher) publishRouted(
	ctx context.Context,
	messages []RoutedMessage,
) error {
	if publisher == nil || publisher.hub == nil || publisher.projection == nil || ctx == nil ||
		len(messages) == 0 {
		return ErrInvalidConfig
	}
	projected, err := publisher.projection.ProjectBatch(ctx, messages)
	if err != nil {
		return fmt.Errorf("project realtime event batch: %w", err)
	}
	if len(projected) != len(messages) {
		return ErrReplayConflict
	}
	for index, routed := range messages {
		if err := publisher.hub.Broadcast(ctx, routed.Topic, projected[index].Message); err != nil {
			if errors.Is(err, commonwebsocket.ErrShuttingDown) ||
				errors.Is(err, commonwebsocket.ErrClosed) {
				return errors.Join(ErrPublisherUnavailable, err)
			}
			return fmt.Errorf("publish realtime event: %w", err)
		}
	}
	return nil
}
