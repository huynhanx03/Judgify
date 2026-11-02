package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestratingrun"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestuserrating"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingprojectiongeneration"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingrunchain"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingrunchainentry"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingsequenceslot"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratingstate"
	"github.com/huynhanx03/judgify/internal/ports"
)

// PrepareRerating captures the complete active rating fence. It performs no
// shadow writes, which keeps an idempotent HTTP retry cheap and makes the
// operation request the only durable description needed by a restarted worker.
func (repository *RatingLedgerRepository) PrepareRerating(
	ctx context.Context,
	operationID uuid.UUID,
	triggerContestID uuid.UUID,
	targetProjectionGenerationID uuid.UUID,
	targetRunChainID uuid.UUID,
	preparedAt time.Time,
) (*entity.RatingReratingPlan, error) {
	if err := repository.validateTransaction(ctx); err != nil ||
		operationID == uuid.Nil || triggerContestID == uuid.Nil ||
		targetProjectionGenerationID == uuid.Nil ||
		targetRunChainID == uuid.Nil ||
		targetProjectionGenerationID == targetRunChainID ||
		preparedAt.IsZero() {
		if err != nil {
			return nil, err
		}
		return nil, invalidRatingReratingLedger()
	}
	client := repository.client.DB(ctx)
	state, generation, chain, err := repository.lockOrInitializeState(
		ctx,
		client,
		preparedAt.UTC(),
	)
	if err != nil {
		return nil, err
	}
	if state.AppliedSequence < 1 ||
		state.AppliedSequence >
			contract.MaximumRatingReratingSequences ||
		targetProjectionGenerationID == generation.ID ||
		targetRunChainID == chain.ID {
		return nil, ports.ErrRatingNotAvailable
	}
	slot, err := client.RatingSequenceSlot.Query().
		Where(ratingsequenceslot.ContestIDEQ(triggerContestID)).
		Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrRatingNotAvailable
		}
		return nil, fmt.Errorf("resolve historical rerating start: %w", err)
	}
	if slot.RatingSequence < 1 ||
		slot.RatingSequence > state.AppliedSequence {
		return nil, ports.ErrRatingNotAvailable
	}
	activeRun, err := repository.findActiveRun(
		ctx,
		client,
		chain.ID,
		slot.RatingSequence,
	)
	if err != nil {
		return nil, err
	}
	if activeRun.AlgorithmVersion != canonicalRatingAlgorithmVersion ||
		activeRun.RatingSequence != slot.RatingSequence {
		return nil, errors.Join(
			ports.ErrRatingReratingInvariant,
			ErrRatingLedgerInvariant,
		)
	}
	initialProjectionChecksum := emptyRatingProjectionChecksum()
	initialRunChainChecksum := emptyRatingRunChainChecksum()
	return &entity.RatingReratingPlan{
		OperationID:                  operationID,
		TriggerContestID:             triggerContestID,
		FromSequence:                 slot.RatingSequence,
		ThroughSequence:              state.AppliedSequence,
		SourceProjectionRevision:     state.ProjectionRevision,
		SourceProjectionGenerationID: generation.ID,
		SourceRunChainID:             chain.ID,
		SourceProjectionChecksum:     cloneOptionalChecksum(generation.Checksum),
		SourceRunChainChecksum:       cloneOptionalChecksum(chain.Checksum),
		TargetProjectionGenerationID: targetProjectionGenerationID,
		TargetRunChainID:             targetRunChainID,
		AlgorithmVersion:             canonicalRatingAlgorithmVersion,
		InitialProjectionChecksum:    initialProjectionChecksum[:],
		InitialRunChainChecksum:      initialRunChainChecksum[:],
	}, nil
}

