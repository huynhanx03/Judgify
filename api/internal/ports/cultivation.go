// Package ports defines Judgify's stable internal capability catalog.
package ports

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

// Cultivation capabilities are added here during the Cultivation migration.

var ErrElementVersionConflict = errors.New("element: optimistic version conflict")

// ElementRepository defines the element data access interface.
type ElementRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Element], error)
	FindAll(ctx context.Context) ([]*entity.Element, error)
	FindAllBounded(ctx context.Context, limit int) ([]*entity.Element, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Element, error)
	Create(ctx context.Context, e *entity.Element) error
	Update(ctx context.Context, e *entity.Element) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

type ElementCache interface {
	Get(context.Context, uuid.UUID, ElementCacheLoader) (*contract.ElementResponse, error)
	FindAll(context.Context, ElementListCacheLoader) ([]*contract.ElementResponse, error)
	Invalidate(context.Context, uuid.UUID) error
}

type ElementCacheLoader func(context.Context) (*contract.ElementResponse, error)
type ElementListCacheLoader func(context.Context) ([]*contract.ElementResponse, error)

// ElementService defines the element business logic interface.
type ElementService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ElementResponse], error)
	FindAll(ctx context.Context) ([]*contract.ElementResponse, error)
	Get(ctx context.Context, id uuid.UUID) (*contract.ElementResponse, error)
	Create(ctx context.Context, req *contract.CreateElementCommand) (*contract.ElementResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *contract.UpdateElementCommand) (*contract.ElementResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// LevelRepository defines the level data access interface.
type LevelRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Level], error)
	FindAll(ctx context.Context) ([]*entity.Level, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Level, error)
	Create(ctx context.Context, e *entity.Level) error
	Update(ctx context.Context, e *entity.Level) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

type LevelCache interface {
	Get(context.Context, uuid.UUID, LevelCacheLoader) (*contract.LevelResponse, error)
	FindAll(context.Context, LevelListCacheLoader) ([]*contract.LevelResponse, error)
	Invalidate(context.Context, uuid.UUID) error
}

type LevelCacheLoader func(context.Context) (*contract.LevelResponse, error)
type LevelListCacheLoader func(context.Context) ([]*contract.LevelResponse, error)

// LevelService defines the level business logic interface.
type LevelService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.LevelResponse], error)
	FindAll(ctx context.Context) ([]*contract.LevelResponse, error)
	Get(ctx context.Context, id uuid.UUID) (*contract.LevelResponse, error)
	Create(ctx context.Context, command *contract.CreateLevelCommand) (*contract.LevelResponse, error)
	Update(ctx context.Context, id uuid.UUID, command *contract.UpdateLevelCommand) (*contract.LevelResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProgressionLedgerPage struct {
	Entries    []entity.ProgressionLedgerEntry
	NextCursor *uuid.UUID
}

type ProgressionRepository interface {
	Snapshot(context.Context, uuid.UUID) (entity.ProgressionSnapshot, error)
	AdjustmentDelta(context.Context, uuid.UUID, uuid.UUID) (int64, error)
	ApplyAdjustment(context.Context, entity.AdjustmentCommand) (entity.AdjustmentResult, error)
	ListLedger(context.Context, uuid.UUID, *uuid.UUID, int) (ProgressionLedgerPage, error)
}

type ProgressionService interface {
	PreviewAdjustment(context.Context, uuid.UUID, int64, *uuid.UUID) (entity.AdjustmentPreview, error)
	ApplyAdjustment(context.Context, entity.AdjustmentCommand) (entity.AdjustmentResult, error)
	ListLedger(context.Context, uuid.UUID, *uuid.UUID, int) (ProgressionLedgerPage, error)
}

// RankRepository defines the rank data access interface.
type RankRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Rank], error)
	FindAll(ctx context.Context) ([]*entity.Rank, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Rank, error)
	Create(ctx context.Context, e *entity.Rank) error
	Update(ctx context.Context, e *entity.Rank) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

type RankCache interface {
	Get(context.Context, uuid.UUID, RankCacheLoader) (*contract.RankResponse, error)
	FindAll(context.Context, RankListCacheLoader) ([]*contract.RankResponse, error)
	Invalidate(context.Context, uuid.UUID) error
}

type RankCacheLoader func(context.Context) (*contract.RankResponse, error)
type RankListCacheLoader func(context.Context) ([]*contract.RankResponse, error)

