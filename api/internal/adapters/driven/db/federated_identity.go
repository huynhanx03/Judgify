package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/builder"
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate/federatedidentity"
	"github.com/huynhanx03/judgify/internal/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/core/entity"
	"github.com/huynhanx03/judgify/internal/ports"
)

const fedIdentityRepoName = "FederatedIdentityRepository"

type FederatedIdentityRepository struct {
	client *dbEnt.EntClient
}

func NewFederatedIdentityRepository(client *dbEnt.EntClient) ports.FederatedIdentityRepository {
	return &FederatedIdentityRepository{client: client}
}

func (r *FederatedIdentityRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.FederatedIdentity], error) {
	client := r.client.DB(ctx)

	query := client.FederatedIdentity.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}

	entities := make([]*entity.FederatedIdentity, len(records))
	for i, record := range records {
		entities[i] = mapper.ToFederatedIdentityEntity(record)
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

	return &d.Paginated[*entity.FederatedIdentity]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *FederatedIdentityRepository) Get(ctx context.Context, id int) (*entity.FederatedIdentity, error) {
	record, err := r.client.DB(ctx).FederatedIdentity.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return mapper.ToFederatedIdentityEntity(record), nil
}

// GetByProviderAndExternalID finds a federated identity by provider and external ID.
func (r *FederatedIdentityRepository) GetByProviderAndExternalID(ctx context.Context, provider, externalID string) (*entity.FederatedIdentity, error) {
	record, err := r.client.DB(ctx).FederatedIdentity.Query().
		Where(
			federatedidentity.Provider(provider),
			federatedidentity.ExternalID(externalID),
		).Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return mapper.ToFederatedIdentityEntity(record), nil
}

func (r *FederatedIdentityRepository) Create(ctx context.Context, e *entity.FederatedIdentity) error {
	create := builder.BuildCreateFederatedIdentity(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, fedIdentityRepoName)
	}

	if created := mapper.ToFederatedIdentityEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *FederatedIdentityRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).FederatedIdentity.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return nil
}

func (r *FederatedIdentityRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).FederatedIdentity.Query().Where(federatedidentity.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, fedIdentityRepoName)
}
