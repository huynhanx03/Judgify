// Package ports defines Judgify's stable internal capability catalog.
package ports

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

var (
	ErrInvalidCampaignAudience              = errors.New("identity: invalid campaign audience")
	IdentityErrCampaignAudienceTooLarge     = errors.New("identity: campaign audience exceeds limit")
	ErrCampaignRoleNotFound                 = errors.New("identity: campaign role not found")
	ErrCampaignRecentAuthenticationRequired = errors.New("identity: recent authentication required")
)

type CampaignAudienceSelector struct {
	Kind    string
	RoleID  *uuid.UUID
	UserIDs []uuid.UUID
}

type CampaignAudienceMember struct {
	UserID   uuid.UUID
	Username string
}

type CampaignAudienceSnapshot struct {
	Members []CampaignAudienceMember
	At      time.Time
}

type CampaignRecipientEligibility struct {
	Eligible bool
	SkipCode string
}

// CampaignAudienceParticipant is Identity's narrow transaction-aware
// participant for notification campaigns. Notification never imports account
// repositories or Ent entities.
type CampaignAudienceParticipant interface {
	SnapshotCampaignAudience(
		context.Context,
		CampaignAudienceSelector,
		int,
		time.Time,
	) (CampaignAudienceSnapshot, error)
	CheckCampaignRecipient(
		context.Context,
		uuid.UUID,
		string,
		*uuid.UUID,
	) (CampaignRecipientEligibility, error)
	RequireRecentReauthentication(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		time.Time,
	) error
}

// AuthorizedTraitOffer is an opaque application capability passed through
// Identity without interpreting cultivation choices. The onboarding workflow
// resolves it from the durable offer aggregate and defensively clones slices.
type AuthorizedTraitOffer struct {
	OfferID           uuid.UUID
	PreSessionID      uuid.UUID
	ExpectedSequence  int64
	CatalogRevisionID uuid.UUID
	CatalogChecksum   [sha256.Size]byte
	OfferChecksum     [sha256.Size]byte
	RootBoneID        uuid.UUID
	TalentIDs         []uuid.UUID
}

// RegistrationLifecycle is the narrow application boundary needed by account
// registration. Consume and Provision are invoked with the same transaction-
// scoped context as user/session persistence, so any failure rolls back all
// participating domains and the replay nonce together.
type RegistrationLifecycle interface {
	Authorize(
		preSessionID uuid.UUID,
		offerID uuid.UUID,
		expectedSequence int64,
		catalogRevisionID uuid.UUID,
		catalogChecksum string,
		offerChecksum string,
		rootBoneID uuid.UUID,
		talentIDs []uuid.UUID,
	) (AuthorizedTraitOffer, error)
	Consume(ctx context.Context, authorized AuthorizedTraitOffer) error
	Provision(ctx context.Context, userID uuid.UUID, authorized AuthorizedTraitOffer) error
}

var ErrAttributeDefinitionVersionConflict = errors.New("attribute definition version conflict")

type AttributeLifecycleAction string

const (
	AttributeLifecycleActivate AttributeLifecycleAction = "activate"
	AttributeLifecycleArchive  AttributeLifecycleAction = "archive"
)

func (action AttributeLifecycleAction) Valid() bool {
	return action == AttributeLifecycleActivate || action == AttributeLifecycleArchive
}

type PreviewAttributeLifecycleCommand struct {
	Action              AttributeLifecycleAction
	DefinitionID        uuid.UUID
	CandidateRevisionID uuid.UUID
	ExpectedVersion     int64
}

type ApplyAttributeLifecycleCommand struct {
	Action              AttributeLifecycleAction
	DefinitionID        uuid.UUID
	CandidateRevisionID uuid.UUID
	ExpectedVersion     int64
	CommandID           uuid.UUID
	RequestHash         [sha256.Size]byte
	ReviewedPreviewHash [sha256.Size]byte
	Reason              string
}

type AttributeLifecyclePreview struct {
	Action            AttributeLifecycleAction
	DefinitionID      uuid.UUID
	ExpectedVersion   int64
	ResultingVersion  int64
	CurrentRevision   *dto.AttributeDefinitionRevisionResponse
	CandidateRevision *dto.AttributeDefinitionRevisionResponse
	SourceValueCount  int64
	TargetValueCount  int64
	RequiresMigration bool
}

// HashAttributeLifecyclePreview creates the stable, domain-separated digest
// echoed by an apply command. It binds the confirmation to the exact facts an
// operator reviewed, including affected-value count and migration mode, while
// remaining reproducible after a lost response from a committed command.
func HashAttributeLifecyclePreview(preview AttributeLifecyclePreview) ([sha256.Size]byte, error) {
	if !preview.Action.Valid() || preview.DefinitionID == uuid.Nil ||
		preview.ExpectedVersion < 1 ||
		preview.ResultingVersion != preview.ExpectedVersion+1 ||
		preview.SourceValueCount < 0 || preview.TargetValueCount < 0 {
		return [sha256.Size]byte{}, fmt.Errorf("invalid attribute lifecycle preview")
	}
	payload, err := json.Marshal(preview)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("encode attribute lifecycle preview: %w", err)
	}
	if len(payload) == 0 || len(payload) > 64<<10 {
		return [sha256.Size]byte{}, fmt.Errorf("invalid attribute lifecycle preview encoding")
	}
	framed := make([]byte, 0, len(payload)+64)
	framed = append(framed, "judgify/identity/attribute-lifecycle-preview/v1"...)
	framed = append(framed, 0)
	framed = append(framed, payload...)
	return sha256.Sum256(framed), nil
}

type AttributeCommandReceipt struct {
	EventID          uuid.UUID
	EventType        string
	AggregateID      uuid.UUID
	AggregateVersion int64
	Payload          json.RawMessage
	OccurredAt       time.Time
}

// AttributeDefinitionRepository defines the interface for attribute definition persistence.
type AttributeDefinitionRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.AttributeDefinition], error)
	FindAll(ctx context.Context) ([]*entity.AttributeDefinition, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.AttributeDefinition, error)
	GetByKey(ctx context.Context, key string) (*entity.AttributeDefinition, error)
	Create(ctx context.Context, e *entity.AttributeDefinition) error
	GetRevision(ctx context.Context, id uuid.UUID) (*entity.AttributeDefinitionRevision, error)
	ListRevisions(ctx context.Context, definitionID uuid.UUID, before int64, limit int) ([]*entity.AttributeDefinitionRevision, error)
	CreateRevision(ctx context.Context, revision *entity.AttributeDefinitionRevision) error
	SetInitialRevision(ctx context.Context, definitionID, revisionID uuid.UUID) error
	SetDraftRevision(ctx context.Context, definitionID uuid.UUID, expectedVersion int64, revision *entity.AttributeDefinitionRevision, requiredOnOnboarding *bool) (*entity.AttributeDefinition, error)
	ActivateRevision(ctx context.Context, definitionID uuid.UUID, expectedVersion int64, sourceRevisionID uuid.UUID, revision *entity.AttributeDefinitionRevision, requireEmptySource bool) (*entity.AttributeDefinition, error)
	Archive(ctx context.Context, definitionID uuid.UUID, expectedVersion int64) (*entity.AttributeDefinition, error)
	CountValues(ctx context.Context, definitionID, revisionID uuid.UUID) (int64, error)
	LoadCommandReceipt(ctx context.Context, commandID uuid.UUID) (AttributeCommandReceipt, bool, error)
}

// AttributeDefinitionCache owns L1 lookup caching for profile schema metadata.
type AttributeDefinitionCache interface {
	Get(context.Context, uuid.UUID, AttributeDefinitionCacheLoader) (*entity.AttributeDefinition, error)
	Invalidate(context.Context, uuid.UUID, string) error
}

type AttributeDefinitionCacheLoader func(context.Context) (*entity.AttributeDefinition, error)