// RankService defines the rank business logic interface.
type RankService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.RankResponse], error)
	FindAll(ctx context.Context) ([]*contract.RankResponse, error)
	Get(ctx context.Context, id uuid.UUID) (*contract.RankResponse, error)
	Create(ctx context.Context, command *contract.CreateRankCommand) (*contract.RankResponse, error)
	Update(ctx context.Context, id uuid.UUID, command *contract.UpdateRankCommand) (*contract.RankResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type RankingProjectionQuery struct {
	Kind   entity.RankingKind
	Cursor *entity.RankingCursor
	Limit  int
}

type RankingProjectionRepository interface {
	Page(context.Context, RankingProjectionQuery) (entity.RankingProjectionPage, error)
	Privacy(context.Context, uuid.UUID) (entity.RankingPrivacy, error)
	UpdatePrivacy(context.Context, entity.RankingPrivacyCommand) (entity.RankingPrivacyResult, error)
}

type RankingService interface {
	Page(context.Context, entity.RankingKind, string, int) (*contract.RankingPageResponse, error)
	Privacy(context.Context, uuid.UUID) (*contract.RankingPrivacyResponse, error)
	UpdatePrivacy(context.Context, entity.RankingPrivacyCommand) (*contract.RankingPrivacyResponse, error)
}

type RankingReconciliationRepository interface {
	Preview(context.Context) (entity.RankingReconciliationPlan, error)
	Prepare(context.Context, uuid.UUID, entity.RankingReconciliationPlan) (entity.RankingReconciliationPlan, error)
	ReconcileBatch(context.Context, entity.RankingReconciliationBatchQuery) (entity.RankingReconciliationBatch, error)
	State(context.Context) (entity.RankingReconciliationState, error)
}

type RankingOperationStarter interface {
	StartInTransaction(context.Context, operation.StartRequest) (operation.CreateResult, error)
}

type RankingOperationReader interface {
	Get(context.Context, uuid.UUID) (operation.Operation, error)
	List(context.Context, operation.ListFilter) (operation.Page, error)
}

type RankingReconciliationService interface {
	Preview(context.Context, entity.RankingReconciliationPreviewCommand) (*contract.RankingReconciliationPreviewResponse, error)
	Start(context.Context, entity.RankingReconciliationCommand) (*contract.RankingReconciliationStartResponse, error)
	Status(context.Context) (*contract.RankingReconciliationStatusResponse, error)
}

// RarityRepository defines the rarity data access interface.
type RarityRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Rarity], error)
	FindAll(ctx context.Context) ([]*entity.Rarity, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Rarity, error)
	Create(ctx context.Context, r *entity.Rarity) error
	Update(ctx context.Context, r *entity.Rarity) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

type RarityCache interface {
	Get(context.Context, uuid.UUID, RarityCacheLoader) (*contract.RarityResponse, error)
	FindAll(context.Context, RarityListCacheLoader) ([]*contract.RarityResponse, error)
	Invalidate(context.Context, uuid.UUID) error
}

type RarityCacheLoader func(context.Context) (*contract.RarityResponse, error)
type RarityListCacheLoader func(context.Context) ([]*contract.RarityResponse, error)

