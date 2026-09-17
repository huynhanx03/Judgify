package db

import (
	"context"
	stdsql "database/sql"
	"encoding/hex"
	stdjson "encoding/json"
	"time"

	"github.com/google/uuid"

	cultivationcontract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

const sha256Size = 32

const canonicalSolveRewardTransitionSelect = `
	SELECT transition.event_id, transition.id, transition.solve_id,
		transition.user_id, transition.problem_id,
		transition.from_version, transition.to_version,
		transition.from_status, transition.to_status,
		transition.from_submission_id, transition.to_submission_id,
		transition.from_judgement_id, transition.to_judgement_id,
		transition.from_generation, transition.to_generation,
		transition.from_problem_revision_id, transition.to_problem_revision_id,
		transition.from_reward_facts, transition.to_reward_facts,
		transition.from_reward_facts_checksum, transition.to_reward_facts_checksum,
		transition.from_reward_grant_source_id, transition.to_reward_grant_source_id,
		transition.from_reward_provenance, transition.to_reward_provenance,
		transition.from_solved_at, transition.to_solved_at,
		transition.provenance, transition.occurred_at, transition.correlation_id
	FROM user_problem_solve_transitions AS transition `

type solveRewardRowScanner interface {
	Scan(...any) error
}

func readCanonicalSolveRewardTransition(
	ctx context.Context,
	tx *stdsql.Tx,
	transitionID uuid.UUID,
) (cultivationcontract.SolveRewardTransition, error) {
	event, err := scanCanonicalSolveRewardEvent(tx.QueryRowContext(
		ctx,
		canonicalSolveRewardTransitionSelect+" WHERE transition.id = $1",
		transitionID,
	))
	if err != nil {
		return cultivationcontract.SolveRewardTransition{}, err
	}
	transition, err := cultivationcontract.DecodeSolveRewardTransition(event)
	if err != nil {
		return cultivationcontract.SolveRewardTransition{}, permanentSolveReward(err)
	}
	return transition, nil
}

func scanCanonicalSolveRewardEvent(
	scanner solveRewardRowScanner,
) (submissioncontract.ProblemSolveStatusChangedV1, error) {
	if scanner == nil {
		return submissioncontract.ProblemSolveStatusChangedV1{}, ErrInvalidSolveRewardProjector
	}
	var (
		eventID, transitionID, solveID, userID, problemID uuid.UUID
		fromVersion, toVersion                            int64
		fromStatus, toStatus                              string
		fromSubmission, toSubmission                      uuid.NullUUID
		fromJudgement, toJudgement                        uuid.NullUUID
		fromGeneration, toGeneration                      stdsql.NullInt64
		fromProblemRevision, toProblemRevision            uuid.NullUUID
		fromRewardFacts, toRewardFacts                    []byte
		fromRewardChecksum, toRewardChecksum              []byte
		fromRewardGrantSource, toRewardGrantSource        uuid.NullUUID
		fromRewardProvenance, toRewardProvenance          stdsql.NullString
		fromSolvedAt, toSolvedAt                          stdsql.NullTime
		provenance, correlationID                         string
		occurredAt                                        time.Time
	)
	err := scanner.Scan(
		&eventID, &transitionID, &solveID, &userID, &problemID,
		&fromVersion, &toVersion, &fromStatus, &toStatus,
		&fromSubmission, &toSubmission, &fromJudgement, &toJudgement,
		&fromGeneration, &toGeneration, &fromProblemRevision, &toProblemRevision,
		&fromRewardFacts, &toRewardFacts, &fromRewardChecksum, &toRewardChecksum,
		&fromRewardGrantSource, &toRewardGrantSource,
		&fromRewardProvenance, &toRewardProvenance,
		&fromSolvedAt, &toSolvedAt, &provenance, &occurredAt, &correlationID,
	)
	if err != nil {
		return submissioncontract.ProblemSolveStatusChangedV1{}, err
	}
	fromChecksum, err := solveRewardChecksumString(fromRewardChecksum)
	if err != nil {
		return submissioncontract.ProblemSolveStatusChangedV1{}, err
	}
	toChecksum, err := solveRewardChecksumString(toRewardChecksum)
	if err != nil {
		return submissioncontract.ProblemSolveStatusChangedV1{}, err
	}
	event := submissioncontract.ProblemSolveStatusChangedV1{
		EventID: eventID, TransitionID: transitionID, SolveID: solveID,
		UserID: userID, ProblemID: problemID, Provenance: provenance,
		OccurredAt: occurredAt.UTC(), CorrelationID: correlationID,
		From: submissioncontract.SolveStateV1{
			Version: fromVersion, Status: fromStatus,
			SubmissionID:        nullUUIDPointer(fromSubmission),
			JudgementID:         nullUUIDPointer(fromJudgement),
			Generation:          nullInt64Pointer(fromGeneration),
			ProblemRevisionID:   nullUUIDPointer(fromProblemRevision),
			RewardGrantSourceID: nullUUIDPointer(fromRewardGrantSource),
			RewardProvenance:    nullStringPointer(fromRewardProvenance),
			RewardFacts:         cloneRewardFacts(fromRewardFacts),
			RewardFactsChecksum: fromChecksum,
			SolvedAt:            nullTimePointer(fromSolvedAt),
		},
		To: submissioncontract.SolveStateV1{
			Version: toVersion, Status: toStatus,
			SubmissionID:        nullUUIDPointer(toSubmission),
			JudgementID:         nullUUIDPointer(toJudgement),
			Generation:          nullInt64Pointer(toGeneration),
			ProblemRevisionID:   nullUUIDPointer(toProblemRevision),
			RewardGrantSourceID: nullUUIDPointer(toRewardGrantSource),
			RewardProvenance:    nullStringPointer(toRewardProvenance),
			RewardFacts:         cloneRewardFacts(toRewardFacts),
			RewardFactsChecksum: toChecksum,
			SolvedAt:            nullTimePointer(toSolvedAt),
		},
	}
	return event, nil
}

func solveRewardChecksumString(value []byte) (*string, error) {
	if len(value) == 0 {
		return nil, nil
	}
	if len(value) != sha256Size {
		return nil, permanentSolveReward(ErrSolveRewardInvariant)
	}
	encoded := hex.EncodeToString(value)
	return &encoded, nil
}

func cloneRewardFacts(value []byte) stdjson.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(stdjson.RawMessage(nil), value...)
}

func nullStringPointer(value stdsql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func nullTimePointer(value stdsql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}
