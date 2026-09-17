package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commontx "github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	entity "github.com/huynhanx03/judgify/internal/notification/domain"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

type notificationService struct {
	repository   ports.Repository
	transactions commontx.Manager
	snapshots    commontx.Manager
	outbox       eventing.OutboxAppender
	now          func() time.Time
	newID        func() (uuid.UUID, error)
}

func NewNotificationService(
	repository ports.Repository,
	transactions commontx.Manager,
	snapshots commontx.Manager,
	outbox eventing.OutboxAppender,
) (ports.Service, error) {
	if repository == nil || transactions == nil || snapshots == nil || outbox == nil {
		return nil, ports.ErrInvalidRequest
	}
	return &notificationService{
		repository:   repository,
		transactions: transactions,
		snapshots:    snapshots,
		outbox:       outbox,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.NewV7,
	}, nil
}

var _ ports.Service = (*notificationService)(nil)

func (service *notificationService) CreateFromEvent(
	ctx context.Context,
	command ports.CreateCommand,
) (*contract.NotificationResponse, bool, error) {
	if service == nil || ctx == nil || validateCreateCommand(command) != nil {
		return nil, false, notificationError(ports.ErrInvalidRequest)
	}
	command.OccurredAt = command.OccurredAt.UTC()
	var created *entity.Notification
	var inbox entity.InboxState
	var changed bool
	err := service.transactions.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		created, inbox, changed, err = service.repository.Create(txCtx, command)
		if err != nil || !changed {
			return err
		}
		eventID, err := service.newID()
		if err != nil || eventID == uuid.Nil {
			return fmt.Errorf("create notification event identity: %w", err)
		}
		payload, err := notificationPayload(created, inbox)
		if err != nil {
			return err
		}
		eventOccurredAt := service.now().UTC()
		causationID := command.SourceEventID
		if err := service.outbox.Append(txCtx, eventing.Event{
			ID:               eventID,
			Type:             contract.EventCreated,
			AggregateType:    contract.NotificationAggregateType,
			AggregateID:      created.ID,
			AggregateVersion: created.Version,
			CausationID:      &causationID,
			Payload:          payload,
			OccurredAt:       eventOccurredAt,
		}); err != nil {
			return fmt.Errorf("append notification created event: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, false, notificationError(err)
	}
	if !changed {
		logger.FromContext(ctx).Debug(
			"notification create replayed or suppressed",
			zap.String("user_id", command.UserID.String()),
			zap.String("source_event_id", command.SourceEventID.String()),
			zap.String("notification_type", command.Type),
			zap.Bool("canonical_notification_exists", created != nil),
		)
		return toNotificationResponse(created), false, nil
	}
	logger.FromContext(ctx).Info(
		"notification persisted",
		zap.String("notification_id", created.ID.String()),
		zap.String("user_id", created.UserID.String()),
		zap.String("source_event_id", created.SourceEventID.String()),
		zap.String("notification_type", created.Type),
		zap.Int64("inbox_version", inbox.Version),
	)
	return toNotificationResponse(created), true, nil
}

func (service *notificationService) List(
	ctx context.Context,
	userID uuid.UUID,
	request *contract.ListRequest,
) (*contract.ListResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	query := ports.ListQuery{UserID: userID, Limit: contract.DefaultPageSize, Now: service.now().UTC()}
	if request != nil {
		query.Cursor = request.Cursor
		query.IncludeArchived = request.IncludeArchived
		if request.Limit > 0 {
			query.Limit = request.Limit
		}
	}
	if query.Limit < 1 || query.Limit > contract.MaximumPageSize {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	var records []entity.Notification
	var next *string
	var inbox entity.InboxState
	err := service.snapshots.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		records, next, inbox, err = service.repository.List(txCtx, query)
		return err
	})
	if err != nil {
		return nil, notificationError(err)
	}
	items := make([]contract.NotificationResponse, len(records))
	for index := range records {
		items[index] = *toNotificationResponse(&records[index])
	}
	logger.FromContext(ctx).Debug(
		"notification inbox listed",
		zap.String("user_id", userID.String()),
		zap.Int("item_count", len(items)),
		zap.Int64("unread_count", inbox.UnreadCount),
		zap.Int64("inbox_version", inbox.Version),
	)
	return &contract.ListResponse{
		Items:        items,
		NextCursor:   cloneString(next),
		UnreadCount:  inbox.UnreadCount,
		InboxVersion: inbox.Version,
	}, nil
}

