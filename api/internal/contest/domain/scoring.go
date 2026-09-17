package domain

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
)

const ICPCWrongAttemptPenaltySeconds = 20 * 60

var ErrInvalidStandingVerdict = errors.New("contest standing: invalid verdict")

// ApplyVerdict mutates a standing according to ICPC scoring. It accepts the
// historical JSON representation used by the API but normalizes decoded JSON
// numbers safely (encoding/json decodes them as float64, not int).
func ApplyVerdict(
	standing *ContestStanding,
	problemID uuid.UUID,
	accepted bool,
	elapsedSeconds int,
) error {
	if standing == nil || standing.ContestID == uuid.Nil || standing.UserID == uuid.Nil ||
		problemID == uuid.Nil || elapsedSeconds < 0 ||
		int64(elapsedSeconds) > math.MaxInt64/1_000 ||
		standing.SolvedCount < 0 || standing.Penalty < 0 {
		return ErrInvalidStandingVerdict
	}
	if standing.ProblemResults == nil {
		standing.ProblemResults = make(map[string]any)
	}
	key := problemID.String()
	result := normalizedProblemResult(standing.ProblemResults[key])
	if result.accepted {
		return nil
	}
	if accepted {
		result.accepted = true
		result.elapsedSeconds = elapsedSeconds
		result.elapsedMilliseconds = int64(elapsedSeconds) * 1_000
		standing.SolvedCount++
		standing.Penalty += elapsedSeconds + result.wrongAttempts*ICPCWrongAttemptPenaltySeconds
	} else {
		if result.wrongAttempts == math.MaxInt {
			return ErrInvalidStandingVerdict
		}
		result.wrongAttempts++
	}
	standing.ProblemResults[key] = map[string]any{
		"ac":       result.accepted,
		"wa_count": result.wrongAttempts,
		"time":     result.elapsedSeconds,
		"time_ms":  result.elapsedMilliseconds,
	}
	return nil
}

type problemResult struct {
	accepted            bool
	wrongAttempts       int
	elapsedSeconds      int
	elapsedMilliseconds int64
}

// ContestProblemScore is the complete ICPC contribution of one problem after
// all currently active submission verdicts have been ordered by the contest
// sequence. It is suitable for rejudge-safe replacement.
type ContestProblemScore struct {
	Accepted                bool
	WrongAttempts           int
	ElapsedMilliseconds     int64
	LastScoringSubmissionID *uuid.UUID
	AcceptedSequence        *int64
}

// ReplaceProblemScore replaces one problem's contribution after validating
// that the stored JSON projection and aggregate counters still agree. This
// makes a rejudge a deterministic replacement rather than another increment.
func ReplaceProblemScore(
	standing *ContestStanding,
	problemID uuid.UUID,
	score ContestProblemScore,
) error {
	if standing == nil || standing.ContestID == uuid.Nil ||
		standing.UserID == uuid.Nil || problemID == uuid.Nil ||
		score.WrongAttempts < 0 || score.ElapsedMilliseconds < 0 ||
		(!score.Accepted && score.ElapsedMilliseconds != 0) ||
		score.ElapsedMilliseconds/1_000 > int64(math.MaxInt) {
		return ErrInvalidStandingVerdict
	}
	if standing.ProblemResults == nil {
		standing.ProblemResults = make(map[string]any)
	}
	if err := validateStandingProjection(standing); err != nil {
		return err
	}

	key := problemID.String()
	if stored, exists := standing.ProblemResults[key]; exists {
		previous, ok := strictProblemResult(stored)
		if !ok {
			return ErrInvalidStandingVerdict
		}
		solved, penalty, ok := problemContribution(previous)
		if !ok || standing.SolvedCount < solved ||
			standing.Penalty < penalty {
			return ErrInvalidStandingVerdict
		}
		standing.SolvedCount -= solved
		standing.Penalty -= penalty
	}

	next := problemResult{
		accepted:            score.Accepted,
		wrongAttempts:       score.WrongAttempts,
		elapsedSeconds:      int(score.ElapsedMilliseconds / 1_000),
		elapsedMilliseconds: score.ElapsedMilliseconds,
	}
	solved, penalty, ok := problemContribution(next)
	if !ok || standing.SolvedCount > math.MaxInt-solved ||
		standing.Penalty > math.MaxInt-penalty {
		return ErrInvalidStandingVerdict
	}
	standing.SolvedCount += solved
	standing.Penalty += penalty
	standing.ProblemResults[key] = map[string]any{
		"ac":       next.accepted,
		"wa_count": next.wrongAttempts,
		"time":     next.elapsedSeconds,
		"time_ms":  next.elapsedMilliseconds,
	}
	return validateStandingProjection(standing)
}

func validateStandingProjection(standing *ContestStanding) error {
	if standing == nil || standing.SolvedCount < 0 ||
		standing.Penalty < 0 {
		return ErrInvalidStandingVerdict
	}
	solvedTotal := 0
	penaltyTotal := 0
	for key, raw := range standing.ProblemResults {
		id, err := uuid.Parse(key)
		if err != nil || id == uuid.Nil || id.String() != key {
			return ErrInvalidStandingVerdict
		}
		result, ok := strictProblemResult(raw)
		if !ok {
			return ErrInvalidStandingVerdict
		}
		solved, penalty, ok := problemContribution(result)
		if !ok || solvedTotal > math.MaxInt-solved ||
			penaltyTotal > math.MaxInt-penalty {
			return ErrInvalidStandingVerdict
		}
		solvedTotal += solved
		penaltyTotal += penalty
	}
	if solvedTotal != standing.SolvedCount ||
		penaltyTotal != standing.Penalty {
		return ErrInvalidStandingVerdict
	}
	return nil
}

