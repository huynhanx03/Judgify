// Package ports defines Judgify's stable internal capability catalog.
package ports

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// Problem capabilities are added here during the Problem migration.

const authoringCatalogVersionPrefix = "v1."

var (
	authoringCatalogReleasePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	authoringCatalogVersionPattern = regexp.MustCompile(`^v1\.[0-9a-f]{64}$`)
)

// NewAuthoringCatalogVersion derives a browser-safe opaque version from one
// immutable release binding. The binding itself (for example, an approval
// checksum) never crosses the HTTP boundary.
func NewAuthoringCatalogVersion(releaseID, binding string) (string, error) {
	if !authoringCatalogReleasePattern.MatchString(releaseID) ||
		binding == "" || len(binding) > 4096 || !utf8.ValidString(binding) ||
		strings.IndexByte(binding, 0) >= 0 {
		return "", ErrInvalidAuthoringCatalog
	}
	hasher := sha256.New()
	writeAuthoringCatalogVersionPart(hasher, "judgify.problem.authoring-catalog.v1")
	writeAuthoringCatalogVersionPart(hasher, releaseID)
	writeAuthoringCatalogVersionPart(hasher, binding)
	return authoringCatalogVersionPrefix + hex.EncodeToString(hasher.Sum(nil)), nil
}

// IsAuthoringCatalogVersion accepts only the public opaque token format.
func IsAuthoringCatalogVersion(value string) bool {
	return authoringCatalogVersionPattern.MatchString(value)
}

// MatchesAuthoringCatalogVersion avoids data-dependent comparison of tokens.
func MatchesAuthoringCatalogVersion(expected, candidate string) bool {
	if !IsAuthoringCatalogVersion(expected) || !IsAuthoringCatalogVersion(candidate) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(candidate)) == 1
}

type authoringCatalogVersionWriter interface {
	Write([]byte) (int, error)
}

func writeAuthoringCatalogVersionPart(
	writer authoringCatalogVersionWriter,
	value string,
) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(value))
}

func TestAuthoringCatalogVersionIsDeterministicOpaqueAndReleaseBound(t *testing.T) {
	t.Parallel()

	first, err := NewAuthoringCatalogVersion(
		"judge-release.2026.08.1",
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatalf("NewAuthoringCatalogVersion(first) error = %v", err)
	}
	second, err := NewAuthoringCatalogVersion(
		"judge-release.2026.08.1",
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatalf("NewAuthoringCatalogVersion(second) error = %v", err)
	}
	otherRelease, err := NewAuthoringCatalogVersion(
		"judge-release.2026.08.2",
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatalf("NewAuthoringCatalogVersion(other release) error = %v", err)
	}
	if first != second || first == otherRelease || !IsAuthoringCatalogVersion(first) {
		t.Fatalf(
			"versions = %q / %q / %q, want deterministic release-bound opaque tokens",
			first,
			second,
			otherRelease,
		)
	}
	if !MatchesAuthoringCatalogVersion(first, second) ||
		MatchesAuthoringCatalogVersion(first, otherRelease) {
		t.Fatal("MatchesAuthoringCatalogVersion() violated exact-match contract")
	}
}

func TestAuthoringCatalogVersionRejectsMalformedInputsAndTokens(t *testing.T) {
	t.Parallel()

	for name, input := range map[string][2]string{
		"empty release":     {"", "binding"},
		"malformed release": {"Judge Release", "binding"},
		"empty binding":     {"judge-release.1", ""},
		"nul binding":       {"judge-release.1", "bad\x00binding"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewAuthoringCatalogVersion(input[0], input[1]); !errors.Is(err, ErrInvalidAuthoringCatalog) {
				t.Fatalf("NewAuthoringCatalogVersion() error = %v", err)
			}
		})
	}

	for _, token := range []string{
		"",
		"v1.short",
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"v1.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		if IsAuthoringCatalogVersion(token) ||
			MatchesAuthoringCatalogVersion(token, token) {
			t.Fatalf("malformed token %q was accepted", token)
		}
	}
}

var ErrDifficultyVersionConflict = errors.New("difficulty: optimistic version conflict")