// AttributeDefinitionService defines the attribute definition business logic interface.
type AttributeDefinitionService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.AttributeDefinitionResponse], error)
	Get(ctx context.Context, id uuid.UUID) (*dto.AttributeDefinitionResponse, error)
	ListRevisions(ctx context.Context, id uuid.UUID, cursor string, limit int) (*dto.AttributeDefinitionRevisionPage, error)
	Create(ctx context.Context, req *dto.CreateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *dto.UpdateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
	PreviewLifecycle(ctx context.Context, command PreviewAttributeLifecycleCommand) (AttributeLifecyclePreview, error)
	ApplyLifecycle(ctx context.Context, command ApplyAttributeLifecycleCommand) (*dto.AttributeLifecycleReceipt, error)
}

// AuthenticationService defines the authentication business logic interface.
type AuthenticationService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error)
	RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) (*dto.LogoutResponse, error)

	// OAuth
	OAuthStart(ctx context.Context, req *dto.OAuthStartRequest) (*dto.OAuthStartResponse, error)
	OAuthCallback(ctx context.Context, req *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error)
	OAuthInspectLogin(ctx context.Context, req *dto.OAuthLoginInspectionRequest) (*dto.OAuthLoginInspectionResponse, error)
	OAuthFinalizeLogin(ctx context.Context, req *dto.OAuthFinalizeLoginRequest) (*dto.OAuthFinalizeLoginResponse, error)
	OAuthFinalizeLink(ctx context.Context, req *dto.OAuthFinalizeLinkRequest) (*dto.OAuthFinalizeLinkResponse, error)
	OAuthIdentities(ctx context.Context, userID uuid.UUID) (*dto.OAuthIdentityListResponse, error)
	OAuthUnlink(ctx context.Context, userID, sessionID uuid.UUID, provider string) (*dto.OAuthFinalizeLinkResponse, error)

	// Password Recovery
	ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error)
	ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error)
}

type ReplaceRolePoliciesCommand struct {
	CommandID        uuid.UUID
	ExpectedRevision int64
	Reason           string
	RequestHash      [sha256.Size]byte
	Rules            []dto.PolicyRule
}

type ReplaceUserRolesCommand struct {
	CommandID        uuid.UUID
	ExpectedRevision int64
	Reason           string
	RequestHash      [sha256.Size]byte
	Roles            []string
}

// AuthorizationService defines DB-backed Casbin authorization operations.
type AuthorizationService interface {
	authorization.Authorizer
	SetUserRoles(ctx context.Context, userID uuid.UUID, roles []string) error
	SetUserRolesAtRevision(ctx context.Context, userID uuid.UUID, expectedRevision int64, reason string, roles []string) (*dto.UserRoleSnapshot, error)
	PreviewUserRoles(ctx context.Context, userID uuid.UUID, expectedRevision int64, roles []string) (*dto.UserRolePreview, error)
	ApplyUserRoles(ctx context.Context, userID uuid.UUID, command ReplaceUserRolesCommand) (*dto.UserRoleMutationReceipt, error)
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error)
	GetUserRoleSnapshot(ctx context.Context, userID uuid.UUID) (*dto.UserRoleSnapshot, error)
	EnsureCanDeactivateUser(ctx context.Context, userID uuid.UUID) error
	SetRolePolicies(ctx context.Context, roleKey string, expectedRevision int64, reason string, rules []dto.PolicyRule) (*dto.RolePolicySnapshot, error)
	ApplyRolePolicies(ctx context.Context, roleKey string, command ReplaceRolePoliciesCommand) (*dto.RolePolicyMutationReceipt, error)
	GetRolePolicies(ctx context.Context, roleKey string) (*dto.RolePolicySnapshot, error)
	PreviewRolePolicies(ctx context.Context, roleKey string, expectedRevision int64, rules []dto.PolicyRule) (*dto.RolePolicyPreview, error)
	DeleteRole(ctx context.Context, roleKey string) error
}

// AuthorizationCatalogReader exposes the deploy-owned capability catalog
// without widening the mutation surface consumed by other identity services.
type AuthorizationCatalogReader interface {
	Catalog(ctx context.Context) (*dto.AuthorizationCatalogResponse, error)
}

// EffectiveCapabilityReader exposes a catalog-bounded principal projection
// without exposing roles or raw authorization policy rows.
type EffectiveCapabilityReader interface {
	EffectiveCapabilities(ctx context.Context, userID uuid.UUID) (*dto.EffectiveCapabilitiesResponse, error)
}

var (
	// ErrInvalidAuthorizationPolicy reports malformed or unbounded persistence
	// input. Policy values are deliberately excluded from the error so callers
	// can log the classification without disclosing policy or user subjects.
	ErrInvalidAuthorizationPolicy = errors.New("identity: invalid authorization policy persistence input")
	// ErrAuthorizationRevisionConflict reports that the desired-revision compare
	// and swap lost to another committed mutation.
	ErrAuthorizationRevisionConflict = errors.New("identity: authorization revision conflict")
	// ErrAuthorizationStateNotInitialized reports a missing singleton state row.
	// Reads never create or silently substitute this security-sensitive fence.
	ErrAuthorizationStateNotInitialized = errors.New("identity: authorization state is not initialized")
)

// AuthorizationState is the durable, single-tenant authorization revision
// fence. UpdatedBy is nil for system-owned initialization or reconciliation.
type AuthorizationState struct {
	DesiredRevision int64
	UpdatedBy       *uuid.UUID
}

// AuthorizationCommandReceipt is the immutable, transport-neutral projection
// of the outbox row whose event identity is the canonical administrative
// command UUID. It contains no relay lease state and no raw Casbin tuple.
type AuthorizationCommandReceipt struct {
	EventID          uuid.UUID
	EventType        string
	AggregateID      uuid.UUID
	AggregateVersion int64
	Payload          json.RawMessage
	OccurredAt       time.Time
}

// AuthorizationPolicyRepository owns canonical Casbin rows and the durable
// desired-revision fence. Implementations resolve their database client from
// ctx so every operation joins an enclosing platform transaction.
type AuthorizationPolicyRepository interface {
	EnsureState(ctx context.Context, initialRevision int64) error
	// LoadRules loads the committed startup snapshot through the pinned Casbin
	// Ent adapter. Implementations must perform no runtime DDL or AutoSave.
	LoadRules(ctx context.Context, model string) ([]authcasbin.PolicyRule, error)
	State(ctx context.Context) (AuthorizationState, error)
	LockState(ctx context.Context) (AuthorizationState, error)
	ListRules(ctx context.Context) ([]authcasbin.PolicyRule, error)
	ReplaceRolePolicies(ctx context.Context, roleSubject string, rules []authcasbin.PolicyRule) error
	ReplaceUserRoles(ctx context.Context, userSubject string, roleSubjects []string) error
	DeleteRoleReferences(ctx context.Context, roleSubject string) error
	DeleteUserReferences(ctx context.Context, userSubject string) error
	AdvanceRevision(
		ctx context.Context,
		expected, next int64,
		updatedBy *uuid.UUID,
	) error
	ActiveRoleKeys(ctx context.Context, keys []string) (map[string]bool, error)
	// AuthorizationEligibleUserIDs accepts every non-deleted account state
	// whose durable role references must survive, including suspension.
	AuthorizationEligibleUserIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
	// ActiveUserIDs is deliberately narrower and must remain active-only for
	// invariants such as retaining at least one effective superadministrator.
	ActiveUserIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
	LoadCommandReceipt(ctx context.Context, commandID uuid.UUID) (AuthorizationCommandReceipt, bool, error)
}

// CacheService defines the interface for cache-related operations.
type CacheService interface {
	// InvalidatePermissionConfig increments the permission config version, invalidating all role permission caches.
	InvalidatePermissionConfig(ctx context.Context) error

	// GetPermissionConfigVersion retrieves the current permission config version.
	GetPermissionConfigVersion(ctx context.Context) (int64, error)
}