func strictProblemResult(value any) (problemResult, bool) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 4 {
		return problemResult{}, false
	}
	accepted, acceptedOK := object["ac"].(bool)
	wrong, wrongOK := strictNonNegativeJSONInt(object["wa_count"])
	elapsed, elapsedOK := strictNonNegativeJSONInt(object["time"])
	elapsedMilliseconds, elapsedMillisecondsOK :=
		strictNonNegativeJSONInt64(object["time_ms"])
	if !acceptedOK || !wrongOK || !elapsedOK ||
		!elapsedMillisecondsOK ||
		elapsedMilliseconds/1_000 != int64(elapsed) ||
		(!accepted && (elapsed != 0 || elapsedMilliseconds != 0)) {
		return problemResult{}, false
	}
	return problemResult{
		accepted:            accepted,
		wrongAttempts:       wrong,
		elapsedSeconds:      elapsed,
		elapsedMilliseconds: elapsedMilliseconds,
	}, true
}

func problemContribution(value problemResult) (int, int, bool) {
	if value.wrongAttempts < 0 || value.elapsedSeconds < 0 ||
		value.elapsedMilliseconds < 0 ||
		value.elapsedMilliseconds/1_000 != int64(value.elapsedSeconds) ||
		(!value.accepted &&
			(value.elapsedSeconds != 0 || value.elapsedMilliseconds != 0)) {
		return 0, 0, false
	}
	if !value.accepted {
		return 0, 0, true
	}
	if value.wrongAttempts >
		(math.MaxInt-value.elapsedSeconds)/
			ICPCWrongAttemptPenaltySeconds {
		return 0, 0, false
	}
	return 1, value.elapsedSeconds +
		value.wrongAttempts*ICPCWrongAttemptPenaltySeconds, true
}

func strictNonNegativeJSONInt(value any) (int, bool) {
	parsed := nonNegativeJSONInt(value)
	switch typed := value.(type) {
	case int:
		return parsed, typed >= 0
	case int8:
		return parsed, typed >= 0
	case int16:
		return parsed, typed >= 0
	case int32:
		return parsed, typed >= 0
	case int64:
		return parsed, typed >= 0 &&
			uint64(typed) <= uint64(math.MaxInt)
	case uint:
		return parsed, uint64(typed) <= uint64(math.MaxInt)
	case uint8, uint16:
		return parsed, true
	case uint32:
		return parsed, uint64(typed) <= uint64(math.MaxInt)
	case uint64:
		return parsed, typed <= uint64(math.MaxInt)
	case float64:
		return parsed, typed >= 0 &&
			typed < float64(math.MaxInt) &&
			math.Trunc(typed) == typed
	case json.Number:
		number, err := typed.Int64()
		return parsed, err == nil && number >= 0 &&
			uint64(number) <= uint64(math.MaxInt)
	default:
		return 0, false
	}
}

func strictNonNegativeJSONInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), typed >= 0
	case int8:
		return int64(typed), typed >= 0
	case int16:
		return int64(typed), typed >= 0
	case int32:
		return int64(typed), typed >= 0
	case int64:
		return typed, typed >= 0
	case uint:
		if uint64(typed) > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(typed), true
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		if typed > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(typed), true
	case float64:
		if typed < 0 || typed >= float64(math.MaxInt64) ||
			math.Trunc(typed) != typed {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		number, err := typed.Int64()
		return number, err == nil && number >= 0
	default:
		return 0, false
	}
}

func normalizedProblemResult(value any) problemResult {
	object, ok := value.(map[string]any)
	if !ok {
		return problemResult{}
	}
	accepted, _ := object["ac"].(bool)
	elapsedSeconds := nonNegativeJSONInt(object["time"])
	if int64(elapsedSeconds) > math.MaxInt64/1_000 {
		return problemResult{}
	}
	elapsedMilliseconds, validMilliseconds :=
		strictNonNegativeJSONInt64(object["time_ms"])
	if !validMilliseconds ||
		elapsedMilliseconds/1_000 != int64(elapsedSeconds) {
		elapsedMilliseconds = int64(elapsedSeconds) * 1_000
	}
	return problemResult{
		accepted:            accepted,
		wrongAttempts:       nonNegativeJSONInt(object["wa_count"]),
		elapsedSeconds:      elapsedSeconds,
		elapsedMilliseconds: elapsedMilliseconds,
	}
}

func nonNegativeJSONInt(value any) int {
	var number int64
	switch typed := value.(type) {
	case int:
		if typed >= 0 {
			return typed
		}
		return 0
	case int8:
		number = int64(typed)
	case int16:
		number = int64(typed)
	case int32:
		number = int64(typed)
	case int64:
		number = typed
	case uint:
		if uint64(typed) > uint64(math.MaxInt) {
			return 0
		}
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		if uint64(typed) > uint64(math.MaxInt) {
			return 0
		}
		return int(typed)
	case uint64:
		if typed > uint64(math.MaxInt) {
			return 0
		}
		return int(typed)
	case float64:
		if typed < 0 || typed >= float64(math.MaxInt) || math.Trunc(typed) != typed {
			return 0
		}
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0
		}
		number = parsed
	default:
		return 0
	}
	if number < 0 || uint64(number) > uint64(math.MaxInt) {
		return 0
	}
	return int(number)
}
