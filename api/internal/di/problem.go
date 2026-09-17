package di

import (
	"context"
	"fmt"

	"github.com/huynhanx03/go-common/pkg/common/cache"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/infrastructure/releasecatalog"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driver/http"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
	service "github.com/huynhanx03/judgify/internal/problem/usecase"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// ProblemContainer holds the composed Problem domain dependencies.
type ProblemContainer struct {
	ProblemHandlerGroup             *http.ProblemHandlerGroup
	ReleaseID                       string
	ApprovedReleaseManifestChecksum string

	ProblemRepo    ports.ProblemRepository
	TagRepo        ports.TagRepository
	DifficultyRepo ports.DifficultyRepository

	ProblemService     ports.ProblemService
	AuthoringQuery     ports.AuthoringQuery
	AuthoringCatalog   ports.AuthoringCatalogQuery
	DraftService       ports.DraftService
	PublicationService ports.PublicationService
	ImmutableReader    ports.ImmutableProblemReader
	PublicSamples      ports.PublicSampleReader
	TagService         ports.TagService
	DifficultyService  ports.DifficultyService
}

// NewProblemContainer creates the complete read + immutable authoring graph.
// Production uses the same strict release artifact as workers. A development
// API may instead expose non-executable, code-owned authoring metadata.
func NewProblemContainer(
	ctx context.Context,
	client *internalent.EntClient,
	localCache cache.LocalCache[string, any],
	txMgr tx.Manager,
	judgeConfig settings.Judge,
	allowDevelopmentAuthoringCatalog bool,
) (*ProblemContainer, error) {
	if ctx == nil || txMgr == nil || client == nil || client.Client() == nil ||
		localCache == nil {
		return nil, fmt.Errorf("initialize problem: invalid dependency")
	}
	catalog, err := loadProblemCatalog(
		ctx,
		judgeConfig,
		allowDevelopmentAuthoringCatalog,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize problem release catalog: %w", err)
	}
	approvedManifestChecksum := ""
	if judgeConfig.APIAuthoringCatalogMode == settings.JudgeAPICatalogApprovedRelease {
		approvedCatalog, ok := catalog.(interface{ ApprovalChecksum() string })
		if !ok || approvedCatalog.ApprovalChecksum() == "" {
			return nil, fmt.Errorf("initialize problem release catalog: approval binding unavailable")
		}
		approvedManifestChecksum = approvedCatalog.ApprovalChecksum()
	}
	commonlogger.FromContext(ctx).Info(
		"problem authoring catalog initialized",
		zap.String("release_id", catalog.ReleaseID()),
		zap.String("catalog_mode", string(judgeConfig.APIAuthoringCatalogMode)),
	)
	contentRepository, err := contentrender.NewEntRepository(client.Client())
	if err != nil {
		return nil, fmt.Errorf("initialize problem content repository: %w", err)
	}
	revisionRepository, err := db.NewRevisionRepository(
		client.Client(),
		contentRepository,
		entity.IdentityAssetDecoder{},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize problem revision repository: %w", err)
	}
	taxonomyCatalog, err := db.NewTaxonomyCatalog(client.Client())
	if err != nil {
		return nil, fmt.Errorf("initialize problem taxonomy catalog: %w", err)
	}
	renderer, err := contentrender.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("initialize problem renderer: %w", err)
	}
	authorizer := service.AggregateAuthorizer{}
	authoringQuery, err := service.NewAuthoringQuery(
		service.AuthoringQueryDependencies{
			Repository: revisionRepository,
			Authorizer: authorizer,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize problem authoring query: %w", err)
	}
	authoringCatalog, err := service.NewAuthoringCatalog(catalog)
	if err != nil {
		return nil, fmt.Errorf("initialize problem authoring catalog: %w", err)
	}
	transactions := platformtx.NewEntUnitOfWork(client.Client())
	auditAppender := audit.NewRepository()
	outboxAppender := eventing.NewOutbox()
	draftService, err := service.NewDraftService(service.DraftDependencies{
		Transactions: transactions,
		Repository:   revisionRepository,
		Authorizer:   authorizer,
		Catalog:      catalog,
		Runtimes:     catalog,
		Checkers:     catalog,
		Taxonomy:     taxonomyCatalog,
		Renderer:     renderer,
		Content:      contentRepository,
		Audit:        auditAppender,
		Outbox:       outboxAppender,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize problem draft service: %w", err)
	}
	publicationService, err := service.NewPublicationService(
		service.PublicationDependencies{
			Transactions: transactions,
			Repository:   revisionRepository,
			Authorizer:   authorizer,
			Catalog:      catalog,
			Runtimes:     catalog,
			Checkers:     catalog,
			Taxonomy:     taxonomyCatalog,
			Assets:       entity.IdentityAssetVerifier{},
			Samples:      entity.IdentityAssetDecoder{},
			Audit:        auditAppender,
			Outbox:       outboxAppender,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize problem publication service: %w", err)
	}

	// Repositories
	tagRepo := db.NewTagRepository(client)
	difficultyRepo := db.NewDifficultyRepository(client)
	problemRepo := db.NewProblemRepository(client)

	// Services
	caches := appcache.New(localCache)
	tagService := service.NewTagService(tagRepo, caches.ProblemTag)
	difficultyService := service.NewDifficultyService(difficultyRepo, caches.ProblemDifficulty)
	problemService := service.NewProblemService(problemRepo, tagRepo, difficultyRepo, txMgr)
	publicSamples, err := service.NewPublicSampleReader(
		revisionRepository,
		entity.IdentityAssetDecoder{},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize public sample reader: %w", err)
	}

	// Handlers
	problemHandlerGroup := &http.ProblemHandlerGroup{
		ProblemHandler: http.NewProblemHandlerWithAuthoring(
			problemService,
			authoringQuery,
			authoringCatalog,
			draftService,
			publicationService,
		),
		TestCaseHandler:   http.NewTestCaseHandler(publicSamples),
		TagHandler:        http.NewTagHandler(tagService),
		DifficultyHandler: http.NewDifficultyHandler(difficultyService),
	}

	return &ProblemContainer{
		ProblemHandlerGroup:             problemHandlerGroup,
		ReleaseID:                       catalog.ReleaseID(),
		ApprovedReleaseManifestChecksum: approvedManifestChecksum,

		ProblemRepo:    problemRepo,
		TagRepo:        tagRepo,
		DifficultyRepo: difficultyRepo,

		ProblemService:     problemService,
		AuthoringQuery:     authoringQuery,
		AuthoringCatalog:   authoringCatalog,
		DraftService:       draftService,
		PublicationService: publicationService,
		ImmutableReader:    revisionRepository,
		PublicSamples:      publicSamples,
		TagService:         tagService,
		DifficultyService:  difficultyService,
	}, nil
}

type problemCatalog interface {
	ports.AuthoringCatalogSource
	ports.AuthoringCatalogFence
	ports.ExecutionCatalog
	ports.CheckerCatalog
}

func loadProblemCatalog(
	ctx context.Context,
	config settings.Judge,
	allowDevelopment bool,
) (problemCatalog, error) {
	switch config.APIAuthoringCatalogMode {
	case settings.JudgeAPICatalogApprovedRelease:
		return releasecatalog.Load(
			ctx,
			config.ReleaseID,
			config.ReleaseManifestPath,
		)
	case settings.JudgeAPICatalogDevelopment:
		if !allowDevelopment {
			return nil, fmt.Errorf("development authoring catalog is disabled")
		}
		return service.NewDevelopmentAuthoringCatalog(config.ReleaseID)
	default:
		return nil, fmt.Errorf("unknown API authoring catalog mode")
	}
}