func (service *notificationService) CampaignContent(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) (*contract.CampaignContentResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil ||
		notificationID == uuid.Nil {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	var content entity.CampaignContent
	err := service.snapshots.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		content, err = service.repository.CampaignContent(
			txCtx,
			userID,
			notificationID,
			service.now().UTC(),
		)
		return err
	})
	if err != nil {
		return nil, notificationError(err)
	}
	logger.FromContext(ctx).Debug(
		"notification campaign content read",
		zap.String("notification_id", notificationID.String()),
		zap.String("campaign_revision_id", content.RevisionID.String()),
		zap.String("user_id", userID.String()),
	)
	return &contract.CampaignContentResponse{
		NotificationID: content.NotificationID,
		RevisionID:     content.RevisionID,
		Title:          content.Title,
		SanitizedHTML:  content.SanitizedHTML,
		ActionPath:     cloneString(content.ActionPath),
	}, nil
}

func (service *notificationService) Unread(
	ctx context.Context,
	userID uuid.UUID,
) (*contract.UnreadCountResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	var inbox entity.InboxState
	err := service.snapshots.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		inbox, err = service.repository.Unread(txCtx, userID, service.now().UTC())
		return err
	})
	if err != nil {
		return nil, notificationError(err)
	}
	return &contract.UnreadCountResponse{Count: inbox.UnreadCount, Version: inbox.Version}, nil
}

func (service *notificationService) MarkRead(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) (*contract.MutationResponse, error) {
	return service.mutate(ctx, userID, notificationID, contract.EventRead, func(
		txCtx context.Context,
		now time.Time,
	) (ports.MutationResult, error) {
		return service.repository.MarkRead(txCtx, userID, notificationID, now)
	})
}

func (service *notificationService) MarkAllRead(
	ctx context.Context,
	userID uuid.UUID,
) (*contract.MutationResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	now := service.now().UTC()
	var result ports.MutationResult
	err := service.transactions.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = service.repository.MarkAllRead(txCtx, userID, now)
		if err != nil || !result.Changed {
			return err
		}
		return service.appendMutationEvent(
			txCtx, contract.EventRead, contract.NotificationInboxAggregateType, userID,
			result.Inbox.Version, nil, result.Inbox, now,
		)
	})
	if err != nil {
		return nil, notificationError(err)
	}
	service.logMutation(ctx, contract.EventRead, userID, nil, result)
	return toMutationResponse(result), nil
}

func (service *notificationService) Archive(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
) (*contract.MutationResponse, error) {
	return service.mutate(ctx, userID, notificationID, contract.EventArchived, func(
		txCtx context.Context,
		now time.Time,
	) (ports.MutationResult, error) {
		return service.repository.Archive(txCtx, userID, notificationID, now)
	})
}

func (service *notificationService) Preferences(
	ctx context.Context,
	userID uuid.UUID,
) ([]contract.PreferenceResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	var preferences []entity.Preference
	err := service.snapshots.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		preferences, err = service.repository.Preferences(txCtx, userID)
		return err
	})
	if err != nil {
		return nil, notificationError(err)
	}
	responses := make([]contract.PreferenceResponse, len(preferences))
	for index, preference := range preferences {
		responses[index] = toPreferenceResponse(preference)
	}
	return responses, nil
}