var (
	ErrCredentialAuthorityLost = errors.New("identity: credential authority lost")
	ErrCredentialConflict      = errors.New("identity: credential changed concurrently")
)

// PasswordCredentialRotation carries only password-hash material between the
// application service and its persistence adapter. Raw passwords never cross
// this port or enter transaction-bound storage logic.
type PasswordCredentialRotation struct {
	UserID       uuid.UUID
	CredentialID uuid.UUID
	ExpectedHash string
	Replacement  string
}

// CredentialRepository defines the credential data access interface.
type CredentialRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Credential], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Credential, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, credType string) (*entity.Credential, error)
	Create(ctx context.Context, e *entity.Credential) error
	Update(ctx context.Context, e *entity.Credential) error
	// RotatePassword atomically locks user then credential, replaces the hash
	// only when ExpectedHash is still current, and advances security_version.
	RotatePassword(ctx context.Context, rotation PasswordCredentialRotation) (uint64, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

var (
	ErrFederatedIdentityNotFound = errors.New("identity: federated identity not found")
	ErrLastAuthenticationMethod  = errors.New("identity: cannot remove last authentication method")
)

// FederatedIdentityRepository defines the federated identity data access interface.
type FederatedIdentityRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.FederatedIdentity], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.FederatedIdentity, error)
	GetByProviderAndSubject(ctx context.Context, provider, providerSubject string) (*entity.FederatedIdentity, error)
	GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*entity.FederatedIdentity, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*entity.FederatedIdentity, error)
	Create(ctx context.Context, e *entity.FederatedIdentity) error
	RecordAuthentication(ctx context.Context, e *entity.FederatedIdentity, at time.Time) error
	Unlink(ctx context.Context, userID, sessionID uuid.UUID, provider string, at time.Time) (*entity.FederatedIdentity, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

var (
	ErrOAuthCredentialRejected = errors.New("identity: oauth credential rejected")
	ErrOAuthCredentialExpired  = errors.New("identity: oauth credential expired")
	ErrOAuthTransitionConflict = errors.New("identity: oauth single-use transition conflict")
	ErrOAuthPurposeUnsupported = errors.New("identity: oauth purpose is not persisted")
)

// OAuthCallbackCompletion is prepared after the provider exchange. StateHash,
// TicketHash, and both ciphertexts are the only credential material admitted
// to this persistence boundary.
type OAuthCallbackCompletion struct {
	Provider       string
	StateHash      []byte `json:"-"`
	ExpectedFlowID uuid.UUID
	Ticket         entity.OAuthCompletionTicket
	CompletedAt    time.Time
}

type OAuthTicketClaim struct {
	TicketHash []byte `json:"-"`
	Purpose    entity.OAuthPurpose
	Binding    entity.OAuthBinding
	Now        time.Time
}

type OAuthTicketLookup struct {
	TicketHash []byte `json:"-"`
	Purpose    entity.OAuthPurpose
	Now        time.Time
}

// OAuthPendingFlow carries the immutable flow and the canonical completion
// binding. Link callbacks resolve UserID from the stored session; callers do
// not supply or trust a callback cookie for account authority.
type OAuthPendingFlow struct {
	Flow              entity.OAuthFlow
	CompletionBinding entity.OAuthBinding
}

type OAuthTicketClaimResult struct {
	Ticket *entity.OAuthCompletionTicket
	Fresh  bool
}

// OAuthDurableRepository owns the single-use database state machines.
//
// CreateFlow, CompleteCallback, and ConsumeTicket require an active platform
// UnitOfWork transaction. PendingFlow and TicketSnapshot are non-locking reads:
// the former supports remote provider exchange before CompleteCallback locks
// the flow, while the latter supports pre-transaction decryption and exact
// retry planning before ConsumeTicket locks and revalidates the ticket.
type OAuthDurableRepository interface {
	CreateFlow(context.Context, *entity.OAuthFlow) error
	PendingFlow(context.Context, string, []byte, time.Time) (*OAuthPendingFlow, error)
	CompleteCallback(context.Context, OAuthCallbackCompletion) error
	TicketSnapshot(context.Context, OAuthTicketLookup) (*entity.OAuthCompletionTicket, error)
	ConsumeTicket(context.Context, OAuthTicketClaim) (*OAuthTicketClaimResult, error)
}

var (
	ErrPasswordResetRejected    = errors.New("identity: password reset credential rejected")
	ErrPasswordResetRateLimited = errors.New("identity: password reset issuance rate limited")
)

// PasswordResetIssue contains only protected persistence material. Recipient
// plaintext and the raw one-time credential stay in the service/delivery path.
type PasswordResetIssue struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	ContactID             uuid.UUID
	ContactVersion        uint64
	UserSecurityVersion   uint64
	Token                 entity.KeyedTokenHash
	IssuedAt              time.Time
	ExpiresAt             time.Time
	ShortWindowStart      time.Time
	DailyWindowStart      time.Time
	MaximumPerShortWindow int
	MaximumPerDay         int
}

// PasswordResetRepository owns row locking and the single-use state machine.
type PasswordResetRepository interface {
	EligibleContact(context.Context, uuid.UUID, uint64) (*entity.RecoveryContact, error)
	Issue(context.Context, PasswordResetIssue) error
	ValidateIssued(context.Context, []entity.KeyedTokenHash, time.Time) error
	LockIssued(context.Context, []entity.KeyedTokenHash, time.Time) (*entity.PasswordResetIntent, error)
	Consume(context.Context, *entity.PasswordResetIntent, time.Time) error
}

type PasswordResetDeliveryMessage struct {
	Recipient              string
	Token                  string
	ExpiresAt              time.Time
	ProviderIdempotencyKey string
}

type RecoveryContactVerificationDeliveryMessage struct {
	Recipient              string
	Token                  string
	ExpiresAt              time.Time
	ProviderIdempotencyKey string
}

// AccountInvitationDeliveryMessage is intentionally shape-compatible with
// the other one-time security messages while remaining a distinct closed
// purpose at the delivery boundary.
type AccountInvitationDeliveryMessage struct {
	Recipient              string
	Token                  string
	ExpiresAt              time.Time
	ProviderIdempotencyKey string
}

// PasswordResetDelivery is the explicit secret-bearing boundary. Production
// adapters deliver; development may inject a discard/capture sink, but no
// implementation may log the message fields.
type PasswordResetDelivery interface {
	Deliver(context.Context, PasswordResetDeliveryMessage) error
}

type RecoveryContactVerificationDelivery interface {
	DeliverVerification(context.Context, RecoveryContactVerificationDeliveryMessage) error
}

type AccountInvitationDelivery interface {
	DeliverInvitation(context.Context, AccountInvitationDeliveryMessage) error
}

type SecurityEmailDelivery interface {
	PasswordResetDelivery
	RecoveryContactVerificationDelivery
	AccountInvitationDelivery
}

// PasswordResetDeliveryFailure lets a provider adapter expose only bounded,
// non-secret retry policy to the dispatcher.
type PasswordResetDeliveryFailure interface {
	error
	Retryable() bool
	ErrorClass() string
}

type PasswordResetLifecycle interface {
	Issue(context.Context, *entity.User) error
	// Obscure performs the bounded cryptographic preparation used by a real
	// issuance without persisting or scheduling anything. Public account-miss
	// paths use it to avoid a cheap enumeration branch.
	Obscure(context.Context) error
	Consume(context.Context, string, string) error
}

var (
	ErrProfileSchemaUnavailable = errors.New("identity profile schema unavailable")
	ErrProfileSchemaConflict    = errors.New("identity profile schema changed")
)

const ProfileSchemaStateKey = "onboarding"

type ProfileSchemaFence struct {
	ID       uuid.UUID
	Revision int64
}