// DifficultyRepository defines the difficulty data access interface.
type DifficultyRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Difficulty], error)
	FindAll(ctx context.Context) ([]*entity.Difficulty, error)
	FindAllBounded(ctx context.Context, limit int) ([]*entity.Difficulty, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Difficulty, error)
	Create(ctx context.Context, e *entity.Difficulty) error
	Update(ctx context.Context, e *entity.Difficulty) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// DifficultyCache isolates cache-aside policy from the Difficulty use case.
// The loader remains owned by the use case; keys, TTL, duplicate suppression,
// and commit-safe invalidation belong to the driven cache adapter.
type DifficultyCache interface {
	Get(context.Context, uuid.UUID, DifficultyCacheLoader) (*contract.DifficultyResult, error)
	FindAll(context.Context, DifficultyListCacheLoader) ([]*contract.DifficultyResult, error)
	Invalidate(context.Context, uuid.UUID) error
	InvalidateAll(context.Context) error
}

type DifficultyCacheLoader func(context.Context) (*contract.DifficultyResult, error)
type DifficultyListCacheLoader func(context.Context) ([]*contract.DifficultyResult, error)

// DifficultyService defines the difficulty business logic interface.
type DifficultyService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.DifficultyResult], error)
	FindAll(ctx context.Context) ([]*contract.DifficultyResult, error)
	Get(ctx context.Context, id uuid.UUID) (*contract.DifficultyResult, error)
	Create(ctx context.Context, req *contract.CreateDifficultyCommand) (*contract.DifficultyResult, error)
	Update(ctx context.Context, id uuid.UUID, req *contract.UpdateDifficultyCommand) (*contract.DifficultyResult, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

var (
	ErrImmutableProblemNotFound = errors.New(
		"problem immutable read: exact revision not found",
	)
	ErrInvalidImmutableProblem = errors.New(
		"problem immutable read: invalid or inconsistent projection",
	)
)

// ImmutableDifficultyProjection is the exact historical taxonomy label bound
// into a problem revision. It is deliberately independent from the mutable
// Difficulty aggregate.
type ImmutableDifficultyProjection struct {
	ID          uuid.UUID
	Revision    int64
	Name        string
	Level       int
	Description string
}

// ImmutableTagProjection is the exact historical tag label selected by the
// immutable reward snapshot.
type ImmutableTagProjection struct {
	ID       uuid.UUID
	Revision int64
	Name     string
}

// ImmutableProblemProjection is a browser-safe view of an exact Problem
// revision and testset. It never contains checker configuration, hidden tests,
// asset identities, checksums, or executable commands.
type ImmutableProblemProjection struct {
	ProblemID          uuid.UUID
	ProblemSlug        string
	ProblemRevisionID  uuid.UUID
	TestsetRevisionID  uuid.UUID
	RevisionNumber     int64
	Title              string
	StatementMarkdown  string
	Difficulty         ImmutableDifficultyProjection
	Tags               []ImmutableTagProjection
	Limits             entity.Limits
	AllowedRuntimeKeys []string
	SubmissionCount    int
	AcceptedCount      int
	CreatedAt          time.Time
	Samples            []entity.PublicSample
}

// ImmutableProblemReader resolves exact frozen identities selected by another
// trusted aggregate, such as a contest. Callers must establish authorization
// and visibility before invoking it.
type ImmutableProblemReader interface {
	LoadImmutableProblem(
		ctx context.Context,
		problemID uuid.UUID,
		problemRevisionID uuid.UUID,
		testsetRevisionID uuid.UUID,
	) (ImmutableProblemProjection, error)
}

var ErrProblemSlugConflict = errors.New("problem: stable slug already exists")

// ProblemRepository defines the problem data access interface.
type ProblemRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Problem], error)
	FindPublished(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Problem], error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Problem, error)
	GetPublished(ctx context.Context, id uuid.UUID) (*entity.Problem, error)
	Create(ctx context.Context, e *entity.Problem) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	AddTags(ctx context.Context, problemID uuid.UUID, tagIDs []uuid.UUID) error
	RemoveTags(ctx context.Context, problemID uuid.UUID, tagIDs []uuid.UUID) error
	ReplaceTags(ctx context.Context, problemID uuid.UUID, tagIDs []uuid.UUID) error
	GetTagIDs(ctx context.Context, problemID uuid.UUID) ([]uuid.UUID, error)
	InsertUserSolved(ctx context.Context, userID, problemID uuid.UUID) error
	IsFirstSolve(ctx context.Context, userID, problemID uuid.UUID) (bool, error)
	GetSolvedProblemIDs(ctx context.Context, userID uuid.UUID, problemIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	RecalculateStats(ctx context.Context, since *time.Time) error
}