func (service *notificationService) UpdatePreference(
	ctx context.Context,
	userID uuid.UUID,
	request *contract.UpdatePreferenceRequest,
) (*contract.PreferenceResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil || request == nil ||
		!contract.ValidCategory(request.Category) || request.ExpectedVersion < 0 ||
		(contract.RequiredCategory(request.Category) && !request.InApp) {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	var preference entity.Preference
	err := service.transactions.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		preference, err = service.repository.UpdatePreference(
			txCtx, userID, request.Category, request.InApp,
			request.ExpectedVersion, service.now().UTC(),
		)
		return err
	})
	if err != nil {
		return nil, notificationError(err)
	}
	logger.FromContext(ctx).Info(
		"notification preference updated",
		zap.String("user_id", userID.String()),
		zap.String("category", preference.Category),
		zap.Bool("in_app", preference.InApp),
		zap.Int64("preference_version", preference.Version),
	)
	response := toPreferenceResponse(preference)
	return &response, nil
}

type mutationFunc func(context.Context, time.Time) (ports.MutationResult, error)

func (service *notificationService) mutate(
	ctx context.Context,
	userID uuid.UUID,
	notificationID uuid.UUID,
	eventType string,
	mutation mutationFunc,
) (*contract.MutationResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil || notificationID == uuid.Nil || mutation == nil {
		return nil, notificationError(ports.ErrInvalidRequest)
	}
	now := service.now().UTC()
	var result ports.MutationResult
	err := service.transactions.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = mutation(txCtx, now)
		if err != nil || !result.Changed {
			return err
		}
		return service.appendMutationEvent(
			txCtx, eventType, contract.NotificationAggregateType, notificationID,
			result.Notification.Version, result.Notification, result.Inbox, now,
		)
	})
	if err != nil {
		return nil, notificationError(err)
	}
	service.logMutation(ctx, eventType, userID, &notificationID, result)
	return toMutationResponse(result), nil
}

func (service *notificationService) appendMutationEvent(
	ctx context.Context,
	eventType string,
	aggregateType string,
	aggregateID uuid.UUID,
	aggregateVersion int64,
	notification *entity.Notification,
	inbox entity.InboxState,
	occurredAt time.Time,
) error {
	eventID, err := service.newID()
	if err != nil || eventID == uuid.Nil {
		return fmt.Errorf("create notification mutation event identity: %w", err)
	}
	payload, err := notificationPayload(notification, inbox)
	if err != nil {
		return err
	}
	if err := service.outbox.Append(ctx, eventing.Event{
		ID: eventID, Type: eventType, AggregateType: aggregateType,
		AggregateID: aggregateID, AggregateVersion: aggregateVersion,
		Payload: payload, OccurredAt: occurredAt,
	}); err != nil {
		return fmt.Errorf("append notification mutation event: %w", err)
	}
	return nil
}

func (service *notificationService) logMutation(
	ctx context.Context,
	eventType string,
	userID uuid.UUID,
	notificationID *uuid.UUID,
	result ports.MutationResult,
) {
	fields := []zap.Field{
		zap.String("user_id", userID.String()),
		zap.String("notification_event_type", eventType),
		zap.Bool("changed", result.Changed),
		zap.Int64("unread_count", result.Inbox.UnreadCount),
		zap.Int64("inbox_version", result.Inbox.Version),
	}
	if notificationID != nil {
		fields = append(fields, zap.String("notification_id", notificationID.String()))
	}
	logger.FromContext(ctx).Debug("notification inbox mutation completed", fields...)
}

func notificationPayload(notification *entity.Notification, inbox entity.InboxState) (json.RawMessage, error) {
	payload, err := json.Marshal(contract.InboxEventPayload{
		UserID:       inbox.UserID,
		Notification: toInboxNotificationPayload(notification),
		UnreadCount:  inbox.UnreadCount,
		InboxVersion: inbox.Version,
	})
	if err != nil {
		return nil, fmt.Errorf("encode notification event payload: %w", err)
	}
	return payload, nil
}

