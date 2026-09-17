package realtime

import (
	"context"

	"github.com/google/uuid"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
)

// AuthorizationRevisionSource reads the committed single-tenant revision
// fence. Implementations must never surface an in-flight transaction.
type AuthorizationRevisionSource interface {
	Current(context.Context) (AuthorizationRevisionChange, error)
}

type AuthorizationChangePublisher interface {
	PublishAuthorization(context.Context, AuthorizationRevisionChange) error
}

// ConnectionRevoker is the narrow application boundary used when committed
// security state makes an already-upgraded socket unsafe. Implementations must
// be idempotent: durable event delivery and revision tailing can retry.
type ConnectionRevoker interface {
	RevokeAllAuthenticated(context.Context) (int, error)
	RevokeSubject(context.Context, uuid.UUID) (int, error)
	RevokeSession(context.Context, uuid.UUID) (int, error)
}

// RoutedPublisher is shared by domain-specific realtime adapters. Every
// implementation must durably project before attempting in-memory fan-out.
type RoutedPublisher interface {
	PublishMessage(context.Context, string, commonwebsocket.Message) error
}