// RarityService defines the rarity business logic interface.
type RarityService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.RarityResponse], error)
	FindAll(ctx context.Context) ([]*contract.RarityResponse, error)
	Get(ctx context.Context, id uuid.UUID) (*contract.RarityResponse, error)
	Create(ctx context.Context, command *contract.CreateRarityCommand) (*contract.RarityResponse, error)
	Update(ctx context.Context, id uuid.UUID, command *contract.UpdateRarityCommand) (*contract.RarityResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// RewardProfileRepository is the sole write/read boundary for reward-bearing
// trait assignments. Callers never derive rewards from the mutable user_traits
// projection.
type RewardProfileRepository interface {
	Current(context.Context, uuid.UUID) (entity.UserRewardProfileRevision, error)
	EffectiveAt(context.Context, uuid.UUID, time.Time) (entity.UserRewardProfileRevision, error)
	PreviewAssignment(context.Context, uuid.UUID, entity.RewardProfileSelection) (entity.RewardProfileAssignmentPreview, error)
	Assign(context.Context, entity.RewardProfileAssignmentCommand) (entity.RewardProfileAssignmentResult, error)
	ProvisionOnboarding(context.Context, uuid.UUID, entity.TraitOffer, entity.RewardProfileSelection) (entity.UserRewardProfileRevision, error)
}

type RewardProfileService interface {
	Current(context.Context, uuid.UUID) (entity.UserRewardProfileRevision, error)
	PreviewAssignment(context.Context, uuid.UUID, entity.RewardProfileSelection) (entity.RewardProfileAssignmentPreview, error)
	Assign(context.Context, entity.RewardProfileAssignmentCommand) (entity.RewardProfileAssignmentResult, error)
}

type RewardRulePage struct {
	Rules      []entity.RewardRuleRevision
	NextCursor *int64
}

type RewardRuleRepository interface {
	Current(context.Context, time.Time) (entity.RewardRuleRevision, error)
	List(context.Context, *int64, int) (RewardRulePage, error)
	Publish(context.Context, entity.RewardRulePublicationCommand) (entity.RewardRulePublicationResult, error)
}

type RewardRuleService interface {
	Current(context.Context) (entity.RewardRuleRevision, error)
	List(context.Context, *int64, int) (RewardRulePage, error)
	PreviewPublication(context.Context, entity.RewardRuleConfiguration) (entity.RewardRulePublicationPreview, error)
	Publish(context.Context, entity.RewardRulePublicationCommand) (entity.RewardRulePublicationResult, error)
}

type SolveRewardOutcome string

const (
	SolveRewardGranted      SolveRewardOutcome = "grant"
	SolveRewardReversed     SolveRewardOutcome = "reversal"
	SolveRewardPointerMoved SolveRewardOutcome = "pointer_move"
	SolveRewardDuplicate    SolveRewardOutcome = "duplicate"
	SolveRewardGapDeferred  SolveRewardOutcome = "gap_deferred"
)

// SolveRewardResult distinguishes an applied transition from an idempotent
// duplicate and a safely deferred aggregate gap. A deferred gap is successful
// delivery: PostgreSQL reconciliation, not Forge redelivery, repairs ordering.
type SolveRewardResult struct {
	Outcome              SolveRewardOutcome
	TransitionApplied    bool
	LedgerEntryID        *uuid.UUID
	TotalExperienceDelta int64
	ElementDeltaCount    int
}

// SolveRewardProjector applies live events and rebuilds the same projection
// from immutable PostgreSQL transition history.
type SolveRewardProjector interface {
	ApplySolveTransition(
		context.Context,
		submissioncontract.ProblemSolveStatusChangedV1,
	) (SolveRewardResult, error)
	ReconcileSolveRewards(context.Context, int) (int, error)
}

// PermanentSolveRewardError marks corrupt, conflicting, or semantically
// impossible input. Delivery adapters may durably quarantine it; ordinary
// database/cancellation failures remain retryable and must not advance offsets.
type PermanentSolveRewardError struct{ Cause error }

func (err *PermanentSolveRewardError) Error() string {
	if err == nil || err.Cause == nil {
		return "cultivation: permanent solve reward rejection"
	}
	return "cultivation: permanent solve reward rejection: " + err.Cause.Error()
}

func (err *PermanentSolveRewardError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func (*PermanentSolveRewardError) Permanent() bool { return true }

func PermanentSolveReward(cause error) error {
	if cause == nil {
		return nil
	}
	var classified *PermanentSolveRewardError
	if errors.As(cause, &classified) {
		return cause
	}
	return &PermanentSolveRewardError{Cause: cause}
}

func IsPermanentSolveReward(err error) bool {
	var classified *PermanentSolveRewardError
	return errors.As(err, &classified)
}

// TraitRepository defines the trait data access interface.
type TraitRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Trait], error)
	FindAll(ctx context.Context) ([]*entity.Trait, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Trait, error)
	Create(ctx context.Context, e *entity.Trait) error
	Update(ctx context.Context, e *entity.Trait, expectedVersion int64) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

type TraitCache interface {
	Get(context.Context, uuid.UUID, TraitCacheLoader) (*contract.TraitResponse, error)
	FindAll(context.Context, TraitListCacheLoader) ([]*contract.TraitResponse, error)
	Invalidate(context.Context, uuid.UUID) error
}

type TraitCacheLoader func(context.Context) (*contract.TraitResponse, error)
type TraitListCacheLoader func(context.Context) ([]*contract.TraitResponse, error)