func (repository *RatingLedgerRepository) EnsureReratingTargets(
	ctx context.Context,
	plan entity.RatingReratingPlan,
	createdAt time.Time,
) error {
	if err := repository.validateReratingPlan(ctx, plan); err != nil ||
		createdAt.IsZero() {
		if err != nil {
			return err
		}
		return invalidRatingReratingLedger()
	}
	client := repository.client.DB(ctx)
	generation, err := client.RatingProjectionGeneration.Query().
		Where(
			ratingprojectiongeneration.IDEQ(
				plan.TargetProjectionGenerationID,
			),
		).
		Only(ctx)
	if generate.IsNotFound(err) {
		generation, err = client.RatingProjectionGeneration.Create().
			SetID(plan.TargetProjectionGenerationID).
			SetStatus(ratingprojectiongeneration.StatusBuilding).
			SetBasedOnProjectionRevision(
				plan.SourceProjectionRevision,
			).
			SetThroughSequence(0).
			SetCreatedAt(createdAt.UTC()).
			SetUpdatedAt(createdAt.UTC()).
			Save(ctx)
	}
	if err != nil {
		return fmt.Errorf("ensure shadow rating projection: %w", err)
	}
	if generation.Status != ratingprojectiongeneration.StatusBuilding ||
		generation.BasedOnProjectionRevision !=
			plan.SourceProjectionRevision ||
		generation.ThroughSequence != 0 ||
		len(optionalChecksumValue(generation.Checksum)) != 0 ||
		generation.VerifiedAt != nil ||
		generation.ActivatedAt != nil {
		return ports.ErrRatingReratingConflict
	}

	chain, err := client.RatingRunChain.Query().
		Where(ratingrunchain.IDEQ(plan.TargetRunChainID)).
		Only(ctx)
	if generate.IsNotFound(err) {
		chain, err = client.RatingRunChain.Create().
			SetID(plan.TargetRunChainID).
			SetStatus(ratingrunchain.StatusBuilding).
			SetBasedOnChainID(plan.SourceRunChainID).
			SetThroughSequence(0).
			SetCreatedAt(createdAt.UTC()).
			SetUpdatedAt(createdAt.UTC()).
			Save(ctx)
	}
	if err != nil {
		return fmt.Errorf("ensure shadow rating run chain: %w", err)
	}
	if chain.Status != ratingrunchain.StatusBuilding ||
		chain.BasedOnChainID == nil ||
		*chain.BasedOnChainID != plan.SourceRunChainID ||
		chain.ThroughSequence != 0 ||
		len(optionalChecksumValue(chain.Checksum)) != 0 ||
		chain.VerifiedAt != nil ||
		chain.ActivatedAt != nil {
		return ports.ErrRatingReratingConflict
	}
	return nil
}

func (repository *RatingLedgerRepository) ReplayReratingPrefix(
	ctx context.Context,
	plan entity.RatingReratingPlan,
	sequence int64,
	previousProjectionChecksum []byte,
	previousRunChainChecksum []byte,
	appliedAt time.Time,
) (*entity.RatingReratingStepResult, error) {
	if err := repository.validateReratingStep(
		ctx,
		plan,
		sequence,
		previousProjectionChecksum,
		previousRunChainChecksum,
		appliedAt,
	); err != nil ||
		sequence >= plan.FromSequence {
		if err != nil {
			return nil, err
		}
		return nil, invalidRatingReratingLedger()
	}
	client := repository.client.DB(ctx)
	entry, err := client.RatingRunChainEntry.Query().
		Where(
			ratingrunchainentry.ChainIDEQ(plan.SourceRunChainID),
			ratingrunchainentry.RatingSequenceEQ(sequence),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("read source rating prefix: %w", err)
	}
	run, err := repository.loadRunByID(ctx, client, entry.RatingRunID)
	if err != nil {
		return nil, err
	}
	if run.RatingSequence != sequence ||
		run.AlgorithmVersion != plan.AlgorithmVersion ||
		(run.Status != string(contestratingrun.StatusApplied) &&
			run.Status != string(contestratingrun.StatusSkipped)) {
		return nil, errors.Join(
			ports.ErrRatingReratingInvariant,
			ErrRatingLedgerInvariant,
		)
	}
	if len(run.Results) > 0 {
		if err := repository.advanceProjectionRatings(
			ctx,
			client,
			plan.TargetProjectionGenerationID,
			sequence,
			run.Results,
			appliedAt,
		); err != nil {
			return nil, err
		}
	}
	if _, err := client.RatingRunChainEntry.Create().
		SetChainID(plan.TargetRunChainID).
		SetRatingSequence(sequence).
		SetRatingRunID(run.ID).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("copy source rating prefix entry: %w", err)
	}
	return &entity.RatingReratingStepResult{
		RatingRunID: run.ID,
		ProjectionChecksum: nextProjectionChecksum(
			previousProjectionChecksum,
			sequence,
			run.Results,
		),
		RunChainChecksum: nextChainChecksum(
			previousRunChainChecksum,
			sequence,
			run.ID,
		),
	}, nil
}

