package db

import (
	"context"
	"fmt"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ports"
)

const userElementExpRepoName = "User Element Exp"

type UserElementExpRepository struct {
	client *dbEnt.EntClient
}

func NewUserElementExpRepository(client *dbEnt.EntClient) ports.UserElementExpRepository {
	return &UserElementExpRepository{client: client}
}

func (r *UserElementExpRepository) CreateBulk(
	ctx context.Context,
	entities []*entity.UserElementExp,
) error {
	if len(entities) == 0 {
		return nil
	}
	builders := make([]*generate.UserElementExpCreate, len(entities))
	for index, item := range entities {
		if item == nil {
			return commonEnt.MapEntError(fmt.Errorf("nil user element exp"), userElementExpRepoName)
		}
		builders[index] = BuildCreateUserElementExp(r.client.DB(ctx), item)
	}
	if _, err := r.client.DB(ctx).UserElementExp.CreateBulk(builders...).Save(ctx); err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}
	return nil
}
