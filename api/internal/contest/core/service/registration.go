package service

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/internal/contest/constant"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"
)

type registrationService struct {
	regRepo     ports.RegistrationRepository
	contestRepo ports.ContestRepository
}

// NewRegistrationService creates a new RegistrationService instance.
func NewRegistrationService(regRepo ports.RegistrationRepository, contestRepo ports.ContestRepository) ports.RegistrationService {
	return &registrationService{regRepo: regRepo, contestRepo: contestRepo}
}

// Register validates contest state and capacity, then registers the user.
func (s *registrationService) Register(ctx context.Context, contestID, userID int) error {
	c, err := s.contestRepo.Get(ctx, contestID)
	if err != nil {
		return contestNotFound()
	}

	if c.Status != constant.StatusDraft && c.Status != constant.StatusUpcoming {
		return apperr.New(response.CodeBadRequest, "cannot register: contest is "+c.Status, nil)
	}

	if c.MaxParticipants > 0 {
		count, err := s.regRepo.CountByContest(ctx, contestID)
		if err != nil {
			return err
		}
		if count >= c.MaxParticipants {
			return apperr.New(response.CodeConflict, "contest is full", nil)
		}
	}

	exists, err := s.regRepo.Exists(ctx, contestID, userID)
	if err != nil {
		return err
	}
	if exists {
		return apperr.New(response.CodeConflict, "already registered", nil)
	}

	if err := s.regRepo.Create(ctx, contestID, userID); err != nil {
		return err
	}

	logger.FromContext(ctx).Info("user registered for contest", zap.Int("contest_id", contestID), zap.Int("user_id", userID))
	return nil
}

// Unregister removes a user's registration. Not allowed if contest is running or ended.
func (s *registrationService) Unregister(ctx context.Context, contestID, userID int) error {
	c, err := s.contestRepo.Get(ctx, contestID)
	if err != nil {
		return contestNotFound()
	}

	if c.Status == constant.StatusRunning || c.Status == constant.StatusEnded {
		return apperr.New(response.CodeBadRequest, fmt.Sprintf("cannot unregister: contest is %s", c.Status), nil)
	}

	exists, err := s.regRepo.Exists(ctx, contestID, userID)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjRegistration), nil)
	}

	if err := s.regRepo.Delete(ctx, contestID, userID); err != nil {
		return err
	}

	logger.FromContext(ctx).Info("user unregistered from contest", zap.Int("contest_id", contestID), zap.Int("user_id", userID))
	return nil
}

// IsRegistered checks if a user is registered for a contest.
func (s *registrationService) IsRegistered(ctx context.Context, contestID, userID int) (bool, error) {
	return s.regRepo.Exists(ctx, contestID, userID)
}
