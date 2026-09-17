package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestratingresult"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestratingrun"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestregistration"
	"github.com/huynhanx03/judgify/internal/ent/generate/conteststanding"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestuserrating"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratinghistory"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingprojectiongeneration"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingrunchain"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingrunchainentry"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingsequenceslot"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingstate"
	"github.com/huynhanx03/judgify/internal/ent/generate/submission"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/ent/generate/userstats"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	canonicalRatingStateID          = "global"
	canonicalRatingAlgorithmVersion = "expected-rank-v1"
	maximumRatingParticipants       = 10000
)

var (
	ErrInvalidRatingLedger      = errors.New("rating ledger: invalid dependency or command")
	ErrRatingSequenceBlocked    = ports.ErrRatingSequenceBlocked
	ErrRatingSettlementNotReady = ports.ErrRatingSettlementNotReady
	ErrRatingLedgerInvariant    = errors.New("rating ledger: canonical state invariant failed")
)

// RatingLedgerRepository persists the active generation and run chain. Every
// mutating method joins the transaction established by ratingService.
type RatingLedgerRepository struct {
	client *dbent.EntClient
}

func NewRatingLedgerRepository(
	client *dbent.EntClient,
) *RatingLedgerRepository {
	return &RatingLedgerRepository{client: client}
}