type ProfileSchemaRepository interface {
	Ensure(context.Context) error
	LockShared(context.Context) (ProfileSchemaFence, error)
	LockExclusive(context.Context) (ProfileSchemaFence, error)
	Advance(context.Context, ProfileSchemaFence) (ProfileSchemaFence, error)
	ListRequired(context.Context) ([]*entity.AttributeDefinition, error)
}

type ProfileSchemaService interface {
	GetOnboardingSchema(context.Context) (*dto.ProfileSchemaResponse, error)
	ResolveRequiredValues(
		context.Context,
		uuid.UUID,
		dto.ProfileSchemaEvidence,
	) ([]RevisionedUserAttributeValueMutation, error)
}

var (
	ErrInvalidPublicProfile          = errors.New("identity: invalid public profile")
	ErrPublicProfileNotFound         = errors.New("identity: public profile not found")
	ErrInvalidProfilePrivacyCommand  = errors.New("identity: invalid profile privacy command")
	ErrProfilePrivacyVersionConflict = errors.New("identity: profile privacy version conflict")
	ErrProfilePrivacyReceiptMismatch = errors.New("identity: profile privacy receipt mismatch")
)

// PublicProfileRepository owns the anonymous projection gate and the
// self-service privacy aggregate. Implementations must return private,
// suspended, deleted and absent accounts through the same not-found path.
type PublicProfileRepository interface {
	GetPublicByUsername(ctx context.Context, username string) (*entity.PublicProfile, error)
	GetPrivacy(ctx context.Context, userID uuid.UUID) (*entity.ProfilePrivacyState, error)
	UpdatePrivacyCAS(
		ctx context.Context,
		userID uuid.UUID,
		expectedVersion int64,
		visibility entity.ProfileVisibility,
	) (*entity.ProfilePrivacyState, error)
	LoadPrivacyCommandReceipt(
		ctx context.Context,
		commandID uuid.UUID,
	) (ProfilePrivacyCommandReceipt, bool, error)
}

type UpdateProfilePrivacyCommand struct {
	UserID          uuid.UUID
	ActorID         uuid.UUID
	ActorSessionID  uuid.UUID
	CommandID       uuid.UUID
	Visibility      entity.ProfileVisibility
	ExpectedVersion int64
}

type ProfilePrivacyCommandReceipt struct {
	EventID          uuid.UUID
	EventType        string
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	Payload          json.RawMessage
	OccurredAt       time.Time
}

type PublicProfileService interface {
	GetPublic(ctx context.Context, username string) (*dto.PublicProfileResponse, error)
	GetPrivacy(ctx context.Context, userID uuid.UUID) (*dto.ProfilePrivacyResponse, error)
	UpdatePrivacy(
		ctx context.Context,
		command UpdateProfilePrivacyCommand,
	) (*dto.ProfilePrivacyMutationReceipt, error)
}

const (
	// MinimumReceiptCleanupBatchSize gives every cleanup category at least one
	// row per sweep. MaximumReceiptCleanupBatchSize bounds lock footprint.
	MinimumReceiptCleanupBatchSize = 13
	MaximumReceiptCleanupBatchSize = 10_000
)

// ReceiptCleanupRequest carries both lifecycle cutoffs so persistence adapters
// never embed retention policy. Now expires live state; TerminalBefore deletes
// already-terminal records after their configured retention window.
type ReceiptCleanupRequest struct {
	Now            time.Time
	TerminalBefore time.Time
	BatchSize      int
}

// ReceiptCleanupResult contains only low-cardinality operational counts. The
// cleanup boundary never returns receipt, session, user, or credential IDs.
type ReceiptCleanupResult struct {
	SessionIssuanceErased                  int
	RefreshRotationErased                  int
	RecoveryCommandReceiptsDeleted         int
	SessionCommandReceiptsDeleted          int
	VerificationTokensExpired              int
	PasswordResetTokensExpired             int
	SecurityEmailDeliveriesCancelled       int
	PendingRecoveryContactsRevoked         int
	RevokedRecoveryContactsErased          int
	TerminalSecurityEmailDeliveriesDeleted int
	TerminalVerificationTokensDeleted      int
	TerminalPasswordResetTokensDeleted     int
	TerminalRecoveryContactsDeleted        int
	MayHaveMore                            bool
}

func (result ReceiptCleanupResult) Total() int {
	return result.SessionIssuanceErased +
		result.RefreshRotationErased +
		result.RecoveryCommandReceiptsDeleted +
		result.SessionCommandReceiptsDeleted +
		result.VerificationTokensExpired +
		result.PasswordResetTokensExpired +
		result.SecurityEmailDeliveriesCancelled +
		result.PendingRecoveryContactsRevoked +
		result.RevokedRecoveryContactsErased +
		result.TerminalSecurityEmailDeliveriesDeleted +
		result.TerminalVerificationTokensDeleted +
		result.TerminalPasswordResetTokensDeleted +
		result.TerminalRecoveryContactsDeleted
}

// Add combines batch results without exposing row identifiers.
func (result *ReceiptCleanupResult) Add(batch ReceiptCleanupResult) {
	if result == nil {
		return
	}
	result.SessionIssuanceErased += batch.SessionIssuanceErased
	result.RefreshRotationErased += batch.RefreshRotationErased
	result.RecoveryCommandReceiptsDeleted += batch.RecoveryCommandReceiptsDeleted
	result.SessionCommandReceiptsDeleted += batch.SessionCommandReceiptsDeleted
	result.VerificationTokensExpired += batch.VerificationTokensExpired
	result.PasswordResetTokensExpired += batch.PasswordResetTokensExpired
	result.SecurityEmailDeliveriesCancelled += batch.SecurityEmailDeliveriesCancelled
	result.PendingRecoveryContactsRevoked += batch.PendingRecoveryContactsRevoked
	result.RevokedRecoveryContactsErased += batch.RevokedRecoveryContactsErased
	result.TerminalSecurityEmailDeliveriesDeleted += batch.TerminalSecurityEmailDeliveriesDeleted
	result.TerminalVerificationTokensDeleted += batch.TerminalVerificationTokensDeleted
	result.TerminalPasswordResetTokensDeleted += batch.TerminalPasswordResetTokensDeleted
	result.TerminalRecoveryContactsDeleted += batch.TerminalRecoveryContactsDeleted
	result.MayHaveMore = result.MayHaveMore || batch.MayHaveMore
}

// ReceiptCleanupRepository removes expired identity secrets and ephemeral
// records. Implementations must retain non-secret audit rows where required,
// be idempotent, and lease rows inside the caller's transaction so multiple
// processes can safely cooperate.
type ReceiptCleanupRepository interface {
	EraseExpired(context.Context, ReceiptCleanupRequest) (ReceiptCleanupResult, error)
}

var (
	ErrRecoveryContactRejected                     = errors.New("identity: recovery contact credential rejected")
	ErrRecoveryContactRecentAuthenticationRequired = errors.New("identity: recent authentication required")
	ErrRecoveryContactVersionConflict              = errors.New("identity: recovery contact version conflict")
	ErrRecoveryContactIdempotencyConflict          = errors.New("identity: recovery contact idempotency conflict")
	ErrRecoveryContactRateLimited                  = errors.New("identity: recovery contact rate limited")
	ErrRecoveryContactRemovalRejected              = errors.New("identity: recovery contact removal rejected")
	ErrAccountInvitationRejected                   = errors.New("identity: account invitation rejected")
	ErrAccountInvitationConflict                   = errors.New("identity: account invitation state conflict")
)

// RecoveryContactIssue is the transaction-bound persistence command for a
// pending contact and its first one-time verification credential. Plaintext
// address and raw token bytes are intentionally absent.
type RecoveryContactIssue struct {
	Contact         entity.RecoveryContact
	LookupHash      []byte
	FingerprintHash []byte
	TokenID         uuid.UUID
	Token           entity.KeyedTokenHash
	IssuedAt        time.Time
	ExpiresAt       time.Time
}

