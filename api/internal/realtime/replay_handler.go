package realtime

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"sync"

	"github.com/google/uuid"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"
)

type ReplayRequest struct {
	RequestID   string `json:"request_id"`
	Topic       string `json:"topic"`
	AfterCursor string `json:"after_cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type ReplayComplete struct {
	RequestID  string `json:"request_id"`
	Topic      string `json:"topic"`
	Cursor     string `json:"cursor,omitempty"`
	HeadCursor string `json:"head_cursor,omitempty"`
	Delivered  int    `json:"delivered"`
	HasMore    bool   `json:"has_more"`
}

type ReplayHandler struct {
	projection ReplayProjection
	authorizer *TopicAuthorizer
	requests   replayRequestFence
}

const maximumReplayRequestReceipts = 4096

type replayRequestFence struct {
	mu      sync.Mutex
	entries map[string][sha256.Size]byte
	order   []string
}

func RegisterReplayHandler(
	hub *commonwebsocket.Hub,
	projection ReplayProjection,
	authorizer *TopicAuthorizer,
) error {
	if hub == nil || projection == nil || authorizer == nil {
		return ErrInvalidConfig
	}
	handler := &ReplayHandler{projection: projection, authorizer: authorizer}
	if err := commonwebsocket.On(hub, ReplayRequestV1, handler.Handle); err != nil {
		return errors.Join(ErrInvalidConfig, err)
	}
	return nil
}

func (handler *ReplayHandler) Handle(
	ctx context.Context,
	connection *commonwebsocket.Conn,
	request *ReplayRequest,
) error {
	if handler == nil || handler.projection == nil || handler.authorizer == nil ||
		ctx == nil || connection == nil || request == nil ||
		!canonicalReplayRequestID(request.RequestID) {
		return commonwebsocket.ErrInvalidEnvelope
	}
	if err := handler.authorizer.Authorize(
		ctx,
		connection.Principal(),
		request.Topic,
	); err != nil {
		return commonwebsocket.ErrForbiddenTopic
	}
	replayed, err := handler.requests.register(connection.ID(), *request)
	if err != nil {
		return commonwebsocket.ErrInvalidEnvelope
	}
	page, err := handler.projection.Replay(
		ctx,
		request.Topic,
		request.AfterCursor,
		request.Limit,
	)
	if err != nil {
		if IsReplayRequestError(err) {
			return commonwebsocket.ErrInvalidEnvelope
		}
		logger.FromContext(ctx).Warn(
			"realtime replay unavailable; requesting authoritative resync",
			zap.String("topic", request.Topic),
			zap.String("error_type", "replay_unavailable"),
		)
		return sendReplayResync(ctx, connection, request.Topic)
	}
	if page.ResyncRequired {
		logger.FromContext(ctx).Info(
			"realtime replay cursor outside retention window",
			zap.String("topic", request.Topic),
		)
		return sendReplayResync(ctx, connection, request.Topic)
	}
	lastCursor := request.AfterCursor
	for _, message := range page.Messages {
		if err := connection.Send(ctx, message); err != nil {
			return err
		}
		lastCursor = message.Envelope.Cursor
	}
	if len(page.Messages) == 0 && page.HeadCursor != "" {
		lastCursor = page.HeadCursor
	}
	data, err := json.Marshal(ReplayComplete{
		RequestID:  request.RequestID,
		Topic:      request.Topic,
		Cursor:     lastCursor,
		HeadCursor: page.HeadCursor,
		Delivered:  len(page.Messages),
		HasMore:    page.HasMore,
	})
	if err != nil {
		return err
	}
	acknowledgement := commonwebsocket.Message{
		Envelope: commonwebsocket.Envelope{
			Version:   commonwebsocket.ProtocolVersion,
			Operation: commonwebsocket.OperationAck,
			ID:        request.RequestID,
			Cursor:    lastCursor,
			Topic:     request.Topic,
			Type:      ReplayCompleteV1,
			Data:      data,
		},
		Class: commonwebsocket.DeliveryCritical,
	}
	logger.FromContext(ctx).Debug(
		"realtime replay page delivered",
		zap.String("topic", request.Topic),
		zap.Int("delivered", len(page.Messages)),
		zap.Bool("has_more", page.HasMore),
		zap.Bool("request_replayed", replayed),
	)
	return connection.Send(ctx, acknowledgement)
}

func (fence *replayRequestFence) register(
	connectionID string,
	request ReplayRequest,
) (bool, error) {
	if fence == nil || connectionID == "" ||
		!canonicalReplayRequestID(request.RequestID) {
		return false, ErrInvalidReplayRequest
	}
	encoded, err := json.Marshal(struct {
		RequestID   string `json:"request_id"`
		Topic       string `json:"topic"`
		AfterCursor string `json:"after_cursor"`
		Limit       int    `json:"limit"`
	}{
		RequestID:   request.RequestID,
		Topic:       request.Topic,
		AfterCursor: request.AfterCursor,
		Limit:       request.Limit,
	})
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256(encoded)
	key := connectionID + "\x00" + request.RequestID
	fence.mu.Lock()
	defer fence.mu.Unlock()
	if existing, found := fence.entries[key]; found {
		if subtle.ConstantTimeCompare(existing[:], digest[:]) != 1 {
			return false, ErrInvalidReplayRequest
		}
		return true, nil
	}
	if fence.entries == nil {
		fence.entries = make(map[string][sha256.Size]byte, maximumReplayRequestReceipts)
	}
	fence.entries[key] = digest
	fence.order = append(fence.order, key)
	for len(fence.order) > maximumReplayRequestReceipts {
		oldest := fence.order[0]
		fence.order = fence.order[1:]
		delete(fence.entries, oldest)
	}
	return false, nil
}

func sendReplayResync(
	ctx context.Context,
	connection *commonwebsocket.Conn,
	topic string,
) error {
	return connection.Send(ctx, commonwebsocket.Message{
		Envelope: commonwebsocket.Envelope{
			Version:   commonwebsocket.ProtocolVersion,
			Operation: commonwebsocket.OperationResyncRequired,
			Topic:     topic,
			Type:      commonwebsocket.ProtocolTypeResyncRequired,
		},
		Class: commonwebsocket.DeliveryCritical,
	})
}

func canonicalReplayRequestID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
