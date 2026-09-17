package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	maximumPublicStandings    = 500
	maximumSettlementStanding = 10000
)

var errInvalidStandingProjection = errors.New("contest standing: invalid projection")
var errPublicStandingAccessUnavailable = errors.New("contest standing: public access boundary is unavailable")

// limitedStandingRepository is intentionally optional while the repository
// contract is being used by jobs that need the complete final standings. The
// production repository implements it; test and alternate repositories retain
// their existing contract without opening an unbounded public endpoint.
type limitedStandingRepository interface {
	FindByContestLimit(ctx context.Context, contestID uuid.UUID, limit int) ([]*entity.ContestStanding, error)
}

type standingService struct {
	standingRepo ports.StandingRepository
	contestRepo  ports.ContestRepository
	clock        func() time.Time
}

// NewStandingService creates a standings reader. Contest lookup is optional
// only for internal final-settlement callers; the application injects it for
// every public HTTP read.
func NewStandingService(
	standingRepo ports.StandingRepository,
	contestRepos ...ports.ContestRepository,
) ports.StandingService {
	service := &standingService{
		standingRepo: standingRepo,
		clock:        func() time.Time { return time.Now().UTC() },
	}
	if len(contestRepos) > 0 {
		service.contestRepo = contestRepos[0]
	}
	return service
}

// GetStandings returns the public, bounded standings table. It uses the
// contest repository's public boundary so a direct standings URL cannot expose
// a draft/private contest, and applies the configured freeze only while the
// contest is running.
func (s *standingService) GetStandings(ctx context.Context, contestID uuid.UUID) (*contract.StandingsResponse, error) {
	if s == nil || s.standingRepo == nil || contestID == uuid.Nil {
		return nil, errInvalidStandingProjection
	}

	if s.contestRepo == nil {
		return nil, errPublicStandingAccessUnavailable
	}
	contest, err := s.contestRepo.GetPublic(ctx, contestID)
	if err != nil {
		return nil, err
	}

	// Freeze projection can change order. Load the whole bounded public working
	// set before calculating the frozen table, then trim only after ranking.
	standings, err := s.findStandings(ctx, contestID, maximumSettlementStanding+1)
	if err != nil {
		return nil, err
	}
	if len(standings) > maximumSettlementStanding {
		return nil, errInvalidStandingProjection
	}

	view := "official"
	var frozenAt *time.Time
	if s.freezeActive(contest) {
		view = "frozen"
		frozenAt = utcStandingTimePointer(contest.FreezeStandingsAt)
		frozen := make([]*entity.ContestStanding, len(standings))
		for i, standing := range standings {
			frozen[i], err = frozenStandingProjection(standing)
			if err != nil {
				return nil, err
			}
		}
		standings = frozen
	}

	responses, err := toRankedStandingResponses(standings)
	if err != nil {
		return nil, err
	}
	if len(responses) > maximumPublicStandings {
		responses = responses[:maximumPublicStandings]
	}
	version := contest.StandingsProjectionVersion
	if version < 1 {
		version = 1
	}
	return &contract.StandingsResponse{
		ContestID:         contestID,
		View:              view,
		ProjectionVersion: version,
		LastCursor:        strconv.FormatInt(version, 10),
		FrozenAt:          frozenAt,
		Items:             responses,
	}, nil
}

// GetOfficialStandings is an operator-only projection exposed on the admin
// router. Authorization is enforced before this method and no public handler
// calls it.
func (s *standingService) GetOfficialStandings(
	ctx context.Context,
	contestID uuid.UUID,
) (*contract.StandingsResponse, error) {
	if s == nil || s.standingRepo == nil || s.contestRepo == nil ||
		contestID == uuid.Nil {
		return nil, errInvalidStandingProjection
	}
	contest, err := s.contestRepo.Get(ctx, contestID)
	if err != nil {
		return nil, err
	}
	items, err := s.GetFinalStandings(ctx, contestID)
	if err != nil {
		return nil, err
	}
	version := contest.StandingsProjectionVersion
	if version < 1 {
		version = 1
	}
	return &contract.StandingsResponse{
		ContestID:         contestID,
		View:              "official",
		ProjectionVersion: version,
		LastCursor:        strconv.FormatInt(version, 10),
		Items:             items,
	}, nil
}