// ProblemService defines the problem business logic interface.
type ProblemService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ProblemResult], error)
	Get(ctx context.Context, id uuid.UUID) (*contract.ProblemResult, error)
	FindAdmin(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ProblemResult], error)
	GetAdmin(ctx context.Context, id uuid.UUID) (*contract.ProblemResult, error)
	Create(ctx context.Context, authorID uuid.UUID, req *contract.CreateProblemCommand) (*contract.ProblemResult, error)
}

var (
	ErrInvalidPublication  = errors.New("problem publication: invalid command")
	ErrPublicationConflict = errors.New("problem publication: optimistic conflict")
	ErrPublicationNotFound = errors.New("problem publication: candidate not found")
	ErrPublicationDenied   = errors.New("problem publication: denied")

	ErrInvalidDraft  = errors.New("problem draft: invalid command")
	ErrDraftConflict = errors.New("problem draft: optimistic conflict")
	ErrDraftNotFound = errors.New("problem draft: aggregate not found")
	ErrDraftDenied   = errors.New("problem draft: denied")

	ErrInvalidAuthoringQuery          = errors.New("problem authoring query: invalid request or projection")
	ErrAuthoringDraftNotFound         = errors.New("problem authoring query: draft not found")
	ErrAuthoringDraftDenied           = errors.New("problem authoring query: denied")
	ErrInvalidAuthoringCatalog        = errors.New("problem authoring catalog: invalid source or projection")
	ErrAuthoringCatalogChanged        = errors.New("problem authoring catalog: version changed")
	ErrRuntimeProfileUnavailable      = errors.New("problem runtime profile: unavailable")
	ErrCheckerProfileUnavailable      = errors.New("problem checker profile: unavailable")
	ErrInvalidCheckerConfiguration    = errors.New("problem checker profile: invalid configuration")
	ErrExecutionCapabilityUnavailable = errors.New("problem execution capability: runtime and checker pairing unavailable")
)

// DraftTestcase is one authored testcase input. A nil TestCaseID creates a new
// stable lineage; a non-nil ID reuses that lineage after repository ownership
// validation. Input and ExpectedOutput are internal-only and must never enter
// ordinary/public projections.
type DraftTestcase struct {
	TestCaseID     *uuid.UUID
	Kind           string
	Input          []byte
	ExpectedOutput []byte
}

type DraftTestcaseGroup struct {
	StopOnFailure bool
	Cases         []DraftTestcase
}

type RewardIntent struct {
	DifficultyID uuid.UUID
	TagIDs       []uuid.UUID
}

// SaveDraftCommand contains authored facts only. Checksums, canonical JSON,
// immutable row IDs, revision numbers, and timestamps are server-derived.
type SaveDraftCommand struct {
	ProblemID          uuid.UUID
	ExpectedVersion    int64
	CatalogVersion     string
	Title              string
	StatementMarkdown  []byte
	Reward             RewardIntent
	Limits             entity.Limits
	CheckerKey         string
	CheckerVersion     string
	CheckerConfig      json.RawMessage
	AllowedRuntimeKeys []string
	Groups             []DraftTestcaseGroup
	Reason             string
}

type DraftRevisionNumbers struct {
	ProblemRevision int64
	TestsetRevision int64
}

type DraftResult struct {
	ProblemID         uuid.UUID
	RevisionID        uuid.UUID
	TestsetRevisionID uuid.UUID
	RenderSourceID    uuid.UUID
	RenderArtifactID  uuid.UUID
	ArtifactStatus    string
	Diagnostics       []contentrender.Diagnostic
	Version           int64
	SavedAt           time.Time
}

