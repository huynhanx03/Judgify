package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/submission"
	"github.com/huynhanx03/judgify/internal/submission/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/submission/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/submission/core/entity"
	"github.com/huynhanx03/judgify/internal/submission/ports"
)

const submissionRepoName = "SubmissionRepository"

type SubmissionRepository struct {
	client *dbEnt.EntClient
}

func NewSubmissionRepository(client *dbEnt.EntClient) ports.SubmissionRepository {
	return &SubmissionRepository{client: client}
}

func (r *SubmissionRepository) Get(ctx context.Context, id int) (*entity.Submission, error) {
	record, err := r.client.DB(ctx).Submission.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}
	return mapper.ToSubmissionEntity(record), nil
}

func (r *SubmissionRepository) FindByProblemID(ctx context.Context, problemID int) ([]*entity.Submission, error) {
	records, err := r.client.DB(ctx).Submission.Query().
		Where(submission.ProblemID(problemID)).
		Order(submission.ByID()).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}

	entities := make([]*entity.Submission, len(records))
	for i, record := range records {
		entities[i] = mapper.ToSubmissionEntity(record)
	}
	return entities, nil
}

func (r *SubmissionRepository) FindByUserID(ctx context.Context, userID int) ([]*entity.Submission, error) {
	records, err := r.client.DB(ctx).Submission.Query().
		Where(submission.UserID(userID)).
		Order(submission.ByID()).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}

	entities := make([]*entity.Submission, len(records))
	for i, record := range records {
		entities[i] = mapper.ToSubmissionEntity(record)
	}
	return entities, nil
}

func (r *SubmissionRepository) Create(ctx context.Context, e *entity.Submission) error {
	create := builder.BuildCreateSubmission(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, submissionRepoName)
	}
	if created := mapper.ToSubmissionEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *SubmissionRepository) Update(ctx context.Context, e *entity.Submission) error {
	update := builder.BuildUpdateSubmission(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, submissionRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}
