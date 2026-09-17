package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	// minContestsForK20 is the number of contests a user needs before K drops to 20.
	minContestsForK20 = 5
	// kFactorNew is the K-factor for users with fewer than 5 rated contests.
	kFactorNew = 40.0
	// kFactorEstablished is the K-factor for experienced users.
	kFactorEstablished = 20.0
	// eloDivisor is the Elo divisor constant.
	eloDivisor = 400.0
	// largeContestApproxThreshold switches to bucket approximation for expected rank.
	largeContestApproxThreshold = 2000
	// ratingBucketSize is the rating span of each approximation bucket.
	ratingBucketSize = 100
)

type ratingService struct {
	ledger ports.RatingLedgerRepository
	txMgr  tx.Manager
	outbox eventing.OutboxAppender
	clock  func() time.Time
}

type ratingParticipant struct {
	userID        uuid.UUID
	currentRating int
	rank          int
	actualRank    float64
	solvedCount   int
	penalty       int
	contestCount  int
}

type ratingBucket struct {
	representativeRating int
	count                int
}

// NewRatingService creates a new RatingService instance.
func NewRatingService(
	ledger ports.RatingLedgerRepository,
	txMgr tx.Manager,
	outbox eventing.OutboxAppender,
) ports.RatingService {
	return &ratingService{
		ledger: ledger,
		txMgr:  txMgr,
		outbox: outbox,
		clock:  func() time.Time { return time.Now().UTC() },
	}
}

// CalculateRating computes expected-rank rating changes for all participants of a contest.
func (s *ratingService) CalculateRating(ctx context.Context, contestID uuid.UUID) error {
	if s == nil || s.txMgr == nil || s.ledger == nil ||
		s.outbox == nil || s.clock == nil || ctx == nil ||
		contestID == uuid.Nil {
		return errors.New("rating service dependencies are required")
	}

	log := logger.FromContext(ctx)

	return s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		now := s.clock().UTC()
		if now.IsZero() {
			return errors.New("rating service clock returned zero time")
		}
		snapshot, err := s.ledger.Prepare(txCtx, contestID, now)
		if err != nil {
			return err
		}
		if snapshot.IdempotentRun != nil {
			log.Debug(
				"rating settlement replay resolved canonical run",
				zap.String("contest_id", contestID.String()),
				zap.String(
					"rating_run_id",
					snapshot.IdempotentRun.ID.String(),
				),
				zap.Int64(
					"rating_sequence",
					snapshot.IdempotentRun.RatingSequence,
				),
			)
			return nil
		}
		inputs := make([]RatingInput, len(snapshot.Participants))
		usernames := make(
			map[uuid.UUID]string,
			len(snapshot.Participants),
		)
		for index, participant := range snapshot.Participants {
			inputs[index] = RatingInput{
				UserID:                participant.UserID,
				SolvedCount:           participant.SolvedCount,
				PenaltySeconds:        participant.PenaltySeconds,
				PreviousRating:        participant.PreviousRating,
				PreviousRatedContests: participant.PreviousRatedContests,
			}
			usernames[participant.UserID] = participant.Username
		}
		deltas, err := CalculateRatingDeltas(inputs)
		if err != nil {
			return err
		}
		results := make([]entity.RatingRunResult, len(deltas))
		for index, delta := range deltas {
			results[index] = entity.RatingRunResult{
				UserID:                delta.UserID,
				Username:              usernames[delta.UserID],
				PreviousRating:        delta.PreviousRating,
				PreviousRatedContests: delta.PreviousRatedContests,
				Rank:                  delta.Rank,
				ActualRank:            delta.ActualRank,
				ExpectedRank:          delta.ExpectedRank,
				Delta:                 delta.Delta,
				NewRating:             delta.NewRating,
			}
		}
		inputChecksum := ratingSettlementInputChecksum(*snapshot)
		runID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate rating run id: %w", err)
		}
		run, err := s.ledger.Apply(txCtx, entity.ApplyRatingRun{
			Snapshot:      *snapshot,
			RunID:         runID,
			InputChecksum: inputChecksum,
			Results:       results,
			AppliedAt:     now,
		})
		if err != nil {
			return err
		}
		eventID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate rating event id: %w", err)
		}
		payload, err := json.Marshal(contract.RatingCalculatedV1{
			EventID:                    eventID,
			RatingRunID:                run.ID,
			ContestID:                  run.ContestID,
			RatingSequence:             run.RatingSequence,
			RunVersion:                 run.RunVersion,
			StandingsProjectionVersion: run.StandingsProjectionVersion,
			AlgorithmVersion:           run.AlgorithmVersion,
			ParticipantCount:           run.ParticipantCount,
			OccurredAt:                 now,
		})
		if err != nil {
			return fmt.Errorf("encode rating event: %w", err)
		}
		if err := s.outbox.Append(txCtx, eventing.Event{
			ID:               eventID,
			Type:             contract.EventRatingCalculatedV1,
			AggregateType:    contract.AggregateRatingRun,
			AggregateID:      run.ID,
			AggregateVersion: run.RunVersion,
			Payload:          payload,
			OccurredAt:       now,
		}); err != nil {
			return err
		}
		log.Info(
			"rating run applied",
			zap.String("contest_id", contestID.String()),
			zap.String("rating_run_id", run.ID.String()),
			zap.Int64("rating_sequence", run.RatingSequence),
			zap.Int("participants", run.ParticipantCount),
			zap.String("algorithm_version", run.AlgorithmVersion),
		)
		return nil
	})
}

