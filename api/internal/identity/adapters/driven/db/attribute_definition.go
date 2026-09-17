package db

import (
	"context"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinitionrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/userattributevalue"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const attrDefRepoName = "Attribute Definition"

type AttributeDefinitionRepository struct {
	client *dbEnt.EntClient
}

func NewAttributeDefinitionRepository(client *dbEnt.EntClient) ports.AttributeDefinitionRepository {
	return &AttributeDefinitionRepository{client: client}
}

func (r *AttributeDefinitionRepository) Get(ctx context.Context, id uuid.UUID) (*entity.AttributeDefinition, error) {
	record, err := r.client.DB(ctx).AttributeDefinition.Query().
		Where(attributedefinition.ID(id)).
		WithDraftRevision().
		WithActiveRevision().
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	return ToAttributeDefinitionEntity(record), nil
}

func (r *AttributeDefinitionRepository) GetByKey(ctx context.Context, key string) (*entity.AttributeDefinition, error) {
	record, err := r.client.DB(ctx).AttributeDefinition.Query().
		Where(attributedefinition.Key(key)).
		WithDraftRevision().
		WithActiveRevision().
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	return ToAttributeDefinitionEntity(record), nil
}

func (r *AttributeDefinitionRepository) FindAll(ctx context.Context) ([]*entity.AttributeDefinition, error) {
	records, err := r.client.DB(ctx).AttributeDefinition.Query().
		WithDraftRevision().
		WithActiveRevision().
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	entities := make([]*entity.AttributeDefinition, len(records))
	for i, record := range records {
		entities[i] = ToAttributeDefinitionEntity(record)
	}
	return entities, nil
}

func (r *AttributeDefinitionRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.AttributeDefinition], error) {
	client := r.client.DB(ctx)

	query := client.AttributeDefinition.Query().
		WithDraftRevision().
		WithActiveRevision()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, attributeDefinitionQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, attributeDefinitionQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}

	entities := make([]*entity.AttributeDefinition, len(records))
	for i, record := range records {
		entities[i] = ToAttributeDefinitionEntity(record)
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
	create := BuildCreateAttributeDefinition(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, attrDefRepoName)
	}

	if created := ToAttributeDefinitionEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *AttributeDefinitionRepository) GetRevision(ctx context.Context, id uuid.UUID) (*entity.AttributeDefinitionRevision, error) {
	record, err := r.client.DB(ctx).AttributeDefinitionRevision.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName+" Revision")
	}
	return ToAttributeDefinitionRevisionEntity(record), nil
}

func (r *AttributeDefinitionRepository) ListRevisions(
	ctx context.Context,
	definitionID uuid.UUID,
	before int64,
	limit int,
) ([]*entity.AttributeDefinitionRevision, error) {
	if definitionID == uuid.Nil || before < 0 || limit < 1 || limit > 101 {
		return nil, ports.ErrAttributeDefinitionVersionConflict
	}
	query := r.client.DB(ctx).AttributeDefinitionRevision.Query().
		Where(attributedefinitionrevision.DefinitionID(definitionID))
	if before > 0 {
		query.Where(attributedefinitionrevision.RevisionLT(before))
	}
	records, err := query.
		Order(attributedefinitionrevision.ByRevision(sql.OrderDesc())).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, attrDefRepoName+" Revision")
	}
	result := make([]*entity.AttributeDefinitionRevision, len(records))
	for index, record := range records {
		result[index] = ToAttributeDefinitionRevisionEntity(record)
	}
	return result, nil
}

func (r *AttributeDefinitionRepository) CreateRevision(ctx context.Context, revision *entity.AttributeDefinitionRevision) error {
	if revision == nil {
		return ports.ErrAttributeDefinitionVersionConflict
	}
	builder := r.client.DB(ctx).AttributeDefinitionRevision.Create().
		SetDefinitionID(revision.DefinitionID).
		SetRevision(revision.Revision).
		SetDataType(attributedefinitionrevision.DataType(revision.DataType)).
		SetLabel(revision.Label).
		SetDescription(revision.Description).
		SetVisibility(attributedefinitionrevision.Visibility(revision.Visibility)).
		SetUserEditable(revision.UserEditable).
		SetDisplayOrder(revision.DisplayOrder).
		SetValidation(append([]byte(nil), revision.Validation...)).
		SetNillableCreatedBy(revision.CreatedBy).
		SetCreatedAt(revision.CreatedAt)
	if revision.ID != uuid.Nil {
		builder.SetID(revision.ID)
	}
	record, err := builder.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, attrDefRepoName+" Revision")
	}
	*revision = *ToAttributeDefinitionRevisionEntity(record)
	return nil
}

func (r *AttributeDefinitionRepository) SetInitialRevision(ctx context.Context, definitionID, revisionID uuid.UUID) error {
	updated, err := r.client.DB(ctx).AttributeDefinition.UpdateOneID(definitionID).
		Where(
			attributedefinition.Version(1),
			attributedefinition.DraftRevisionIDIsNil(),
			attributedefinition.ActiveRevisionIDIsNil(),
		).
		SetDraftRevisionID(revisionID).
		SetActiveRevisionID(revisionID).
		Save(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.ErrAttributeDefinitionVersionConflict
		}
		return commonEnt.MapEntError(err, attrDefRepoName)
	}
	if updated == nil || updated.Version != 1 {
		return ports.ErrAttributeDefinitionVersionConflict
	}
	return nil
}

