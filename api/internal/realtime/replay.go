package realtime

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"

	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
)

const (
	ReplayRequestV1  = "realtime.replay.request.v1"
	ReplayCompleteV1 = "realtime.replay.complete.v1"

	replayCursorPrefix = "r1."
)

type ReplayConfig struct {
	MaximumEvents    int
	MaximumAge       time.Duration
	MaximumBatchSize int
	DefaultPageSize  int
	MaximumPageSize  int
}

func DefaultReplayConfig() ReplayConfig {
	return ReplayConfig{
		MaximumEvents:    100_000,
		MaximumAge:       24 * time.Hour,
		MaximumBatchSize: 16,
		DefaultPageSize:  128,
		MaximumPageSize:  512,
	}
}

func (config ReplayConfig) Validate() error {
	if config.MaximumEvents < 1 || config.MaximumAge <= 0 ||
		config.MaximumBatchSize < 1 || config.MaximumBatchSize > 100 ||
		config.DefaultPageSize < 1 || config.MaximumPageSize < config.DefaultPageSize ||
		config.MaximumPageSize > config.MaximumEvents {
		return ErrInvalidConfig
	}
	return nil
}

type ProjectedMessage struct {
	Message  commonwebsocket.Message
	Inserted bool
}

type ReplayPage struct {
	Messages       []commonwebsocket.Message
	HeadCursor     string
	HasMore        bool
	ResyncRequired bool
}

type ReplayProjection interface {
	Project(ctx context.Context, routed RoutedMessage) (ProjectedMessage, error)
	ProjectBatch(ctx context.Context, routed []RoutedMessage) ([]ProjectedMessage, error)
	Replay(ctx context.Context, topic string, afterCursor string, limit int) (ReplayPage, error)
}

func NormalizeReplayLimit(config ReplayConfig, requested int) (int, error) {
	if err := config.Validate(); err != nil {
		return 0, err
	}
	if requested == 0 {
		return config.DefaultPageSize, nil
	}
	if requested < 1 || requested > config.MaximumPageSize {
		return 0, ErrInvalidReplayRequest
	}
	return requested, nil
}

func EncodeReplayCursor(sequence int64) (string, error) {
	if sequence < 1 {
		return "", ErrInvalidReplayRequest
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(sequence))
	return replayCursorPrefix + base64.RawURLEncoding.EncodeToString(encoded[:]), nil
}

func DecodeReplayCursor(cursor string) (int64, error) {
	if len(cursor) <= len(replayCursorPrefix) ||
		cursor[:len(replayCursorPrefix)] != replayCursorPrefix {
		return 0, ErrInvalidReplayRequest
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor[len(replayCursorPrefix):])
	if err != nil || len(decoded) != 8 {
		return 0, ErrInvalidReplayRequest
	}
	sequence := binary.BigEndian.Uint64(decoded)
	if sequence == 0 || sequence > uint64(^uint64(0)>>1) {
		return 0, ErrInvalidReplayRequest
	}
	return int64(sequence), nil
}

func IsReplayRequestError(err error) bool {
	return errors.Is(err, ErrInvalidReplayRequest)
}
