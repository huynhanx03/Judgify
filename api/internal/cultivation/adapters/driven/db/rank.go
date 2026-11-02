package db

import (
	"context"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/rank"
	"github.com/huynhanx03/judgify/internal/ports"
)

const rankRepoName = "Rank"

type RankRepository struct {
	client *dbEnt.EntClient
}

func NewRankRepository(client *dbEnt.EntClient) ports.RankRepository {
	return &RankRepository{client: client}
}

func (r *RankRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Rank], error) {
	query := r.client.DB(ctx).Rank.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, rankQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rankRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, rankQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rankRepoName)
	}

	entities := make([]*entity.Rank, len(records))
	for i, rec := range records {
		entities[i] = ToRankEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.Rank]{Records: &entities, Pagination: meta}, nil
}

func (r *RankRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Rank, error) {
	rec, err := r.client.DB(ctx).Rank.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rankRepoName)
	}
	return ToRankEntity(rec), nil
}

func (r *RankRepository) Create(ctx context.Context, e *entity.Rank) error {
	rec, err := BuildCreateRank(r.client.DB(ctx), e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, rankRepoName)
	}
	if created := ToRankEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *RankRepository) Update(ctx context.Context, e *entity.Rank) error {
	rec, err := BuildUpdateRank(r.client.DB(ctx), e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, rankRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *RankRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).Rank.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, rankRepoName)
	}
	return nil
}

func (r *RankRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Rank.Query().Where(rank.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, rankRepoName)
	}
	return exists, nil
}

func (r *RankRepository) FindAll(ctx context.Context) ([]*entity.Rank, error) {
	records, err := r.client.DB(ctx).Rank.Query().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rankRepoName)
	}
	entities := make([]*entity.Rank, len(records))
	for i, rec := range records {
		entities[i] = ToRankEntity(rec)
	}
	return entities, nil
}
