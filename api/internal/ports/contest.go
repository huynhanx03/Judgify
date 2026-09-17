// Package ports defines Judgify's stable internal capability catalog.
package ports

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
)

// Contest capabilities are added here during the Contest migration.

var (
	ErrInvalidContestCommand     = errors.New("contest: invalid command")
	ErrContestCommandDenied      = errors.New("contest: command denied")
	ErrInvalidContestReservation = errors.New("contest: invalid submission reservation")
)

type TransitionCommand struct {
	ContestID       uuid.UUID
	Target          entity.ContestState
	ExpectedVersion int64
	IdempotencyKey  uuid.UUID
	// System distinguishes a scheduler-owned command from a human operator.
	// System commands intentionally have no user/session principal; their
	// command identity remains durable in contest_transitions and audit/outbox.
	System         bool
	ActorUserID    uuid.UUID
	ActorSessionID *uuid.UUID
	Reason         string
	CommandHash    [32]byte
}

type TransitionResult struct {
	ContestID   uuid.UUID
	From        entity.ContestState
	To          entity.ContestState
	Version     int64
	OccurredAt  time.Time
	CommandID   uuid.UUID
	ExactReplay bool
}

// LifecycleRepository joins an existing transaction and owns Contest rows.
// It locks the contest first, uses PostgreSQL time, and writes immutable
// transition history before changing the aggregate.
type LifecycleRepository interface {
	Transition(ctx context.Context, command TransitionCommand) (TransitionResult, error)
}

type LifecycleService interface {
	Transition(ctx context.Context, command TransitionCommand) (TransitionResult, error)
}

type RegistrationCommand struct {
	ContestID                   uuid.UUID
	UserID                      uuid.UUID
	ExpectedContestVersion      int64
	ExpectedRegistrationVersion *int64
	IdempotencyKey              uuid.UUID
	ActorUserID                 uuid.UUID
	ActorSessionID              *uuid.UUID
	Reason                      string
	CommandHash                 [32]byte
}

type RegistrationMutation string

const (
	RegistrationRegister RegistrationMutation = "register"
	RegistrationWithdraw RegistrationMutation = "withdraw"
	RegistrationBan      RegistrationMutation = "ban"
)

type RegistrationResult struct {
	Registration   entity.ContestRegistration
	ContestVersion int64
	OccurredAt     time.Time
	CommandID      uuid.UUID
	ExactReplay    bool
}

type RegistrationCommandRepository interface {
	Apply(ctx context.Context, command RegistrationCommand, mutation RegistrationMutation) (RegistrationResult, error)
}

type RegistrationCommandService interface {
	Apply(ctx context.Context, command RegistrationCommand, mutation RegistrationMutation) (RegistrationResult, error)
}

type ReserveContestSubmissionRequest struct {
	ContestID        uuid.UUID
	ContestProblemID uuid.UUID
	ProblemID        uuid.UUID
	UserID           uuid.UUID
	RuntimeKey       string
}

// ContestSubmissionReservation contains only Contest-owned immutable facts.
// Problem intake verifies the returned tuple but may not replace it with a
// mutable current publication pointer.
type ContestSubmissionReservation struct {
	ContestID                 uuid.UUID
	ContestProblemID          uuid.UUID
	ProblemID                 uuid.UUID
	ProblemRevisionID         uuid.UUID
	TestsetRevisionID         uuid.UUID
	ScoringPolicy             entity.ScoringPolicy
	ScoringPolicyVersion      int64
	Lane                      string
	SubmittedAt               time.Time
	SubmissionElapsedMillis   int64
	ContestSubmissionSequence int64
	CheckerKey                string
	CheckerVersion            string
	CheckerConfig             json.RawMessage
	AllowedRuntimeKeys        []string
	ProblemSnapshotChecksum   [32]byte
	// ImplicitRegistrationTransition lets the enclosing submission workflow
	// append the corresponding safe outbox event in the same UnitOfWork.
	ImplicitRegistrationTransition *entity.ContestRegistrationTransition
}