func (r *AttributeDefinitionRepository) SetDraftRevision(ctx context.Context, definitionID uuid.UUID, expectedVersion int64, revision *entity.AttributeDefinitionRevision, requiredOnOnboarding *bool) (*entity.AttributeDefinition, error) {
	return r.moveRevisionPointer(ctx, definitionID, expectedVersion, uuid.Nil, revision, requiredOnOnboarding, false, false)
}

func (r *AttributeDefinitionRepository) ActivateRevision(
	ctx context.Context,
	definitionID uuid.UUID,
	expectedVersion int64,
	sourceRevisionID uuid.UUID,
	revision *entity.AttributeDefinitionRevision,
	requireEmptySource bool,
) (*entity.AttributeDefinition, error) {
	return r.moveRevisionPointer(
		ctx,
		definitionID,
		expectedVersion,
		sourceRevisionID,
		revision,
		nil,
		true,
		requireEmptySource,
	)
}

func (r *AttributeDefinitionRepository) Archive(
	ctx context.Context,
	definitionID uuid.UUID,
	expectedVersion int64,
) (*entity.AttributeDefinition, error) {
	if definitionID == uuid.Nil || expectedVersion < 1 {
		return nil, ports.ErrAttributeDefinitionVersionConflict
	}
	record, err := r.client.DB(ctx).AttributeDefinition.UpdateOneID(definitionID).
		Where(
			attributedefinition.Version(expectedVersion),
			attributedefinition.StatusEQ(attributedefinition.StatusActive),
			attributedefinition.KindEQ(attributedefinition.KindCustom),
		).
		SetStatus(attributedefinition.StatusArchived).
		SetVersion(expectedVersion + 1).
		Save(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAttributeDefinitionVersionConflict
		}
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	return ToAttributeDefinitionEntity(record), nil
}

func (r *AttributeDefinitionRepository) CountValues(
	ctx context.Context,
	definitionID uuid.UUID,
	revisionID uuid.UUID,
) (int64, error) {
	if definitionID == uuid.Nil || revisionID == uuid.Nil {
		return 0, ports.ErrAttributeDefinitionVersionConflict
	}
	count, err := r.client.DB(ctx).UserAttributeValue.Query().
		Where(
			userattributevalue.AttributeID(definitionID),
			userattributevalue.DefinitionRevisionID(revisionID),
		).
		Count(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	return int64(count), nil
}

func (r *AttributeDefinitionRepository) LoadCommandReceipt(
	ctx context.Context,
	commandID uuid.UUID,
) (ports.AttributeCommandReceipt, bool, error) {
	if commandID == uuid.Nil {
		return ports.AttributeCommandReceipt{}, false, ports.ErrAttributeDefinitionVersionConflict
	}
	record, err := r.client.DB(ctx).OutboxEvent.Get(ctx, commandID)
	if generate.IsNotFound(err) {
		return ports.AttributeCommandReceipt{}, false, nil
	}
	if err != nil {
		return ports.AttributeCommandReceipt{}, false, commonEnt.MapEntError(err, attrDefRepoName+" Command")
	}
	if record == nil {
		return ports.AttributeCommandReceipt{}, false, ports.ErrAttributeDefinitionVersionConflict
	}
	return ports.AttributeCommandReceipt{
		EventID:          record.ID,
		EventType:        record.EventType,
		AggregateID:      record.AggregateID,
		AggregateVersion: record.AggregateVersion,
		Payload:          append([]byte(nil), record.Payload...),
		OccurredAt:       record.OccurredAt.UTC(),
	}, true, nil
}

func (r *AttributeDefinitionRepository) moveRevisionPointer(
	ctx context.Context,
	definitionID uuid.UUID,
	expectedVersion int64,
	sourceRevisionID uuid.UUID,
	revision *entity.AttributeDefinitionRevision,
	requiredOnOnboarding *bool,
	activate bool,
	requireEmptySource bool,
) (*entity.AttributeDefinition, error) {
	if revision == nil || revision.DefinitionID != definitionID || expectedVersion < 1 ||
		(activate && sourceRevisionID == uuid.Nil) || (!activate && requireEmptySource) {
		return nil, ports.ErrAttributeDefinitionVersionConflict
	}
	update := r.client.DB(ctx).AttributeDefinition.UpdateOneID(definitionID).
		Where(
			attributedefinition.Version(expectedVersion),
			attributedefinition.StatusEQ(attributedefinition.StatusActive),
		).
		SetVersion(expectedVersion + 1).
		SetDraftRevisionID(revision.ID)
	if requiredOnOnboarding != nil {
		update.SetRequiredOnOnboarding(*requiredOnOnboarding)
	}
	if activate {
		update.SetActiveRevisionID(revision.ID)
		if requireEmptySource {
			update.Where(attributedefinition.Not(
				attributedefinition.HasValuesWith(
					userattributevalue.DefinitionRevisionID(sourceRevisionID),
				),
			))
		}
	}
	record, err := update.Save(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrAttributeDefinitionVersionConflict
		}
		return nil, commonEnt.MapEntError(err, attrDefRepoName)
	}
	return ToAttributeDefinitionEntity(record), nil
}
