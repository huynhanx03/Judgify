package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/credential"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const credentialRepoName = "CredentialRepository"

type CredentialRepository struct {
	client *dbEnt.EntClient
}

func NewCredentialRepository(client *dbEnt.EntClient) ports.CredentialRepository {
	return &CredentialRepository{client: client}
}

func (r *CredentialRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Credential], error) {
	client := r.client.DB(ctx)

	query := client.Credential.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}

	entities := make([]*entity.Credential, len(records))
	for i, record := range records {
		entities[i] = mapper.ToCredentialEntity(record)
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

	return &d.Paginated[*entity.Credential]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *CredentialRepository) Get(ctx context.Context, id int) (*entity.Credential, error) {
	record, err := r.client.DB(ctx).Credential.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}
	return mapper.ToCredentialEntity(record), nil
}

func (r *CredentialRepository) GetByUserID(ctx context.Context, userID int, credType string) (*entity.Credential, error) {
	record, err := r.client.DB(ctx).Credential.Query().
		Where(
			credential.UserID(userID),
			credential.Type(credType),
		).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}
	return mapper.ToCredentialEntity(record), nil
}

func (r *CredentialRepository) Create(ctx context.Context, e *entity.Credential) error {
	create := builder.BuildCreateCredential(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, credentialRepoName)
	}

	if created := mapper.ToCredentialEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *CredentialRepository) Update(ctx context.Context, e *entity.Credential) error {
	update := builder.BuildUpdateCredential(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, credentialRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *CredentialRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Credential.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, credentialRepoName)
	}
	return nil
}

func (r *CredentialRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Credential.Query().Where(credential.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, credentialRepoName)
}