func (repository *RatingLedgerRepository) Prepare(
	ctx context.Context,
	contestID uuid.UUID,
	preparedAt time.Time,
) (*entity.RatingSettlementSnapshot, error) {
	if err := repository.validateTransaction(ctx); err != nil ||
		contestID == uuid.Nil || preparedAt.IsZero() {
		if err != nil {
			return nil, err
		}
		return nil, ErrInvalidRatingLedger
	}
	client := repository.client.DB(ctx)
	state, generation, chain, err :=
		repository.lockOrInitializeState(ctx, client, preparedAt.UTC())
	if err != nil {
		return nil, err
	}
	contestRecord, err := client.Contest.Query().
		Where(
			contest.IDEQ(contestID),
			contest.StatusEQ(contest.StatusEnded),
			contest.RatedEQ(true),
		).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock rated contest: %w", err)
	}
	if contestRecord.RatingAlgorithmVersion !=
		canonicalRatingAlgorithmVersion ||
		contestRecord.StandingsProjectionVersion < 1 {
		return nil, ErrRatingLedgerInvariant
	}
	slot, err := repository.ensureSequenceSlot(
		ctx,
		client,
		state,
		contestRecord,
		preparedAt.UTC(),
	)
	if err != nil {
		return nil, err
	}
	snapshot := &entity.RatingSettlementSnapshot{
		ContestID:                  contestID,
		RatingSequence:             slot.RatingSequence,
		RatingSequenceSlotID:       slot.ID,
		StandingsProjectionVersion: contestRecord.StandingsProjectionVersion,
		AlgorithmVersion:           contestRecord.RatingAlgorithmVersion,
		ProjectionRevision:         state.ProjectionRevision,
		ProjectionGenerationID:     generation.ID,
		RunChainID:                 chain.ID,
		ProjectionChecksum:         cloneOptionalChecksum(generation.Checksum),
		RunChainChecksum:           cloneOptionalChecksum(chain.Checksum),
	}
	if slot.RatingSequence <= state.AppliedSequence ||
		contestRecord.RatingCompletedAt != nil {
		run, findErr := repository.findActiveRun(
			ctx,
			client,
			chain.ID,
			slot.RatingSequence,
		)
		if findErr != nil {
			return nil, errors.Join(ErrRatingLedgerInvariant, findErr)
		}
		snapshot.IdempotentRun = run
		snapshot.RunVersion = run.RunVersion
		return snapshot, nil
	}
	if slot.RatingSequence != state.AppliedSequence+1 {
		return nil, ErrRatingSequenceBlocked
	}
	if err := repository.ensureScoringFinal(
		ctx,
		client,
		contestID,
	); err != nil {
		return nil, err
	}
	participants, err := repository.loadParticipants(
		ctx,
		client,
		contestID,
		generation.ID,
		true,
	)
	if err != nil {
		return nil, err
	}
	if len(participants) == 0 ||
		len(participants) > maximumRatingParticipants {
		return nil, ErrRatingSettlementNotReady
	}
	snapshot.Participants = participants
	snapshot.RunVersion, err = repository.nextRunVersion(
		ctx,
		client,
		contestID,
	)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (repository *RatingLedgerRepository) Apply(
	ctx context.Context,
	command entity.ApplyRatingRun,
) (*entity.RatingRun, error) {
	if err := repository.validateApply(ctx, command); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	state, err := client.RatingState.Query().
		Where(ratingstate.IDEQ(canonicalRatingStateID)).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock rating state for apply: %w", err)
	}
	snapshot := command.Snapshot
	if state.AppliedSequence+1 != snapshot.RatingSequence ||
		state.ProjectionRevision != snapshot.ProjectionRevision ||
		state.ActiveProjectionGenerationID !=
			snapshot.ProjectionGenerationID ||
		state.ActiveRunChainID != snapshot.RunChainID {
		return nil, ErrRatingSequenceBlocked
	}
	existingEntry, entryErr := client.RatingRunChainEntry.Query().
		Where(
			ratingrunchainentry.ChainIDEQ(snapshot.RunChainID),
			ratingrunchainentry.RatingSequenceEQ(
				snapshot.RatingSequence,
			),
		).
		Only(ctx)
	if entryErr == nil {
		return repository.loadRunByID(ctx, client, existingEntry.RatingRunID)
	}
	if !generate.IsNotFound(entryErr) {
		return nil, fmt.Errorf("read rating run chain entry: %w", entryErr)
	}
	contestRecord, err := client.Contest.Query().
		Where(
			contest.IDEQ(snapshot.ContestID),
			contest.StatusEQ(contest.StatusEnded),
			contest.RatedEQ(true),
			contest.StandingsProjectionVersionEQ(
				snapshot.StandingsProjectionVersion,
			),
			contest.RatingCompletedAtIsNil(),
		).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil ||
		contestRecord.RatingAlgorithmVersion != snapshot.AlgorithmVersion {
		if err != nil {
			return nil, fmt.Errorf("lock contest rating apply fence: %w", err)
		}
		return nil, ErrRatingLedgerInvariant
	}
	slot, err := client.RatingSequenceSlot.Query().
		Where(
			ratingsequenceslot.IDEQ(
				snapshot.RatingSequenceSlotID,
			),
			ratingsequenceslot.ContestIDEQ(snapshot.ContestID),
			ratingsequenceslot.RatingSequenceEQ(
				snapshot.RatingSequence,
			),
		).
		Only(ctx)
	if err != nil || slot.ContestID != snapshot.ContestID {
		if err != nil {
			return nil, fmt.Errorf("resolve rating sequence slot: %w", err)
		}
		return nil, ErrRatingLedgerInvariant
	}

	runRecord, err := client.ContestRatingRun.Create().
		SetID(command.RunID).
		SetContestID(snapshot.ContestID).
		SetRatingSequenceSlotID(snapshot.RatingSequenceSlotID).
		SetRatingSequence(snapshot.RatingSequence).
		SetRunVersion(snapshot.RunVersion).
		SetStandingsProjectionVersion(
			snapshot.StandingsProjectionVersion,
		).
		SetAlgorithmVersion(snapshot.AlgorithmVersion).
		SetStatus(contestratingrun.StatusApplied).
		SetInputChecksum(bytes.Clone(command.InputChecksum)).
		SetParticipantCount(len(command.Results)).
		SetAppliedAt(command.AppliedAt.UTC()).
		SetCreatedAt(command.AppliedAt.UTC()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create immutable rating run: %w", err)
	}
	if err := repository.createRunResults(
		ctx,
		client,
		runRecord.ID,
		command,
	); err != nil {
		return nil, err
	}
	if err := repository.advanceCurrentRatings(
		ctx,
		client,
		command,
	); err != nil {
		return nil, err
	}
	if _, err := client.RatingRunChainEntry.Create().
		SetChainID(snapshot.RunChainID).
		SetRatingSequence(snapshot.RatingSequence).
		SetRatingRunID(runRecord.ID).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("append immutable rating run chain: %w", err)
	}
	projectionChecksum := nextProjectionChecksum(
		snapshot.ProjectionChecksum,
		snapshot.RatingSequence,
		command.Results,
	)
	chainChecksum := nextChainChecksum(
		snapshot.RunChainChecksum,
		snapshot.RatingSequence,
		runRecord.ID,
	)
	if _, err := client.RatingProjectionGeneration.UpdateOneID(
		snapshot.ProjectionGenerationID,
	).
		Where(
			ratingprojectiongeneration.StatusEQ(
				ratingprojectiongeneration.StatusActive,
			),
			ratingprojectiongeneration.ThroughSequenceEQ(
				state.AppliedSequence,
			),
			ratingprojectiongeneration.ChecksumEQ(
				snapshot.ProjectionChecksum,
			),
		).
		SetThroughSequence(snapshot.RatingSequence).
		SetChecksum(projectionChecksum).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("advance active rating projection: %w", err)
	}
	if _, err := client.RatingRunChain.UpdateOneID(snapshot.RunChainID).
		Where(
			ratingrunchain.StatusEQ(ratingrunchain.StatusActive),
			ratingrunchain.ThroughSequenceEQ(state.AppliedSequence),
			ratingrunchain.ChecksumEQ(snapshot.RunChainChecksum),
		).
		SetThroughSequence(snapshot.RatingSequence).
		SetChecksum(chainChecksum).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("advance active rating run chain: %w", err)
	}
	updated, err := client.RatingState.UpdateOneID(canonicalRatingStateID).
		Where(
			ratingstate.AppliedSequenceEQ(state.AppliedSequence),
			ratingstate.ProjectionRevisionEQ(
				snapshot.ProjectionRevision,
			),
			ratingstate.ActiveProjectionGenerationIDEQ(
				snapshot.ProjectionGenerationID,
			),
			ratingstate.ActiveRunChainIDEQ(snapshot.RunChainID),
		).
		SetAppliedSequence(snapshot.RatingSequence).
		SetProjectionRevision(snapshot.ProjectionRevision + 1).
		Save(ctx)
	if err != nil ||
		updated.AppliedSequence != snapshot.RatingSequence {
		if err != nil {
			return nil, fmt.Errorf("advance rating serialization fence: %w", err)
		}
		return nil, ErrRatingLedgerInvariant
	}
	if _, err := client.Contest.UpdateOneID(snapshot.ContestID).
		Where(
			contest.RatingCompletedAtIsNil(),
			contest.StandingsProjectionVersionEQ(
				snapshot.StandingsProjectionVersion,
			),
		).
		SetRatingCompletedAt(command.AppliedAt.UTC()).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("complete contest rating settlement: %w", err)
	}
	return repository.loadRunByID(ctx, client, runRecord.ID)
}

