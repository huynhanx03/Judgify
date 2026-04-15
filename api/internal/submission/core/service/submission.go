package service

import (
	"context"
	"encoding/json"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/logger"
	"github.com/huynhanx03/judgify/pkg/mq/forge"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/submission/core/dto"
	"github.com/huynhanx03/judgify/internal/submission/core/mapper"
	"github.com/huynhanx03/judgify/internal/submission/ports"
)


type submissionService struct {
	submissionRepo  ports.SubmissionRepository
	producer        *forge.Producer
	contestProducer *forge.Producer
}

// NewSubmissionService creates a new SubmissionService instance.
func NewSubmissionService(submissionRepo ports.SubmissionRepository, producer *forge.Producer, contestProducer *forge.Producer) ports.SubmissionService {
	return &submissionService{submissionRepo: submissionRepo, producer: producer, contestProducer: contestProducer}
}

func (s *submissionService) Get(ctx context.Context, id int) (*dto.SubmissionResponse, error) {
	e, err := s.submissionRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToSubmissionResponse(e), nil
}

func (s *submissionService) FindByProblemID(ctx context.Context, problemID int) ([]*dto.SubmissionResponse, error) {
	entities, err := s.submissionRepo.FindByProblemID(ctx, problemID)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.SubmissionResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToSubmissionResponse(e)
	}
	return responses, nil
}

func (s *submissionService) FindByUserAndProblem(ctx context.Context, userID, problemID int) ([]*dto.SubmissionResponse, error) {
	entities, err := s.submissionRepo.FindByUserAndProblem(ctx, userID, problemID)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.SubmissionResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToSubmissionResponse(e)
	}
	return responses, nil
}

func (s *submissionService) Create(ctx context.Context, userID int, req *dto.CreateSubmissionRequest) (*dto.SubmissionResponse, error) {
	e := mapper.ToSubmissionEntityFromCreate(req, userID)
	if err := s.submissionRepo.Create(ctx, e); err != nil {
		return nil, err
	}

	// Choose producer based on contest
	p := s.producer
	if e.ContestID != nil && s.contestProducer != nil {
		p = s.contestProducer
	}
	if err := s.publishJudgeJob(p, e.ID); err != nil {
		logger.FromContext(ctx).Error("failed to enqueue judge job", zap.Int("submission_id", e.ID), zap.Error(err))
		return nil, apperr.New(response.CodeInternalServer, "failed to enqueue judge job", err)
	}

	logger.FromContext(ctx).Info("submission created and enqueued for judging", zap.Int("submission_id", e.ID), zap.Int("problem_id", e.ProblemID), zap.Int("user_id", e.UserID))

	return mapper.ToSubmissionResponse(e), nil
}

// publishJudgeJob sends submission ID to the judge topic via the given producer.
func (s *submissionService) publishJudgeJob(producer *forge.Producer, submissionID int) error {
	key := []byte(constant.TopicJudge)
	value, err := json.Marshal(map[string]int{"submission_id": submissionID})
	if err != nil {
		return err
	}
	return producer.Send(key, value, nil)
}
