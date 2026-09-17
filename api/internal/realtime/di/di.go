package di

import (
	"fmt"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	"go.uber.org/zap"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/realtime"
	realtimedb "github.com/huynhanx03/judgify/internal/realtime/adapters/driven/db"
	realtimehttp "github.com/huynhanx03/judgify/internal/realtime/adapters/driver/http"
)

// RealtimeContainer owns API-process live delivery. It never constructs or
// opens Forge; PostgreSQL is the durable source and REST remains truth.
type RealtimeContainer struct {
	Handler                   *realtimehttp.HandlerGroup
	TicketStore               *realtime.TicketStore
	Hub                       *commonwebsocket.Hub
	Publisher                 *realtime.HubPublisher
	ReplayProjection          *realtimedb.ReplayProjection
	HubLifecycle              *realtime.HubLifecycle
	AuthorizationTailer       *realtime.AuthorizationTailer
	AuthorizationSource       realtime.AuthorizationRevisionSource
	HubHealth                 commonhealth.Checker
	AuthorizationTailerHealth commonhealth.Checker
}

func NewRealtimeContainer(
	client *internalent.EntClient,
	participantAccess realtime.ContestParticipantAccess,
	staffAccess authorization.Authorizer,
	browserOrigins []string,
	trustedProxies []string,
	log *zap.Logger,
) (*RealtimeContainer, error) {
	if client == nil || client.Client() == nil || participantAccess == nil ||
		staffAccess == nil || log == nil {
		return nil, realtime.ErrInvalidConfig
	}
	options := buildHubOptions(browserOrigins, trustedProxies)
	hub, err := commonwebsocket.NewHub(options)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime websocket hub: %w", err)
	}
	hubLifecycle, err := realtime.NewHubLifecycle(hub)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime hub lifecycle: %w", err)
	}
	tickets, err := realtime.NewTicketStore(realtime.DefaultTicketConfig(), log)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime tickets: %w", err)
	}
	authorizationSource, err := realtimedb.NewAuthorizationSource(client)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime authorization source: %w", err)
	}
	publicContestAccess, err := realtimedb.NewContestPublicAccess(client)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime contest visibility: %w", err)
	}
	topicAuthorizer, err := realtime.NewTopicAuthorizer(
		participantAccess,
		publicContestAccess,
		staffAccess,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime topic authorization: %w", err)
	}
	replayProjection, err := realtimedb.NewReplayProjection(
		client,
		realtime.DefaultReplayConfig(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime replay projection: %w", err)
	}
	if err := realtime.RegisterReplayHandler(hub, replayProjection, topicAuthorizer); err != nil {
		return nil, fmt.Errorf("register realtime replay handler: %w", err)
	}
	publisher, err := realtime.NewHubPublisher(hub, replayProjection)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime publisher: %w", err)
	}
	authorizationTailer, err := realtime.NewAuthorizationTailer(
		realtime.DefaultAuthorizationTailerConfig(),
		authorizationSource,
		publisher,
		publisher,
		log,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime authorization tailer: %w", err)
	}
	handler, err := realtimehttp.NewHandlerGroup(
		tickets,
		hub.Handler(tickets.PrincipalResolver, topicAuthorizer.Authorize),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize realtime transport: %w", err)
	}
	return &RealtimeContainer{
		Handler:                   handler,
		TicketStore:               tickets,
		Hub:                       hub,
		Publisher:                 publisher,
		ReplayProjection:          replayProjection,
		HubLifecycle:              hubLifecycle,
		AuthorizationTailer:       authorizationTailer,
		AuthorizationSource:       authorizationSource,
		HubHealth:                 hubLifecycle,
		AuthorizationTailerHealth: authorizationTailer,
	}, nil
}

func buildHubOptions(browserOrigins, trustedProxies []string) commonwebsocket.Options {
	options := commonwebsocket.DefaultOptions()
	options.AllowedOrigins = append([]string(nil), browserOrigins...)
	options.TrustedProxies = append([]string(nil), trustedProxies...)
	return options
}
