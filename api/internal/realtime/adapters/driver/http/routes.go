package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/realtime"
)

// HandlerGroup owns only realtime transport adaptation. Ticket validation,
// topic authorization and connection limits remain in their dedicated cores.
type HandlerGroup struct {
	tickets   *realtime.TicketStore
	websocket nethttp.Handler
}

func NewHandlerGroup(
	tickets *realtime.TicketStore,
	websocket nethttp.Handler,
) (*HandlerGroup, error) {
	if tickets == nil || websocket == nil {
		return nil, realtime.ErrInvalidConfig
	}
	return &HandlerGroup{tickets: tickets, websocket: websocket}, nil
}

func (handler *HandlerGroup) IssueTicket(ctx context.Context) (*realtime.IssuedTicket, error) {
	if handler == nil || handler.tickets == nil {
		return nil, realtime.ErrInvalidConfig
	}
	ticket, err := handler.tickets.IssueFromContext(ctx)
	if err != nil {
		switch {
		case errors.Is(err, realtime.ErrInvalidPrincipal),
			errors.Is(err, realtime.ErrTicketUnavailable):
			return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", err)
		case errors.Is(err, realtime.ErrTicketCapacity):
			return nil, apperr.New(apperr.CodeTooManyRequests, "too many realtime tickets", err)
		default:
			return nil, err
		}
	}
	return &ticket, nil
}

func (handler *HandlerGroup) WebSocket(c *gin.Context) {
	if handler == nil || handler.websocket == nil {
		c.AbortWithStatus(nethttp.StatusServiceUnavailable)
		return
	}
	handler.websocket.ServeHTTP(c.Writer, c.Request)
}

// NoStore prevents browsers, proxies and service workers from retaining the
// single-use credential returned by the following ticket handler.
func NoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Next()
}