// PreparedRecoveryContactRegistration is a fully protected write plan. It is
// built before opening the account transaction and contains no plaintext
// address or raw one-time credential.
type PreparedRecoveryContactRegistration struct {
	Issue    RecoveryContactIssue
	Delivery SecurityEmailDeliveryIntent
}

// RecoveryContactRepository owns storage locking and activation state. Both
// methods that mutate state must join the caller's existing transaction.
type RecoveryContactRepository interface {
	CreatePending(context.Context, RecoveryContactIssue) error
	VerifyIssued(
		context.Context,
		[]entity.KeyedTokenHash,
		time.Time,
	) (*entity.RecoveryContactVerificationResult, error)
}

// AccountInvitationAuthority is a row-locked, secret-free projection of one
// invitation token and its exact account/contact fence. Accepted indicates an
// idempotent replay candidate and still requires password verification.
type AccountInvitationAuthority struct {
	TokenID             uuid.UUID
	ContactID           uuid.UUID
	UserID              uuid.UUID
	ContactVersion      uint64
	UserSecurityVersion uint64
	Accepted            bool
}

type AccountInvitationResendState struct {
	UserID              uuid.UUID
	Username            string
	UserSecurityVersion uint64
	Contact             entity.RecoveryContact
	LastTokenID         uuid.UUID
	LastIssuedAt        time.Time
	ExpiresAt           time.Time
	DeliveryState       entity.RecoveryContactDeliveryState
}

type PreparedAccountInvitationResend struct {
	Contact           entity.RecoveryContact
	TokenID           uuid.UUID
	Token             entity.KeyedTokenHash
	IssuedAt          time.Time
	ExpiresAt         time.Time
	HourlyWindowStart time.Time
	DailyWindowStart  time.Time
	MaximumPerHour    int
	MaximumPerDay     int
	Cooldown          time.Duration
	Delivery          SecurityEmailDeliveryIntent
}

type AccountInvitationResendCommand struct {
	UserID                      uuid.UUID
	ExpectedUserSecurityVersion uint64
	ExpectedContactVersion      uint64
	ExpectedTokenID             uuid.UUID
	Prepared                    PreparedAccountInvitationResend
}

// AccountInvitationRepository reuses recovery-contact storage while preserving
// a distinct activation state machine and token domain. Mutations require the
// caller's transaction; GetInvitationResendState is a bounded read snapshot.
type AccountInvitationRepository interface {
	GetInvitationResendState(
		context.Context,
		uuid.UUID,
	) (*AccountInvitationResendState, error)
	RotateInvitation(
		context.Context,
		AccountInvitationResendCommand,
	) error
	LockInvitation(
		context.Context,
		[]entity.KeyedTokenHash,
		time.Time,
	) (*AccountInvitationAuthority, error)
	AcceptInvitation(
		context.Context,
		AccountInvitationAuthority,
		time.Time,
	) (*entity.User, error)
	RevokeInvitation(context.Context, uuid.UUID, time.Time) error
}

// RecoveryContactState is an internal protected snapshot. It may contain
// encrypted destinations and therefore must never be serialized by a driver.
type RecoveryContactState struct {
	UserID                       uuid.UUID
	SessionID                    uuid.UUID
	Current                      *entity.RecoveryContact
	Pending                      *entity.RecoveryContact
	LastReauthenticatedAt        time.Time
	PendingVerificationExpiresAt time.Time
	PendingLastIssuedAt          time.Time
	PendingDeliveryState         entity.RecoveryContactDeliveryState
	CanRemove                    bool
}

// RecoveryContactManagementRepository supplies the authenticated, canonical
// PostgreSQL snapshot used to render and guard self-service recovery changes.
type RecoveryContactManagementRepository interface {
	GetState(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		time.Time,
	) (*RecoveryContactState, error)
}

type RecoveryContactCommandKind string

const (
	RecoveryContactCommandReplace       RecoveryContactCommandKind = "replace"
	RecoveryContactCommandResend        RecoveryContactCommandKind = "resend"
	RecoveryContactCommandCancelPending RecoveryContactCommandKind = "cancel_pending"
	RecoveryContactCommandRemove        RecoveryContactCommandKind = "remove"
)

type RecoveryContactMutationCommand struct {
	ReceiptID          uuid.UUID
	UserID             uuid.UUID
	SessionID          uuid.UUID
	IdempotencyKey     uuid.UUID
	Kind               RecoveryContactCommandKind
	RequestFingerprint []byte
	Now                time.Time
	RecentAfter        time.Time
	ReceiptExpiresAt   time.Time
}

type RecoveryContactReplacementCommand struct {
	Command                RecoveryContactMutationCommand
	ExpectedCurrentVersion uint64
	Issue                  RecoveryContactIssue
	ReplacementWindowStart time.Time
	MaximumReplacements    int
}

type RecoveryContactResendCommand struct {
	Command                RecoveryContactMutationCommand
	ContactID              uuid.UUID
	ExpectedPendingVersion uint64
	TokenID                uuid.UUID
	Token                  entity.KeyedTokenHash
	IssuedAt               time.Time
	ExpiresAt              time.Time
	HourlyWindowStart      time.Time
	DailyWindowStart       time.Time
	MaximumPerHour         int
	MaximumPerDay          int
	Cooldown               time.Duration
}

type RecoveryContactVersionCommand struct {
	Command         RecoveryContactMutationCommand
	ExpectedVersion uint64
}

// RecoveryContactMutationRepository applies protected state transitions in a
// caller transaction. applied=false means an exact durable receipt replay.
type RecoveryContactMutationRepository interface {
	StartReplacement(context.Context, RecoveryContactReplacementCommand) (bool, error)
	ResendPending(context.Context, RecoveryContactResendCommand) (bool, error)
	CancelPending(context.Context, RecoveryContactVersionCommand) (bool, error)
	RemoveCurrent(context.Context, RecoveryContactVersionCommand) (bool, error)
}

// RecoveryContactStore is the complete persistence contract required by the
// recovery-contact application service. Keeping this composition explicit
// makes an incomplete adapter fail at compile time instead of silently
// disabling management commands at runtime.
type RecoveryContactStore interface {
	RecoveryContactRepository
	RecoveryContactManagementRepository
	RecoveryContactMutationRepository
}

// RecoveryContactSessionRepository is intentionally narrower than the shared
// session lifecycle port so unrelated authentication consumers are not forced
// to depend on recovery-specific capabilities.
type RecoveryContactSessionRepository interface {
	MarkReauthenticated(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		time.Time,
	) (bool, error)
}

// RecoveryContactLifecycle is the application boundary used by registration
// and the public verification route. RegisterPending joins the account
// transaction; Verify owns its activation transaction.
type RecoveryContactLifecycle interface {
	PrepareRegistration(uuid.UUID, string) (*PreparedRecoveryContactRegistration, error)
	PersistRegistration(context.Context, *PreparedRecoveryContactRegistration) error
	Verify(context.Context, string) (*entity.RecoveryContactVerificationResult, error)
}

// AccountInvitationLifecycle is the narrower preparation boundary consumed by
// administrative invitations. Keeping it separate prevents registration and
// public verification consumers from depending on invitation-only behavior.
type AccountInvitationLifecycle interface {
	PrepareInvitation(uuid.UUID, string) (*PreparedRecoveryContactRegistration, error)
	PersistInvitation(context.Context, *PreparedRecoveryContactRegistration) error
	PrepareInvitationResend(
		AccountInvitationResendState,
	) (*PreparedAccountInvitationResend, error)
}

type RecoveryContactManagement interface {
	Get(context.Context, uuid.UUID, uuid.UUID) (*entity.RecoveryContactProjection, error)
	Reauthenticate(context.Context, uuid.UUID, uuid.UUID, string) (time.Time, error)
	Replace(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, uint64) (*entity.RecoveryContactProjection, error)
	Resend(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uint64) (*entity.RecoveryContactProjection, error)
	CancelPending(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uint64) (*entity.RecoveryContactProjection, error)
	Remove(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uint64) (*entity.RecoveryContactProjection, error)
}