func (repository *RatingLedgerRepository) PrepareReratingSequence(
	ctx context.Context,
	plan entity.RatingReratingPlan,
	sequence int64,
	preparedAt time.Time,
) (*entity.RatingReratingSequenceSnapshot, error) {
	if err := repository.validateReratingPlan(ctx, plan); err != nil ||
		sequence < plan.FromSequence ||
		sequence > plan.ThroughSequence || preparedAt.IsZero() {
		if err != nil {
			return nil, err
		}
		return nil, invalidRatingReratingLedger()
	}
	client := repository.client.DB(ctx)
	slot, err := client.RatingSequenceSlot.Query().
		Where(ratingsequenceslot.RatingSequenceEQ(sequence)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve rerating sequence slot: %w", err)
	}
	contestRecord, err := client.Contest.Query().
		Where(
			contest.IDEQ(slot.ContestID),
			contest.StatusEQ(contest.StatusEnded),
			contest.RatedEQ(true),
			contest.RatingCompletedAtNotNil(),
		).
		Modify(func(selector *entsql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock rerated contest: %w", err)
	}
	if contestRecord.RatingAlgorithmVersion != plan.AlgorithmVersion ||
		contestRecord.StandingsProjectionVersion < 1 {
		return nil, ports.ErrRatingReratingConflict
	}
	if err := repository.ensureScoringFinal(
		ctx,
		client,
		contestRecord.ID,
	); err != nil {
		return nil, err
	}
	participants, err := repository.loadParticipants(
		ctx,
		client,
		contestRecord.ID,
		plan.TargetProjectionGenerationID,
		false,
	)
	if err != nil {
		return nil, err
	}
	runVersion, err := repository.nextRunVersion(
		ctx,
		client,
		contestRecord.ID,
	)
	if err != nil {
		return nil, err
	}
	return &entity.RatingReratingSequenceSnapshot{
		ContestID:                  contestRecord.ID,
		RatingSequence:             sequence,
		RatingSequenceSlotID:       slot.ID,
		RunVersion:                 runVersion,
		StandingsProjectionVersion: contestRecord.StandingsProjectionVersion,
		AlgorithmVersion:           contestRecord.RatingAlgorithmVersion,
		Participants:               participants,
	}, nil
}

func (repository *RatingLedgerRepository) ApplyReratingSequence(
	ctx context.Context,
	command entity.ApplyRatingReratingSequence,
) (*entity.RatingReratingStepResult, error) {
	if err := repository.validateReratingApply(ctx, command); err != nil {
		return nil, err
	}
	client := repository.client.DB(ctx)
	snapshot := command.Snapshot
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
		SetActorID(command.Plan.ActorUserID).
		SetReason(command.Plan.Reason).
		SetInputChecksum(bytes.Clone(command.InputChecksum)).
		SetParticipantCount(len(command.Results)).
		SetAppliedAt(command.AppliedAt.UTC()).
		SetCreatedAt(command.AppliedAt.UTC()).
		SetUpdatedAt(command.AppliedAt.UTC()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shadow rating run: %w", err)
	}
	applyCommand := entity.ApplyRatingRun{
		Snapshot: entity.RatingSettlementSnapshot{
			ContestID:                  snapshot.ContestID,
			RatingSequence:             snapshot.RatingSequence,
			RatingSequenceSlotID:       snapshot.RatingSequenceSlotID,
			RunVersion:                 snapshot.RunVersion,
			StandingsProjectionVersion: snapshot.StandingsProjectionVersion,
			AlgorithmVersion:           snapshot.AlgorithmVersion,
			ProjectionRevision:         command.Plan.SourceProjectionRevision,
			ProjectionGenerationID:     command.Plan.TargetProjectionGenerationID,
			RunChainID:                 command.Plan.TargetRunChainID,
			ProjectionChecksum:         command.PreviousProjectionChecksum,
			RunChainChecksum:           command.PreviousRunChainChecksum,
			Participants:               snapshot.Participants,
		},
		RunID:         command.RunID,
		InputChecksum: command.InputChecksum,
		Results:       command.Results,
		AppliedAt:     command.AppliedAt,
	}
	if err := repository.createRunResults(
		ctx,
		client,
		runRecord.ID,
		applyCommand,
	); err != nil {
		return nil, err
	}
	if err := repository.advanceProjectionRatings(
		ctx,
		client,
		command.Plan.TargetProjectionGenerationID,
		snapshot.RatingSequence,
		command.Results,
		command.AppliedAt,
	); err != nil {
		return nil, err
	}
	if _, err := client.RatingRunChainEntry.Create().
		SetChainID(command.Plan.TargetRunChainID).
		SetRatingSequence(snapshot.RatingSequence).
		SetRatingRunID(runRecord.ID).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("append shadow rating run chain: %w", err)
	}
	return &entity.RatingReratingStepResult{
		RatingRunID: runRecord.ID,
		ProjectionChecksum: nextProjectionChecksum(
			command.PreviousProjectionChecksum,
			snapshot.RatingSequence,
			command.Results,
		),
		RunChainChecksum: nextChainChecksum(
			command.PreviousRunChainChecksum,
			snapshot.RatingSequence,
			runRecord.ID,
		),
	}, nil
}