// ContestSubmissionReserver is deliberately transaction-bound. It never
// opens a nested UnitOfWork and never reads Identity or Submission tables.
type ContestSubmissionReserver interface {
	ReserveContestSubmission(
		ctx context.Context,
		request ReserveContestSubmissionRequest,
	) (ContestSubmissionReservation, error)
}

var ErrInvalidContestContentCommand = errors.New("contest content: invalid command")

type ContentCursor struct {
	OccurredAt time.Time
	ID         uuid.UUID
}

const contentCursorBytes = 24

func EncodeContentCursor(cursor ContentCursor) (string, error) {
	if cursor.OccurredAt.IsZero() || cursor.ID == uuid.Nil {
		return "", ErrInvalidContestContentCommand
	}
	encoded := make([]byte, contentCursorBytes)
	binary.BigEndian.PutUint64(encoded[:8], uint64(cursor.OccurredAt.UTC().UnixNano()))
	copy(encoded[8:], cursor.ID[:])
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func DecodeContentCursor(value string) (*ContentCursor, error) {
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != contentCursorBytes {
		return nil, ErrInvalidContestContentCommand
	}
	var id uuid.UUID
	copy(id[:], decoded[8:])
	occurredAt := time.Unix(0, int64(binary.BigEndian.Uint64(decoded[:8]))).UTC()
	if id == uuid.Nil || occurredAt.IsZero() {
		return nil, ErrInvalidContestContentCommand
	}
	return &ContentCursor{OccurredAt: occurredAt, ID: id}, nil
}

type ContentListQuery struct {
	ContestID uuid.UUID
	ActorID   uuid.UUID
	After     *ContentCursor
	Limit     int
	Status    string
}

type ContentCommandKind string

const (
	ContentCommandAnnouncementCreate    ContentCommandKind = "announcement_create"
	ContentCommandAnnouncementEdit      ContentCommandKind = "announcement_edit"
	ContentCommandAnnouncementPublish   ContentCommandKind = "announcement_publish"
	ContentCommandAnnouncementWithdraw  ContentCommandKind = "announcement_withdraw"
	ContentCommandClarificationSubmit   ContentCommandKind = "clarification_submit"
	ContentCommandClarificationAnswer   ContentCommandKind = "clarification_answer"
	ContentCommandClarificationClose    ContentCommandKind = "clarification_close"
	ContentCommandClarificationWithdraw ContentCommandKind = "clarification_withdraw"
)

func (value ContentCommandKind) Valid() bool {
	switch value {
	case ContentCommandAnnouncementCreate,
		ContentCommandAnnouncementEdit,
		ContentCommandAnnouncementPublish,
		ContentCommandAnnouncementWithdraw,
		ContentCommandClarificationSubmit,
		ContentCommandClarificationAnswer,
		ContentCommandClarificationClose,
		ContentCommandClarificationWithdraw:
		return true
	default:
		return false
	}
}

// ContentCommandReceipt is persisted before the aggregate mutation. The hash
// binds all intent fields. The bounded immutable projection makes a lost HTTP
// response byte-semantically replayable even if the aggregate advances later.
type ContentCommandReceipt struct {
	ContestID        uuid.UUID
	Kind             ContentCommandKind
	IdempotencyKey   uuid.UUID
	RequestHash      [sha256.Size]byte
	ActorUserID      uuid.UUID
	ActorSessionID   uuid.UUID
	ResultOwner      entity.ContentOwnerType
	ResultContentID  uuid.UUID
	ResultRevisionID *uuid.UUID
	ResultVersion    int64
	ResultProjection []byte
}

type ContentCommandReservation struct {
	ExactReplay bool
	Receipt     ContentCommandReceipt
}

type AnnouncementView struct {
	Announcement  entity.Announcement
	Revision      entity.ContentRevision
	SanitizedHTML string
}

type DueAnnouncement struct {
	ContestID      uuid.UUID
	AnnouncementID uuid.UUID
	Version        int64
}

type ClarificationView struct {
	Clarification     entity.Clarification
	Question          *entity.ContentRevision
	QuestionHTML      *string
	PublicSummaryHTML *string
	AnswerHTML        *string
}

// SharedClarificationView is intentionally incapable of carrying requester
// identity or raw question content. It is the only repository result allowed
// on the participant-shared feed.
type SharedClarificationView struct {
	ID                uuid.UUID
	ContestID         uuid.UUID
	ContestProblemID  *uuid.UUID
	Status            entity.ClarificationStatus
	Audience          entity.ClarificationAudience
	Version           int64
	PublicSummaryHTML string
	AnswerHTML        string
	CreatedAt         time.Time
	AnsweredAt        time.Time
	ClosedAt          *time.Time
	UpdatedAt         time.Time
}

type AnnouncementDraftCommand struct {
	ContestID       uuid.UUID
	AnnouncementID  uuid.UUID
	ExpectedVersion int64
	Audience        entity.AnnouncementAudience
	ScheduledFor    *time.Time
	Revision        entity.ContentRevision
	ActorUserID     uuid.UUID
	ActorSessionID  *uuid.UUID
	CommandID       uuid.UUID
	Reason          string
}

type AnnouncementStateCommand struct {
	ContestID       uuid.UUID
	AnnouncementID  uuid.UUID
	ExpectedVersion int64
	ActorUserID     uuid.UUID
	ActorSessionID  *uuid.UUID
	CommandID       uuid.UUID
	Reason          string
	System          bool
}

type ClarificationSubmitCommand struct {
	ContestID        uuid.UUID
	ClarificationID  uuid.UUID
	RequesterID      uuid.UUID
	ContestProblemID *uuid.UUID
	Question         entity.ContentRevision
	ActorSessionID   *uuid.UUID
	CommandID        uuid.UUID
}

type ClarificationAnswerCommand struct {
	ContestID       uuid.UUID
	ClarificationID uuid.UUID
	ExpectedVersion int64
	Audience        entity.ClarificationAudience
	PublicSummary   *entity.ContentRevision
	Answer          entity.ContentRevision
	ActorUserID     uuid.UUID
	ActorSessionID  *uuid.UUID
	CommandID       uuid.UUID
	Reason          string
}

type ClarificationStateCommand struct {
	ContestID       uuid.UUID
	ClarificationID uuid.UUID
	ExpectedVersion int64
	ActorUserID     uuid.UUID
	ActorSessionID  *uuid.UUID
	CommandID       uuid.UUID
	Reason          string
}

// ContentRepository owns SQL visibility predicates and joins an existing
// transaction for mutations. No participant mapper receives raw third-party
// questions or requester identity.
type ContentRepository interface {
	ReserveCommand(context.Context, ContentCommandReceipt) (ContentCommandReservation, error)
	CompleteCommand(context.Context, ContentCommandReceipt) error
	CreateAnnouncement(context.Context, AnnouncementDraftCommand) (AnnouncementView, error)
	EditAnnouncement(context.Context, AnnouncementDraftCommand) (AnnouncementView, error)
	PublishAnnouncement(context.Context, AnnouncementStateCommand) (AnnouncementView, error)
	WithdrawAnnouncement(context.Context, AnnouncementStateCommand) (AnnouncementView, error)
	ListPublicAnnouncements(context.Context, ContentListQuery) ([]AnnouncementView, *ContentCursor, error)
	ListParticipantAnnouncements(context.Context, ContentListQuery) ([]AnnouncementView, *ContentCursor, error)
	ListAdminAnnouncements(context.Context, ContentListQuery) ([]AnnouncementView, *ContentCursor, error)
	GetAdminAnnouncement(context.Context, uuid.UUID, uuid.UUID) (AnnouncementView, error)
	FindDueAnnouncements(context.Context, int) ([]DueAnnouncement, error)

	SubmitClarification(context.Context, ClarificationSubmitCommand) (ClarificationView, error)
	AnswerClarification(context.Context, ClarificationAnswerCommand) (ClarificationView, error)
	CloseClarification(context.Context, ClarificationStateCommand) (ClarificationView, error)
	WithdrawClarification(context.Context, ClarificationStateCommand) (ClarificationView, error)
	ListParticipantClarifications(context.Context, ContentListQuery) ([]SharedClarificationView, *ContentCursor, error)
	ListOwnClarifications(context.Context, ContentListQuery) ([]ClarificationView, *ContentCursor, error)
	GetOwnClarification(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (ClarificationView, error)
	ListAdminClarifications(context.Context, ContentListQuery) ([]ClarificationView, *ContentCursor, error)
	GetAdminClarification(context.Context, uuid.UUID, uuid.UUID) (ClarificationView, error)
	ListRevisions(context.Context, entity.ContentOwnerType, uuid.UUID) ([]entity.ContentRevision, error)
}

type ContentService interface {
	CreateAnnouncement(context.Context, uuid.UUID, uuid.UUID, *contract.CreateAnnouncementRequest) (*contract.AnnouncementResponse, error)
	EditAnnouncement(context.Context, uuid.UUID, uuid.UUID, *contract.EditAnnouncementRequest) (*contract.AnnouncementResponse, error)
	PublishAnnouncement(context.Context, uuid.UUID, uuid.UUID, *contract.AnnouncementCommandRequest) (*contract.AnnouncementResponse, error)
	WithdrawAnnouncement(context.Context, uuid.UUID, uuid.UUID, *contract.AnnouncementCommandRequest) (*contract.AnnouncementResponse, error)
	ListPublicAnnouncements(context.Context, *contract.ContestContentListRequest) (*contract.ContestContentPage[contract.AnnouncementResponse], error)
	ListParticipantAnnouncements(context.Context, uuid.UUID, *contract.ContestContentListRequest) (*contract.ContestContentPage[contract.AnnouncementResponse], error)
	ListAdminAnnouncements(context.Context, *contract.ContestContentListRequest) (*contract.ContestContentPage[contract.AnnouncementResponse], error)
	GetAdminAnnouncement(context.Context, uuid.UUID, uuid.UUID) (*contract.AnnouncementResponse, error)
	PublishDueAnnouncements(context.Context, int) (int, error)

	SubmitClarification(context.Context, uuid.UUID, uuid.UUID, *contract.SubmitClarificationRequest) (*contract.ClarificationResponse, error)
	AnswerClarification(context.Context, uuid.UUID, uuid.UUID, *contract.AnswerClarificationRequest) (*contract.ClarificationResponse, error)
	CloseClarification(context.Context, uuid.UUID, uuid.UUID, *contract.ClarificationCommandRequest) (*contract.ClarificationResponse, error)
	WithdrawClarification(context.Context, uuid.UUID, uuid.UUID, *contract.ClarificationCommandRequest) (*contract.ClarificationResponse, error)
	ListParticipantClarifications(context.Context, uuid.UUID, *contract.ContestContentListRequest) (*contract.ContestContentPage[contract.ClarificationResponse], error)
	ListOwnClarifications(context.Context, uuid.UUID, *contract.ContestContentListRequest) (*contract.ContestContentPage[contract.ClarificationResponse], error)
	GetOwnClarification(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*contract.ClarificationResponse, error)
	ListAdminClarifications(context.Context, *contract.ContestContentListRequest) (*contract.ContestContentPage[contract.ClarificationResponse], error)
	GetAdminClarification(context.Context, uuid.UUID, uuid.UUID) (*contract.ClarificationResponse, error)
	ListRevisions(context.Context, entity.ContentOwnerType, uuid.UUID) ([]contract.ContentRevisionResponse, error)
}

// ContestRepository defines the contest data access interface.
type ContestRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error)
	FindPublic(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Contest, error)
	GetPublic(ctx context.Context, id uuid.UUID) (*entity.Contest, error)
	CreateDraft(ctx context.Context, e *entity.Contest, problems []entity.ContestDraftProblem) error
	UpdateDraft(ctx context.Context, e *entity.Contest, expectedVersion int64, problems []entity.ContestDraftProblem) error
	DeleteDraft(ctx context.Context, id uuid.UUID) error
	GetReadModels(ctx context.Context, contestIDs []uuid.UUID, includeDraftProblems bool) (map[uuid.UUID]ContestReadModel, error)
	GetContestProblemProjections(ctx context.Context, contestID uuid.UUID) ([]ContestProblemProjection, error)
	GetContestProblemSnapshotRef(ctx context.Context, contestID uuid.UUID, contestProblemID uuid.UUID) (ContestProblemSnapshotRef, error)
	FindIDsByStatus(ctx context.Context, status string) ([]uuid.UUID, error)
}

