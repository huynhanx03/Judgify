package db

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/element"
	"github.com/huynhanx03/judgify/internal/ent/generate/tag"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const tagRepoName = "Tag"

type TagRepository struct {
	client *dbEnt.EntClient
}

func NewTagRepository(client *dbEnt.EntClient) ports.TagRepository {
	return &TagRepository{client: client}
}

func (r *TagRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Tag], error) {
	client := r.client.DB(ctx)

	query := client.Tag.Query().WithElements().Where(tagRelationPredicates(opts)...)
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, tagQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, tagQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}

	entities := make([]*entity.Tag, len(records))
	for i, record := range records {
		entities[i] = ToTagEntity(record)
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

	return &d.Paginated[*entity.Tag]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *TagRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Tag, error) {
	record, err := r.client.DB(ctx).Tag.Query().Where(tag.ID(id)).WithElements().Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}
	return ToTagEntity(record), nil
}

func (r *TagRepository) Create(ctx context.Context, e *entity.Tag) error {
	if err := validateTagWrite(r, ctx, e, false); err != nil {
		return err
	}
	var created *entity.Tag
	err := r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		elements, facts, err := resolveTagElements(txCtx, client, e.ElementIDs())
		if err != nil {
			return err
		}
		write := *e
		write.Elements = tagElements(elements)
		record, err := BuildCreateTag(client, &write).Save(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, tagRepoName)
		}
		checksum := tagRevisionChecksum(record.ID, 1, record.Name, facts)
		revision, err := client.TagRevision.Create().
			SetTagID(record.ID).
			SetRevision(1).
			SetName(record.Name).
			SetChecksum(checksum[:]).
			Save(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, tagRepoName)
		}
		for _, fact := range facts {
			if err := client.TagRevisionElement.Create().
				SetTagRevisionID(revision.ID).
				SetElementID(fact.ID).
				SetElementRevision(fact.Revision).
				Exec(txCtx); err != nil {
				return commonEnt.MapEntError(err, tagRepoName)
			}
		}
		full, err := client.Tag.Query().Where(tag.ID(record.ID)).WithElements().Only(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, tagRepoName)
		}
		created = ToTagEntity(full)
		return nil
	})
	if err != nil {
		return err
	}
	*e = *created
	return nil
}

func (r *TagRepository) Update(ctx context.Context, e *entity.Tag) error {
	if err := validateTagWrite(r, ctx, e, true); err != nil {
		return err
	}
	var updated *entity.Tag
	err := r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		stored, err := client.Tag.Query().
			Where(tag.ID(e.ID), tag.DeletedAtIsNil()).
			Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
			Only(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, tagRepoName)
		}
		if stored.Version != e.Version {
			return ports.ErrTagVersionConflict
		}
		if stored.Version >= problemcontract.MaximumRewardMappingRevision {
			return fmt.Errorf("%s: maximum revision reached", tagRepoName)
		}
		elements, facts, err := resolveTagElements(txCtx, client, e.ElementIDs())
		if err != nil {
			return err
		}
		nextVersion := stored.Version + 1
		checksum := tagRevisionChecksum(stored.ID, nextVersion, e.Name, facts)
		revision, err := client.TagRevision.Create().
			SetTagID(stored.ID).
			SetRevision(nextVersion).
			SetName(e.Name).
			SetChecksum(checksum[:]).
			Save(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, tagRepoName)
		}
		for _, fact := range facts {
			if err := client.TagRevisionElement.Create().
				SetTagRevisionID(revision.ID).
				SetElementID(fact.ID).
				SetElementRevision(fact.Revision).
				Exec(txCtx); err != nil {
				return commonEnt.MapEntError(err, tagRepoName)
			}
		}
		write := *e
		write.Elements = tagElements(elements)
		if _, err := BuildUpdateTag(client, &write, nextVersion).
			Where(tag.Version(e.Version)).
			Save(txCtx); err != nil {
			if generate.IsNotFound(err) {
				return ports.ErrTagVersionConflict
			}
			return commonEnt.MapEntError(err, tagRepoName)
		}
		full, err := client.Tag.Query().Where(tag.ID(stored.ID)).WithElements().Only(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, tagRepoName)
		}
		updated = ToTagEntity(full)
		return nil
	})
	if err != nil {
		return err
	}
	*e = *updated
	return nil
}

func validateTagWrite(r *TagRepository, ctx context.Context, value *entity.Tag, update bool) error {
	if r == nil || r.client == nil || ctx == nil || value == nil || ctx.Err() != nil {
		return errors.New("tag: invalid write")
	}
	if !utf8.ValidString(value.Name) || value.Name == "" || utf8.RuneCountInString(value.Name) > 50 ||
		(update && (value.ID == uuid.Nil || value.Version < 1)) {
		return errors.New("tag: invalid authored facts")
	}
	return nil
}

func resolveTagElements(ctx context.Context, client *generate.Client, ids []uuid.UUID) ([]*generate.Element, []elementRevisionFact, error) {
	canonical := append([]uuid.UUID(nil), ids...)
	sort.Slice(canonical, func(i, j int) bool { return bytes.Compare(canonical[i][:], canonical[j][:]) < 0 })
	for index, id := range canonical {
		if id == uuid.Nil || (index > 0 && id == canonical[index-1]) {
			return nil, nil, errors.New("tag: invalid element IDs")
		}
	}
	if len(canonical) == 0 {
		return []*generate.Element{}, []elementRevisionFact{}, nil
	}
	records, err := client.Element.Query().
		Where(element.IDIn(canonical...), element.DeletedAtIsNil()).
		Order(element.ByID()).
		Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
		All(ctx)
	if err != nil {
		return nil, nil, commonEnt.MapEntError(err, tagRepoName)
	}
	if len(records) != len(canonical) {
		return nil, nil, errors.New("tag: requested element is missing or deleted")
	}
	sort.Slice(records, func(i, j int) bool { return bytes.Compare(records[i].ID[:], records[j].ID[:]) < 0 })
	facts := make([]elementRevisionFact, len(records))
	for index, record := range records {
		if record.Version < 1 || record.ID != canonical[index] {
			return nil, nil, errors.New("tag: invalid current element")
		}
		facts[index] = elementRevisionFact{ID: record.ID, Revision: record.Version}
	}
	return records, facts, nil
}

func tagElements(records []*generate.Element) []entity.TagElement {
	items := make([]entity.TagElement, len(records))
	for index, record := range records {
		items[index] = entity.TagElement{ID: record.ID, Name: record.Name, Code: record.Code}
	}
	return items
}

func (r *TagRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).Tag.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, tagRepoName)
	}
	return nil
}

func (r *TagRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Tag.Query().Where(tag.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, tagRepoName)
	}
	return exists, nil
}

func (r *TagRepository) FindByIDs(ctx context.Context, ids []uuid.UUID) ([]*entity.Tag, error) {
	records, err := r.client.DB(ctx).Tag.Query().
		Where(tag.IDIn(ids...)).
		WithElements().
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}

	entities := make([]*entity.Tag, len(records))
	for i, m := range records {
		entities[i] = ToTagEntity(m)
	}
	return entities, nil
}

func (r *TagRepository) FindAll(ctx context.Context) ([]*entity.Tag, error) {
	records, err := r.client.DB(ctx).Tag.Query().WithElements().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}
	entities := make([]*entity.Tag, len(records))
	for i, rec := range records {
		entities[i] = ToTagEntity(rec)
	}
	return entities, nil
}