// DraftRepository joins an existing UnitOfWork. Implementations insert all
// immutable rows and only mutate the aggregate through the final versioned
// five-column draft selection CAS.
type DraftRepository interface {
	LockAggregate(ctx context.Context, problemID uuid.UUID) (entity.ProblemAggregate, error)
	NextRevisionNumbers(ctx context.Context, problemID uuid.UUID) (DraftRevisionNumbers, error)
	ResolveAssets(ctx context.Context, candidates []entity.TestAsset) ([]entity.TestAsset, error)
	StoreTestset(ctx context.Context, testset entity.TestsetRevision) error
	StoreRevision(ctx context.Context, revision entity.Revision) error
	AdvanceDraft(
		ctx context.Context,
		problemID uuid.UUID,
		expectedVersion int64,
		selection entity.RenderSelection,
		savedAt time.Time,
	) (entity.ProblemAggregate, error)
}

type DraftAuthorizer interface {
	AuthorizeSaveDraft(ctx context.Context, aggregate entity.ProblemAggregate) error
}

type DraftRenderer interface {
	Render(ctx context.Context, request contentrender.RenderRequest) (contentrender.Artifact, error)
}

type DraftService interface {
	SaveDraft(ctx context.Context, command SaveDraftCommand) (DraftResult, error)
}

// AuthoringTestcase is the only projection allowed to expose hidden authored
// content. Storage identities, asset checksums, encodings, and revision row IDs
// deliberately remain behind the repository boundary.
type AuthoringTestcase struct {
	TestCaseID     uuid.UUID
	Kind           string
	Input          string
	ExpectedOutput string
}

type AuthoringTestcaseGroup struct {
	StopOnFailure bool
	Cases         []AuthoringTestcase
}

// AuthoringDraft is an explicit update-only projection for the admin editor.
// Version and immutable selection IDs are returned together so every later
// save/publish/archive command remains optimistically fenced.
type AuthoringDraft struct {
	ProblemID           uuid.UUID
	Slug                string
	Lifecycle           entity.Lifecycle
	Version             int64
	RevisionID          uuid.UUID
	TestsetRevisionID   uuid.UUID
	Title               string
	StatementMarkdown   string
	DifficultyID        uuid.UUID
	TagIDs              []uuid.UUID
	Limits              entity.Limits
	CheckerKey          string
	CheckerVersion      string
	CheckerConfig       json.RawMessage
	AllowedRuntimeKeys  []string
	ArtifactStatus      string
	ArtifactDiagnostics []string
	SavedAt             time.Time
	Groups              []AuthoringTestcaseGroup
}

// AuthoringQueryRepository performs the public aggregate read separately from
// the sensitive draft projection. The application service must authorize the
// aggregate before calling LoadAuthoringDraft.
type AuthoringQueryRepository interface {
	LoadAuthoringAggregate(
		ctx context.Context,
		problemID uuid.UUID,
	) (entity.ProblemAggregate, error)
	LoadAuthoringDraft(
		ctx context.Context,
		aggregate entity.ProblemAggregate,
	) (AuthoringDraft, error)
}

type AuthoringQueryAuthorizer interface {
	AuthorizeReadDraft(ctx context.Context, aggregate entity.ProblemAggregate) error
}

type AuthoringQuery interface {
	GetDraft(ctx context.Context, problemID uuid.UUID) (AuthoringDraft, error)
}

// AuthoringCatalogSource is implemented by the release-scoped, code-owned
// runtime catalog. It deliberately exposes cloned domain profiles only to the
// application boundary; HTTP receives a much smaller safe projection below.
// CatalogVersion is opaque and binds that projection to one immutable source.
type AuthoringCatalogSource interface {
	ReleaseID() string
	CatalogVersion() string
	RuntimeProfiles() []entity.RuntimeProfile
	CheckerProfiles() []entity.CheckerProfile
	AuthoringCapabilities() []AuthoringCapabilityOption
}

// AuthoringCatalogFence is the write-side precondition for catalog-backed
// mutations. Implementations must be immutable for the process lifetime.
type AuthoringCatalogFence interface {
	ValidateAuthoringCatalogVersion(context.Context, string) error
}

type AuthoringRuntimeOption struct {
	Key            string
	Language       string
	DisplayName    string
	ProfileVersion string
}

