package service

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/internal/contest/constant"
	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/core/mapper"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"
)

type contestService struct {
	contestRepo ports.ContestRepository
	regRepo     ports.RegistrationRepository
}

// NewContestService creates a new ContestService instance.
func NewContestService(contestRepo ports.ContestRepository, regRepo ports.RegistrationRepository) ports.ContestService {
	return &contestService{contestRepo: contestRepo, regRepo: regRepo}
}

// Find retrieves contests with pagination, including participant count for each.
func (s *contestService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ContestResponse], error) {
	contests, err := s.contestRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if contests.Records == nil {
		return &d.Paginated[*dto.ContestResponse]{
			Records:    &[]*dto.ContestResponse{},
			Pagination: contests.Pagination,
		}, nil
	}

	entities := *contests.Records
	responses := make([]*dto.ContestResponse, len(entities))
	for i, c := range entities {
		count, _ := s.contestRepo.CountRegistrations(ctx, c.ID)
		problemIDs, _ := s.contestRepo.GetProblemIDs(ctx, c.ID)
		responses[i] = mapper.ToContestResponse(c, count, problemIDs)
	}

	return &d.Paginated[*dto.ContestResponse]{
		Records:    &responses,
		Pagination: contests.Pagination,
	}, nil
}

// Get retrieves a contest by ID with participant count, problem IDs, and registration status.
func (s *contestService) Get(ctx context.Context, id int, userID int) (*dto.ContestResponse, error) {
	c, err := s.contestRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	count, _ := s.contestRepo.CountRegistrations(ctx, id)
	problemIDs, _ := s.contestRepo.GetProblemIDs(ctx, id)
	resp := mapper.ToContestResponse(c, count, problemIDs)

	if userID > 0 {
		registered, err := s.regRepo.Exists(ctx, id, userID)
		if err == nil {
			resp.IsRegistered = &registered
		}
	}

	return resp, nil
}

// Create validates times and creates a contest, optionally attaching problems.
func (s *contestService) Create(ctx context.Context, authorID int, req *dto.CreateContestRequest) (*dto.ContestResponse, error) {
	if !req.StartTime.Before(req.EndTime) {
		return nil, apperr.New(response.CodeBadRequest, "start_time must be before end_time", nil)
	}

	e := &entity.Contest{
		Title:           req.Title,
		Description:     req.Description,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		Status:          constant.StatusDraft,
		AuthorID:        authorID,
		MaxParticipants: req.MaxParticipants,
	}

	if err := s.contestRepo.Create(ctx, e); err != nil {
		return nil, err
	}

	if len(req.ProblemIDs) > 0 {
		if err := s.contestRepo.AddProblems(ctx, e.ID, req.ProblemIDs); err != nil {
			return nil, err
		}
	}

	created, err := s.contestRepo.Get(ctx, e.ID)
	if err != nil {
		return nil, err
	}

	count, _ := s.contestRepo.CountRegistrations(ctx, e.ID)
	problemIDs, _ := s.contestRepo.GetProblemIDs(ctx, e.ID)

	logger.FromContext(ctx).Info("contest created", zap.Int("contest_id", e.ID), zap.Int("author_id", authorID))
	return mapper.ToContestResponse(created, count, problemIDs), nil
}

// Update applies non-nil fields and replaces problems if provided.
func (s *contestService) Update(ctx context.Context, id int, req *dto.UpdateContestRequest) (*dto.ContestResponse, error) {
	c, err := s.contestRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		c.Title = *req.Title
	}
	if req.Description != nil {
		c.Description = *req.Description
	}
	if req.StartTime != nil {
		c.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		c.EndTime = *req.EndTime
	}
	if req.MaxParticipants != nil {
		c.MaxParticipants = *req.MaxParticipants
	}

	if !c.StartTime.Before(c.EndTime) {
		return nil, apperr.New(response.CodeBadRequest, "start_time must be before end_time", nil)
	}

	c.ID = id
	if err := s.contestRepo.Update(ctx, c); err != nil {
		return nil, err
	}

	if req.ProblemIDs != nil {
		if err := s.contestRepo.RemoveAllProblems(ctx, id); err != nil {
			return nil, err
		}
		if len(req.ProblemIDs) > 0 {
			if err := s.contestRepo.AddProblems(ctx, id, req.ProblemIDs); err != nil {
				return nil, err
			}
		}
	}

	updated, err := s.contestRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	count, _ := s.contestRepo.CountRegistrations(ctx, id)
	problemIDs, _ := s.contestRepo.GetProblemIDs(ctx, id)

	logger.FromContext(ctx).Info("contest updated", zap.Int("contest_id", id))
	return mapper.ToContestResponse(updated, count, problemIDs), nil
}

// Delete removes a contest by ID.
func (s *contestService) Delete(ctx context.Context, id int) error {
	_, err := s.contestRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.contestRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("contest deleted", zap.Int("contest_id", id))
	return nil
}

// contestNotFound returns a not-found error for the contest object.
func contestNotFound() *apperr.AppError {
	return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjContest), nil)
}
