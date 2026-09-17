// Package ports defines Judgify's stable internal capability catalog.
package ports

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
)

// Material capabilities are added here during the Material migration.

// MaterialCategoryRepository defines the material category data access interface.
type MaterialCategoryRepository interface {
	FindAll(ctx context.Context) ([]*entity.MaterialCategory, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.MaterialCategory], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.MaterialCategory, error)
	Create(ctx context.Context, e *entity.MaterialCategory) error
	Update(ctx context.Context, e *entity.MaterialCategory) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// MaterialCategoryCache isolates cache-aside policy from category use cases.
type MaterialCategoryCache interface {
	Get(context.Context, uuid.UUID, MaterialCategoryCacheLoader) (*contract.MaterialCategoryResult, error)
	FindAll(context.Context, MaterialCategoryListCacheLoader) ([]*contract.MaterialCategoryResult, error)
	Invalidate(context.Context, uuid.UUID) error
}

type MaterialCategoryCacheLoader func(context.Context) (*contract.MaterialCategoryResult, error)
type MaterialCategoryListCacheLoader func(context.Context) ([]*contract.MaterialCategoryResult, error)

// MaterialRepository defines the material data access interface.
type MaterialRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Material], error)
	FindPublic(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Material], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Material, error)
	GetPublic(ctx context.Context, id uuid.UUID) (*entity.Material, error)
	Create(ctx context.Context, e *entity.Material) error
	Update(ctx context.Context, e *entity.Material) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	IncrementViewCount(ctx context.Context, id uuid.UUID) error
	AddTags(ctx context.Context, materialID uuid.UUID, tagIDs []uuid.UUID) error
	RemoveAllTags(ctx context.Context, materialID uuid.UUID) error
}