func validateCreateCommand(command ports.CreateCommand) error {
	contentKind := contract.ContentKindCatalog
	if command.CampaignRevisionID != nil {
		contentKind = contract.ContentKindCampaign
	}
	probe := entity.Notification{
		ID:                 uuid.MustParse("00000000-0000-7000-8000-000000000001"),
		UserID:             command.UserID,
		Type:               command.Type,
		Category:           command.Category,
		ContentKind:        contentKind,
		TitleKey:           command.TitleKey,
		BodyKey:            command.BodyKey,
		Params:             append([]byte(nil), command.Params...),
		CampaignRevisionID: cloneUUID(command.CampaignRevisionID),
		ActionPath:         cloneString(command.ActionPath),
		SourceEventID:      command.SourceEventID,
		Version:            1,
		CreatedAt:          command.OccurredAt.UTC(),
		ExpiresAt:          cloneTime(command.ExpiresAt),
	}
	return probe.Validate()
}

func notificationError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return err
	}
	switch {
	case errors.Is(err, ports.ErrInvalidRequest), errors.Is(err, ports.ErrInvalidCursor), errors.Is(err, contract.ErrInvalidNotification):
		return apperr.New(apperr.CodeBadRequest, "invalid notification request", err)
	case errors.Is(err, ports.ErrNotFound):
		return apperr.New(apperr.CodeNotFound, "notification not found", err)
	case errors.Is(err, ports.ErrPreferenceVersionConflict):
		return apperr.New(apperr.CodeConflict, "notification preference changed; reload and try again", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return apperr.New(apperr.CodeDatabaseError, "notification service unavailable", err)
	}
}

func toNotificationResponse(value *entity.Notification) *contract.NotificationResponse {
	if value == nil {
		return nil
	}
	return &contract.NotificationResponse{
		ID:                 value.ID,
		Type:               value.Type,
		Category:           value.Category,
		ContentKind:        value.ContentKind,
		TitleKey:           cloneString(value.TitleKey),
		BodyKey:            cloneString(value.BodyKey),
		Params:             append(json.RawMessage(nil), value.Params...),
		CampaignRevisionID: cloneUUID(value.CampaignRevisionID),
		CampaignTitle:      cloneString(value.CampaignTitle),
		ActionPath:         cloneString(value.ActionPath),
		Version:            value.Version,
		CreatedAt:          value.CreatedAt.UTC(),
		ReadAt:             cloneTime(value.ReadAt),
		ArchivedAt:         cloneTime(value.ArchivedAt),
		ExpiresAt:          cloneTime(value.ExpiresAt),
	}
}

func toInboxNotificationPayload(value *entity.Notification) *contract.InboxNotificationPayload {
	response := toNotificationResponse(value)
	if response == nil {
		return nil
	}
	return &contract.InboxNotificationPayload{ID: response.ID, Type: response.Type, Category: response.Category, ContentKind: response.ContentKind, TitleKey: response.TitleKey, BodyKey: response.BodyKey, Params: response.Params, CampaignRevisionID: response.CampaignRevisionID, CampaignTitle: response.CampaignTitle, ActionPath: response.ActionPath, Version: response.Version, CreatedAt: response.CreatedAt, ReadAt: response.ReadAt, ArchivedAt: response.ArchivedAt, ExpiresAt: response.ExpiresAt}
}

func toMutationResponse(result ports.MutationResult) *contract.MutationResponse {
	return &contract.MutationResponse{
		Notification: toNotificationResponse(result.Notification),
		UnreadCount:  result.Inbox.UnreadCount,
		InboxVersion: result.Inbox.Version,
		Changed:      result.Changed,
	}
}

func toPreferenceResponse(value entity.Preference) contract.PreferenceResponse {
	return contract.PreferenceResponse{
		Category: value.Category,
		InApp:    value.InApp,
		Required: contract.RequiredCategory(value.Category),
		Version:  value.Version,
	}
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