func (repository *RatingLedgerRepository) ActivateRerating(
	ctx context.Context,
	command entity.ActivateRatingRerating,
) error {
	if err := repository.validateReratingStep(
		ctx,
		command.Plan,
		command.Plan.ThroughSequence,
		command.ProjectionChecksum,
		command.RunChainChecksum,
		command.ActivatedAt,
	); err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	state, err := client.RatingState.Query().
		Where(ratingstate.IDEQ(canonicalRatingStateID)).
		Modify(func(selector *entsql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("lock rating activation fence: %w", err)
	}
	plan := command.Plan
	if state.ProjectionRevision != plan.SourceProjectionRevision ||
		state.AppliedSequence != plan.ThroughSequence ||
		state.ActiveProjectionGenerationID !=
			plan.SourceProjectionGenerationID ||
		state.ActiveRunChainID != plan.SourceRunChainID {
		return ports.ErrRatingReratingConflict
	}
	sourceProjection, err := client.RatingProjectionGeneration.Query().
		Where(
			ratingprojectiongeneration.IDEQ(
				plan.SourceProjectionGenerationID,
			),
			ratingprojectiongeneration.StatusEQ(
				ratingprojectiongeneration.StatusActive,
			),
		).
		Only(ctx)
	if err != nil ||
		sourceProjection.ThroughSequence != plan.ThroughSequence ||
		!bytes.Equal(
			optionalChecksumValue(sourceProjection.Checksum),
			plan.SourceProjectionChecksum,
		) {
		return ports.ErrRatingReratingConflict
	}
	sourceChain, err := client.RatingRunChain.Query().
		Where(
			ratingrunchain.IDEQ(plan.SourceRunChainID),
			ratingrunchain.StatusEQ(ratingrunchain.StatusActive),
		).
		Only(ctx)
	if err != nil ||
		sourceChain.ThroughSequence != plan.ThroughSequence ||
		!bytes.Equal(
			optionalChecksumValue(sourceChain.Checksum),
			plan.SourceRunChainChecksum,
		) {
		return ports.ErrRatingReratingConflict
	}
	if err := repository.verifyReratingTarget(ctx, client, command); err != nil {
		return err
	}
	now := command.ActivatedAt.UTC()
	if _, err := client.RatingProjectionGeneration.UpdateOneID(
		plan.TargetProjectionGenerationID,
	).
		Where(
			ratingprojectiongeneration.StatusEQ(
				ratingprojectiongeneration.StatusBuilding,
			),
			ratingprojectiongeneration.ThroughSequenceEQ(0),
		).
		SetStatus(ratingprojectiongeneration.StatusActive).
		SetThroughSequence(plan.ThroughSequence).
		SetChecksum(bytes.Clone(command.ProjectionChecksum)).
		SetVerifiedAt(now).
		SetActivatedAt(now).
		SetUpdatedAt(now).
		Save(ctx); err != nil {
		return fmt.Errorf("activate shadow rating projection: %w", err)
	}
	if _, err := client.RatingRunChain.UpdateOneID(
		plan.TargetRunChainID,
	).
		Where(
			ratingrunchain.StatusEQ(ratingrunchain.StatusBuilding),
			ratingrunchain.ThroughSequenceEQ(0),
		).
		SetStatus(ratingrunchain.StatusActive).
		SetThroughSequence(plan.ThroughSequence).
		SetChecksum(bytes.Clone(command.RunChainChecksum)).
		SetVerifiedAt(now).
		SetActivatedAt(now).
		SetUpdatedAt(now).
		Save(ctx); err != nil {
		return fmt.Errorf("activate shadow rating run chain: %w", err)
	}
	if _, err := client.RatingProjectionGeneration.UpdateOneID(
		plan.SourceProjectionGenerationID,
	).
		Where(
			ratingprojectiongeneration.StatusEQ(
				ratingprojectiongeneration.StatusActive,
			),
		).
		SetStatus(ratingprojectiongeneration.StatusRetired).
		SetUpdatedAt(now).
		Save(ctx); err != nil {
		return fmt.Errorf("retire source rating projection: %w", err)
	}
	if _, err := client.RatingRunChain.UpdateOneID(plan.SourceRunChainID).
		Where(
			ratingrunchain.StatusEQ(ratingrunchain.StatusActive),
		).
		SetStatus(ratingrunchain.StatusRetired).
		SetUpdatedAt(now).
		Save(ctx); err != nil {
		return fmt.Errorf("retire source rating run chain: %w", err)
	}
	if err := repository.reconcileActivatedUserStats(
		ctx,
		client,
		plan,
		now,
	); err != nil {
		return err
	}
	if _, err := client.RatingState.UpdateOneID(canonicalRatingStateID).
		Where(
			ratingstate.ProjectionRevisionEQ(
				plan.SourceProjectionRevision,
			),
			ratingstate.AppliedSequenceEQ(plan.ThroughSequence),
			ratingstate.ActiveProjectionGenerationIDEQ(
				plan.SourceProjectionGenerationID,
			),
			ratingstate.ActiveRunChainIDEQ(plan.SourceRunChainID),
		).
		SetActiveProjectionGenerationID(
			plan.TargetProjectionGenerationID,
		).
		SetActiveRunChainID(plan.TargetRunChainID).
		SetProjectionRevision(plan.SourceProjectionRevision + 1).
		SetUpdatedAt(now).
		Save(ctx); err != nil {
		return fmt.Errorf("switch active rating pointers: %w", err)
	}
	return nil
}

func (repository *RatingLedgerRepository) verifyReratingTarget(
	ctx context.Context,
	client *generate.Client,
	command entity.ActivateRatingRerating,
) error {
	plan := command.Plan
	generation, err := client.RatingProjectionGeneration.Query().
		Where(
			ratingprojectiongeneration.IDEQ(
				plan.TargetProjectionGenerationID,
			),
			ratingprojectiongeneration.StatusEQ(
				ratingprojectiongeneration.StatusBuilding,
			),
			ratingprojectiongeneration.BasedOnProjectionRevisionEQ(
				plan.SourceProjectionRevision,
			),
		).
		Only(ctx)
	if err != nil || generation.ThroughSequence != 0 ||
		len(optionalChecksumValue(generation.Checksum)) != 0 {
		return ports.ErrRatingReratingConflict
	}
	chain, err := client.RatingRunChain.Query().
		Where(
			ratingrunchain.IDEQ(plan.TargetRunChainID),
			ratingrunchain.StatusEQ(ratingrunchain.StatusBuilding),
			ratingrunchain.BasedOnChainIDEQ(plan.SourceRunChainID),
		).
		Only(ctx)
	if err != nil || chain.ThroughSequence != 0 ||
		len(optionalChecksumValue(chain.Checksum)) != 0 {
		return ports.ErrRatingReratingConflict
	}
	entryCount, err := client.RatingRunChainEntry.Query().
		Where(ratingrunchainentry.ChainIDEQ(plan.TargetRunChainID)).
		Count(ctx)
	if err != nil || int64(entryCount) != plan.ThroughSequence {
		return ports.ErrRatingReratingConflict
	}
	hasOutOfRangeEntry, err := client.RatingRunChainEntry.Query().
		Where(
			ratingrunchainentry.ChainIDEQ(plan.TargetRunChainID),
			ratingrunchainentry.RatingSequenceGT(
				plan.ThroughSequence,
			),
		).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("verify shadow rating sequence range: %w", err)
	}
	if hasOutOfRangeEntry {
		return ports.ErrRatingReratingConflict
	}
	var invalid int64
	rows, err := client.Contest.QueryContext(ctx, `
		SELECT COUNT(*)
		FROM rating_run_chain_entries AS target_entry
		JOIN contest_rating_runs AS target_run
		  ON target_run.id = target_entry.rating_run_id
		JOIN rating_sequence_slots AS slot
		  ON slot.id = target_run.rating_sequence_slot_id
		JOIN contests AS contest
		  ON contest.id = target_run.contest_id
		WHERE target_entry.chain_id = $1
		  AND target_entry.rating_sequence >= $2
		  AND (
		    target_run.rating_sequence <> target_entry.rating_sequence OR
		    slot.rating_sequence <> target_entry.rating_sequence OR
		    slot.contest_id <> target_run.contest_id OR
		    target_run.status <> 'applied' OR
		    target_run.algorithm_version <> $3 OR
		    target_run.standings_projection_version <>
		      contest.standings_projection_version OR
		    contest.status <> 'ended' OR
		    contest.rated IS NOT TRUE OR
		    contest.rating_completed_at IS NULL
		  )`,
		plan.TargetRunChainID,
		plan.FromSequence,
		plan.AlgorithmVersion,
	)
	if err != nil {
		return fmt.Errorf("verify rerated suffix inputs: %w", err)
	}
	if rows.Next() {
		err = rows.Scan(&invalid)
	} else if rows.Err() != nil {
		err = rows.Err()
	} else {
		err = sql.ErrNoRows
	}
	err = errors.Join(err, rows.Close())
	if err != nil {
		return fmt.Errorf("scan rerated suffix verification: %w", err)
	}
	if invalid != 0 {
		return ports.ErrRatingReratingConflict
	}
	if plan.FromSequence > 1 {
		rows, err = client.Contest.QueryContext(ctx, `
			SELECT COUNT(*)
			FROM rating_run_chain_entries AS source_entry
			LEFT JOIN rating_run_chain_entries AS target_entry
			  ON target_entry.chain_id = $1
			 AND target_entry.rating_sequence =
			     source_entry.rating_sequence
			WHERE source_entry.chain_id = $2
			  AND source_entry.rating_sequence < $3
			  AND target_entry.rating_run_id IS DISTINCT FROM
			      source_entry.rating_run_id`,
			plan.TargetRunChainID,
			plan.SourceRunChainID,
			plan.FromSequence,
		)
		if err != nil {
			return fmt.Errorf("verify replayed rating prefix: %w", err)
		}
		if rows.Next() {
			err = rows.Scan(&invalid)
		} else if rows.Err() != nil {
			err = rows.Err()
		} else {
			err = sql.ErrNoRows
		}
		err = errors.Join(err, rows.Close())
		if err != nil {
			return fmt.Errorf("scan replayed prefix verification: %w", err)
		}
		if invalid != 0 {
			return ports.ErrRatingReratingConflict
		}
	}
	invalidRatings, err := client.ContestUserRating.Query().
		Where(
			contestuserrating.ProjectionGenerationIDEQ(
				plan.TargetProjectionGenerationID,
			),
			contestuserrating.Or(
				contestuserrating.RatingLT(0),
				contestuserrating.RatedContestCountLT(1),
				contestuserrating.AppliedSequenceGT(
					plan.ThroughSequence,
				),
			),
		).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("verify shadow rating rows: %w", err)
	}
	if invalidRatings ||
		len(command.ProjectionChecksum) != sha256.Size ||
		len(command.RunChainChecksum) != sha256.Size {
		return ports.ErrRatingReratingConflict
	}
	rows, err = client.Contest.QueryContext(ctx, `
		WITH selected_results AS (
			SELECT
				result.user_id,
				result.new_rating,
				entry.rating_sequence,
				COUNT(*) OVER (
					PARTITION BY result.user_id
				) AS rated_contest_count,
				ROW_NUMBER() OVER (
					PARTITION BY result.user_id
					ORDER BY entry.rating_sequence DESC
				) AS latest
			FROM rating_run_chain_entries AS entry
			JOIN contest_rating_results AS result
			  ON result.rating_run_id = entry.rating_run_id
			WHERE entry.chain_id = $1
		),
		latest AS (
			SELECT
				user_id,
				new_rating,
				rating_sequence,
				rated_contest_count
			FROM selected_results
			WHERE latest = 1
		)
		SELECT COUNT(*)
		FROM latest
		FULL JOIN contest_user_ratings AS current
		  ON current.projection_generation_id = $2
		 AND current.user_id = latest.user_id
		WHERE latest.user_id IS NULL
		   OR current.user_id IS NULL
		   OR current.rating <> latest.new_rating
		   OR current.rated_contest_count <>
		      latest.rated_contest_count
		   OR current.applied_sequence <> latest.rating_sequence
		   OR current.revision <> latest.rated_contest_count`,
		plan.TargetRunChainID,
		plan.TargetProjectionGenerationID,
	)
	if err != nil {
		return fmt.Errorf("verify shadow rating projection: %w", err)
	}
	if rows.Next() {
		err = rows.Scan(&invalid)
	} else if rows.Err() != nil {
		err = rows.Err()
	} else {
		err = sql.ErrNoRows
	}
	err = errors.Join(err, rows.Close())
	if err != nil {
		return fmt.Errorf("scan shadow rating projection: %w", err)
	}
	if invalid != 0 {
		return ports.ErrRatingReratingConflict
	}
	return nil
}

