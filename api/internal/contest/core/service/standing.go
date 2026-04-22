package service

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/core/mapper"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/internal/contest/store"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"
)

const (
	// penaltyPerWA is the ICPC penalty in seconds for each wrong answer before AC.
	penaltyPerWA = 1200
)

type standingService struct {
	standingRepo ports.StandingRepository
	hub          *store.LeaderboardHub
}

// NewStandingService creates a new StandingService instance.
func NewStandingService(standingRepo ports.StandingRepository, hub *store.LeaderboardHub) ports.StandingService {
	return &standingService{standingRepo: standingRepo, hub: hub}
}

// GetStandings returns all standings for a contest, sorted by ICPC rules with computed ranks.
func (s *standingService) GetStandings(ctx context.Context, contestID int) ([]*dto.StandingResponse, error) {
	standings, err := s.standingRepo.FindByContest(ctx, contestID)
	if err != nil {
		return nil, err
	}

	// ICPC sort: solved_count DESC, penalty ASC
	sort.Slice(standings, func(i, j int) bool {
		if standings[i].SolvedCount != standings[j].SolvedCount {
			return standings[i].SolvedCount > standings[j].SolvedCount
		}
		return standings[i].Penalty < standings[j].Penalty
	})

	responses := make([]*dto.StandingResponse, len(standings))
	for i, st := range standings {
		rank := i + 1
		// Username left empty; will be populated later via user service.
		responses[i] = mapper.ToStandingResponse(rank, st, "")
	}

	return responses, nil
}

// UpdateFromVerdict applies ICPC scoring based on a judge verdict.
func (s *standingService) UpdateFromVerdict(ctx context.Context, contestID, userID, problemID int, accepted bool, submitTimeSec int) error {
	probKey := strconv.Itoa(problemID)

	st, err := s.standingRepo.Get(ctx, contestID, userID)
	if err != nil {
		// Standing not found, create new.
		st = &entity.ContestStanding{
			ContestID:      contestID,
			UserID:         userID,
			SolvedCount:    0,
			Penalty:        0,
			ProblemResults: make(map[string]any),
		}
	}

	if st.ProblemResults == nil {
		st.ProblemResults = make(map[string]any)
	}

	probResult, _ := st.ProblemResults[probKey].(map[string]any)
	if probResult == nil {
		probResult = map[string]any{
			"ac":       false,
			"wa_count": 0,
			"time":     0,
		}
	}

	if accepted {
		ac, _ := probResult["ac"].(bool)
		if !ac {
			probResult["ac"] = true
			probResult["time"] = submitTimeSec
			st.SolvedCount++

			waCount, _ := probResult["wa_count"].(int)
			if waCount < 0 {
				waCount = 0
			}
			st.Penalty += submitTimeSec + waCount*penaltyPerWA
		}
	} else {
		waCount, _ := probResult["wa_count"].(int)
		probResult["wa_count"] = waCount + 1
	}

	st.ProblemResults[probKey] = probResult

	if err := s.standingRepo.Upsert(ctx, st); err != nil {
		return err
	}

	logger.FromContext(ctx).Info("standing updated from verdict",
		zap.Int("contest_id", contestID),
		zap.Int("user_id", userID),
		zap.Int("problem_id", problemID),
		zap.Bool("accepted", accepted),
	)

	// Broadcast updated standings to SSE subscribers
	if s.hub != nil {
		s.broadcastStandings(ctx, contestID)
	}

	return nil
}

// broadcastStandings fetches current standings and broadcasts to SSE subscribers.
func (s *standingService) broadcastStandings(ctx context.Context, contestID int) {
	standings, err := s.GetStandings(ctx, contestID)
	if err != nil {
		return
	}
	data, err := json.Marshal(standings)
	if err != nil {
		return
	}
	s.hub.Broadcast(contestID, data)
}
