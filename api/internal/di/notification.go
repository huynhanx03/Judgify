package di

import (
	internalent "github.com/huynhanx03/judgify/internal/ent"
	notificationdb "github.com/huynhanx03/judgify/internal/notification/adapters/driven/db"
	notificationhttp "github.com/huynhanx03/judgify/internal/notification/adapters/driver/http"
	service "github.com/huynhanx03/judgify/internal/notification/usecase"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

type NotificationContainer struct {
	Handler    *notificationhttp.Handler
	Repository ports.Repository
	Service    ports.Service
	Consumer   *service.Consumer
}

func NewNotificationContainer(
	client *internalent.EntClient,
) (*NotificationContainer, error) {
	repository, err := notificationdb.NewRepository(client)
	if err != nil {
		return nil, err
	}
	notificationService, err := service.NewNotificationService(
		repository,
		internalent.NewTxManager(client),
		internalent.NewReadSnapshotTxManager(client),
		eventing.NewOutbox(),
	)
	if err != nil {
		return nil, err
	}
	consumer, err := service.NewConsumer(notificationService)
	if err != nil {
		return nil, err
	}
	handler, err := notificationhttp.NewHandler(notificationService)
	if err != nil {
		return nil, err
	}
	return &NotificationContainer{
		Handler:    handler,
		Repository: repository,
		Service:    notificationService,
		Consumer:   consumer,
	}, nil
}