func (repository *RatingLedgerRepository) reconcileActivatedUserStats(
	ctx context.Context,
	client *generate.Client,
	plan entity.RatingReratingPlan,
	now time.Time,
) error {
	result, err := client.UserStats.ExecContext(ctx, `
		WITH affected AS (
			SELECT user_id
			FROM contest_user_ratings
			WHERE projection_generation_id = $1
			UNION
			SELECT user_id
			FROM contest_user_ratings
			WHERE projection_generation_id = $2
		),
		target AS (
			SELECT user_id, rating
			FROM contest_user_ratings
			WHERE projection_generation_id = $2
		)
		UPDATE user_stats AS stats
		SET rating = COALESCE(target.rating, 1200),
		    updated_at = $3
		FROM affected
		LEFT JOIN target ON target.user_id = affected.user_id
		WHERE stats.user_id = affected.user_id
		  AND stats.deleted_at IS NULL`,
		plan.SourceProjectionGenerationID,
		plan.TargetProjectionGenerationID,
		now,
	)
	if err != nil {
		return fmt.Errorf("reconcile activated rating profiles: %w", err)
	}
	affectedCount, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read activated profile count: %w", err)
	}
	var totalAffected, existingProfiles int64
	rows, err := client.UserStats.QueryContext(ctx, `
		SELECT
			COUNT(*) AS total_affected,
			COUNT(stats.user_id) AS existing_profiles
		FROM (
			SELECT user_id
			FROM contest_user_ratings
			WHERE projection_generation_id = $1
			UNION
			SELECT user_id
			FROM contest_user_ratings
			WHERE projection_generation_id = $2
		) AS affected
		LEFT JOIN user_stats AS stats
		  ON stats.user_id = affected.user_id
		 AND stats.deleted_at IS NULL`,
		plan.SourceProjectionGenerationID,
		plan.TargetProjectionGenerationID,
	)
	if err != nil {
		return fmt.Errorf("verify activated profile population: %w", err)
	}
	if rows.Next() {
		err = rows.Scan(&totalAffected, &existingProfiles)
	} else if rows.Err() != nil {
		err = rows.Err()
	} else {
		err = sql.ErrNoRows
	}
	err = errors.Join(err, rows.Close())
	if err != nil {
		return fmt.Errorf("scan activated profile population: %w", err)
	}
	if affectedCount != totalAffected ||
		existingProfiles != totalAffected {
		return errors.Join(
			ports.ErrRatingReratingInvariant,
			ErrRatingLedgerInvariant,
		)
	}
	return nil
}