// RecoveryContactApplication is the complete driver-facing contract. Public
// verification and authenticated management are implemented by one service,
// so HTTP composition must provide both capabilities rather than relying on a
// runtime type assertion.
type RecoveryContactApplication interface {
	RecoveryContactLifecycle
	RecoveryContactManagement
}

var (
	ErrInvalidRoleCommand  = errors.New("identity: invalid role command")
	ErrRoleVersionConflict = errors.New("identity: role version conflict")
)

// RoleRepository defines the role data access interface.
type RoleRepository interface {
	FindAll(ctx context.Context) ([]*entity.Role, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Role], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Role, error)
	GetByKey(ctx context.Context, key string) (*entity.Role, error)
	Create(ctx context.Context, e *entity.Role) error
	UpdateCAS(ctx context.Context, e *entity.Role, expectedVersion int64) error
	LoadCommandReceipt(ctx context.Context, commandID uuid.UUID) (RoleCommandReceipt, bool, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// RoleCache owns L1 lookup caching for mutable role metadata. Security policy
// evaluation remains a separate concern and is not cached through this port.
type RoleCache interface {
	Get(context.Context, uuid.UUID, RoleCacheLoader) (*dto.RoleResponse, error)
	FindAll(context.Context, RoleListCacheLoader) ([]*dto.RoleResponse, error)
	ResolveByKey(context.Context, string, RoleEntityCacheLoader) (*entity.Role, error)
	Invalidate(context.Context, uuid.UUID, string) error
}

type RoleCacheLoader func(context.Context) (*dto.RoleResponse, error)
type RoleListCacheLoader func(context.Context) ([]*dto.RoleResponse, error)
type RoleEntityCacheLoader func(context.Context) (*entity.Role, error)

// RoleService defines the role business logic interface.
type RoleService interface {
	FindAll(ctx context.Context) ([]*dto.RoleResponse, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RoleResponse], error)
	Get(ctx context.Context, id uuid.UUID) (*dto.RoleResponse, error)
	Create(ctx context.Context, req *dto.CreateRoleRequest, requestHash [sha256.Size]byte) (*dto.RoleMutationReceipt, error)
	PreviewMetadata(ctx context.Context, id uuid.UUID, action string, req *dto.UpdateRoleRequest) (*dto.RoleMetadataPreview, error)
	ApplyMetadata(ctx context.Context, id uuid.UUID, action string, req *dto.UpdateRoleRequest, requestHash [sha256.Size]byte) (*dto.RoleMutationReceipt, error)
	GetPolicies(ctx context.Context, id uuid.UUID) (*dto.RolePolicySnapshot, error)
	SetPolicies(ctx context.Context, id uuid.UUID, req *dto.RolePoliciesUpdateRequest) (*dto.RolePolicySnapshot, error)
}

// RoleCommandReceipt is the durable transport-neutral event identity used to
// validate an idempotent role command replay without exposing relay state.
type RoleCommandReceipt struct {
	EventID          uuid.UUID
	EventType        string
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	Payload          json.RawMessage
	OccurredAt       time.Time
}

// SecurityEmailKind is persisted as a closed, low-cardinality discriminator.
// Secret-bearing message fields are never persisted outside encrypted payloads.
type SecurityEmailKind string

const (
	SecurityEmailKindRecoveryContactVerification SecurityEmailKind = "recovery_contact_verification"
	SecurityEmailKindPasswordReset               SecurityEmailKind = "password_reset"
	SecurityEmailKindAccountInvitation           SecurityEmailKind = "account_invitation"
)

var ErrSecurityEmailLeaseLost = errors.New("identity: security email lease lost")

// SecurityEmailDeliveryIntent contains only encrypted or non-secret fields.
// Enqueue must join the caller's active transaction.
type SecurityEmailDeliveryIntent struct {
	ID                     uuid.UUID
	Kind                   SecurityEmailKind
	VerificationTokenID    uuid.UUID
	PasswordResetTokenID   uuid.UUID
	ProviderIdempotencyKey string
	PayloadCiphertext      []byte
	KeyID                  string
	PayloadExpiresAt       time.Time
	AvailableAt            time.Time
}

// SecurityEmailScheduler is the narrow transaction-bound dependency used by
// credential issuance paths. Provider I/O is deliberately absent.
type SecurityEmailScheduler interface {
	Enqueue(context.Context, SecurityEmailDeliveryIntent) error
}

