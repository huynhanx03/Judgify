package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const attrDefRepoName = "AttributeDefinitionRepository"

type AttributeDefinitionRepository struct {
	client *dbEnt.EntClient
}

func NewAttributeDefinitionRepository(client *dbEnt.EntClient) ports.AttributeDefinitionRepository {
	return &AttributeDefinitionRepository{client: client}
}

func (r *AttributeDefinitionRepository) Get(ctx context.Context, id int) (*entity.AttributeDefinition, error) {
	record, err := r.client.DB(ctx).AttributeDefinition.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	return mapper.ToAttributeDefinitionEntity(record), nil
}

func (r *AttributeDefinitionRepository) GetByKey(ctx context.Context, key string) (*entity.AttributeDefinition, error) {
	record, err := r.client.DB(ctx).AttributeDefinition.Query().
		Where(attributedefinition.Key(key)).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	return mapper.ToAttributeDefinitionEntity(record), nil
}

func (r *AttributeDefinitionRepository) FindAll(ctx context.Context) ([]*entity.AttributeDefinition, error) {
	records, err := r.client.DB(ctx).AttributeDefinition.Query().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	entities := make([]*entity.AttributeDefinition, len(records))
	for i, record := range records {
		entities[i] = mapper.ToAttributeDefinitionEntity(record)
	}
	return entities, nil
}

func (r *AttributeDefinitionRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.AttributeDefinition], error) {
	client := r.client.DB(ctx)

	query := client.AttributeDefinition.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}

	entities := make([]*entity.AttributeDefinition, len(records))
	for i, record := range records {
		entities[i] = mapper.ToAttributeDefinitionEntity(record)
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

	return &d.Paginated[*entity.AttributeDefinition]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *AttributeDefinitionRepository) Create(ctx context.Context, e *entity.AttributeDefinition) error {
	create := builder.BuildCreateAttributeDefinition(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, attrDefRepoName)
	}

	if created := mapper.ToAttributeDefinitionEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *AttributeDefinitionRepository) Update(ctx context.Context, e *entity.AttributeDefinition) error {
	update := builder.BuildUpdateAttributeDefinition(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, attrDefRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *AttributeDefinitionRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).AttributeDefinition.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, attrDefRepoName)
	}
	return nil
}

func (r *AttributeDefinitionRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).AttributeDefinition.Query().Where(attributedefinition.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, attrDefRepoName)
}