func (repository *RatingLedgerRepository) FindActiveByContest(
	ctx context.Context,
	contestID uuid.UUID,
) (*entity.RatingRun, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		contestID == uuid.Nil {
		return nil, ErrInvalidRatingLedger
	}
	client := repository.client.DB(ctx)
	state, err := client.RatingState.Query().
		Where(ratingstate.IDEQ(canonicalRatingStateID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("read active rating state: %w", err)
	}
	slot, err := client.RatingSequenceSlot.Query().
		Where(ratingsequenceslot.ContestIDEQ(contestID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("read contest rating sequence: %w", err)
	}
	return repository.findActiveRun(
		ctx,
		client,
		state.ActiveRunChainID,
		slot.RatingSequence,
	)
}

func (repository *RatingLedgerRepository) validateTransaction(
	ctx context.Context,
) error {
	if repository == nil || repository.client == nil || ctx == nil {
		return ErrInvalidRatingLedger
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return errors.Join(
			platformtx.ErrTransactionRequired,
			ErrInvalidRatingLedger,
		)
	}
	return nil
}

func (repository *RatingLedgerRepository) lockOrInitializeState(
	ctx context.Context,
	client *generate.Client,
	now time.Time,
) (
	*generate.RatingState,
	*generate.RatingProjectionGeneration,
	*generate.RatingRunChain,
	error,
) {
	state, err := client.RatingState.Query().
		Where(ratingstate.IDEQ(canonicalRatingStateID)).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if generate.IsNotFound(err) {
		return repository.initializeState(ctx, client, now)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("lock rating state: %w", err)
	}
	generation, err := client.RatingProjectionGeneration.Query().
		Where(
			ratingprojectiongeneration.IDEQ(
				state.ActiveProjectionGenerationID,
			),
			ratingprojectiongeneration.StatusEQ(
				ratingprojectiongeneration.StatusActive,
			),
		).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil {
		return nil, nil, nil,
			fmt.Errorf("lock active rating projection: %w", err)
	}
	chain, err := client.RatingRunChain.Query().
		Where(
			ratingrunchain.IDEQ(state.ActiveRunChainID),
			ratingrunchain.StatusEQ(ratingrunchain.StatusActive),
		).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil {
		return nil, nil, nil,
			fmt.Errorf("lock active rating run chain: %w", err)
	}
	if generation.ThroughSequence != state.AppliedSequence ||
		chain.ThroughSequence != state.AppliedSequence ||
		len(optionalChecksumValue(generation.Checksum)) != sha256.Size ||
		len(optionalChecksumValue(chain.Checksum)) != sha256.Size {
		return nil, nil, nil, ErrRatingLedgerInvariant
	}
	return state, generation, chain, nil
}

func (repository *RatingLedgerRepository) initializeState(
	ctx context.Context,
	client *generate.Client,
	now time.Time,
) (
	*generate.RatingState,
	*generate.RatingProjectionGeneration,
	*generate.RatingRunChain,
	error,
) {
	projectionChecksum := emptyRatingProjectionChecksum()
	chainChecksum := emptyRatingRunChainChecksum()
	generation, err := client.RatingProjectionGeneration.Create().
		SetStatus(ratingprojectiongeneration.StatusActive).
		SetBasedOnProjectionRevision(0).
		SetThroughSequence(0).
		SetChecksum(projectionChecksum[:]).
		SetVerifiedAt(now).
		SetActivatedAt(now).
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, nil, nil,
			fmt.Errorf("initialize rating projection: %w", err)
	}
	chain, err := client.RatingRunChain.Create().
		SetStatus(ratingrunchain.StatusActive).
		SetThroughSequence(0).
		SetChecksum(chainChecksum[:]).
		SetVerifiedAt(now).
		SetActivatedAt(now).
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, nil, nil,
			fmt.Errorf("initialize rating run chain: %w", err)
	}
	state, err := client.RatingState.Create().
		SetID(canonicalRatingStateID).
		SetNextSequence(1).
		SetAppliedSequence(0).
		SetProjectionRevision(1).
		SetActiveProjectionGenerationID(generation.ID).
		SetActiveRunChainID(chain.ID).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initialize rating state: %w", err)
	}
	return state, generation, chain, nil
}

