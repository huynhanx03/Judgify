package di

import (
	"fmt"

	"github.com/huynhanx03/go-common/pkg/common/cache"
	"github.com/huynhanx03/judgify/internal/application/workflow/admincommand"
	"github.com/huynhanx03/judgify/internal/application/workflow/materiallifecycle"
	appcache "github.com/huynhanx03/judgify/internal/cache"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/material/adapters/driven/db"
	materialHttp "github.com/huynhanx03/judgify/internal/material/adapters/driver/http"
	service "github.com/huynhanx03/judgify/internal/material/usecase"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// MaterialContainer holds the composed Material domain dependencies.
type MaterialContainer struct {
	MaterialHandlerGroup *materialHttp.MaterialHandlerGroup
	CategoryRepo         ports.MaterialCategoryRepository
	MaterialRepo         ports.MaterialRepository
	CategoryService      ports.MaterialCategoryService
	MaterialService      ports.MaterialService
	EditorialService     ports.MaterialEditorialService
	QueryService         ports.MaterialQueryService
	ViewService          ports.MaterialViewService
	ViewProjection       ports.MaterialViewRepository
}

// NewMaterialContainer creates a new MaterialContainer.
func NewMaterialContainer(
	client *internalent.EntClient,
	localCache cache.LocalCache[string, any],
	confirmationSecret []byte,
	adminConfig settings.AdminCommand,
) (*MaterialContainer, error) {
	if client == nil || client.Client() == nil || localCache == nil {
		return nil, fmt.Errorf("initialize material: invalid database")
	}

	// Repositories
	categoryRepo := db.NewMaterialCategoryRepository(client)
	materialRepo := db.NewMaterialRepository(client)
	productionRepo, err := db.NewProductionRepository(client)
	if err != nil {
		return nil, fmt.Errorf("initialize material production repository: %w", err)
	}
	contentRepository, err := contentrender.NewEntRepository(client.Client())
	if err != nil {
		return nil, fmt.Errorf("initialize material content repository: %w", err)
	}
	renderer, err := contentrender.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("initialize material renderer: %w", err)
	}
	transactions := platformtx.NewEntUnitOfWork(client.Client())
	auditAppender := audit.NewRepository()
	journal := platformmutation.NewAuditJournal(
		transactions,
		auditAppender,
	)
	productionJournal := platformmutation.NewJournal(
		transactions, auditAppender, eventing.NewOutbox(),
	)

	// Services
	caches := appcache.New(localCache)
	categoryService := service.NewMaterialCategoryService(categoryRepo, caches.MaterialCategory, journal)
	materialService := service.NewMaterialService(materialRepo, journal)
	editorialService, err := service.NewProductionService(service.ProductionDependencies{
		Repository: productionRepo, Content: contentRepository,
		Renderer: renderer, Journal: productionJournal,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize material editorial service: %w", err)
	}
	confirmationProtector, err := admincommand.NewConfirmationProtector(
		confirmationSecret,
		adminConfig.ConfirmationTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize material command confirmation: %w", err)
	}
	lifecycleWorkflow, err := materiallifecycle.New(
		editorialService,
		confirmationProtector,
		materiallifecycle.Config{
			RecentAuthenticationTTL: adminConfig.RecentAuthenticationTTL,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize material lifecycle workflow: %w", err)
	}
	queryService, err := service.NewSearchService(productionRepo, contentRepository)
	if err != nil {
		return nil, fmt.Errorf("initialize material query service: %w", err)
	}
	viewService, err := service.NewViewService(service.ViewDependencies{
		Repository: productionRepo, Transactions: transactions,
		Outbox: eventing.NewOutbox(), Secret: confirmationSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize material view service: %w", err)
	}

	// Handlers
	handlerGroup := &materialHttp.MaterialHandlerGroup{
		CategoryHandler: materialHttp.NewMaterialCategoryHandler(categoryService),
		MaterialHandler: materialHttp.NewProductionMaterialHandler(
			materialService, editorialService, queryService, viewService,
			lifecycleWorkflow,
		),
	}

	return &MaterialContainer{
		MaterialHandlerGroup: handlerGroup,
		CategoryRepo:         categoryRepo,
		MaterialRepo:         materialRepo,
		CategoryService:      categoryService,
		MaterialService:      materialService,
		EditorialService:     editorialService,
		QueryService:         queryService,
		ViewService:          viewService,
		ViewProjection:       productionRepo,
	}, nil
}
