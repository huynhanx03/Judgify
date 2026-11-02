package domain

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaximumContestReasonBytes = 1024
	MaximumContestAliasBytes  = 16
)

type ContestState string

const (
	ContestDraft     ContestState = "draft"
	ContestUpcoming  ContestState = "upcoming"
	ContestRunning   ContestState = "running"
	ContestEnded     ContestState = "ended"
	ContestCancelled ContestState = "cancelled"
)

func (state ContestState) Valid() bool {
	switch state {
	case ContestDraft, ContestUpcoming, ContestRunning, ContestEnded, ContestCancelled:
		return true
	default:
		return false
	}
}

type RegistrationMode string

const (
	RegistrationModeRegistered RegistrationMode = "registered"
	RegistrationModeOpen       RegistrationMode = "open"
)

func (mode RegistrationMode) Valid() bool {
	return mode == RegistrationModeRegistered || mode == RegistrationModeOpen
}

type RegistrationStatus string

const (
	RegistrationRegistered RegistrationStatus = "registered"
	RegistrationWithdrawn  RegistrationStatus = "withdrawn"
	RegistrationBanned     RegistrationStatus = "banned"
)

func (status RegistrationStatus) Valid() bool {
	switch status {
	case RegistrationRegistered, RegistrationWithdrawn, RegistrationBanned:
		return true
	default:
		return false
	}
}

type RegistrationSource string

const (
	RegistrationSourceExplicit     RegistrationSource = "explicit"
	RegistrationSourceImplicitOpen RegistrationSource = "implicit_open"
	RegistrationSourceAdmin        RegistrationSource = "admin"
)

func (source RegistrationSource) Valid() bool {
	switch source {
	case RegistrationSourceExplicit, RegistrationSourceImplicitOpen, RegistrationSourceAdmin:
		return true
	default:
		return false
	}
}

type ScoringPolicy string

const ScoringPolicyICPC ScoringPolicy = "icpc"

func (policy ScoringPolicy) Valid() bool { return policy == ScoringPolicyICPC }

var (
	ErrContestTransition             = errors.New("contest: invalid lifecycle transition")
	ErrContestVersionConflict        = errors.New("contest: optimistic version conflict")
	ErrContestCommandConflict        = errors.New("contest: idempotency conflict")
	ErrContestPolicy                 = errors.New("contest: invalid participation policy")
	ErrContestReason                 = errors.New("contest: invalid operator reason")
	ErrContestProblemSnapshot        = errors.New("contest: invalid immutable problem snapshot")
	ErrContestRegistrationTransition = errors.New("contest: invalid registration transition")
	ErrContestRegistrationBanned     = errors.New("contest: participant is banned")
	ErrContestRegistrationMode       = errors.New("contest: registration mode forbids command")
	ErrContestSubmissionExists       = errors.New("contest: withdrawal forbidden after official submission")
	ErrContestNotRunning             = errors.New("contest: not accepting official submissions")
	ErrContestProblemUnavailable     = errors.New("contest: problem snapshot unavailable")
	ErrContestSequence               = errors.New("contest: invalid submission sequence")
)

func ValidateContestTransition(current, next ContestState) error {
	if !current.Valid() || !next.Valid() || current == next {
		return ErrContestTransition
	}
	if next == ContestCancelled {
		if current == ContestDraft || current == ContestUpcoming || current == ContestRunning {
			return nil
		}
		return ErrContestTransition
	}
	switch {
	case current == ContestDraft && next == ContestUpcoming:
		return nil
	case current == ContestUpcoming && next == ContestRunning:
		return nil
	case current == ContestRunning && next == ContestEnded:
		return nil
	default:
		return ErrContestTransition
	}
}

func ValidateContestReason(reason string, required bool) error {
	if len(reason) > MaximumContestReasonBytes || !utf8.ValidString(reason) || reason != strings.TrimSpace(reason) {
		return ErrContestReason
	}
	if required && reason == "" {
		return ErrContestReason
	}
	for _, character := range reason {
		if unicode.IsControl(character) {
			return ErrContestReason
		}
	}
	return nil
}