func emptyRatingProjectionChecksum() [sha256.Size]byte {
	return sha256.Sum256(
		[]byte("judgify/rating-projection/expected-rank-v1/empty\n"),
	)
}

func emptyRatingRunChainChecksum() [sha256.Size]byte {
	return sha256.Sum256(
		[]byte("judgify/rating-run-chain/expected-rank-v1/empty\n"),
	)
}

func optionalChecksumValue(checksum *[]byte) []byte {
	if checksum == nil {
		return nil
	}
	return *checksum
}

func cloneOptionalChecksum(checksum *[]byte) []byte {
	return bytes.Clone(optionalChecksumValue(checksum))
}

func (repository *RatingLedgerRepository) ensureSequenceSlot(
	ctx context.Context,
	client *generate.Client,
	state *generate.RatingState,
	contestRecord *generate.Contest,
	now time.Time,
) (*generate.RatingSequenceSlot, error) {
	slot, err := client.RatingSequenceSlot.Query().
		Where(ratingsequenceslot.ContestIDEQ(contestRecord.ID)).
		Only(ctx)
	if err == nil {
		return slot, nil
	}
	if !generate.IsNotFound(err) {
		return nil, fmt.Errorf("read contest rating slot: %w", err)
	}
	earliestID, err := client.Contest.Query().
		Where(
			contest.StatusEQ(contest.StatusEnded),
			contest.RatedEQ(true),
			contest.RatingCompletedAtIsNil(),
		).
		Order(
			generate.Asc(contest.FieldEndTime),
			generate.Asc(contest.FieldID),
		).
		FirstID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve next rating contest: %w", err)
	}
	if earliestID != contestRecord.ID {
		return nil, ErrRatingSequenceBlocked
	}
	slot, err = client.RatingSequenceSlot.Create().
		SetRatingSequence(state.NextSequence).
		SetContestID(contestRecord.ID).
		SetEffectiveEndAt(contestRecord.EndTime.UTC()).
		SetAssignedAt(now).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("assign contest rating sequence: %w", err)
	}
	updated, err := client.RatingState.UpdateOneID(canonicalRatingStateID).
		Where(ratingstate.NextSequenceEQ(state.NextSequence)).
		SetNextSequence(state.NextSequence + 1).
		Save(ctx)
	if err != nil || updated.NextSequence != state.NextSequence+1 {
		if err != nil {
			return nil, fmt.Errorf("advance next rating sequence: %w", err)
		}
		return nil, ErrRatingLedgerInvariant
	}
	state.NextSequence = updated.NextSequence
	return slot, nil
}

func (repository *RatingLedgerRepository) ensureScoringFinal(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
) error {
	unresolved, err := client.Submission.Query().
		Where(
			submission.ContestIDEQ(contestID),
			submission.DeletedAtIsNil(),
			submission.Or(
				submission.LatestGenerationPhaseNotIn(
					submission.LatestGenerationPhaseTerminal,
					submission.LatestGenerationPhaseCancelled,
				),
				submission.LatestGenerationOutcomeEQ(
					submission.LatestGenerationOutcomeSystemError,
				),
			),
		).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("check contest rating readiness: %w", err)
	}
	if unresolved {
		return ErrRatingSettlementNotReady
	}
	return nil
}