func (repository *RatingLedgerRepository) validateReratingPlan(
	ctx context.Context,
	plan entity.RatingReratingPlan,
) error {
	if err := repository.validateTransaction(ctx); err != nil {
		return err
	}
	if plan.OperationID == uuid.Nil ||
		plan.TriggerContestID == uuid.Nil ||
		plan.ActorUserID == uuid.Nil ||
		strings.TrimSpace(plan.Reason) == "" ||
		len(plan.Reason) > 1024 ||
		plan.FromSequence < 1 ||
		plan.ThroughSequence < plan.FromSequence ||
		plan.ThroughSequence >
			contract.MaximumRatingReratingSequences ||
		plan.SourceProjectionRevision < 1 ||
		plan.SourceProjectionGenerationID == uuid.Nil ||
		plan.SourceRunChainID == uuid.Nil ||
		plan.TargetProjectionGenerationID == uuid.Nil ||
		plan.TargetRunChainID == uuid.Nil ||
		plan.SourceProjectionGenerationID ==
			plan.TargetProjectionGenerationID ||
		plan.SourceRunChainID == plan.TargetRunChainID ||
		plan.TargetProjectionGenerationID == plan.TargetRunChainID ||
		plan.AlgorithmVersion != canonicalRatingAlgorithmVersion ||
		len(plan.SourceProjectionChecksum) != sha256.Size ||
		len(plan.SourceRunChainChecksum) != sha256.Size ||
		len(plan.InitialProjectionChecksum) != sha256.Size ||
		len(plan.InitialRunChainChecksum) != sha256.Size {
		return invalidRatingReratingLedger()
	}
	return nil
}