// GetContestRatingChanges returns rating changes for a contest.
func (s *ratingService) GetContestRatingChanges(ctx context.Context, contestID uuid.UUID) ([]*contract.RatingChangeResponse, error) {
	if s == nil || s.ledger == nil || ctx == nil ||
		contestID == uuid.Nil {
		return nil, errors.New("rating service dependencies are required")
	}
	run, err := s.ledger.FindActiveByContest(ctx, contestID)
	if err != nil {
		if errors.Is(err, ports.ErrRatingNotAvailable) {
			return []*contract.RatingChangeResponse{}, nil
		}
		return nil, err
	}

	responses := make([]*contract.RatingChangeResponse, len(run.Results))
	for i, result := range run.Results {
		responses[i] = &contract.RatingChangeResponse{
			UserID:    result.UserID,
			Username:  result.Username,
			OldRating: result.PreviousRating,
			NewRating: result.NewRating,
			Rank:      result.Rank,
			Delta:     result.Delta,
		}
	}
	return responses, nil
}

func ratingSettlementInputChecksum(
	snapshot entity.RatingSettlementSnapshot,
) []byte {
	participants := append(
		[]entity.RatingParticipantSnapshot(nil),
		snapshot.Participants...,
	)
	sort.Slice(participants, func(left, right int) bool {
		return participants[left].UserID.String() <
			participants[right].UserID.String()
	})
	hash := sha256.New()
	_, _ = hash.Write(
		[]byte("judgify/rating-input/expected-rank-v1\n"),
	)
	_, _ = hash.Write(snapshot.ContestID[:])
	writeRatingAlgorithmInt64(hash, snapshot.RatingSequence)
	writeRatingAlgorithmInt64(
		hash,
		snapshot.StandingsProjectionVersion,
	)
	_, _ = hash.Write([]byte(snapshot.AlgorithmVersion))
	_, _ = hash.Write([]byte{0})
	for _, participant := range participants {
		_, _ = hash.Write(participant.UserID[:])
		writeRatingAlgorithmInt64(
			hash,
			int64(participant.SolvedCount),
		)
		writeRatingAlgorithmInt64(
			hash,
			int64(participant.PenaltySeconds),
		)
		writeRatingAlgorithmInt64(
			hash,
			int64(participant.PreviousRating),
		)
		writeRatingAlgorithmInt64(
			hash,
			int64(participant.PreviousRatedContests),
		)
	}
	return hash.Sum(nil)
}