// ContestReadModel batches list/detail enrichments to avoid per-row count and
// membership queries.
type ContestReadModel struct {
	ParticipantCount int
	ProblemCount     int
	DraftProblems    []entity.ContestDraftProblem
}

// ContestProblemProjection is the bounded browser-safe identity of one frozen
// contest membership. Executable snapshots and checker configuration remain
// behind the submission-intake boundary.
type ContestProblemProjection struct {
	ContestProblemID   uuid.UUID
	ProblemID          uuid.UUID
	ProblemSlug        string
	ProblemTitle       string
	DisplayOrder       int
	Alias              string
	Points             *int
	VisibleBeforeStart bool
}

// ContestProblemSnapshotRef is the contest-owned authorization and visibility
// boundary for one frozen problem. Problem content remains Problem-owned and
// is resolved through its immutable read port only after this reference is
// validated.
type ContestProblemSnapshotRef struct {
	ContestID          uuid.UUID
	ContestProblemID   uuid.UUID
	ProblemID          uuid.UUID
	ProblemRevisionID  uuid.UUID
	TestsetRevisionID  uuid.UUID
	ProblemSlug        string
	State              entity.ContestState
	StartTime          time.Time
	EndTime            time.Time
	CancelledAt        *time.Time
	DatabaseTime       time.Time
	DisplayOrder       int
	Alias              string
	Points             *int
	VisibleBeforeStart bool
}

