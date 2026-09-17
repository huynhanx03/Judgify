package usecase

import (
	"context"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

// Consumer is the narrow durable-event entry point used by a dispatcher or a
// campaign delivery workflow. It deliberately accepts the notification-owned
// command rather than importing another domain's entity or persistence model.
type Consumer struct {
	service ports.Service
}

func NewConsumer(service ports.Service) (*Consumer, error) {
	if service == nil {
		return nil, ports.ErrInvalidRequest
	}
	return &Consumer{service: service}, nil
}

func (consumer *Consumer) Consume(
	ctx context.Context,
	command ports.CreateCommand,
) (*contract.NotificationResponse, bool, error) {
	if consumer == nil || consumer.service == nil {
		return nil, false, ports.ErrInvalidRequest
	}
	return consumer.service.CreateFromEvent(ctx, command)
}
