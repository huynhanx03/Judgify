package domain

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaximumContestTitleBytes       = 300
	MaximumContestDescriptionBytes = 100_000
	MaximumContestProblems         = 500
	MaximumContestParticipants     = 1_000_000
)

var (
	ErrContestDraftInvalid      = errors.New("contest: invalid draft")
	ErrContestDraftProblem      = errors.New("contest: invalid draft problem")
	ErrContestDraftProblemLimit = errors.New("contest: draft problem limit exceeded")
)

// ContestDraftProblem is the complete mutable contest-owned presentation
// configuration for a published problem. DisplayOrder is one-based and must
// be contiguous inside a draft.
type ContestDraftProblem struct {
	ProblemID          uuid.UUID
	ProblemSlug        string
	ProblemTitle       string
	DisplayOrder       int
	Alias              string
	Points             *int
	VisibleBeforeStart bool
}

// ValidateContestDraft checks all aggregate invariants that do not require a
// database read. Referential publication state is checked transactionally by
// the repository.
func ValidateContestDraft(
	contest Contest,
	problems []ContestDraftProblem,
) error {
	if contest.AuthorID == uuid.Nil ||
		contest.EffectiveState() != ContestDraft ||
		contest.Version < 1 ||
		!validContestText(contest.Title, MaximumContestTitleBytes, false) ||
		strings.ContainsAny(contest.Title, "\r\n") ||
		!validContestText(
			contest.Description,
			MaximumContestDescriptionBytes,
			true,
		) ||
		contest.StartTime.IsZero() ||
		contest.EndTime.IsZero() ||
		ValidateParticipationPolicy(contest) != nil {
		return ErrContestDraftInvalid
	}
	if len(problems) > MaximumContestProblems {
		return ErrContestDraftProblemLimit
	}

	problemIDs := make(map[uuid.UUID]struct{}, len(problems))
	aliases := make(map[string]struct{}, len(problems))
	for index, problem := range problems {
		if problem.ProblemID == uuid.Nil ||
			problem.DisplayOrder != index+1 ||
			!contestAliasPattern.MatchString(problem.Alias) ||
			len(problem.Alias) > MaximumContestAliasBytes ||
			(contest.ScoringPolicy == ScoringPolicyICPC &&
				problem.Points != nil) ||
			(problem.Points != nil && *problem.Points <= 0) {
			return ErrContestDraftProblem
		}
		if _, exists := problemIDs[problem.ProblemID]; exists {
			return ErrContestDraftProblem
		}
		if _, exists := aliases[problem.Alias]; exists {
			return ErrContestDraftProblem
		}
		problemIDs[problem.ProblemID] = struct{}{}
		aliases[problem.Alias] = struct{}{}
	}
	return nil
}

func validContestText(value string, maximumBytes int, allowEmpty bool) bool {
	if !utf8.ValidString(value) ||
		len(value) > maximumBytes ||
		value != strings.TrimSpace(value) ||
		(!allowEmpty && value == "") {
		return false
	}
	for _, character := range value {
		if character == '\n' || character == '\r' || character == '\t' {
			continue
		}
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
