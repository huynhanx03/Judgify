package db

import (
	"context"
	"fmt"
	"sort"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/userstats"
)

const userStatsRepoName = "User Stats"

type UserStatsRepository struct {
	client *dbEnt.EntClient
}

func NewUserStatsRepository(client *dbEnt.EntClient) ports.UserStatsRepository {
	return &UserStatsRepository{client: client}
}

func (r *UserStatsRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserStats], error) {
	query := r.client.DB(ctx).UserStats.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}

	entities := make([]*entity.UserStats, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToUserStatsEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.UserStats]{Records: &entities, Pagination: meta}, nil
}

func (r *UserStatsRepository) Get(ctx context.Context, id int) (*entity.UserStats, error) {
	rec, err := r.client.DB(ctx).UserStats.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}
	return mapper.ToUserStatsEntity(rec), nil
}

func (r *UserStatsRepository) GetByUserID(ctx context.Context, userID int) (*entity.UserStats, error) {
	rec, err := r.client.DB(ctx).UserStats.Query().
		Where(userstats.UserID(userID)).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}
	return mapper.ToUserStatsEntity(rec), nil
}

func (r *UserStatsRepository) GetByUserIDs(ctx context.Context, userIDs []int) (map[int]*entity.UserStats, error) {
	statsByUserID := make(map[int]*entity.UserStats, len(userIDs))
	if len(userIDs) == 0 {
		return statsByUserID, nil
	}

	records, err := r.client.DB(ctx).UserStats.Query().
		Where(userstats.UserIDIn(userIDs...)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}

	for _, rec := range records {
		stats := mapper.ToUserStatsEntity(rec)
		if stats != nil {
			statsByUserID[stats.UserID] = stats
		}
	}
	return statsByUserID, nil
}

func (r *UserStatsRepository) Create(ctx context.Context, e *entity.UserStats) error {
	rec, err := builder.BuildCreateUserStats(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	if created := mapper.ToUserStatsEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

// UpdateRatings updates many user ratings in a single bulk upsert statement.
func (r *UserStatsRepository) UpdateRatings(ctx context.Context, ratingsByUserID map[int]int) error {
	if len(ratingsByUserID) == 0 {
		return nil
	}

	userIDs := make([]int, 0, len(ratingsByUserID))
	for userID := range ratingsByUserID {
		userIDs = append(userIDs, userID)
	}
	sort.Ints(userIDs)

	builders := make([]*generate.UserStatsCreate, 0, len(userIDs))
	for _, userID := range userIDs {
		builders = append(
			builders,
			r.client.DB(ctx).UserStats.Create().
				SetUserID(userID).
				SetRating(ratingsByUserID[userID]),
		)
	}

	err := r.client.DB(ctx).UserStats.CreateBulk(builders...).
		OnConflictColumns(userstats.FieldUserID).
		UpdateRating().
		Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	return nil
}

func (r *UserStatsRepository) Update(ctx context.Context, e *entity.UserStats) error {
	rec, err := builder.BuildUpdateUserStats(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *UserStatsRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).UserStats.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	return nil
}

func (r *UserStatsRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).UserStats.Query().Where(userstats.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, userStatsRepoName)
	}
	return exists, nil
}

// IncrementSubmission atomically increments total_submissions (and accepted_count when accepted).
func (r *UserStatsRepository) IncrementSubmission(ctx context.Context, userID int, accepted bool) error {
	client := r.client.DB(ctx)
	upd := client.UserStats.Update().
		Where(userstats.UserID(userID)).
		AddTotalSubmissions(1)
	if accepted {
		upd = upd.AddAcceptedCount(1)
	}
	_, err := upd.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	return nil
}

// GetTopSortedWithUser returns top N user stats sorted by field with eager-loaded username.
func (r *UserStatsRepository) GetTopSortedWithUser(ctx context.Context, sortField string, limit int) ([]*entity.UserStatsWithUser, error) {
	query := r.client.DB(ctx).UserStats.Query().WithUser()

	// Apply ORDER BY descending
	orderCol := userstats.FieldRating
	switch sortField {
	case "total_exp":
		orderCol = userstats.FieldTotalExp
	case "rating":
		orderCol = userstats.FieldRating
	}
	query.Order(func(s *sql.Selector) {
		s.OrderBy(sql.Desc(orderCol))
	}).Limit(limit)

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userStatsRepoName)
	}

	results := make([]*entity.UserStatsWithUser, len(records))
	for i, rec := range records {
		entry := &entity.UserStatsWithUser{
			Stats: mapper.ToUserStatsEntity(rec),
		}
		if u, err := rec.Edges.UserOrErr(); err == nil {
			entry.Username = u.Username
		}
		results[i] = entry
	}
	return results, nil
}

// AddExp atomically adds EXP to user stats. Row must exist (created at registration).
func (r *UserStatsRepository) AddExp(ctx context.Context, userID int, exp int64) error {
	client := r.client.DB(ctx)

	n, err := client.UserStats.Update().
		Where(userstats.UserID(userID)).
		AddTotalExp(exp).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	if n == 0 {
		return commonEnt.MapEntError(fmt.Errorf("user stats not found for user_id %d", userID), userStatsRepoName)
	}
	return nil
}
