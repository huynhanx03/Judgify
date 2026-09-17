package realtime

import (
	"context"
	"net/http"
	"net/url"

	"github.com/huynhanx03/go-common/pkg/security/authentication"
)

const (
	TicketQueryParameter    = "ticket"
	maximumTicketQueryBytes = 128
)

// PrincipalResolver consumes a ticket when present and otherwise returns the
// anonymous principal used for public realtime topics. It intentionally does
// not accept bearer or refresh credentials at the WebSocket boundary.
func (store *TicketStore) PrincipalResolver(
	ctx context.Context,
	request *http.Request,
) (authentication.Principal, error) {
	if store == nil || ctx == nil || request == nil || request.URL == nil {
		return authentication.Principal{}, ErrInvalidConfig
	}
	if request.URL.RawQuery == "" {
		return authentication.Anonymous(), nil
	}
	if len(request.URL.RawQuery) > maximumTicketQueryBytes {
		return authentication.Principal{}, ErrTicketUnavailable
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil || len(query) != 1 {
		return authentication.Principal{}, ErrTicketUnavailable
	}
	values, exists := query[TicketQueryParameter]
	if !exists || len(values) != 1 || values[0] == "" {
		return authentication.Principal{}, ErrTicketUnavailable
	}
	return store.Consume(ctx, values[0])
}
