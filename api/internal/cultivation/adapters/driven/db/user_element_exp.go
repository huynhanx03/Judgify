package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/userelementexp"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const userElementExpRepoName = "User Element Exp"

type UserElementExpRepository struct {
	client *dbEnt.EntClient
}

func NewUserElementExpRepository(client *dbEnt.EntClient) ports.UserElementExpRepository {
	return &UserElementExpRepository{client: client}
}

func (r *UserElementExpRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserElementExp], error) {
	query := r.client.DB(ctx).UserElementExp.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userElementExpRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userElementExpRepoName)
	}

	entities := make([]*entity.UserElementExp, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToUserElementExpEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.UserElementExp]{Records: &entities, Pagination: meta}, nil
}

func (r *UserElementExpRepository) Get(ctx context.Context, id int) (*entity.UserElementExp, error) {
	rec, err := r.client.DB(ctx).UserElementExp.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userElementExpRepoName)
	}
	return mapper.ToUserElementExpEntity(rec), nil
}

func (r *UserElementExpRepository) Create(ctx context.Context, e *entity.UserElementExp) error {
	rec, err := builder.BuildCreateUserElementExp(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}
	if created := mapper.ToUserElementExpEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *UserElementExpRepository) Update(ctx context.Context, e *entity.UserElementExp) error {
	rec, err := builder.BuildUpdateUserElementExp(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *UserElementExpRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).UserElementExp.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}
	return nil
}

func (r *UserElementExpRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).UserElementExp.Query().Where(userelementexp.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, userElementExpRepoName)
	}
	return exists, nil
}

// GetByUserID returns element exp details (with element name/code) for a user.
func (r *UserElementExpRepository) GetByUserID(ctx context.Context, userID int) ([]entity.ElementExpDetail, error) {
	records, err := r.client.DB(ctx).UserElementExp.Query().
		Where(userelementexp.UserID(userID)).
		WithElement().
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userElementExpRepoName)
	}

	details := make([]entity.ElementExpDetail, 0, len(records))
	for _, rec := range records {
		el, elErr := rec.Edges.ElementOrErr()
		if elErr != nil || el == nil {
			continue
		}
		details = append(details, entity.ElementExpDetail{
			Code: el.Code,
			Name: el.Name,
			Exp:  rec.Exp,
		})
	}
	return details, nil
}

// AddExpByElement atomically adds EXP for a user+element combination.
// Creates the row if it does not exist yet.
func (r *UserElementExpRepository) AddExpByElement(ctx context.Context, userID, elementID int, exp int64) error {
	client := r.client.DB(ctx)

	// Try to update existing row
	n, err := client.UserElementExp.Update().
		Where(userelementexp.UserID(userID), userelementexp.ElementID(elementID)).
		AddExp(exp).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}

	// No row updated → create new
	if n == 0 {
		_, err = client.UserElementExp.Create().
			SetUserID(userID).
			SetElementID(elementID).
			SetExp(exp).
			Save(ctx)
		if err != nil {
			return commonEnt.MapEntError(err, userElementExpRepoName)
		}
	}
	return nil
}