func (repository *RatingLedgerRepository) loadParticipants(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
	generationID uuid.UUID,
	rejectLegacyGap bool,
) ([]entity.RatingParticipantSnapshot, error) {
	var participantRows []struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := client.Submission.Query().
		Where(
			submission.ContestIDEQ(contestID),
			submission.DeletedAtIsNil(),
			submission.ContestProblemIDNotNil(),
			submission.ContestSubmissionSequenceNotNil(),
		).
		GroupBy(submission.FieldUserID).
		Scan(ctx, &participantRows); err != nil {
		return nil, fmt.Errorf("load rated participant identities: %w", err)
	}
	if len(participantRows) == 0 ||
		len(participantRows) > maximumRatingParticipants {
		return nil, ErrRatingSettlementNotReady
	}
	userIDs := make([]uuid.UUID, len(participantRows))
	for index, row := range participantRows {
		if row.UserID == uuid.Nil {
			return nil, ErrRatingLedgerInvariant
		}
		userIDs[index] = row.UserID
	}
	disqualified, err := client.ContestRegistration.Query().
		Where(
			contestregistration.ContestIDEQ(contestID),
			contestregistration.UserIDIn(userIDs...),
			contestregistration.StatusEQ(
				contestregistration.StatusBanned,
			),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load disqualified contestants: %w", err)
	}
	if len(disqualified) > 0 {
		banned := make(map[uuid.UUID]struct{}, len(disqualified))
		for _, registration := range disqualified {
			banned[registration.UserID] = struct{}{}
		}
		eligible := userIDs[:0]
		for _, userID := range userIDs {
			if _, excluded := banned[userID]; !excluded {
				eligible = append(eligible, userID)
			}
		}
		userIDs = eligible
	}
	if len(userIDs) == 0 {
		return nil, ErrRatingSettlementNotReady
	}
	sort.Slice(userIDs, func(left, right int) bool {
		return userIDs[left].String() < userIDs[right].String()
	})
	userRecords, err := client.User.Query().
		Where(user.IDIn(userIDs...)).
		All(ctx)
	if err != nil || len(userRecords) != len(userIDs) {
		if err != nil {
			return nil, fmt.Errorf("load rated participant users: %w", err)
		}
		return nil, ErrRatingLedgerInvariant
	}
	usernames := make(map[uuid.UUID]string, len(userRecords))
	for _, record := range userRecords {
		usernames[record.ID] = record.Username
	}
	standingRecords, err := client.ContestStanding.Query().
		Where(
			conteststanding.ContestIDEQ(contestID),
			conteststanding.UserIDIn(userIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load final rated standings: %w", err)
	}
	standings := make(map[uuid.UUID]*generate.ContestStanding, len(standingRecords))
	for _, record := range standingRecords {
		standings[record.UserID] = record
	}
	ratingRecords, err := client.ContestUserRating.Query().
		Where(
			contestuserrating.ProjectionGenerationIDEQ(generationID),
			contestuserrating.UserIDIn(userIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load prior canonical ratings: %w", err)
	}
	ratings := make(map[uuid.UUID]*generate.ContestUserRating, len(ratingRecords))
	for _, record := range ratingRecords {
		ratings[record.UserID] = record
	}
	participants := make([]entity.RatingParticipantSnapshot, len(userIDs))
	for index, userID := range userIDs {
		username, exists := usernames[userID]
		if !exists || username == "" {
			return nil, ErrRatingLedgerInvariant
		}
		participant := entity.RatingParticipantSnapshot{
			UserID:         userID,
			Username:       username,
			PreviousRating: 1200,
		}
		if standing := standings[userID]; standing != nil {
			if standing.SolvedCount < 0 || standing.Penalty < 0 {
				return nil, ErrRatingLedgerInvariant
			}
			participant.SolvedCount = standing.SolvedCount
			participant.PenaltySeconds = standing.Penalty
		}
		if rating := ratings[userID]; rating != nil {
			if rating.Rating < 0 || rating.RatedContestCount < 1 {
				return nil, ErrRatingLedgerInvariant
			}
			participant.PreviousRating = rating.Rating
			participant.PreviousRatedContests =
				rating.RatedContestCount
		} else if rejectLegacyGap {
			hasHistory, historyErr := client.RatingHistory.Query().
				Where(ratinghistory.UserIDEQ(userID)).
				Exist(ctx)
			if historyErr != nil {
				return nil, fmt.Errorf(
					"verify missing canonical user rating: %w",
					historyErr,
				)
			}
			if hasHistory {
				return nil, ErrRatingLedgerInvariant
			}
		}
		participants[index] = participant
	}
	return participants, nil
}

func (repository *RatingLedgerRepository) nextRunVersion(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
) (int64, error) {
	record, err := client.ContestRatingRun.Query().
		Where(contestratingrun.ContestIDEQ(contestID)).
		Order(generate.Desc(contestratingrun.FieldRunVersion)).
		First(ctx)
	if generate.IsNotFound(err) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read latest contest rating run: %w", err)
	}
	if record.RunVersion < 1 ||
		record.RunVersion == int64(^uint64(0)>>1) {
		return 0, ErrRatingLedgerInvariant
	}
	return record.RunVersion + 1, nil
}

func (repository *RatingLedgerRepository) createRunResults(
	ctx context.Context,
	client *generate.Client,
	runID uuid.UUID,
	command entity.ApplyRatingRun,
) error {
	builders := make(
		[]*generate.ContestRatingResultCreate,
		len(command.Results),
	)
	for index, result := range command.Results {
		builders[index] = client.ContestRatingResult.Create().
			SetRatingRunID(runID).
			SetUserID(result.UserID).
			SetPriorRating(result.PreviousRating).
			SetPriorRatedContestCount(
				result.PreviousRatedContests,
			).
			SetRankPosition(result.Rank).
			SetActualRank(result.ActualRank).
			SetExpectedRank(result.ExpectedRank).
			SetDelta(result.Delta).
			SetNewRating(result.NewRating).
			SetCreatedAt(command.AppliedAt.UTC()).
			SetUpdatedAt(command.AppliedAt.UTC())
	}
	if _, err := client.ContestRatingResult.CreateBulk(builders...).
		Save(ctx); err != nil {
		return fmt.Errorf("create immutable rating results: %w", err)
	}
	return nil
}

func (repository *RatingLedgerRepository) advanceCurrentRatings(
	ctx context.Context,
	client *generate.Client,
	command entity.ApplyRatingRun,
) error {
	snapshot := command.Snapshot
	if err := repository.advanceProjectionRatings(
		ctx,
		client,
		snapshot.ProjectionGenerationID,
		snapshot.RatingSequence,
		command.Results,
		command.AppliedAt,
	); err != nil {
		return err
	}
	historyBuilders := make(
		[]*generate.RatingHistoryCreate,
		0,
		len(command.Results),
	)
	for _, result := range command.Results {
		updated, err := client.UserStats.Update().
			Where(
				userstats.UserIDEQ(result.UserID),
				userstats.DeletedAtIsNil(),
			).
			SetRating(result.NewRating).
			SetUpdatedAt(command.AppliedAt.UTC()).
			Save(ctx)
		if err != nil || updated != 1 {
			if err != nil {
				return fmt.Errorf(
					"mirror canonical user rating: %w",
					err,
				)
			}
			return ErrRatingLedgerInvariant
		}
		historyBuilders = append(
			historyBuilders,
			client.RatingHistory.Create().
				SetUserID(result.UserID).
				SetContestID(snapshot.ContestID).
				SetRatingRunID(command.RunID).
				SetRatingSequence(snapshot.RatingSequence).
				SetRunVersion(snapshot.RunVersion).
				SetOldRating(result.PreviousRating).
				SetNewRating(result.NewRating).
				SetRankPosition(result.Rank).
				SetPriorRatedContestCount(
					result.PreviousRatedContests,
				).
				SetActualRank(result.ActualRank).
				SetExpectedRank(result.ExpectedRank).
				SetDelta(result.Delta).
				SetAlgorithmVersion(snapshot.AlgorithmVersion),
		)
	}
	if _, err := client.RatingHistory.CreateBulk(historyBuilders...).
		Save(ctx); err != nil {
		return fmt.Errorf("append compatibility rating history: %w", err)
	}
	return nil
}

// advanceProjectionRatings applies one sequence to an arbitrary projection
// generation. It deliberately has no user_stats or compatibility-history side
// effects so shadow rerating remains invisible before activation.
func (repository *RatingLedgerRepository) advanceProjectionRatings(
	ctx context.Context,
	client *generate.Client,
	generationID uuid.UUID,
	sequence int64,
	results []entity.RatingRunResult,
	appliedAt time.Time,
) error {
	userIDs := make([]uuid.UUID, len(results))
	for index, result := range results {
		userIDs[index] = result.UserID
	}
	existingRecords, err := client.ContestUserRating.Query().
		Where(
			contestuserrating.ProjectionGenerationIDEQ(
				generationID,
			),
			contestuserrating.UserIDIn(userIDs...),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("lock current rating projection rows: %w", err)
	}
	existing := make(
		map[uuid.UUID]*generate.ContestUserRating,
		len(existingRecords),
	)
	for _, record := range existingRecords {
		existing[record.UserID] = record
	}
	for _, result := range results {
		record := existing[result.UserID]
		if record == nil {
			if result.PreviousRating != 1200 ||
				result.PreviousRatedContests != 0 {
				return ErrRatingLedgerInvariant
			}
			if _, err := client.ContestUserRating.Create().
				SetProjectionGenerationID(
					generationID,
				).
				SetUserID(result.UserID).
				SetRating(result.NewRating).
				SetRatedContestCount(1).
				SetAppliedSequence(sequence).
				SetRevision(1).
				SetCreatedAt(appliedAt.UTC()).
				SetUpdatedAt(appliedAt.UTC()).
				Save(ctx); err != nil {
				return fmt.Errorf(
					"create canonical current user rating: %w",
					err,
				)
			}
		} else {
			if record.Rating != result.PreviousRating ||
				record.RatedContestCount !=
					result.PreviousRatedContests ||
				record.AppliedSequence >= sequence {
				return ErrRatingLedgerInvariant
			}
			if _, err := client.ContestUserRating.UpdateOneID(record.ID).
				Where(
					contestuserrating.RevisionEQ(record.Revision),
					contestuserrating.AppliedSequenceEQ(
						record.AppliedSequence,
					),
				).
				SetRating(result.NewRating).
				SetRatedContestCount(
					result.PreviousRatedContests + 1,
				).
				SetAppliedSequence(sequence).
				SetRevision(record.Revision + 1).
				SetUpdatedAt(appliedAt.UTC()).
				Save(ctx); err != nil {
				return fmt.Errorf(
					"advance canonical current user rating: %w",
					err,
				)
			}
		}
	}
	return nil
}

func (repository *RatingLedgerRepository) validateApply(
	ctx context.Context,
	command entity.ApplyRatingRun,
) error {
	if err := repository.validateTransaction(ctx); err != nil {
		return err
	}
	snapshot := command.Snapshot
	if command.RunID == uuid.Nil || command.AppliedAt.IsZero() ||
		snapshot.ContestID == uuid.Nil ||
		snapshot.RatingSequenceSlotID == uuid.Nil ||
		snapshot.RatingSequence < 1 || snapshot.RunVersion < 1 ||
		snapshot.StandingsProjectionVersion < 1 ||
		snapshot.AlgorithmVersion != canonicalRatingAlgorithmVersion ||
		snapshot.ProjectionRevision < 1 ||
		snapshot.ProjectionGenerationID == uuid.Nil ||
		snapshot.RunChainID == uuid.Nil ||
		len(snapshot.ProjectionChecksum) != sha256.Size ||
		len(snapshot.RunChainChecksum) != sha256.Size ||
		len(command.InputChecksum) != sha256.Size ||
		len(command.Results) == 0 ||
		len(command.Results) != len(snapshot.Participants) {
		return ErrInvalidRatingLedger
	}
	participants := make(
		map[uuid.UUID]entity.RatingParticipantSnapshot,
		len(snapshot.Participants),
	)
	for _, participant := range snapshot.Participants {
		if participant.UserID == uuid.Nil ||
			participant.PreviousRating < 0 ||
			participant.PreviousRatedContests < 0 {
			return ErrInvalidRatingLedger
		}
		participants[participant.UserID] = participant
	}
	seen := make(map[uuid.UUID]struct{}, len(command.Results))
	for _, result := range command.Results {
		participant, exists := participants[result.UserID]
		if !exists || result.UserID == uuid.Nil || result.Rank < 1 ||
			result.ActualRank <= 0 || result.ExpectedRank <= 0 ||
			result.NewRating < 0 ||
			result.NewRating != max(
				0,
				result.PreviousRating+result.Delta,
			) ||
			result.PreviousRating != participant.PreviousRating ||
			result.PreviousRatedContests !=
				participant.PreviousRatedContests {
			return ErrInvalidRatingLedger
		}
		if _, duplicate := seen[result.UserID]; duplicate {
			return ErrInvalidRatingLedger
		}
		seen[result.UserID] = struct{}{}
	}
	return nil
}

func (repository *RatingLedgerRepository) findActiveRun(
	ctx context.Context,
	client *generate.Client,
	chainID uuid.UUID,
	sequence int64,
) (*entity.RatingRun, error) {
	entry, err := client.RatingRunChainEntry.Query().
		Where(
			ratingrunchainentry.ChainIDEQ(chainID),
			ratingrunchainentry.RatingSequenceEQ(sequence),
		).
		Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrRatingNotAvailable
		}
		return nil, fmt.Errorf("resolve active rating run: %w", err)
	}
	return repository.loadRunByID(ctx, client, entry.RatingRunID)
}

