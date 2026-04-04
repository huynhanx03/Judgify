package db

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/userstats"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const userStatsRepoName = "User Stats"

type UserStatsRepository struct {
	client *dbEnt.EntClient
}

func NewUserStatsRepository(client *dbEnt.EntClient) ports.UserStatsRepository {
	return &UserStatsRepository{client: client}
}

func (r *UserStatsRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserStats], error) {
	query := r.client.DB(ctx).UserStats.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}

	entities := make([]*entity.UserStats, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToUserStatsEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.UserStats]{Records: &entities, Pagination: meta}, nil
}

func (r *UserStatsRepository) Get(ctx context.Context, id int) (*entity.UserStats, error) {
	rec, err := r.client.DB(ctx).UserStats.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}
	return mapper.ToUserStatsEntity(rec), nil
}

func (r *UserStatsRepository) GetByUserID(ctx context.Context, userID int) (*entity.UserStats, error) {
	rec, err := r.client.DB(ctx).UserStats.Query().
		Where(userstats.UserID(userID)).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}
	return mapper.ToUserStatsEntity(rec), nil
}

func (r *UserStatsRepository) Create(ctx context.Context, e *entity.UserStats) error {
	rec, err := builder.BuildCreateUserStats(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	if created := mapper.ToUserStatsEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *UserStatsRepository) Update(ctx context.Context, e *entity.UserStats) error {
	rec, err := builder.BuildUpdateUserStats(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *UserStatsRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).UserStats.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	return nil
}

func (r *UserStatsRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).UserStats.Query().Where(userstats.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, userStatsRepoName)
	}
	return exists, nil
}

// IncrementSubmission atomically increments total_submissions (and accepted_count when accepted).
func (r *UserStatsRepository) IncrementSubmission(ctx context.Context, userID int, accepted bool) error {
	client := r.client.DB(ctx)
	upd := client.UserStats.Update().
		Where(userstats.UserID(userID)).
		AddTotalSubmissions(1)
	if accepted {
		upd = upd.AddAcceptedCount(1)
	}
	_, err := upd.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	return nil
}

// AddExp atomically adds EXP to user stats. Row must exist (created at registration).
func (r *UserStatsRepository) AddExp(ctx context.Context, userID int, exp int64) error {
	client := r.client.DB(ctx)

	n, err := client.UserStats.Update().
		Where(userstats.UserID(userID)).
		AddTotalExp(exp).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	if n == 0 {
		return commonEnt.MapEntError(fmt.Errorf("user stats not found for user_id %d", userID), userStatsRepoName)
	}
	return nil
}
