package db

import (
	"context"

	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/profileschemastate"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const profileSchemaRepoName = "Profile Schema"

type ProfileSchemaRepository struct{ client *dbent.EntClient }

func NewProfileSchemaRepository(client *dbent.EntClient) *ProfileSchemaRepository {
	return &ProfileSchemaRepository{client: client}
}

var _ ports.ProfileSchemaRepository = (*ProfileSchemaRepository)(nil)

func (repository *ProfileSchemaRepository) Ensure(ctx context.Context) error {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil {
		return ports.ErrProfileSchemaUnavailable
	}
	record, err := repository.client.DB(ctx).ProfileSchemaState.Query().
		Where(profileschemastate.Key(ports.ProfileSchemaStateKey)).
		Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			// Bootstrap installs the singleton together with the protected
			// definitions it versions. Starting the application must never
			// manufacture a detached revision fence and make a later bootstrap
			// reject an otherwise fresh database.
			return ports.ErrProfileSchemaUnavailable
		}
		return commonent.MapEntError(err, profileSchemaRepoName)
	}
	if record == nil || record.ID == uuid.Nil || record.Revision < 1 {
		return ports.ErrProfileSchemaUnavailable
	}
	return nil
}

func (repository *ProfileSchemaRepository) LockShared(ctx context.Context) (ports.ProfileSchemaFence, error) {
	return repository.lock(ctx, false)
}

func (repository *ProfileSchemaRepository) LockExclusive(ctx context.Context) (ports.ProfileSchemaFence, error) {
	return repository.lock(ctx, true)
}

func (repository *ProfileSchemaRepository) lock(
	ctx context.Context,
	exclusive bool,
) (ports.ProfileSchemaFence, error) {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil {
		return ports.ProfileSchemaFence{}, ports.ErrProfileSchemaUnavailable
	}
	client, err := profileSchemaTransactionalClient(ctx)
	if err != nil {
		return ports.ProfileSchemaFence{}, ports.ErrProfileSchemaUnavailable
	}
	query := client.ProfileSchemaState.Query().
		Where(profileschemastate.Key(ports.ProfileSchemaStateKey))
	if exclusive {
		query = query.ForUpdate()
	} else {
		query = query.ForShare()
	}
	record, err := query.Only(ctx)
	if err != nil || record == nil || record.ID == uuid.Nil || record.Revision < 1 {
		if err != nil {
			return ports.ProfileSchemaFence{}, commonent.MapEntError(err, profileSchemaRepoName)
		}
		return ports.ProfileSchemaFence{}, ports.ErrProfileSchemaUnavailable
	}
	return ports.ProfileSchemaFence{ID: record.ID, Revision: record.Revision}, nil
}

func (repository *ProfileSchemaRepository) Advance(
	ctx context.Context,
	fence ports.ProfileSchemaFence,
) (ports.ProfileSchemaFence, error) {
	if repository == nil || repository.client == nil || fence.ID == uuid.Nil ||
		fence.Revision < 1 || ctx == nil || ctx.Err() != nil {
		return ports.ProfileSchemaFence{}, ports.ErrProfileSchemaUnavailable
	}
	client, err := profileSchemaTransactionalClient(ctx)
	if err != nil {
		return ports.ProfileSchemaFence{}, ports.ErrProfileSchemaUnavailable
	}
	record, err := client.ProfileSchemaState.UpdateOneID(fence.ID).
		Where(
			profileschemastate.Key(ports.ProfileSchemaStateKey),
			profileschemastate.Revision(fence.Revision),
		).
		SetRevision(fence.Revision + 1).
		Save(ctx)
	if generate.IsNotFound(err) {
		return ports.ProfileSchemaFence{}, ports.ErrProfileSchemaConflict
	}
	if err != nil {
		return ports.ProfileSchemaFence{}, commonent.MapEntError(err, profileSchemaRepoName)
	}
	return ports.ProfileSchemaFence{ID: record.ID, Revision: record.Revision}, nil
}

func (repository *ProfileSchemaRepository) ListRequired(
	ctx context.Context,
) ([]*entity.AttributeDefinition, error) {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil {
		return nil, ports.ErrProfileSchemaUnavailable
	}
	client, err := profileSchemaTransactionalClient(ctx)
	if err != nil {
		return nil, ports.ErrProfileSchemaUnavailable
	}
	records, err := client.AttributeDefinition.Query().
		Where(
			attributedefinition.StatusEQ(attributedefinition.StatusActive),
			attributedefinition.RequiredOnOnboarding(true),
			attributedefinition.ActiveRevisionIDNotNil(),
		).
		WithActiveRevision().
		Order(attributedefinition.ByKey()).
		Limit(129).
		All(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, profileSchemaRepoName)
	}
	if len(records) == 0 || len(records) > 128 {
		return nil, ports.ErrProfileSchemaUnavailable
	}
	result := make([]*entity.AttributeDefinition, 0, len(records))
	for _, record := range records {
		definition := ToAttributeDefinitionEntity(record)
		if definition == nil || definition.ActiveRevision == nil ||
			definition.ActiveRevisionID == nil || definition.ID == uuid.Nil ||
			definition.ActiveRevision.ID != *definition.ActiveRevisionID ||
			definition.ActiveRevision.DefinitionID != definition.ID {
			return nil, ports.ErrProfileSchemaUnavailable
		}
		result = append(result, definition)
	}
	return result, nil
}

func profileSchemaTransactionalClient(ctx context.Context) (*generate.Client, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, ports.ErrProfileSchemaUnavailable
	}
	return platformtx.Client(ctx, nil)
}