func (repository *RatingLedgerRepository) loadRunByID(
	ctx context.Context,
	client *generate.Client,
	runID uuid.UUID,
) (*entity.RatingRun, error) {
	runRecord, err := client.ContestRatingRun.Query().
		Where(contestratingrun.IDEQ(runID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("read canonical rating run: %w", err)
	}
	resultRecords, err := client.ContestRatingResult.Query().
		Where(contestratingresult.RatingRunIDEQ(runID)).
		Order(
			generate.Asc(contestratingresult.FieldActualRank),
			generate.Asc(contestratingresult.FieldUserID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("read canonical rating results: %w", err)
	}
	userIDs := make([]uuid.UUID, len(resultRecords))
	for index, result := range resultRecords {
		userIDs[index] = result.UserID
	}
	usernames := make(map[uuid.UUID]string, len(userIDs))
	if len(userIDs) > 0 {
		users, userErr := client.User.Query().
			Where(user.IDIn(userIDs...)).
			All(ctx)
		if userErr != nil {
			return nil, fmt.Errorf(
				"load canonical rating result users: %w",
				userErr,
			)
		}
		for _, record := range users {
			usernames[record.ID] = record.Username
		}
	}
	results := make([]entity.RatingRunResult, len(resultRecords))
	for index, record := range resultRecords {
		results[index] = entity.RatingRunResult{
			UserID:                record.UserID,
			Username:              usernames[record.UserID],
			PreviousRating:        record.PriorRating,
			PreviousRatedContests: record.PriorRatedContestCount,
			Rank:                  record.RankPosition,
			ActualRank:            record.ActualRank,
			ExpectedRank:          record.ExpectedRank,
			Delta:                 record.Delta,
			NewRating:             record.NewRating,
		}
	}
	return &entity.RatingRun{
		ID:                         runRecord.ID,
		ContestID:                  runRecord.ContestID,
		RatingSequence:             runRecord.RatingSequence,
		RatingSequenceSlotID:       runRecord.RatingSequenceSlotID,
		RunVersion:                 runRecord.RunVersion,
		StandingsProjectionVersion: runRecord.StandingsProjectionVersion,
		AlgorithmVersion:           runRecord.AlgorithmVersion,
		Status:                     string(runRecord.Status),
		ParticipantCount:           runRecord.ParticipantCount,
		InputChecksum:              bytes.Clone(runRecord.InputChecksum),
		CreatedAt:                  runRecord.CreatedAt.UTC(),
		AppliedAt:                  utcRatingTimePointer(runRecord.AppliedAt),
		Results:                    results,
	}, nil
}

func nextProjectionChecksum(
	previous []byte,
	sequence int64,
	results []entity.RatingRunResult,
) []byte {
	ordered := append([]entity.RatingRunResult(nil), results...)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].UserID.String() <
			ordered[right].UserID.String()
	})
	hash := sha256.New()
	_, _ = hash.Write([]byte("judgify/rating-projection-step/v1\n"))
	_, _ = hash.Write(previous)
	writeRatingInt64(hash, sequence)
	for _, result := range ordered {
		_, _ = hash.Write(result.UserID[:])
		writeRatingInt64(hash, int64(result.NewRating))
		writeRatingInt64(
			hash,
			int64(result.PreviousRatedContests+1),
		)
	}
	return hash.Sum(nil)
}

func nextChainChecksum(
	previous []byte,
	sequence int64,
	runID uuid.UUID,
) []byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("judgify/rating-run-chain-step/v1\n"))
	_, _ = hash.Write(previous)
	writeRatingInt64(hash, sequence)
	_, _ = hash.Write(runID[:])
	return hash.Sum(nil)
}

type ratingHashWriter interface {
	Write([]byte) (int, error)
}

func writeRatingInt64(writer ratingHashWriter, value int64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	_, _ = writer.Write(encoded[:])
}

func utcRatingTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

var _ ports.RatingLedgerRepository = (*RatingLedgerRepository)(nil)
