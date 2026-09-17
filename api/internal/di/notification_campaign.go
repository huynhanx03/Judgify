package di

import (
	"time"

	"github.com/huynhanx03/go-common/pkg/security/authorization"

	notificationcampaign "github.com/huynhanx03/judgify/internal/application/workflow/notificationcampaign"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	identitydb "github.com/huynhanx03/judgify/internal/identity/adapters/driven/db"
	notificationdb "github.com/huynhanx03/judgify/internal/notification/adapters/driven/db"
	notificationhttp "github.com/huynhanx03/judgify/internal/notification/adapters/driver/http"
	service "github.com/huynhanx03/judgify/internal/notification/usecase"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	identityports "github.com/huynhanx03/judgify/internal/ports"
)

type CampaignConfig struct {
	MaximumRecipients       int
	BatchSize               int
	ConfirmationTTL         time.Duration
	RecentAuthenticationTTL time.Duration
	ConfirmationSecret      []byte
}

type CampaignContainer struct {
	Handler      *notificationhttp.CampaignHandler
	Repository   ports.CampaignRepository
	Service      ports.CampaignService
	Scheduler    ports.CampaignScheduler
	Workflow     *notificationcampaign.Workflow
	Audience     identityports.CampaignAudienceParticipant
	Capabilities ports.CampaignCapabilityChecker
}

// NewCampaignContainer composes the campaign vertical slice without starting
// goroutines. The caller owns the shared operation registry and freezes it
// only after every domain executor has registered.
func NewCampaignContainer(
	client *internalent.EntClient,
	notifications ports.Service,
	authorizer authorization.Authorizer,
	operations *operation.Components,
	registry *operation.Registry,
	config CampaignConfig,
) (*CampaignContainer, error) {
	if client == nil || notifications == nil || authorizer == nil ||
		operations == nil || operations.Repository == nil ||
		operations.Starter == nil || operations.Controller == nil ||
		registry == nil {
		return nil, ports.ErrInvalidRequest
	}
	repository, err := notificationdb.NewCampaignRepository(client)
	if err != nil {
		return nil, err
	}
	audience, err := identitydb.NewCampaignAudienceRepository(client)
	if err != nil {
		return nil, err
	}
	capabilities, err := service.NewCampaignCapabilityChecker(authorizer)
	if err != nil {
		return nil, err
	}
	protector, err := service.NewCampaignConfirmationProtector(
		config.ConfirmationSecret,
	)
	if err != nil {
		return nil, err
	}
	transactions := platformtx.NewEntUnitOfWork(client.Client())
	auditRepository := audit.NewRepository()
	outbox := eventing.NewOutbox()
	contentRepository, err := contentrender.NewEntRepository(client.Client())
	if err != nil {
		return nil, err
	}
	renderer, err := contentrender.NewRenderer()
	if err != nil {
		return nil, err
	}
	campaignService, err := service.NewCampaignService(
		service.CampaignDependencies{
			Repository:   repository,
			Transactions: transactions,
			Content:      contentRepository,
			Renderer:     renderer,
			Audience:     audience,
			Audit:        auditRepository,
			Outbox:       outbox,
			Operations:   operations.Repository,
			Confirmation: protector,
			Capabilities: capabilities,
		},
		service.CampaignServiceConfig{
			MaximumRecipients:       config.MaximumRecipients,
			ConfirmationTTL:         config.ConfirmationTTL,
			RecentAuthenticationTTL: config.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, err
	}
	workflow, err := notificationcampaign.New(
		notificationcampaign.Dependencies{
			Repository:    repository,
			Transactions:  transactions,
			Audience:      audience,
			Capabilities:  capabilities,
			Notifications: notifications,
			Operations:    operations.Repository,
			Starter:       operations.Starter,
			Controller:    operations.Controller,
			Confirmation:  protector,
			Audit:         auditRepository,
			Outbox:        outbox,
		},
		notificationcampaign.Config{
			MaximumRecipients:       config.MaximumRecipients,
			BatchSize:               config.BatchSize,
			RecentAuthenticationTTL: config.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := workflow.Register(registry); err != nil {
		return nil, err
	}
	handler, err := notificationhttp.NewCampaignHandler(
		campaignService,
		workflow,
		workflow,
	)
	if err != nil {
		return nil, err
	}
	return &CampaignContainer{
		Handler:      handler,
		Repository:   repository,
		Service:      campaignService,
		Scheduler:    workflow,
		Workflow:     workflow,
		Audience:     audience,
		Capabilities: capabilities,
	}, nil
}