func (repository *RatingLedgerRepository) validateReratingStep(
	ctx context.Context,
	plan entity.RatingReratingPlan,
	sequence int64,
	previousProjectionChecksum []byte,
	previousRunChainChecksum []byte,
	appliedAt time.Time,
) error {
	if err := repository.validateReratingPlan(ctx, plan); err != nil {
		return err
	}
	if sequence < 1 || sequence > plan.ThroughSequence ||
		len(previousProjectionChecksum) != sha256.Size ||
		len(previousRunChainChecksum) != sha256.Size ||
		appliedAt.IsZero() {
		return invalidRatingReratingLedger()
	}
	return nil
}

func (repository *RatingLedgerRepository) validateReratingApply(
	ctx context.Context,
	command entity.ApplyRatingReratingSequence,
) error {
	snapshot := command.Snapshot
	if err := repository.validateReratingStep(
		ctx,
		command.Plan,
		snapshot.RatingSequence,
		command.PreviousProjectionChecksum,
		command.PreviousRunChainChecksum,
		command.AppliedAt,
	); err != nil {
		return err
	}
	if command.RunID == uuid.Nil ||
		snapshot.ContestID == uuid.Nil ||
		snapshot.RatingSequenceSlotID == uuid.Nil ||
		snapshot.RatingSequence < command.Plan.FromSequence ||
		snapshot.RunVersion < 1 ||
		snapshot.StandingsProjectionVersion < 1 ||
		snapshot.AlgorithmVersion != command.Plan.AlgorithmVersion ||
		len(command.InputChecksum) != sha256.Size ||
		len(snapshot.Participants) == 0 ||
		len(command.Results) != len(snapshot.Participants) {
		return invalidRatingReratingLedger()
	}
	participants := make(
		map[uuid.UUID]entity.RatingParticipantSnapshot,
		len(snapshot.Participants),
	)
	for _, participant := range snapshot.Participants {
		if participant.UserID == uuid.Nil ||
			participant.PreviousRating < 0 ||
			participant.PreviousRatedContests < 0 {
			return invalidRatingReratingLedger()
		}
		participants[participant.UserID] = participant
	}
	seen := make(map[uuid.UUID]struct{}, len(command.Results))
	for _, result := range command.Results {
		participant, exists := participants[result.UserID]
		if !exists || result.Rank < 1 ||
			result.ActualRank <= 0 || result.ExpectedRank <= 0 ||
			result.NewRating < 0 ||
			result.NewRating != max(
				0,
				result.PreviousRating+result.Delta,
			) ||
			result.PreviousRating != participant.PreviousRating ||
			result.PreviousRatedContests !=
				participant.PreviousRatedContests {
			return invalidRatingReratingLedger()
		}
		if _, duplicate := seen[result.UserID]; duplicate {
			return invalidRatingReratingLedger()
		}
		seen[result.UserID] = struct{}{}
	}
	return nil
}

var _ ports.RatingReratingRepository = (*RatingLedgerRepository)(nil)

func invalidRatingReratingLedger() error {
	return errors.Join(
		ports.ErrRatingReratingInvariant,
		ErrInvalidRatingLedger,
	)
}
