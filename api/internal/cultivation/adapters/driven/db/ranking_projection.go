package db

import (
	"context"
	"crypto/subtle"
	stdsql "database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	cultivationRankingStateKey          = "global"
	cultivationRankingProjectionLockKey = "cultivation:ranking:projection"
)

var cultivationRankingStateID = uuid.NewSHA1(
	uuid.NameSpaceURL,
	[]byte("judgify/cultivation/ranking-state/global"),
)

type rankingSQLExecutor interface {
	ExecContext(context.Context, string, ...any) (stdsql.Result, error)
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
}

type RankingProjectionRepository struct {
	client *internalent.EntClient
}

func NewRankingProjectionRepository(client *internalent.EntClient) (*RankingProjectionRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, entity.ErrInvalidRankingQuery
	}
	return &RankingProjectionRepository{client: client}, nil
}

var _ ports.RankingProjectionRepository = (*RankingProjectionRepository)(nil)

func (repository *RankingProjectionRepository) Page(
	ctx context.Context,
	query ports.RankingProjectionQuery,
) (entity.RankingProjectionPage, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		!query.Kind.Valid() || query.Limit < 1 || query.Limit > 100 ||
		(query.Cursor != nil && (query.Cursor.Kind != query.Kind ||
			query.Cursor.ProjectionRevision < 0 || query.Cursor.Score < 0 ||
			query.Cursor.UpdatedAt.IsZero() || query.Cursor.PublicID == uuid.Nil)) {
		return entity.RankingProjectionPage{}, entity.ErrInvalidRankingQuery
	}
	scoreColumn, updatedColumn := "rating", "rating_updated_at"
	if query.Kind == entity.RankingByExperience {
		scoreColumn, updatedColumn = "total_exp", "progression_updated_at"
	}
	statement := fmt.Sprintf(`
		WITH state AS (
			SELECT COALESCE((
				SELECT revision FROM cultivation_ranking_states WHERE key = $1
			), 0)::bigint AS revision
		), ranked AS (
			SELECT ROW_NUMBER() OVER (
				ORDER BY projection.%[1]s DESC,
					COALESCE(projection.%[2]s, projection.created_at) ASC,
					projection.public_id ASC
			) AS position,
			projection.public_id, projection.visibility,
			CASE WHEN projection.visibility = 'public' THEN account.username ELSE NULL END AS display_name,
			projection.rating, projection.total_exp,
			projection.%[1]s::bigint AS score,
			COALESCE(projection.%[2]s, projection.created_at) AS score_updated_at
			FROM cultivation_ranking_projections AS projection
			JOIN users AS account ON account.id = projection.user_id
			JOIN user_stats AS source ON source.user_id = projection.user_id
			WHERE projection.visibility IN ('public', 'anonymous')
			  AND account.deleted_at IS NULL AND account.status = 'active'
			  AND source.deleted_at IS NULL
		), page AS (
			SELECT ranked.*
			FROM ranked CROSS JOIN state
			WHERE ($2::bigint = 0 OR state.revision = $2)
			  AND (NOT $3::boolean
				OR score < $4::bigint
				OR (score = $4 AND score_updated_at > $5::timestamptz)
				OR (score = $4 AND score_updated_at = $5 AND public_id > $6::uuid))
			ORDER BY score DESC, score_updated_at ASC, public_id ASC
			LIMIT $7
		)
		SELECT state.revision, ($2::bigint = 0 OR state.revision = $2) AS revision_matches,
			page.position, page.public_id, page.visibility, page.display_name,
			page.rating, page.total_exp, page.score_updated_at
		FROM state LEFT JOIN page ON TRUE
		ORDER BY page.position ASC`, scoreColumn, updatedColumn)
	expectedRevision := int64(0)
	hasCursor := query.Cursor != nil
	cursorScore := int64(0)
	cursorUpdatedAt := time.Unix(0, 0).UTC()
	cursorPublicID := uuid.Nil
	if query.Cursor != nil {
		expectedRevision = query.Cursor.ProjectionRevision
		cursorScore = query.Cursor.Score
		cursorUpdatedAt = query.Cursor.UpdatedAt.UTC()
		cursorPublicID = query.Cursor.PublicID
	}
	rows, err := repository.client.DB(ctx).CultivationRankingProjection.QueryContext(
		ctx, statement, cultivationRankingStateKey, expectedRevision,
		hasCursor, cursorScore, cursorUpdatedAt, cursorPublicID, query.Limit+1,
	)
	if err != nil {
		return entity.RankingProjectionPage{}, fmt.Errorf("query cultivation ranking projection: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := entity.RankingProjectionPage{Entries: make([]entity.RankingProjectionEntry, 0, query.Limit+1)}
	revisionMatches := true
	for rows.Next() {
		var (
			position, rating, totalExp stdsql.NullInt64
			publicID                   uuid.NullUUID
			visibility, displayName    stdsql.NullString
			updatedAt                  stdsql.NullTime
		)
		if err := rows.Scan(
			&page.ProjectionRevision, &revisionMatches, &position,
			&publicID, &visibility, &displayName, &rating, &totalExp, &updatedAt,
		); err != nil {
			return entity.RankingProjectionPage{}, fmt.Errorf("scan cultivation ranking projection: %w", err)
		}
		if !publicID.Valid {
			continue
		}
		if !position.Valid || !visibility.Valid || !rating.Valid || !totalExp.Valid ||
			!updatedAt.Valid || position.Int64 < 1 || rating.Int64 < 0 || totalExp.Int64 < 0 {
			return entity.RankingProjectionPage{}, ErrSolveRewardInvariant
		}
		entry := entity.RankingProjectionEntry{
			Position: position.Int64, PublicID: publicID.UUID,
			DisplayMode: visibility.String, Rating: int(rating.Int64),
			TotalExp: totalExp.Int64, UpdatedAt: updatedAt.Time.UTC(),
		}
		if displayName.Valid {
			value := displayName.String
			entry.DisplayName = &value
		}
		page.Entries = append(page.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return entity.RankingProjectionPage{}, fmt.Errorf("iterate cultivation ranking projection: %w", err)
	}
	if !revisionMatches {
		return entity.RankingProjectionPage{}, entity.ErrRankingRevisionChanged
	}
	if len(page.Entries) > query.Limit {
		page.HasMore = true
		page.Entries = page.Entries[:query.Limit]
	}
	return page, nil
}

func (repository *RankingProjectionRepository) Privacy(
	ctx context.Context,
	userID uuid.UUID,
) (entity.RankingPrivacy, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil {
		return entity.RankingPrivacy{}, entity.ErrInvalidRankingQuery
	}
	var privacy entity.RankingPrivacy
	privacy.UserID = userID
	err := querySingleRow(ctx, repository.client.DB(ctx).CultivationRankingProjection, `
		SELECT visibility, privacy_version, projection_revision
		FROM cultivation_ranking_projections
		WHERE user_id = $1`, userID).Scan(
		&privacy.Visibility, &privacy.Version, &privacy.ProjectionRevision,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.RankingPrivacy{}, entity.ErrRankingProjectionNotFound
	}
	if err != nil {
		return entity.RankingPrivacy{}, fmt.Errorf("read cultivation ranking privacy: %w", err)
	}
	if !privacy.Visibility.Valid() || privacy.Version < 1 || privacy.ProjectionRevision < 1 {
		return entity.RankingPrivacy{}, ErrSolveRewardInvariant
	}
	return privacy, nil
}

func (repository *RankingProjectionRepository) UpdatePrivacy(
	ctx context.Context,
	command entity.RankingPrivacyCommand,
) (entity.RankingPrivacyResult, error) {
	if repository == nil || repository.client == nil || ctx == nil {
		return entity.RankingPrivacyResult{}, entity.ErrInvalidRankingQuery
	}
	command, err := entity.NormalizeRankingPrivacyCommand(command)
	if err != nil {
		return entity.RankingPrivacyResult{}, err
	}
	executor := repository.client.DB(ctx).CultivationRankingProjection
	requestChecksum := entity.RankingPrivacyRequestChecksum(command)
	if result, found, replayErr := readRankingPrivacyReplay(ctx, executor, command, requestChecksum); replayErr != nil || found {
		return result, replayErr
	}
	if err := lockCultivationRankingProjection(ctx, executor); err != nil {
		return entity.RankingPrivacyResult{}, err
	}
	var current entity.RankingPrivacy
	current.UserID = command.UserID
	err = querySingleRow(ctx, executor, `
		SELECT visibility, privacy_version, projection_revision
		FROM cultivation_ranking_projections
		WHERE user_id = $1
		FOR UPDATE`, command.UserID).Scan(
		&current.Visibility, &current.Version, &current.ProjectionRevision,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.RankingPrivacyResult{}, entity.ErrRankingProjectionNotFound
	}
	if err != nil {
		return entity.RankingPrivacyResult{}, fmt.Errorf("lock cultivation ranking privacy: %w", err)
	}
	if result, found, replayErr := readRankingPrivacyReplay(ctx, executor, command, requestChecksum); replayErr != nil || found {
		return result, replayErr
	}
	if !current.Visibility.Valid() || current.Version != command.ExpectedVersion ||
		current.Visibility == command.Visibility {
		return entity.RankingPrivacyResult{}, entity.ErrRankingPrivacyConflict
	}
	var now time.Time
	if err := querySingleRow(ctx, executor, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return entity.RankingPrivacyResult{}, fmt.Errorf("read ranking privacy database time: %w", err)
	}
	projectionRevision, err := advanceCultivationRankingRevision(ctx, executor, now.UTC())
	if err != nil {
		return entity.RankingPrivacyResult{}, err
	}
	nextVersion := current.Version + 1
	result, err := executor.ExecContext(ctx, `
		UPDATE cultivation_ranking_projections
		SET visibility = $2, privacy_version = $3,
			projection_revision = $4, updated_at = $5
		WHERE user_id = $1 AND privacy_version = $3 - 1`,
		command.UserID, command.Visibility, nextVersion, projectionRevision, now.UTC(),
	)
	if err != nil {
		return entity.RankingPrivacyResult{}, fmt.Errorf("update cultivation ranking privacy: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return entity.RankingPrivacyResult{}, entity.ErrRankingPrivacyConflict
	}
	_, err = executor.ExecContext(ctx, `
		INSERT INTO cultivation_ranking_privacy_changes (
			id, user_id, from_visibility, to_visibility,
			from_version, to_version, projection_revision,
			request_checksum, reason, correlation_id, occurred_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`,
		command.ID, command.UserID, current.Visibility, command.Visibility,
		current.Version, nextVersion, projectionRevision,
		requestChecksum[:], command.Reason, correlation.FromContext(ctx), now.UTC(),
	)
	if err != nil {
		return entity.RankingPrivacyResult{}, fmt.Errorf("append cultivation ranking privacy change: %w", err)
	}
	return entity.RankingPrivacyResult{
		Privacy: entity.RankingPrivacy{
			UserID: command.UserID, Visibility: command.Visibility,
			Version: nextVersion, ProjectionRevision: projectionRevision,
		},
		PreviousVisibility: current.Visibility,
		UpdatedAt:          now.UTC(),
	}, nil
}

func readRankingPrivacyReplay(
	ctx context.Context,
	queryer rankingSQLExecutor,
	command entity.RankingPrivacyCommand,
	requestChecksum [32]byte,
) (entity.RankingPrivacyResult, bool, error) {
	var (
		result         entity.RankingPrivacyResult
		storedUserID   uuid.UUID
		storedChecksum []byte
		toVisibility   entity.RankingVisibility
		fromVersion    int64
	)
	err := querySingleRow(ctx, queryer, `
		SELECT user_id, from_visibility, to_visibility, from_version,
			to_version, projection_revision, request_checksum, occurred_at
		FROM cultivation_ranking_privacy_changes
		WHERE id = $1`, command.ID).Scan(
		&storedUserID, &result.PreviousVisibility, &toVisibility, &fromVersion,
		&result.Privacy.Version, &result.Privacy.ProjectionRevision,
		&storedChecksum, &result.UpdatedAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.RankingPrivacyResult{}, false, nil
	}
	if err != nil {
		return entity.RankingPrivacyResult{}, false, fmt.Errorf("read ranking privacy replay: %w", err)
	}
	if storedUserID != command.UserID || toVisibility != command.Visibility ||
		fromVersion != command.ExpectedVersion || len(storedChecksum) != 32 ||
		subtle.ConstantTimeCompare(storedChecksum, requestChecksum[:]) != 1 {
		return entity.RankingPrivacyResult{}, false, entity.ErrRankingPrivacyConflict
	}
	result.Privacy.UserID = storedUserID
	result.Privacy.Visibility = toVisibility
	result.Replayed = true
	result.UpdatedAt = result.UpdatedAt.UTC()
	return result, true, nil
}

func advanceCultivationRankingRevision(
	ctx context.Context,
	executor rankingSQLExecutor,
	now time.Time,
) (int64, error) {
	if ctx == nil || executor == nil || now.IsZero() {
		return 0, ErrSolveRewardInvariant
	}
	if err := lockCultivationRankingProjection(ctx, executor); err != nil {
		return 0, err
	}
	var revision int64
	err := querySingleRow(ctx, executor, `
		INSERT INTO cultivation_ranking_states (
			id, key, revision, created_at, updated_at
		) VALUES ($1,$2,1,$3,$3)
		ON CONFLICT (key) DO UPDATE
		SET revision = cultivation_ranking_states.revision + 1,
			updated_at = EXCLUDED.updated_at
		RETURNING revision`,
		cultivationRankingStateID, cultivationRankingStateKey, now.UTC(),
	).Scan(&revision)
	if err != nil {
		return 0, fmt.Errorf("advance cultivation ranking revision: %w", err)
	}
	if revision < 1 {
		return 0, ErrSolveRewardInvariant
	}
	return revision, nil
}

func lockCultivationRankingProjection(
	ctx context.Context,
	executor rankingSQLExecutor,
) error {
	if ctx == nil || executor == nil {
		return ErrSolveRewardInvariant
	}
	if _, err := executor.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		cultivationRankingProjectionLockKey,
	); err != nil {
		return fmt.Errorf("lock cultivation ranking projection writes: %w", err)
	}
	return nil
}

func syncCultivationRankingProjection(
	ctx context.Context,
	executor rankingSQLExecutor,
	userID uuid.UUID,
	publicID uuid.UUID,
	projectionRevision int64,
	now time.Time,
) error {
	if ctx == nil || executor == nil || userID == uuid.Nil || publicID == uuid.Nil ||
		projectionRevision < 1 || now.IsZero() {
		return ErrSolveRewardInvariant
	}
	result, err := executor.ExecContext(ctx, `
		INSERT INTO cultivation_ranking_projections (
			id, user_id, public_id, visibility, privacy_version,
			projection_revision, progression_source_version,
			rating_source_version, total_exp, rating,
			progression_updated_at, rating_updated_at, created_at, updated_at
		)
		SELECT $2, stats.user_id, $3, 'public', 1,
			$4, stats.progression_version, stats.rating_version,
			stats.total_exp, stats.rating,
			COALESCE(stats.progression_updated_at, stats.created_at),
			COALESCE(stats.rating_updated_at, stats.created_at), $5, $5
		FROM user_stats AS stats
		JOIN users AS account ON account.id = stats.user_id
		WHERE stats.user_id = $1 AND stats.deleted_at IS NULL
		  AND account.deleted_at IS NULL AND account.status = 'active'
		ON CONFLICT (user_id) DO UPDATE
		SET projection_revision = EXCLUDED.projection_revision,
			progression_source_version = EXCLUDED.progression_source_version,
			rating_source_version = EXCLUDED.rating_source_version,
			total_exp = EXCLUDED.total_exp,
			rating = EXCLUDED.rating,
			progression_updated_at = EXCLUDED.progression_updated_at,
			rating_updated_at = EXCLUDED.rating_updated_at,
			updated_at = EXCLUDED.updated_at`,
		userID, uuid.New(), publicID, projectionRevision, now.UTC(),
	)
	if err != nil {
		return fmt.Errorf("synchronize cultivation ranking projection: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect cultivation ranking projection: %w", err)
	}
	if affected != 1 {
		return ErrSolveRewardInvariant
	}
	return nil
}

func synchronizeProgressionRanking(
	ctx context.Context,
	executor rankingSQLExecutor,
	userID uuid.UUID,
	now time.Time,
) error {
	revision, err := advanceCultivationRankingRevision(ctx, executor, now)
	if err != nil {
		return err
	}
	return syncCultivationRankingProjection(
		ctx,
		executor,
		userID,
		uuid.New(),
		revision,
		now,
	)
}

func currentCultivationRankingRevision(
	ctx context.Context,
	executor rankingSQLExecutor,
) (int64, error) {
	if ctx == nil || executor == nil {
		return 0, ErrSolveRewardInvariant
	}
	var revision int64
	err := querySingleRow(ctx, executor, `
		SELECT revision
		FROM cultivation_ranking_states
		WHERE key = $1`, cultivationRankingStateKey).Scan(&revision)
	if errors.Is(err, stdsql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read cultivation ranking revision: %w", err)
	}
	if revision < 0 {
		return 0, ErrSolveRewardInvariant
	}
	return revision, nil
}
