package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/testcase"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

const testCaseRepoName = "TestCaseRepository"

type TestCaseRepository struct {
	client *dbEnt.EntClient
}

func NewTestCaseRepository(client *dbEnt.EntClient) ports.TestCaseRepository {
	return &TestCaseRepository{client: client}
}

func (r *TestCaseRepository) FindByProblemID(ctx context.Context, problemID int) ([]*entity.TestCase, error) {
	records, err := r.client.DB(ctx).TestCase.Query().
		Where(testcase.ProblemID(problemID)).
		Order(testcase.ByOrderIndex()).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, testCaseRepoName)
	}

	entities := make([]*entity.TestCase, len(records))
	for i, record := range records {
		entities[i] = mapper.ToTestCaseEntity(record)
	}
	return entities, nil
}

func (r *TestCaseRepository) Get(ctx context.Context, id int) (*entity.TestCase, error) {
	record, err := r.client.DB(ctx).TestCase.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, testCaseRepoName)
	}
	return mapper.ToTestCaseEntity(record), nil
}

func (r *TestCaseRepository) Create(ctx context.Context, e *entity.TestCase) error {
	create := builder.BuildCreateTestCase(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, testCaseRepoName)
	}

	if created := mapper.ToTestCaseEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *TestCaseRepository) Update(ctx context.Context, e *entity.TestCase) error {
	update := builder.BuildUpdateTestCase(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, testCaseRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *TestCaseRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).TestCase.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, testCaseRepoName)
	}
	return nil
}

func (r *TestCaseRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).TestCase.Query().Where(testcase.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, testCaseRepoName)
}

func (r *TestCaseRepository) CountByProblemID(ctx context.Context, problemID int) (int, error) {
	count, err := r.client.DB(ctx).TestCase.Query().Where(testcase.ProblemID(problemID)).Count(ctx)
	return count, commonEnt.MapEntError(err, testCaseRepoName)
}