type AuthoringCheckerOption struct {
	Key        string
	Version    string
	ConfigKind entity.CheckerConfigKind
}

// AuthoringCapabilityOption is browser-safe release metadata. It carries no
// image, command, checksum, worker, or activation identity.
type AuthoringCapabilityOption struct {
	RuntimeKey     string
	CheckerKey     string
	CheckerVersion string
}

type AuthoringNumericPolicy struct {
	Default int
	Minimum int
	Maximum int
}

type AuthoringPolicy struct {
	SlugMaximumBytes      int
	TitleMaximumBytes     int
	StatementMaximumBytes int
	ReasonMaximumBytes    int
	TestAssetMaximumBytes int
	MaximumTestcases      int
	MaximumGroups         int
	MaximumTags           int
	MaximumRuntimes       int
	CPUTimeMilliseconds   AuthoringNumericPolicy
	WallTimeMilliseconds  AuthoringNumericPolicy
	MemoryKilobytes       AuthoringNumericPolicy
	OutputBytes           AuthoringNumericPolicy
	ProcessCount          AuthoringNumericPolicy
}

type AuthoringCatalog struct {
	ReleaseID      string
	CatalogVersion string
	Runtimes       []AuthoringRuntimeOption
	Checkers       []AuthoringCheckerOption
	Capabilities   []AuthoringCapabilityOption
	Policy         AuthoringPolicy
}

type AuthoringCatalogQuery interface {
	GetCatalog(ctx context.Context) (AuthoringCatalog, error)
}

type PublishCommand struct {
	ProblemID         uuid.UUID
	DraftRevisionID   uuid.UUID
	TestsetRevisionID uuid.UUID
	ExpectedVersion   int64
	CatalogVersion    string
	Reason            string
}

type ArchiveCommand struct {
	ProblemID       uuid.UUID
	ExpectedVersion int64
	Reason          string
}

type PublicationResult struct {
	ProblemID           uuid.UUID
	PublishedRevisionID uuid.UUID
	TestsetRevisionID   uuid.UUID
	Version             int64
	PublishedAt         time.Time
}

// PublishedProblemProjection is the bounded read-model payload derived from
// the exact immutable revision selected by Publish. It contains no checker,
// hidden testcase, asset, or executable runtime data.
type PublishedProblemProjection struct {
	RevisionID        uuid.UUID
	Title             string
	StatementMarkdown string
	DifficultyID      uuid.UUID
	TagIDs            []uuid.UUID
	CPUTimeMs         int
	MemoryLimitKb     int
}

// JudgeSnapshot is the immutable Problem-owned contract later consumed by the
// submission intake workflow. RuntimeProfileID/CheckerProfileID are populated
// by the Task 3 registry adapter; the content identities already exist here.
type JudgeSnapshot struct {
	ProblemID              uuid.UUID
	ProblemRevisionID      uuid.UUID
	TestsetRevisionID      uuid.UUID
	RuntimeProfileID       uuid.UUID
	CheckerProfileID       uuid.UUID
	RuntimeKey             string
	CheckerKey             string
	CheckerVersion         string
	CheckerConfig          json.RawMessage
	Limits                 entity.Limits
	ContentChecksum        [32]byte
	TestsetChecksum        [32]byte
	RewardSnapshotChecksum [32]byte
}

// PublicationRepository joins an existing UnitOfWork. LockAggregate must take
// a PostgreSQL row lock; AdvancePublished must use a version predicate and
// update all five publication-selection fields in one statement.
type PublicationRepository interface {
	LockAggregate(ctx context.Context, problemID uuid.UUID) (entity.ProblemAggregate, error)
	LoadPublicationCandidate(
		ctx context.Context,
		aggregate entity.ProblemAggregate,
		revisionID uuid.UUID,
		testsetRevisionID uuid.UUID,
	) (entity.PublicationCandidate, error)
	AdvancePublished(
		ctx context.Context,
		problemID uuid.UUID,
		expectedVersion int64,
		selection entity.RenderSelection,
		projection PublishedProblemProjection,
		publishedAt time.Time,
	) (entity.ProblemAggregate, error)
	Archive(
		ctx context.Context,
		problemID uuid.UUID,
		expectedVersion int64,
		archivedAt time.Time,
	) (entity.ProblemAggregate, error)
}

