package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/userattributevalue"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const userAttrValueRepoName = "User Attribute Value"

type UserAttributeValueRepository struct {
	client *dbEnt.EntClient
}

func NewUserAttributeValueRepository(client *dbEnt.EntClient) ports.UserAttributeValueRepository {
	return &UserAttributeValueRepository{client: client}
}

func (r *UserAttributeValueRepository) Get(ctx context.Context, id int) (*entity.UserAttributeValue, error) {
	record, err := r.client.DB(ctx).UserAttributeValue.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	return mapper.ToUserAttributeValueEntity(record), nil
}

func (r *UserAttributeValueRepository) GetByUserID(ctx context.Context, userID int) ([]*entity.UserAttributeValue, error) {
	records, err := r.client.DB(ctx).UserAttributeValue.Query().
		Where(userattributevalue.UserID(userID)).
		WithDefinition().
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	entities := make([]*entity.UserAttributeValue, len(records))
	for i, record := range records {
		entities[i] = mapper.ToUserAttributeValueEntity(record)
	}
	return entities, nil
}

func (r *UserAttributeValueRepository) Create(ctx context.Context, e *entity.UserAttributeValue) error {
	create := builder.BuildCreateUserAttributeValue(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userAttrValueRepoName)
	}

	if created := mapper.ToUserAttributeValueEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *UserAttributeValueRepository) CreateBulk(ctx context.Context, entities []*entity.UserAttributeValue) error {
	builders := make([]*generate.UserAttributeValueCreate, len(entities))
	for i, e := range entities {
		builders[i] = builder.BuildCreateUserAttributeValue(ctx, e)
	}

	if err := r.client.DB(ctx).UserAttributeValue.CreateBulk(builders...).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	return nil
}

func (r *UserAttributeValueRepository) UpdateBulk(ctx context.Context, entities []*entity.UserAttributeValue) error {
	builders := make([]*generate.UserAttributeValueCreate, len(entities))
	for i, e := range entities {
		builders[i] = builder.BuildCreateUserAttributeValue(ctx, e)
	}

	err := r.client.DB(ctx).UserAttributeValue.CreateBulk(builders...).
		OnConflict().
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	return nil
}

func (r *UserAttributeValueRepository) Update(ctx context.Context, e *entity.UserAttributeValue) error {
	update := builder.BuildUpdateUserAttributeValue(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *UserAttributeValueRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).UserAttributeValue.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	return nil
}

func (r *UserAttributeValueRepository) DeleteByUserID(ctx context.Context, userID int) error {
	_, err := r.client.DB(ctx).UserAttributeValue.Delete().Where(userattributevalue.UserID(userID)).Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	return nil
}