// ContestService defines the contest business logic interface.
type ContestService interface {
	FindPublic(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ContestResponse], error)
	GetPublic(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*contract.ContestResponse, error)
	GetProblem(ctx context.Context, contestID uuid.UUID, contestProblemID uuid.UUID) (*contract.ContestProblemDetailResponse, error)
	FindAdmin(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ContestResponse], error)
	GetAdmin(ctx context.Context, id uuid.UUID) (*contract.ContestResponse, error)
	Create(ctx context.Context, authorID uuid.UUID, req *contract.CreateContestRequest) (*contract.ContestResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *contract.UpdateContestRequest) (*contract.ContestResponse, error)
	Delete(ctx context.Context, id uuid.UUID, reason string) error
}

var (
	ErrRatingSequenceBlocked    = errors.New("rating: earlier sequence is not terminal")
	ErrRatingSettlementNotReady = errors.New("rating: contest scoring is not final")
	ErrRatingNotAvailable       = errors.New("rating: canonical result is not available")
	ErrRatingReratingConflict   = errors.New("rating: historical rebuild fence changed")
	ErrRatingReratingInvariant  = errors.New("rating: historical rebuild invariant failed")
)

// RatingSettlementRepository owns the durable lifecycle of contest rating
// settlement. LockPending and Complete must be called inside the same
// transaction that writes rating histories and user ratings.
type RatingSettlementRepository interface {
	FindPending(ctx context.Context, limit int) ([]uuid.UUID, error)
	LockPending(ctx context.Context, contestID uuid.UUID) (bool, error)
	Complete(ctx context.Context, contestID uuid.UUID, completedAt time.Time) error
}