// MaterialCategoryService defines the material category business logic interface.
type MaterialCategoryService interface {
	FindAll(ctx context.Context) ([]*contract.MaterialCategoryResult, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.MaterialCategoryResult], error)
	Get(ctx context.Context, id uuid.UUID) (*contract.MaterialCategoryResult, error)
	Create(ctx context.Context, req *contract.CreateMaterialCategoryCommand) (*contract.MaterialCategoryResult, error)
	Update(ctx context.Context, id uuid.UUID, req *contract.UpdateMaterialCategoryCommand) (*contract.MaterialCategoryResult, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// MaterialService defines the material business logic interface.
type MaterialService interface {
	FindPublic(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.MaterialResponse], error)
	GetPublic(ctx context.Context, id uuid.UUID) (*contract.MaterialResponse, error)
	FindAdmin(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.MaterialResponse], error)
	GetAdmin(ctx context.Context, id uuid.UUID) (*contract.MaterialResponse, error)
	Create(ctx context.Context, authorID uuid.UUID, req *contract.CreateMaterialCommand) (*contract.MaterialResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *contract.UpdateMaterialCommand) (*contract.MaterialResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type MaterialLifecycleAction string

const (
	MaterialLifecyclePublish       MaterialLifecycleAction = "publish"
	MaterialLifecycleArchive       MaterialLifecycleAction = "archive"
	MaterialLifecycleRollback      MaterialLifecycleAction = "rollback"
	MaterialLifecycleSubmitReview  MaterialLifecycleAction = "submit_review"
	MaterialLifecycleReturnToDraft MaterialLifecycleAction = "return_to_draft"
)

func (action MaterialLifecycleAction) Valid() bool {
	return action == MaterialLifecyclePublish ||
		action == MaterialLifecycleArchive ||
		action == MaterialLifecycleRollback ||
		action == MaterialLifecycleSubmitReview ||
		action == MaterialLifecycleReturnToDraft
}

type MaterialLifecycleProjection struct {
	Version                int64
	Status                 entity.MaterialState
	Visibility             entity.MaterialVisibility
	Slug                   string
	RevisionID             *uuid.UUID
	RevisionNumber         int64
	Title                  string
	CategoryID             uuid.UUID
	DifficultyID           uuid.UUID
	TagIDs                 []uuid.UUID
	RenderSourceID         uuid.UUID
	RenderArtifactID       uuid.UUID
	SourceChecksum         string
	RendererVersion        string
	SanitizerPolicyVersion string
}

type MaterialLifecyclePreview struct {
	Action           MaterialLifecycleAction
	MaterialID       uuid.UUID
	TargetRevisionID *uuid.UUID
	Before           MaterialLifecycleProjection
	After            MaterialLifecycleProjection
}

type PreviewMaterialLifecycleCommand struct {
	Action           MaterialLifecycleAction
	MaterialID       uuid.UUID
	TargetRevisionID uuid.UUID
	ExpectedVersion  int64
}

type ApplyMaterialLifecycleCommand struct {
	Action           MaterialLifecycleAction
	MaterialID       uuid.UUID
	TargetRevisionID uuid.UUID
	ExpectedVersion  int64
	CommandID        uuid.UUID
	RequestHash      [sha256.Size]byte
	Reason           string
}

type CreateDraftCommand struct {
	CommandID     uuid.UUID
	MaterialID    uuid.UUID
	Slug          string
	Title         string
	Description   string
	Content       []byte
	CategoryID    uuid.UUID
	DifficultyID  uuid.UUID
	TagIDs        []uuid.UUID
	AuthorID      uuid.UUID
	Visibility    entity.MaterialVisibility
	ChangeSummary string
}

type MaterialSaveDraftCommand struct {
	CommandID       uuid.UUID
	MaterialID      uuid.UUID
	Title           string
	Description     string
	Content         []byte
	CategoryID      uuid.UUID
	DifficultyID    uuid.UUID
	TagIDs          []uuid.UUID
	AuthorID        uuid.UUID
	Visibility      entity.MaterialVisibility
	ExpectedVersion int64
	ChangeSummary   string
}

type RenameMaterialCommand struct {
	CommandID       uuid.UUID
	ActorID         uuid.UUID
	MaterialID      uuid.UUID
	Slug            string
	ExpectedVersion int64
	Reason          string
}

type MaterialEditorialRepository interface {
	CreateIdentity(context.Context, entity.Material, entity.MaterialSlug) error
	LoadAggregate(context.Context, uuid.UUID) (entity.Material, error)
	Lock(context.Context, uuid.UUID) (entity.Material, error)
	NextRevisionNumber(context.Context, uuid.UUID) (int64, error)
	StoreRevision(context.Context, entity.MaterialRevision) error
	AdvanceDraft(context.Context, uuid.UUID, int64, entity.MaterialRevision, entity.MaterialVisibility) (entity.Material, error)
	LoadRevision(context.Context, uuid.UUID, uuid.UUID) (entity.MaterialRevision, error)
	Publish(context.Context, uuid.UUID, int64, entity.MaterialRevision, time.Time, string, string) (entity.Material, error)
	Archive(context.Context, uuid.UUID, int64, time.Time) (entity.Material, error)
	TransitionReview(context.Context, uuid.UUID, int64, entity.MaterialState) (entity.Material, error)
	Rename(context.Context, uuid.UUID, int64, entity.MaterialSlug, time.Time) (entity.Material, error)
	LoadCommandReceipt(context.Context, uuid.UUID) (entity.MaterialCommandReceipt, bool, error)
	LoadAdminProjection(context.Context, uuid.UUID) (entity.PublicMaterial, error)
}

type SearchQuery struct {
	Text          string
	CategoryID    *uuid.UUID
	TagIDs        []uuid.UUID
	DifficultyID  *uuid.UUID
	Status        *entity.MaterialState
	Sort          string
	Cursor        string
	Limit         int
	Authenticated bool
}

type SearchResult struct {
	Records    []entity.PublicMaterial
	NextCursor string
}

type MaterialQueryRepository interface {
	SearchPublic(context.Context, SearchQuery) (SearchResult, error)
	SearchAdmin(context.Context, SearchQuery) (SearchResult, error)
	ResolvePublicSlug(context.Context, string, bool) (entity.SlugResolution, error)
	RelatedPublic(context.Context, uuid.UUID, bool, int) ([]entity.PublicMaterial, error)
	ListRevisions(context.Context, uuid.UUID, int64, int) ([]entity.MaterialRevisionSummary, error)
	LoadRevision(context.Context, uuid.UUID, uuid.UUID) (entity.MaterialRevision, error)
	LoadAdminProjection(context.Context, uuid.UUID) (entity.PublicMaterial, error)
}

type MaterialViewRepository interface {
	RecordDailyView(context.Context, uuid.UUID, time.Time, [32]byte, bool) (entity.DailyViewReceipt, error)
	ApplyDailyViewProjection(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	ReconcileDailyViewProjection(context.Context, int) (int, error)
}

type MaterialEditorialService interface {
	CreateDraft(context.Context, CreateDraftCommand) (*contract.MaterialMutationReceipt, error)
	SaveDraft(context.Context, MaterialSaveDraftCommand) (*contract.MaterialMutationReceipt, error)
	Rename(context.Context, RenameMaterialCommand) (*contract.MaterialMutationReceipt, error)
	PreviewLifecycle(context.Context, PreviewMaterialLifecycleCommand) (MaterialLifecyclePreview, error)
	ApplyLifecycle(context.Context, ApplyMaterialLifecycleCommand) (*contract.MaterialLifecycleReceipt, error)
}

type MaterialQueryService interface {
	Search(context.Context, SearchQuery) (*contract.MaterialSearchResponse, error)
	SearchAdmin(context.Context, SearchQuery) (*contract.AdminMaterialSearchResponse, error)
	GetAdmin(context.Context, uuid.UUID) (*contract.MaterialResponse, error)
	Resolve(context.Context, string, bool) (*contract.MaterialSlugResponse, error)
	Related(context.Context, string, bool, int) ([]*contract.MaterialResponse, error)
	ListRevisions(context.Context, uuid.UUID, string, int) (*contract.MaterialRevisionPage, error)
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*contract.MaterialRevisionResponse, error)
}

type MaterialViewService interface {
	Record(context.Context, uuid.UUID) (bool, error)
}