// ValidateParticipationPolicy is used both before persistence and after a
// locked database read. nil ParticipantLimit means unlimited; the legacy
// MaxParticipants projection remains accepted as zero during cutover.
func ValidateParticipationPolicy(contest Contest) error {
	if !contest.RegistrationMode.Valid() || !contest.StartTime.Before(contest.EndTime) ||
		!contest.ScoringPolicy.Valid() || contest.ScoringPolicyVersion < 1 ||
		contest.StandingsProjectionVersion < 1 ||
		contest.NextSubmissionSequence < 1 ||
		contest.RatingAlgorithmVersion !=
			RatingAlgorithmExpectedRankV1 {
		return ErrContestPolicy
	}
	if contest.ParticipantLimit != nil && *contest.ParticipantLimit <= 0 {
		return ErrContestPolicy
	}
	if contest.ParticipantLimit != nil &&
		*contest.ParticipantLimit > MaximumContestParticipants {
		return ErrContestPolicy
	}
	if contest.ParticipantLimit == nil && contest.MaxParticipants < 0 {
		return ErrContestPolicy
	}
	if contest.FreezeStandingsAt != nil &&
		(contest.FreezeStandingsAt.Before(contest.StartTime) || !contest.FreezeStandingsAt.Before(contest.EndTime)) {
		return ErrContestPolicy
	}
	switch contest.RegistrationMode {
	case RegistrationModeRegistered:
		if contest.RegistrationOpensAt == nil || contest.RegistrationClosesAt == nil ||
			contest.RegistrationOpensAt.After(*contest.RegistrationClosesAt) ||
			contest.RegistrationClosesAt.After(contest.StartTime) {
			return ErrContestPolicy
		}
	case RegistrationModeOpen:
		if contest.RegistrationOpensAt != nil || contest.RegistrationClosesAt != nil {
			return ErrContestPolicy
		}
	}
	return nil
}

var contestAliasPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,15}$`)

// ContestProblem is a frozen Contest-owned judge-policy reference. It contains
// identities and bounded policy only; hidden testcase content remains Problem-owned.
type ContestProblem struct {
	ID                 uuid.UUID
	ContestID          uuid.UUID
	ProblemID          uuid.UUID
	ProblemRevisionID  uuid.UUID
	TestsetRevisionID  uuid.UUID
	DisplayOrder       int
	Alias              string
	Points             *int
	PenaltyPolicy      json.RawMessage
	VisibleBeforeStart bool
	CheckerKey         string
	CheckerVersion     string
	CheckerConfig      json.RawMessage
	AllowedRuntimeKeys []string
	SnapshotChecksum   [32]byte
	CreatedAt          time.Time
}

func (problem ContestProblem) Validate() error {
	if problem.ID == uuid.Nil || problem.ContestID == uuid.Nil || problem.ProblemID == uuid.Nil ||
		problem.ProblemRevisionID == uuid.Nil || problem.TestsetRevisionID == uuid.Nil ||
		problem.DisplayOrder < 1 || !contestAliasPattern.MatchString(problem.Alias) ||
		strings.TrimSpace(problem.CheckerKey) == "" || strings.TrimSpace(problem.CheckerVersion) == "" ||
		len(problem.AllowedRuntimeKeys) == 0 || problem.SnapshotChecksum == ([32]byte{}) ||
		(problem.Points != nil && *problem.Points <= 0) {
		return ErrContestProblemSnapshot
	}
	seen := make(map[string]struct{}, len(problem.AllowedRuntimeKeys))
	for _, runtimeKey := range problem.AllowedRuntimeKeys {
		if runtimeKey == "" || runtimeKey != strings.TrimSpace(runtimeKey) {
			return ErrContestProblemSnapshot
		}
		if _, duplicate := seen[runtimeKey]; duplicate {
			return ErrContestProblemSnapshot
		}
		seen[runtimeKey] = struct{}{}
	}
	return nil
}

type ContestTransition struct {
	ID             uuid.UUID
	ContestID      uuid.UUID
	CommandID      uuid.UUID
	CommandHash    [32]byte
	Version        int64
	From           ContestState
	To             ContestState
	ActorUserID    *uuid.UUID
	ActorSessionID *uuid.UUID
	Reason         string
	OccurredAt     time.Time
}

// ContestRegistrationTransition is immutable registration command evidence.
// Reasons are sensitive operator data and are intentionally absent from public
// event payloads.
type ContestRegistrationTransition struct {
	ID                  uuid.UUID
	ContestID           uuid.UUID
	RegistrationID      uuid.UUID
	UserID              uuid.UUID
	CommandID           uuid.UUID
	CommandHash         [32]byte
	RegistrationVersion int64
	Mutation            string
	FromStatus          string
	ToStatus            RegistrationStatus
	Source              RegistrationSource
	ActorUserID         *uuid.UUID
	ActorSessionID      *uuid.UUID
	Reason              *string
	OccurredAt          time.Time
}