// RatingHistoryRepository defines the rating history data access interface.
type RatingHistoryRepository interface {
	CreateBulk(ctx context.Context, records []*entity.RatingHistory) error
	FindByContest(ctx context.Context, contestID uuid.UUID) ([]*entity.RatingHistory, error)
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
	CountByUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]int, error)
}

// RatingLedgerRepository owns the globally serialized canonical rating state.
// Prepare and Apply require the same caller-owned transaction; Prepare holds
// the singleton state lock until the callback commits or rolls back.
type RatingLedgerRepository interface {
	Prepare(
		ctx context.Context,
		contestID uuid.UUID,
		preparedAt time.Time,
	) (*entity.RatingSettlementSnapshot, error)
	Apply(
		ctx context.Context,
		command entity.ApplyRatingRun,
	) (*entity.RatingRun, error)
	FindActiveByContest(
		ctx context.Context,
		contestID uuid.UUID,
	) (*entity.RatingRun, error)
}

// RatingReratingRepository owns shadow generations and run chains. Every
// method requires the operation runner's transaction so a domain step and its
// durable checkpoint either commit together or both roll back.
type RatingReratingRepository interface {
	PrepareRerating(
		ctx context.Context,
		operationID uuid.UUID,
		triggerContestID uuid.UUID,
		targetProjectionGenerationID uuid.UUID,
		targetRunChainID uuid.UUID,
		preparedAt time.Time,
	) (*entity.RatingReratingPlan, error)
	EnsureReratingTargets(
		ctx context.Context,
		plan entity.RatingReratingPlan,
		createdAt time.Time,
	) error
	ReplayReratingPrefix(
		ctx context.Context,
		plan entity.RatingReratingPlan,
		sequence int64,
		previousProjectionChecksum []byte,
		previousRunChainChecksum []byte,
		appliedAt time.Time,
	) (*entity.RatingReratingStepResult, error)
	PrepareReratingSequence(
		ctx context.Context,
		plan entity.RatingReratingPlan,
		sequence int64,
		preparedAt time.Time,
	) (*entity.RatingReratingSequenceSnapshot, error)
	ApplyReratingSequence(
		ctx context.Context,
		command entity.ApplyRatingReratingSequence,
	) (*entity.RatingReratingStepResult, error)
	ActivateRerating(
		ctx context.Context,
		command entity.ActivateRatingRerating,
	) error
}