// GetMyStanding returns one authenticated participant's official projection,
// even while the public table is frozen. No other participant row crosses the
// boundary.
func (s *standingService) GetMyStanding(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
) (*contract.SelfStandingResponse, error) {
	if s == nil || s.standingRepo == nil || s.contestRepo == nil ||
		contestID == uuid.Nil || userID == uuid.Nil {
		return nil, errInvalidStandingProjection
	}
	contest, err := s.contestRepo.GetPublic(ctx, contestID)
	if err != nil {
		return nil, err
	}
	items, err := s.GetFinalStandings(ctx, contestID)
	if err != nil {
		return nil, err
	}
	var own *contract.StandingResponse
	for _, item := range items {
		if item != nil && item.UserID == userID {
			copy := *item
			own = &copy
			break
		}
	}
	version := contest.StandingsProjectionVersion
	if version < 1 {
		version = 1
	}
	return &contract.SelfStandingResponse{
		ContestID:         contestID,
		ProjectionVersion: version,
		LastCursor:        strconv.FormatInt(version, 10),
		Item:              own,
	}, nil
}

// GetFinalStandings is for internal settlement only. It deliberately bypasses
// public visibility and freeze projection, but refuses oversized tables rather
// than calculating ratings from a partial result set.
func (s *standingService) GetFinalStandings(ctx context.Context, contestID uuid.UUID) ([]*contract.StandingResponse, error) {
	if s == nil || s.standingRepo == nil || contestID == uuid.Nil {
		return nil, errInvalidStandingProjection
	}
	standings, err := s.findStandings(ctx, contestID, maximumSettlementStanding+1)
	if err != nil {
		return nil, err
	}
	if len(standings) > maximumSettlementStanding {
		return nil, errInvalidStandingProjection
	}
	return toRankedStandingResponses(standings)
}

func (s *standingService) findStandings(
	ctx context.Context,
	contestID uuid.UUID,
	limit int,
) ([]*entity.ContestStanding, error) {
	if repository, ok := s.standingRepo.(limitedStandingRepository); ok {
		return repository.FindByContestLimit(ctx, contestID, limit)
	}
	standings, err := s.standingRepo.FindByContest(ctx, contestID)
	if err != nil || limit <= 0 || len(standings) <= limit {
		return standings, err
	}
	return standings[:limit], nil
}

func (s *standingService) freezeActive(contest *entity.Contest) bool {
	if contest == nil || contest.FreezeStandingsAt == nil || s.clock == nil ||
		contest.EffectiveState() != entity.ContestRunning {
		return false
	}
	now := s.clock().UTC()
	return !now.Before(contest.FreezeStandingsAt.UTC()) && now.Before(contest.EndTime.UTC())
}

func projectStandingAtFreeze(
	standing *entity.ContestStanding,
	freezeElapsedMilliseconds int64,
) (*entity.ContestStanding, error) {
	if standing == nil || standing.ContestID == uuid.Nil ||
		standing.UserID == uuid.Nil || freezeElapsedMilliseconds < 0 {
		return nil, errInvalidStandingProjection
	}
	projected := &entity.ContestStanding{
		ContestID:      standing.ContestID,
		UserID:         standing.UserID,
		Username:       standing.Username,
		ProblemResults: make(map[string]any),
	}
	for key, raw := range standing.ProblemResults {
		problemID, err := uuid.Parse(key)
		if err != nil || problemID == uuid.Nil || problemID.String() != key {
			return nil, errInvalidStandingProjection
		}
		accepted, wrongAttempts, elapsedMilliseconds, ok :=
			standingProblemResult(raw)
		if !ok {
			return nil, errInvalidStandingProjection
		}
		// A result whose accepted submission is after the cutoff is entirely
		// hidden. Unsolved attempts are also omitted because the aggregate
		// projection has no per-attempt timestamps and therefore cannot safely
		// distinguish attempts made before vs. after the freeze.
		if !accepted || elapsedMilliseconds > freezeElapsedMilliseconds {
			continue
		}
		if err := entity.ReplaceProblemScore(projected, problemID, entity.ContestProblemScore{
			Accepted:            true,
			WrongAttempts:       wrongAttempts,
			ElapsedMilliseconds: elapsedMilliseconds,
		}); err != nil {
			return nil, errInvalidStandingProjection
		}
	}
	return projected, nil
}