// PublicationAuthorizer enforces object scope in addition to route-level
// Casbin permission. Implementations are local and must not perform network I/O.
type PublicationAuthorizer interface {
	AuthorizePublish(ctx context.Context, aggregate entity.ProblemAggregate) error
	AuthorizeArchive(ctx context.Context, aggregate entity.ProblemAggregate) error
}

// RuntimeCatalog and CheckerCatalog are narrow domain ports. Production binds
// them to the immutable release catalog; database profile rows remain
// projections and never become executable configuration.
type RuntimeCatalog interface {
	ValidateRuntimeKeys(ctx context.Context, runtimeKeys []string) error
}

// ExecutionCatalog prevents problem publication from synthesizing a runtime
// and checker Cartesian product. Release-backed implementations must prove
// every requested runtime against the exact checker identity in one approved
// capability tuple.
type ExecutionCatalog interface {
	RuntimeCatalog
	ValidateExecutionPairings(
		ctx context.Context,
		runtimeKeys []string,
		checkerKey string,
		checkerVersion string,
	) error
}

type CheckerCatalog interface {
	ValidateChecker(
		ctx context.Context,
		key string,
		version string,
		config json.RawMessage,
	) error
}

// TaxonomyCatalog proves that immutable difficulty/tag/element snapshot
// revisions existed. The snapshot itself remains self-contained after publish.
type TaxonomyCatalog interface {
	ResolveRewardFacts(ctx context.Context, intent RewardIntent) (entity.RewardFacts, error)
	VerifyRewardSnapshot(ctx context.Context, snapshot entity.RewardSnapshot) error
}

type PublicationService interface {
	Publish(ctx context.Context, command PublishCommand) (PublicationResult, error)
	Archive(ctx context.Context, command ArchiveCommand) (entity.ProblemAggregate, error)
}

var ErrTagVersionConflict = errors.New("tag: optimistic version conflict")

// TagRepository defines the tag data access interface.
type TagRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Tag], error)
	FindAll(ctx context.Context) ([]*entity.Tag, error)
	Get(ctx context.Context, id uuid.UUID) (*entity.Tag, error)
	Create(ctx context.Context, e *entity.Tag) error
	Update(ctx context.Context, e *entity.Tag) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	FindByIDs(ctx context.Context, ids []uuid.UUID) ([]*entity.Tag, error)
}

// TagCache isolates cache-aside policy from the Tag use case.
type TagCache interface {
	Get(context.Context, uuid.UUID, TagCacheLoader) (*contract.TagResult, error)
	FindAll(context.Context, TagListCacheLoader) ([]*contract.TagResult, error)
	Invalidate(context.Context, uuid.UUID) error
	InvalidateAll(context.Context) error
}

type TagCacheLoader func(context.Context) (*contract.TagResult, error)
type TagListCacheLoader func(context.Context) ([]*contract.TagResult, error)

// TagService defines the tag business logic interface.
type TagService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.TagResult], error)
	FindAll(ctx context.Context) ([]*contract.TagResult, error)
	Get(ctx context.Context, id uuid.UUID) (*contract.TagResult, error)
	Create(ctx context.Context, req *contract.CreateTagCommand) (*contract.TagResult, error)
	Update(ctx context.Context, id uuid.UUID, req *contract.UpdateTagCommand) (*contract.TagResult, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// PublishedTestsetRepository resolves the exact immutable testset selected by
// a currently public Problem aggregate. It must fail closed for draft,
// archived, missing, or internally inconsistent selections.
type PublishedTestsetRepository interface {
	LoadPublishedTestset(
		ctx context.Context,
		problemID uuid.UUID,
	) (entity.TestsetRevision, error)
}

// PublicSampleReader exposes only the bounded projection that is safe for
// unauthenticated problem readers. Authoring uses immutable testset revisions
// and never mutates stable test-case identities through this boundary.
type PublicSampleReader interface {
	FindPublicSamples(ctx context.Context, problemID uuid.UUID) ([]*contract.SampleTestCaseResult, error)
}