type ratingAlgorithmHashWriter interface {
	Write([]byte) (int, error)
}

func writeRatingAlgorithmInt64(
	writer ratingAlgorithmHashWriter,
	value int64,
) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	_, _ = writer.Write(encoded[:])
}

func assignActualRanks(participants []ratingParticipant) {
	for start := 0; start < len(participants); {
		end := start
		for end+1 < len(participants) && sameContestResult(participants[start], participants[end+1]) {
			end++
		}

		actualRank := (float64(start+1) + float64(end+1)) / 2.0
		for i := start; i <= end; i++ {
			participants[i].rank = start + 1
			participants[i].actualRank = actualRank
		}

		start = end + 1
	}
}

func sameContestResult(a, b ratingParticipant) bool {
	return a.solvedCount == b.solvedCount && a.penalty == b.penalty
}

func calculateExpectedRanks(participants []ratingParticipant) []float64 {
	if shouldUseApproximation(len(participants)) {
		return calculateExpectedRanksByBuckets(participants)
	}
	return calculateExpectedRanksExact(participants)
}

func shouldUseApproximation(participantCount int) bool {
	return participantCount > largeContestApproxThreshold
}

func calculateExpectedRanksExact(participants []ratingParticipant) []float64 {
	expectedRanks := make([]float64, len(participants))
	for i := range expectedRanks {
		expectedRanks[i] = 1.0
	}

	scoreCache := make(map[int]float64, len(participants))
	for i := 0; i < len(participants); i++ {
		for j := i + 1; j < len(participants); j++ {
			diff := participants[j].currentRating - participants[i].currentRating
			iBeatsJ := expectedScoreByDiff(diff, scoreCache)
			expectedRanks[i] += 1.0 - iBeatsJ
			expectedRanks[j] += iBeatsJ
		}
	}

	return expectedRanks
}

func calculateExpectedRanksByBuckets(participants []ratingParticipant) []float64 {
	buckets := buildRatingBuckets(participants)
	expectedRanks := make([]float64, len(participants))
	scoreCache := make(map[int]float64, len(buckets)*4)

	for i, p := range participants {
		seed := 1.0
		for _, bucket := range buckets {
			diff := p.currentRating - bucket.representativeRating
			probabilityBucketBeatsParticipant := expectedScoreByDiff(diff, scoreCache)
			seed += float64(bucket.count) * probabilityBucketBeatsParticipant
		}
		// Exclude self-match from bucket aggregation.
		seed -= 0.5
		expectedRanks[i] = seed
	}
	return expectedRanks
}

func buildRatingBuckets(participants []ratingParticipant) []ratingBucket {
	bucketCounts := make(map[int]int)
	bucketRatingSums := make(map[int]int)

	for _, p := range participants {
		bucketID := p.currentRating / ratingBucketSize
		bucketCounts[bucketID]++
		bucketRatingSums[bucketID] += p.currentRating
	}

	bucketIDs := make([]int, 0, len(bucketCounts))
	for bucketID := range bucketCounts {
		bucketIDs = append(bucketIDs, bucketID)
	}
	// Deterministic order keeps calculations stable across runs.
	sort.Ints(bucketIDs)

	buckets := make([]ratingBucket, 0, len(bucketIDs))
	for _, bucketID := range bucketIDs {
		count := bucketCounts[bucketID]
		representativeRating := bucketRatingSums[bucketID] / count
		buckets = append(buckets, ratingBucket{
			representativeRating: representativeRating,
			count:                count,
		})
	}
	return buckets
}

func expectedScoreByDiff(diff int, cache map[int]float64) float64 {
	if score, ok := cache[diff]; ok {
		return score
	}
	score := 1.0 / (1.0 + math.Pow(10, float64(diff)/eloDivisor))
	cache[diff] = score
	return score
}

func ratingDelta(expectedRank, actualRank, k float64, participantCount int) int {
	if participantCount <= 1 {
		return 0
	}

	performance := (expectedRank - actualRank) / float64(participantCount-1)
	return int(math.Round(k * performance))
}