// RatingService defines the rating business logic interface.
type RatingService interface {
	CalculateRating(ctx context.Context, contestID uuid.UUID) error
	GetContestRatingChanges(ctx context.Context, contestID uuid.UUID) ([]*contract.RatingChangeResponse, error)
}

type RatingReratingStarter interface {
	Start(
		ctx context.Context,
		actorUserID uuid.UUID,
		actorSessionID uuid.UUID,
		idempotencyKey uuid.UUID,
		triggerContestID uuid.UUID,
		reason string,
	) (*contract.RatingReratingStartResponse, error)
}

// RegistrationRepository defines the registration data access interface.
type RegistrationRepository interface {
	Register(ctx context.Context, contestID, userID uuid.UUID) error
	Unregister(ctx context.Context, contestID, userID uuid.UUID) error
	Exists(ctx context.Context, contestID, userID uuid.UUID) (bool, error)
	CountByContest(ctx context.Context, contestID uuid.UUID) (int, error)
	FindByContest(ctx context.Context, contestID uuid.UUID) ([]*entity.ContestRegistration, error)
	GetProjection(ctx context.Context, contestID, userID uuid.UUID) (*RegistrationProjection, error)
}

// RegistrationProjectionReader is the least-privilege read capability used by
// public contest details.
type RegistrationProjectionReader interface {
	GetProjection(ctx context.Context, contestID, userID uuid.UUID) (*RegistrationProjection, error)
}

type RegistrationProjection struct {
	ID      uuid.UUID
	Status  entity.RegistrationStatus
	Version int64
}

// RegistrationService defines the registration business logic interface.
type RegistrationService interface {
	Register(ctx context.Context, contestID, userID uuid.UUID) error
	Unregister(ctx context.Context, contestID, userID uuid.UUID) error
	IsRegistered(ctx context.Context, contestID, userID uuid.UUID) (bool, error)
}

// StandingRepository defines the standing data access interface.
type StandingRepository interface {
	Get(ctx context.Context, contestID, userID uuid.UUID) (*entity.ContestStanding, error)
	Upsert(ctx context.Context, e *entity.ContestStanding) error
	FindByContest(ctx context.Context, contestID uuid.UUID) ([]*entity.ContestStanding, error)
}

// StandingService defines the standing business logic interface.
type StandingService interface {
	GetStandings(ctx context.Context, contestID uuid.UUID) (*contract.StandingsResponse, error)
	GetMyStanding(ctx context.Context, contestID, userID uuid.UUID) (*contract.SelfStandingResponse, error)
	GetOfficialStandings(ctx context.Context, contestID uuid.UUID) (*contract.StandingsResponse, error)
}
