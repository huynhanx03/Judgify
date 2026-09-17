package db

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate/difficulty"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const difficultyRepoName = "Difficulty"

type DifficultyRepository struct {
	client *dbEnt.EntClient
}

func NewDifficultyRepository(client *dbEnt.EntClient) ports.DifficultyRepository {
	return &DifficultyRepository{client: client}
}

func (r *DifficultyRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Difficulty], error) {
	client := r.client.DB(ctx)

	query := client.Difficulty.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, difficultyQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, difficultyQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}

	entities := make([]*entity.Difficulty, len(records))
	for i, record := range records {
		entities[i] = ToDifficultyEntity(record)
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

	return &d.Paginated[*entity.Difficulty]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *DifficultyRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Difficulty, error) {
	record, err := r.client.DB(ctx).Difficulty.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}
	return ToDifficultyEntity(record), nil
}

func (r *DifficultyRepository) Create(ctx context.Context, e *entity.Difficulty) error {
	if err := validateDifficultyWrite(r, ctx, e, false); err != nil {
		return err
	}
	var created *entity.Difficulty
	err := r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		record, err := BuildCreateDifficulty(client, e).Save(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, difficultyRepoName)
		}
		checksum := difficultyRevisionChecksum(record.ID, 1, record.Name, record.Level, record.ExpReward, record.Description)
		if err := client.DifficultyRevision.Create().
			SetDifficultyID(record.ID).
			SetRevision(1).
			SetName(record.Name).
			SetLevel(record.Level).
			SetExpReward(record.ExpReward).
			SetDescription(record.Description).
			SetChecksum(checksum[:]).
			Exec(txCtx); err != nil {
			return commonEnt.MapEntError(err, difficultyRepoName)
		}
		created = ToDifficultyEntity(record)
		return nil
	})
	if err != nil {
		return err
	}
	*e = *created
	return nil
}

func (r *DifficultyRepository) Update(ctx context.Context, e *entity.Difficulty) error {
	if err := validateDifficultyWrite(r, ctx, e, true); err != nil {
		return err
	}
	var updated *entity.Difficulty
	err := r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		stored, err := client.Difficulty.Query().
			Where(difficulty.ID(e.ID), difficulty.DeletedAtIsNil()).
			Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
			Only(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, difficultyRepoName)
		}
		if stored.Version != e.Version {
			return ports.ErrDifficultyVersionConflict
		}
		if stored.Version >= problemcontract.MaximumDifficultyRevision {
			return fmt.Errorf("%s: maximum revision reached", difficultyRepoName)
		}
		nextVersion := stored.Version + 1
		checksum := difficultyRevisionChecksum(stored.ID, nextVersion, e.Name, e.Level, e.ExpReward, e.Description)
		if err := client.DifficultyRevision.Create().
			SetDifficultyID(stored.ID).
			SetRevision(nextVersion).
			SetName(e.Name).
			SetLevel(e.Level).
			SetExpReward(e.ExpReward).
			SetDescription(e.Description).
			SetChecksum(checksum[:]).
			Exec(txCtx); err != nil {
			return commonEnt.MapEntError(err, difficultyRepoName)
		}
		record, err := BuildUpdateDifficulty(client, e, nextVersion).
			Where(difficulty.Version(e.Version)).
			Save(txCtx)
		if err != nil {
			if generate.IsNotFound(err) {
				return ports.ErrDifficultyVersionConflict
			}
			return commonEnt.MapEntError(err, difficultyRepoName)
		}
		updated = ToDifficultyEntity(record)
		return nil
	})
	if err != nil {
		return err
	}
	*e = *updated
	return nil
}

func validateDifficultyWrite(r *DifficultyRepository, ctx context.Context, value *entity.Difficulty, update bool) error {
	if r == nil || r.client == nil || ctx == nil || value == nil || ctx.Err() != nil {
		return errors.New("difficulty: invalid write")
	}
	if !utf8.ValidString(value.Name) || !utf8.ValidString(value.Description) ||
		value.Name == "" || utf8.RuneCountInString(value.Name) > 50 || value.Level < 1 ||
		value.ExpReward < 0 || value.ExpReward > problemcontract.MaximumBaseExperience ||
		utf8.RuneCountInString(value.Description) > 255 || (update && (value.ID == uuid.Nil || value.Version < 1)) {
		return errors.New("difficulty: invalid authored facts")
	}
	return nil
}

func (r *DifficultyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).Difficulty.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, difficultyRepoName)
	}
	return nil
}

func (r *DifficultyRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Difficulty.Query().Where(difficulty.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, difficultyRepoName)
	}
	return exists, nil
}

func (r *DifficultyRepository) FindAll(ctx context.Context) ([]*entity.Difficulty, error) {
	return r.findAll(ctx, 0)
}

func (r *DifficultyRepository) FindAllBounded(
	ctx context.Context,
	limit int,
) ([]*entity.Difficulty, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%s: limit must be positive", difficultyRepoName)
	}
	return r.findAll(ctx, limit)
}

func (r *DifficultyRepository) findAll(ctx context.Context, limit int) ([]*entity.Difficulty, error) {
	query := r.client.DB(ctx).Difficulty.Query().Order(difficulty.ByID())
	if limit > 0 {
		query.Limit(limit)
	}
	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}
	entities := make([]*entity.Difficulty, len(records))
	for i, rec := range records {
		entities[i] = ToDifficultyEntity(rec)
	}
	return entities, nil
}