type SecurityEmailClaim struct {
	Owner         string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

// SecurityEmailLease contains ciphertext until the dispatcher opens it in
// memory. It must never be formatted or logged as a whole value.
type SecurityEmailLease struct {
	ID                     uuid.UUID
	Kind                   SecurityEmailKind
	VerificationTokenID    uuid.UUID
	PasswordResetTokenID   uuid.UUID
	ProviderIdempotencyKey string
	PayloadCiphertext      []byte
	KeyID                  string
	PayloadExpiresAt       time.Time
	Attempt                uint32
	Owner                  string
	LeaseToken             uuid.UUID
	LeasedUntil            time.Time
}

type SecurityEmailLeaseOutcome struct {
	ID            uuid.UUID
	Owner         string
	LeaseToken    uuid.UUID
	Attempt       uint32
	At            time.Time
	NextAttemptAt time.Time
	ErrorClass    string
}

// SecurityEmailPrepareCommand is the final lease fence checked immediately
// before ciphertext is opened and provider I/O begins.
type SecurityEmailPrepareCommand struct {
	ID           uuid.UUID
	Kind         SecurityEmailKind
	CredentialID uuid.UUID
	Owner        string
	LeaseToken   uuid.UUID
	Attempt      uint32
	Now          time.Time
}

type SecurityEmailDeliveryRepository interface {
	SecurityEmailScheduler
	Claim(context.Context, SecurityEmailClaim) ([]SecurityEmailLease, error)
	PrepareForSend(context.Context, SecurityEmailPrepareCommand) (bool, error)
	MarkSent(context.Context, SecurityEmailLeaseOutcome) error
	MarkRetry(context.Context, SecurityEmailLeaseOutcome) error
	MarkDead(context.Context, SecurityEmailLeaseOutcome) error
}

// SecurityMutationJournal appends the Identity-owned audit and integration
// event pair to the transaction already bound to ctx. It never owns a
// transaction and performs no network publication.
type SecurityMutationJournal interface {
	Append(context.Context, entity.SecurityMutation) error
}

// SessionRepository owns canonical authentication-session persistence. Every
// implementation must resolve its client from ctx so an enclosing Ent unit of
// work remains atomic.
type SessionRepository interface {
	Create(ctx context.Context, session *entity.Session) error
	GetActiveForAuthentication(ctx context.Context, sessionID, userID uuid.UUID) (entity.SessionSecurityState, error)
	// TouchLastSeen atomically advances both activity timestamps while the
	// session is still active. updated is true only when PostgreSQL accepted
	// exactly this observation; callers must not mutate cached state otherwise.
	// A session's inactivity window is immutable after issuance. Every writer
	// preserves idle_expires_at = min(last_seen_at + window, absolute_expires_at)
	// and must never decrease either activity timestamp.
	TouchLastSeen(
		ctx context.Context,
		sessionID uuid.UUID,
		observedAt, idleExpiresAt time.Time,
	) (updated bool, err error)
	Revoke(ctx context.Context, sessionID uuid.UUID, reason string, at time.Time) error
	RevokeAll(ctx context.Context, userID uuid.UUID, reason string, at time.Time) error
}

// RecoveryContactSessionStore is the complete persistence capability exposed
// by the canonical session adapter at the composition root. Authentication
// consumers continue to depend on SessionRepository, while recovery-contact
// management can request its additional reauthentication CAS without a
// runtime type assertion.
type RecoveryContactSessionStore interface {
	SessionRepository
	RecoveryContactSessionRepository
}

// SessionCacheKey binds a cache entry to one verified access credential and
// its exact identity/version fence. Including TokenID prevents a cache entry
// created for one access-token lifetime from being reused as another token's
// lifetime.
type SessionCacheKey struct {
	SessionID       uuid.UUID
	UserID          uuid.UUID
	TokenID         string
	SecurityVersion uint64
}

// SessionStateLoader reads the canonical PostgreSQL projection on a miss.
type SessionStateLoader func(context.Context) (entity.SessionSecurityState, error)

// SessionStateCache is the process-local, defensive revalidation cache. Its
// invalidation methods are synchronous and are called only after the durable
// state-changing transaction commits.
type SessionStateCache interface {
	Resolve(
		ctx context.Context,
		key SessionCacheKey,
		accessExpiresAt time.Time,
		loader SessionStateLoader,
	) (entity.SessionSecurityState, error)
	InvalidateSession(ctx context.Context, sessionID uuid.UUID) error
	InvalidateUser(ctx context.Context, userID uuid.UUID) error
	MarkTouched(sessionID uuid.UUID, observedAt, idleExpiresAt time.Time)
}

var (
	ErrOwnedSessionNotFound                = errors.New("identity: owned session not found")
	ErrSessionCommandIdempotencyConflict   = errors.New("identity: session command idempotency conflict")
	ErrSessionCommandAuthorityLost         = errors.New("identity: session command authority lost")
	ErrInvalidSessionManagementPersistence = errors.New("identity: invalid session management persistence")
)

// SessionCursor is the stable keyset boundary for the user-owned session
// history. CreatedAt and ID together define a total descending order.
type SessionCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type SessionListQuery struct {
	UserID uuid.UUID
	After  *SessionCursor
	Limit  int
}

type SessionListResult struct {
	Sessions []entity.Session
	Next     *SessionCursor
}

type SessionCommandKind string

const (
	SessionCommandRevoke    SessionCommandKind = "revoke"
	SessionCommandLogoutAll SessionCommandKind = "logout_all"
)

type SessionRevokeCommand struct {
	UserID          uuid.UUID
	ActorSessionID  uuid.UUID
	TargetSessionID uuid.UUID
	IdempotencyKey  uuid.UUID
}

type SessionLogoutAllCommand struct {
	UserID         uuid.UUID
	ActorSessionID uuid.UUID
	IdempotencyKey uuid.UUID
}

// SessionManagementMutation is the closed durable receipt returned by the
// persistence adapter. Applied is false for an exact replay or an already
// terminal no-op. MutationID remains stable for a recovered committed command
// and is zero when no mutation or receipt was created.
type SessionManagementMutation struct {
	MutationID      uuid.UUID
	Kind            SessionCommandKind
	ActorSessionID  uuid.UUID
	TargetSessionID *uuid.UUID
	SecurityVersion uint64
	Applied         bool
}

// SessionManagementRepository is deliberately separate from
// SessionRepository: account UI queries and security commands must not expand
// the latency-sensitive token authentication contract.
type SessionManagementRepository interface {
	List(context.Context, SessionListQuery) (SessionListResult, error)
	RevokeOwned(
		ctx context.Context,
		command SessionRevokeCommand,
		now time.Time,
		receiptExpiresAt time.Time,
	) (SessionManagementMutation, error)
	LogoutAll(
		ctx context.Context,
		command SessionLogoutAllCommand,
		now time.Time,
		receiptExpiresAt time.Time,
	) (SessionManagementMutation, error)
}

// SessionManagementInvalidator is the smallest post-commit cache contract
// needed by account security commands.
type SessionManagementInvalidator interface {
	InvalidateSession(context.Context, uuid.UUID) error
	InvalidateUser(context.Context, uuid.UUID) error
}

type SessionManagementLifecycle interface {
	List(
		ctx context.Context,
		userID, currentSessionID uuid.UUID,
		request *dto.SessionListRequest,
	) (*dto.SessionListResponse, error)
	Revoke(
		ctx context.Context,
		userID, actorSessionID, targetSessionID, idempotencyKey uuid.UUID,
	) error
	LogoutAll(
		ctx context.Context,
		userID, actorSessionID, idempotencyKey uuid.UUID,
	) error
}

// AccessTokenVerifier is the cryptographic verification boundary supplied by
// go-common. Session authentication wraps it with live-state validation.
type AccessTokenVerifier interface {
	Verify(context.Context, string, authentication.TokenPurpose) (authentication.Principal, error)
}

// SessionAuthenticator is the application verifier plus explicit post-commit
// invalidation surface used by logout and security-state commands.
type SessionAuthenticator interface {
	AccessTokenVerifier
	InvalidateSession(context.Context, uuid.UUID) error
	InvalidateUserSessions(context.Context, uuid.UUID) error
}

// AccessTokenIssuer is kept narrow so recoverable issuance can pre-sign before
// entering a mutating transaction.
type AccessTokenIssuer interface {
	Issue(context.Context, authentication.IssueRequest) (authentication.IssuedToken, error)
}

// RefreshTokenRepository persists only hashed opaque refresh credentials.
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *entity.RefreshToken) error
	Get(ctx context.Context, id uuid.UUID) (*entity.RefreshToken, error)
	GetByHash(ctx context.Context, tokenHash []byte) (*entity.RefreshToken, error)
	RevokeIssuedForSession(ctx context.Context, sessionID uuid.UUID, at time.Time) error
	RevokeIssuedForUser(ctx context.Context, userID uuid.UUID, at time.Time) error
}

// SessionEstablishment is an application command, not an HTTP DTO. All
// transport metadata is already bounded and normalized by the adapter.
type SessionEstablishment struct {
	Purpose entity.SessionIssuancePurpose
	// RequestOrTicketID is the durable command identity for every purpose
	// except password login. Registration uses its server-owned pre-session;
	// OAuth uses the locked completion-ticket ID.
	RequestOrTicketID  uuid.UUID
	PreSessionID       uuid.UUID
	IdempotencyKey     uuid.UUID
	User               *entity.User
	RequestFingerprint []byte
	TraitOfferID       uuid.UUID
	OfferChecksum      []byte
	UserAgent          string
	IPHash             []byte
}

type RefreshRotation struct {
	RawRefreshToken string
	IdempotencyKey  uuid.UUID
}

// RegistrationRecoveryVerifier proves account authority after an exact
// fingerprint match and before protected response credentials are opened.
type RegistrationRecoveryVerifier func(context.Context, uuid.UUID) error

// TokenLifecycle is the only authority that may establish, rotate, or revoke
// browser sessions. Raw refresh credentials never enter repository entities.
type TokenLifecycle interface {
	Establish(context.Context, SessionEstablishment) (entity.SessionCredentials, error)
	RecoverRegistration(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		[]byte,
		RegistrationRecoveryVerifier,
	) (entity.SessionCredentials, bool, error)
	Rotate(context.Context, RefreshRotation) (entity.SessionCredentials, error)
	Logout(context.Context, uuid.UUID) error
	RevokeAll(context.Context, uuid.UUID, string) error
}

// OAuthIssuanceRecovery is the narrow exact-retry capability used only after
// a durable completion ticket reports Fresh=false. It can open an existing
// OAuth login/registration receipt but can never establish a new session.
type OAuthIssuanceRecovery interface {
	RecoverOAuth(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		[]byte,
	) (entity.SessionCredentials, bool, error)
}

type IssuanceReceiptSelector struct {
	Purpose        entity.SessionIssuancePurpose
	RequestID      uuid.UUID
	UserID         uuid.UUID
	PreSessionID   uuid.UUID
	IdempotencyKey uuid.UUID
}