func frozenStandingProjection(
	standing *entity.ContestStanding,
) (*entity.ContestStanding, error) {
	if standing == nil || standing.ContestID == uuid.Nil ||
		standing.UserID == uuid.Nil || standing.FrozenSolvedCount < 0 ||
		standing.FrozenPenalty < 0 || standing.FrozenPendingAttemptCount < 0 ||
		standing.FrozenVersion < 1 || standing.FrozenProblemResults == nil ||
		standing.FrozenPendingByProblem == nil {
		return nil, errInvalidStandingProjection
	}
	projected := &entity.ContestStanding{
		ID:                      standing.ID,
		ContestID:               standing.ContestID,
		UserID:                  standing.UserID,
		Username:                standing.Username,
		SolvedCount:             standing.FrozenSolvedCount,
		Penalty:                 standing.FrozenPenalty,
		ProblemResults:          standing.FrozenProblemResults,
		LastAcceptedElapsedMS:   standing.FrozenLastAcceptedMS,
		Version:                 standing.FrozenVersion,
		VisiblePendingAttempts:  standing.FrozenPendingAttemptCount,
		VisiblePendingByProblem: standing.FrozenPendingByProblem,
	}
	for problemID, count := range projected.VisiblePendingByProblem {
		parsed, err := uuid.Parse(problemID)
		if err != nil || parsed == uuid.Nil || parsed.String() != problemID ||
			count < 1 {
			return nil, errInvalidStandingProjection
		}
	}
	return projected, nil
}

func utcStandingTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func standingProblemResult(raw any) (bool, int, int64, bool) {
	result, ok := raw.(map[string]any)
	if !ok || len(result) != 4 {
		return false, 0, 0, false
	}
	accepted, acceptedOK := result["ac"].(bool)
	wrongAttempts, wrongOK := standingJSONInt(result["wa_count"])
	elapsedSeconds, elapsedOK := standingJSONInt(result["time"])
	elapsedMilliseconds, elapsedMillisecondsOK :=
		standingJSONInt64(result["time_ms"])
	if !acceptedOK || !wrongOK || !elapsedOK ||
		!elapsedMillisecondsOK ||
		elapsedMilliseconds/1_000 != int64(elapsedSeconds) ||
		(!accepted && (elapsedSeconds != 0 || elapsedMilliseconds != 0)) {
		return false, 0, 0, false
	}
	return accepted, wrongAttempts, elapsedMilliseconds, true
}

func standingJSONInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, typed >= 0
	case int64:
		if typed < 0 || uint64(typed) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(typed), true
	case float64:
		return int(typed), typed >= 0 && typed < float64(math.MaxInt) && math.Trunc(typed) == typed
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil && parsed >= 0 && uint64(parsed) <= uint64(math.MaxInt)
	default:
		return 0, false
	}
}

func standingJSONInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), typed >= 0
	case int64:
		return typed, typed >= 0
	case float64:
		return int64(typed), typed >= 0 &&
			typed < float64(math.MaxInt64) && math.Trunc(typed) == typed
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil && parsed >= 0
	default:
		return 0, false
	}
}

func toRankedStandingResponses(standings []*entity.ContestStanding) ([]*contract.StandingResponse, error) {
	for _, standing := range standings {
		if standing == nil || standing.UserID == uuid.Nil || standing.SolvedCount < 0 || standing.Penalty < 0 {
			return nil, errInvalidStandingProjection
		}
	}
	sort.Slice(standings, func(i, j int) bool {
		if standings[i].SolvedCount != standings[j].SolvedCount {
			return standings[i].SolvedCount > standings[j].SolvedCount
		}
		if standings[i].Penalty != standings[j].Penalty {
			return standings[i].Penalty < standings[j].Penalty
		}
		leftAccepted := standings[i].LastAcceptedElapsedMS
		rightAccepted := standings[j].LastAcceptedElapsedMS
		if leftAccepted != nil && rightAccepted != nil &&
			*leftAccepted != *rightAccepted {
			return *leftAccepted < *rightAccepted
		}
		if leftAccepted != nil && rightAccepted == nil {
			return true
		}
		if leftAccepted == nil && rightAccepted != nil {
			return false
		}
		return standings[i].UserID.String() < standings[j].UserID.String()
	})

	responses := make([]*contract.StandingResponse, len(standings))
	rank := 0
	for i, standing := range standings {
		if i == 0 || standing.SolvedCount != standings[i-1].SolvedCount || standing.Penalty != standings[i-1].Penalty {
			rank = i + 1
		}
		responses[i] = ToStandingResponse(rank, standing, standing.Username)
	}
	return responses, nil
}
