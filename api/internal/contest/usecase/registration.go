package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type registrationService struct {
	regRepo ports.RegistrationRepository
}

// NewRegistrationService creates a new RegistrationService instance.
func NewRegistrationService(regRepo ports.RegistrationRepository) ports.RegistrationService {
	return &registrationService{regRepo: regRepo}
}

// Register delegates the complete lifecycle/capacity decision to the atomic
// repository command, then translates stable domain outcomes for HTTP clients.
func (s *registrationService) Register(ctx context.Context, contestID, userID uuid.UUID) error {
	if err := s.regRepo.Register(ctx, contestID, userID); err != nil {
		return mapRegistrationCommandError(err)
	}

	logger.FromContext(ctx).Info("user registered for contest", zap.String("contest_id", contestID.String()), zap.String("user_id", userID.String()))
	return nil
}

// Unregister uses the same serialized lifecycle boundary as Register.
func (s *registrationService) Unregister(ctx context.Context, contestID, userID uuid.UUID) error {
	if err := s.regRepo.Unregister(ctx, contestID, userID); err != nil {
		return mapRegistrationCommandError(err)
	}

	logger.FromContext(ctx).Info("user unregistered from contest", zap.String("contest_id", contestID.String()), zap.String("user_id", userID.String()))
	return nil
}

// IsRegistered checks if a user is registered for a contest.
func (s *registrationService) IsRegistered(ctx context.Context, contestID, userID uuid.UUID) (bool, error) {
	return s.regRepo.Exists(ctx, contestID, userID)
}

func mapRegistrationCommandError(err error) error {
	switch {
	case errors.Is(err, entity.ErrContestNotFound):
		return apperr.For(constant.ContestObjContest).NotFound(err)
	case errors.Is(err, entity.ErrContestRegistrationNotOpen):
		return apperr.New(apperr.CodeConflict, "contest registration is not open", err)
	case errors.Is(err, entity.ErrContestRegistrationStarted):
		return apperr.New(apperr.CodeConflict, "contest registration has closed", err)
	case errors.Is(err, entity.ErrContestAlreadyRegistered):
		return apperr.New(apperr.CodeConflict, "already registered", err)
	case errors.Is(err, entity.ErrContestRegistrationFull):
		return apperr.New(apperr.CodeConflict, "contest is full", err)
	case errors.Is(err, entity.ErrContestRegistrationNotFound):
		return apperr.For(constant.ContestObjRegistration).NotFound(err)
	default:
		return err
	}
}