// SessionIssuanceReceiptRepository owns the two deliberately different unique
// command identities used for login and non-login establishment.
type SessionIssuanceReceiptRepository interface {
	Get(ctx context.Context, selector IssuanceReceiptSelector) (*entity.SessionIssuanceReceipt, error)
	Create(ctx context.Context, receipt *entity.SessionIssuanceReceipt) error
	MarkRecovered(ctx context.Context, id uuid.UUID, recoveredAt time.Time) error
}

// RefreshContext is the minimal live state needed to pre-sign a replacement
// access credential. Rotate revalidates all values while holding row locks.
type RefreshContext struct {
	TokenID           uuid.UUID
	SessionID         uuid.UUID
	UserID            uuid.UUID
	Username          string
	SecurityVersion   uint64
	AuthenticatedAt   time.Time
	AbsoluteExpiresAt time.Time
	IdleExpiresAt     time.Time
	RefreshExpiresAt  time.Time
}

type RefreshRotationMutation struct {
	PresentedHash []byte
	Expected      RefreshContext
	Replacement   *entity.RefreshToken
	Receipt       *entity.RefreshReceipt
}

// RefreshRotationRepository performs the single-use compare-and-transition.
// Implementations must lock user, session, then token rows, and represent replay
// containment as a successful outcome so the outer transaction commits it.
type RefreshRotationRepository interface {
	ContextByHash(ctx context.Context, tokenHash []byte, now time.Time) (RefreshContext, error)
	Rotate(ctx context.Context, mutation RefreshRotationMutation, now time.Time) (entity.RefreshRotationResult, error)
	MarkReceiptRecovered(ctx context.Context, receiptID uuid.UUID, recoveredAt time.Time) error
}

var (
	ErrInvalidUserLifecycle  = errors.New("identity: invalid user lifecycle command")
	ErrUserLifecycleConflict = errors.New("identity: user lifecycle version conflict")
	ErrInvalidUserCommand    = errors.New("identity: invalid user command receipt")
)

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.User], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	Create(ctx context.Context, e *entity.User) error
	TransitionLifecycleCAS(
		ctx context.Context,
		id uuid.UUID,
		expectedVersion int64,
		from entity.UserSecurityStatus,
		to entity.UserSecurityStatus,
		reason string,
		at time.Time,
		actorID uuid.UUID,
	) (*entity.User, error)
	LoadCommandReceipt(
		ctx context.Context,
		commandID uuid.UUID,
	) (UserCommandReceipt, bool, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
}

// UserService defines the interface for user business logic.
type UserService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserResponse], error)
	PreviewLifecycle(ctx context.Context, id uuid.UUID, action string, req *dto.UserLifecycleCommandRequest) (*dto.UserLifecyclePreview, error)
	ApplyLifecycle(ctx context.Context, id uuid.UUID, action string, req *dto.UserLifecycleCommandRequest, requestHash [sha256.Size]byte) (*dto.UserLifecycleReceipt, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileAttrs, error)
	GetProfileAttributes(ctx context.Context, userID uuid.UUID) (*dto.ProfileAttributesResponse, error)
	UpdateProfileAttributes(ctx context.Context, userID uuid.UUID, req *dto.UpdateProfileAttributesRequest) (*dto.ProfileAttributesResponse, error)
	GetRole(ctx context.Context, userID uuid.UUID) (*dto.RoleResponse, error)
}

// UserCommandReceipt is the transport-neutral outbox record used to recover
// exact user-scoped command results after a timeout or duplicate delivery.
type UserCommandReceipt struct {
	EventID          uuid.UUID
	EventType        string
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	Payload          json.RawMessage
	OccurredAt       time.Time
}

var (
	// ErrUserAttributeValueConflict is returned when a caller attempts to
	// overwrite an attribute value from an obsolete source version.
	ErrUserAttributeValueConflict = errors.New("user attribute value version conflict")
	// ErrUserAttributeValueWriterUnavailable is fail-closed: revisioned values
	// must never fall back to the legacy, unversioned write methods.
	ErrUserAttributeValueWriterUnavailable = errors.New("user attribute value revisioned writer unavailable")
	ErrInvalidUserAttributeValue           = errors.New("invalid revisioned user attribute value")
	// ErrUserAttributeDefinitionRevisionConflict is returned when the active
	// definition pointer no longer matches the schema revision reviewed by the
	// caller. Callers must reload the active schema instead of coercing data.
	ErrUserAttributeDefinitionRevisionConflict = errors.New("user attribute definition revision conflict")
	// ErrUserAttributeMigrationActive keeps destructive value removal
	// fail-closed while a candidate shadow set is being built.
	ErrUserAttributeMigrationActive   = errors.New("user attribute migration active")
	ErrUserAttributeMigrationConflict = errors.New("user attribute migration value conflict")
)

// UserAttributeValueRepository defines the interface for user attribute value persistence.
type UserAttributeValueRepository interface {
	ActiveProfileAttributeValueReader
	RevisionedUserAttributeValueWriter
}

// RevisionedUserAttributeValueWriter is the sole production write boundary
// for typed identity values. It fences the active definition pointer, applies
// optimistic value versions and maintains candidate shadows. Every caller
// must present the exact immutable schema revision it reviewed.
type RevisionedUserAttributeValueMutation struct {
	UserID                       uuid.UUID
	DefinitionID                 uuid.UUID
	ExpectedDefinitionRevisionID uuid.UUID
	ExpectedValueVersion         int64
	CanonicalValue               json.RawMessage
	Unset                        bool
}

// ActiveProfileAttributeValue is the active-schema projection used by
// profile and onboarding application services. Admin-only definitions are not
// part of this projection. A missing value is represented by HasValue=false
// and ValueVersion=0; JSON false, zero and the empty string remain real values.
type ActiveProfileAttributeValue struct {
	DefinitionID         uuid.UUID
	DefinitionRevisionID uuid.UUID
	Key                  string
	Kind                 string
	RequiredOnOnboarding bool
	DataType             entity.AttributeValueType
	Label                string
	Description          string
	Visibility           entity.AttributeVisibility
	UserEditable         bool
	DisplayOrder         int
	Validation           json.RawMessage
	HasValue             bool
	CanonicalValue       json.RawMessage
	ValueVersion         int64
}

// ActiveProfileAttributeValueReader returns one row per currently active,
// non-admin definition, including definitions for which the user has no row.
// Implementations must enforce the active definition/revision relationship in
// the database query rather than filtering a stale superset in memory.
type ActiveProfileAttributeValueReader interface {
	ListActiveProfileAttributes(
		ctx context.Context,
		userID uuid.UUID,
	) ([]ActiveProfileAttributeValue, error)
}

type RevisionedUserAttributeValueWriter interface {
	ApplyActiveMutation(
		ctx context.Context,
		mutation RevisionedUserAttributeValueMutation,
	) (*entity.UserAttributeValue, error)
}

var (
	ErrInvalidUserInvitation  = errors.New("identity: invalid user invitation")
	ErrUserInvitationConflict = errors.New("identity: user invitation conflict")
)

type UserInvitationService interface {
	PreviewInvitation(
		context.Context,
		*dto.UserInvitationCommandRequest,
	) (*dto.UserInvitationPreview, error)
	ApplyInvitation(
		context.Context,
		*dto.UserInvitationCommandRequest,
		[sha256.Size]byte,
	) (*dto.UserInvitationReceipt, error)
	PreviewInvitationResend(
		context.Context,
		*dto.UserInvitationResendCommandRequest,
	) (*dto.UserInvitationResendPreview, error)
	ApplyInvitationResend(
		context.Context,
		*dto.UserInvitationResendCommandRequest,
		[sha256.Size]byte,
	) (*dto.UserInvitationResendReceipt, error)
	ActivateInvitation(
		context.Context,
		*dto.ActivateUserInvitationRequest,
	) (*dto.ActivateUserInvitationResponse, error)
}
