package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/ports"
)

const contestRepoName = "Contest"

// ContestRepository implements ports.ContestRepository.
type ContestRepository struct {
	client *dbEnt.EntClient
}

// NewContestRepository creates a new ContestRepository.
func NewContestRepository(client *dbEnt.EntClient) ports.ContestRepository {
	return &ContestRepository{client: client}
}

func (r *ContestRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error) {
	client := r.client.DB(ctx)

	query := client.Contest.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}

	entities := make([]*entity.Contest, len(records))
	for i, record := range records {
		entities[i] = mapper.ToContestEntity(record)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(
		paginationOpts.Page,
		paginationOpts.PageSize,
		int64(total),
	)

	return &d.Paginated[*entity.Contest]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *ContestRepository) Get(ctx context.Context, id int) (*entity.Contest, error) {
	record, err := r.client.DB(ctx).Contest.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	return mapper.ToContestEntity(record), nil
}

func (r *ContestRepository) Create(ctx context.Context, e *entity.Contest) error {
	create := builder.BuildCreateContest(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, contestRepoName)
	}
	if created := mapper.ToContestEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *ContestRepository) Update(ctx context.Context, e *entity.Contest) error {
	update := builder.BuildUpdateContest(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, contestRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *ContestRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Contest.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, contestRepoName)
	}
	return nil
}

func (r *ContestRepository) AddProblems(ctx context.Context, contestID int, problemIDs []int) error {
	err := r.client.DB(ctx).Contest.UpdateOneID(contestID).AddProblemIDs(problemIDs...).Exec(ctx)
	return commonEnt.MapEntError(err, contestRepoName)
}

func (r *ContestRepository) RemoveAllProblems(ctx context.Context, contestID int) error {
	err := r.client.DB(ctx).Contest.UpdateOneID(contestID).ClearProblems().Exec(ctx)
	return commonEnt.MapEntError(err, contestRepoName)
}

func (r *ContestRepository) GetProblemIDs(ctx context.Context, contestID int) ([]int, error) {
	ids, err := r.client.DB(ctx).Contest.Query().Where(contest.ID(contestID)).QueryProblems().IDs(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	return ids, nil
}

func (r *ContestRepository) FindIDsByStatus(ctx context.Context, status string) ([]int, error) {
	ids, err := r.client.DB(ctx).Contest.Query().
		Where(contest.StatusEQ(contest.Status(status))).
		IDs(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	return ids, nil
}

func (r *ContestRepository) UpdateStatus(ctx context.Context, id int, status string) error {
	err := r.client.DB(ctx).Contest.UpdateOneID(id).
		SetStatus(contest.Status(status)).
		Exec(ctx)
	return commonEnt.MapEntError(err, contestRepoName)
}

func (r *ContestRepository) CountRegistrations(ctx context.Context, contestID int) (int, error) {
	count, err := r.client.DB(ctx).Contest.Query().
		Where(contest.ID(contestID)).
		QueryRegistrations().
		Count(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, contestRepoName)
	}
	return count, nil
}