// TraitService defines the trait business logic interface.
type TraitService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.TraitResponse], error)
	FindAll(ctx context.Context) ([]*contract.TraitResponse, error)
	Get(ctx context.Context, id uuid.UUID) (*contract.TraitResponse, error)
	Create(ctx context.Context, command *contract.CreateTraitCommand) (*contract.TraitResponse, error)
	Update(ctx context.Context, id uuid.UUID, command *contract.UpdateTraitCommand) (*contract.TraitResponse, error)
	Delete(ctx context.Context, id uuid.UUID, expectedVersion int64, reason string) error
}

// TraitEffectRepository owns append-only effect history. Publication must join
// the caller's transaction because the current trait row, effect revision,
// catalog revision, audit, and outbox are one command.
type TraitEffectRepository interface {
	// LockTimeline must be the first database operation of a trait mutation.
	// Keeping the global reward timeline ahead of trait row locks gives trait
	// publication, profile assignment, and solve capture one lock order.
	LockTimeline(context.Context) error
	Publish(
		context.Context,
		uuid.UUID,
		int64,
		entity.TraitEffectSpec,
	) (entity.TraitEffectRevision, error)
	Current(context.Context, uuid.UUID) (entity.TraitEffectRevision, error)
	EffectiveAt(context.Context, uuid.UUID, time.Time) (entity.TraitEffectRevision, error)
}

type CreateOfferCommand struct {
	PreSessionID   uuid.UUID
	IdempotencyKey uuid.UUID
}

type ConsumeOfferCommand struct {
	OfferID          uuid.UUID
	PreSessionID     uuid.UUID
	ExpectedSequence int64
	RootBoneID       uuid.UUID
	TalentIDs        []uuid.UUID
	UserID           uuid.UUID
}

type TraitOfferRepository interface {
	LockCatalog(context.Context) error
	LoadLatestCatalog(context.Context) (entity.CatalogRevision, bool, error)
	LoadLiveCatalog(context.Context) ([]entity.CatalogTrait, error)
	StoreCatalog(context.Context, entity.CatalogRevision) error
	LockPreSession(context.Context, uuid.UUID) error
	LoadByIdempotency(context.Context, uuid.UUID, uuid.UUID) (entity.TraitOffer, bool, error)
	CountForPreSession(context.Context, uuid.UUID) (int, error)
	NextSequence(context.Context, uuid.UUID) (int64, error)
	ExpireIssued(context.Context, uuid.UUID, time.Time) error
	SupersedeIssued(context.Context, uuid.UUID, time.Time) error
	StoreOffer(context.Context, entity.TraitOffer) error
	LockOffer(context.Context, uuid.UUID) (entity.TraitOffer, error)
	ConsumeOffer(context.Context, uuid.UUID, uuid.UUID, time.Time) error
}

type TraitOfferService interface {
	Create(context.Context, CreateOfferCommand) (*contract.TraitOfferResponse, error)
	RefreshCatalog(context.Context) error
	InvalidatePool(context.Context) error
}

type TraitCatalogInvalidator interface {
	InvalidatePool(context.Context) error
}

type TraitOfferServiceOptions struct {
	MaximumOffers int
	OfferTTL      time.Duration
	Entropy       io.Reader
}

// UserElementExpRepository initializes the bounded per-element projection set
// during registration. Reward application owns all subsequent mutations.
type UserElementExpRepository interface {
	CreateBulk(ctx context.Context, entities []*entity.UserElementExp) error
}

// UserStatsRepository initializes the authoritative user progression row
// during registration. Runtime progression and rating mutations are owned by
// their dedicated ledger repositories.
type UserStatsRepository interface {
	Create(ctx context.Context, e *entity.UserStats) error
}

// UserDifficultyStatsRepository initializes per-difficulty projection rows
// during registration. Solve rewards update them through the reward ledger.
type UserDifficultyStatsRepository interface {
	CreateBulk(ctx context.Context, userID uuid.UUID, difficultyIDs []uuid.UUID) error
}

// UserTraitRepository defines the user trait data access interface.
type UserTraitRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserTrait], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.UserTrait, error)
}

// UserTraitService defines the user trait business logic interface.
type UserTraitService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.UserTraitResponse], error)
	Get(ctx context.Context, id uuid.UUID) (*contract.UserTraitResponse, error)
}
