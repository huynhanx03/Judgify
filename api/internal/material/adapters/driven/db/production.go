package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type ProductionRepository struct{ client *internalent.EntClient }

func NewProductionRepository(client *internalent.EntClient) (*ProductionRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	return &ProductionRepository{client: client}, nil
}

var (
	_ ports.MaterialEditorialRepository = (*ProductionRepository)(nil)
	_ ports.MaterialQueryRepository     = (*ProductionRepository)(nil)
	_ ports.MaterialViewRepository      = (*ProductionRepository)(nil)
)

func (repository *ProductionRepository) CreateIdentity(ctx context.Context, material entity.Material, slug entity.MaterialSlug) error {
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	if material.ID == uuid.Nil || material.AuthorID == uuid.Nil || material.Version != 1 ||
		material.CurrentSlugID != slug.ID || material.CurrentDraftID == uuid.Nil ||
		material.State() != entity.MaterialDraft ||
		!entity.MaterialVisibility(material.Visibility).Valid() ||
		slug.ID == uuid.Nil || slug.MaterialID != material.ID || slug.State != "current" ||
		material.CreatedAt.IsZero() || !material.CreatedAt.Equal(material.UpdatedAt) {
		return entity.ErrMaterialInvalidCommand
	}
	normalizedSlug, err := entity.NormalizeSlug(slug.Slug)
	if err != nil || normalizedSlug != slug.Slug || slug.CreatedAt.IsZero() {
		return entity.ErrMaterialInvalidCommand
	}
	// materials and material_slugs reference each other, so neither row can be
	// written with the pointer already set. current_slug_id starts NULL, which
	// leaves the composite foreign key unenforced under MATCH SIMPLE, and the
	// pointer is claimed once the slug row exists. All three statements share
	// the caller's transaction, so no other reader observes the gap.
	_, err = repository.client.DB(ctx).Material.ExecContext(ctx, `
		INSERT INTO materials (
			id, current_slug_id, current_slug_state, author_id, status, visibility, view_count, version,
			created_at, updated_at
		) VALUES ($1,NULL,'current',$2,'draft',$3,0,1,$4,$4)`,
		material.ID, material.AuthorID, material.Visibility, material.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert material identity: %w", err)
	}
	_, err = repository.client.DB(ctx).Material.ExecContext(ctx, `
		INSERT INTO material_slugs (id, material_id, slug, state, created_at)
		VALUES ($1,$2,$3,'current',$4)`,
		slug.ID, material.ID, slug.Slug, slug.CreatedAt.UTC(),
	)
	if err != nil {
		return mapSlugWriteError(err)
	}
	result, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
		UPDATE materials
		SET current_slug_id = $2
		WHERE id = $1 AND current_slug_id IS NULL`,
		material.ID, slug.ID,
	)
	if err != nil {
		return fmt.Errorf("claim material current slug: %w", err)
	}
	claimed, err := result.RowsAffected()
	if err != nil || claimed != 1 {
		return entity.ErrMaterialInvalidCommand
	}
	return nil
}

func (repository *ProductionRepository) LoadAggregate(
	ctx context.Context,
	materialID uuid.UUID,
) (entity.Material, error) {
	return repository.loadAggregate(ctx, materialID, false)
}

func (repository *ProductionRepository) Lock(
	ctx context.Context,
	materialID uuid.UUID,
) (entity.Material, error) {
	return repository.loadAggregate(ctx, materialID, true)
}

func (repository *ProductionRepository) loadAggregate(
	ctx context.Context,
	materialID uuid.UUID,
	lock bool,
) (entity.Material, error) {
	if materialID == uuid.Nil {
		return entity.Material{}, entity.ErrMaterialInvalidCommand
	}
	if lock {
		if _, err := platformtx.Client(ctx, nil); err != nil {
			return entity.Material{}, err
		}
	}
	query := `
		SELECT m.id, m.current_slug_id, s.slug, m.author_id, m.status, m.visibility,
		       m.current_draft_revision_id, m.published_revision_id,
		       m.published_render_artifact_id, m.view_count, m.version,
		       m.published_at, m.archived_at, m.created_at, m.updated_at
		FROM materials m
		JOIN material_slugs s ON s.id = m.current_slug_id
		 AND s.material_id = m.id AND s.state = 'current'
		WHERE m.id = $1 AND m.deleted_at IS NULL`
	if lock {
		query += ` FOR UPDATE OF m`
	}
	rows, err := repository.client.DB(ctx).Material.QueryContext(
		ctx,
		query,
		materialID,
	)
	if err != nil {
		return entity.Material{}, fmt.Errorf("load material aggregate: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAggregate(rows)
}

func (repository *ProductionRepository) NextRevisionNumber(ctx context.Context, materialID uuid.UUID) (int64, error) {
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		SELECT COALESCE(MAX(revision_number), 0) + 1
		FROM material_revisions WHERE material_id = $1`, materialID)
	if err != nil {
		return 0, fmt.Errorf("query material revision number: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var number int64
	if !rows.Next() || rows.Scan(&number) != nil || number < 1 || number > 1_000_000_000 {
		return 0, entity.ErrMaterialInvalidCommand
	}
	return number, rows.Err()
}

func (repository *ProductionRepository) StoreRevision(ctx context.Context, revision entity.MaterialRevision) error {
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	_, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
		INSERT INTO material_revisions (
			id, material_id, revision_number, title, description, content_markdown,
			category_id, difficulty_id, estimated_read_minutes, author_id, created_at,
			change_summary, normalized_source_checksum, render_source_id,
			render_source_kind, render_artifact_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'material_body',$15)`,
		revision.ID, revision.MaterialID, revision.RevisionNumber,
		revision.Title, revision.Description, string(revision.ContentMarkdown),
		revision.CategoryID, revision.DifficultyID, revision.EstimatedReadMinutes,
		revision.AuthorID, revision.CreatedAt.UTC(), revision.ChangeSummary,
		revision.NormalizedSourceChecksum[:], revision.RenderSourceID, revision.RenderArtifactID,
	)
	if err != nil {
		return fmt.Errorf("insert immutable material revision: %w", err)
	}
	for order, tagID := range revision.TagIDs {
		if _, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
			INSERT INTO material_revision_tags (revision_id, tag_id, display_order)
			VALUES ($1,$2,$3)`, revision.ID, tagID, order); err != nil {
			return fmt.Errorf("insert material revision tag: %w", err)
		}
	}
	return nil
}

func (repository *ProductionRepository) AdvanceDraft(
	ctx context.Context,
	materialID uuid.UUID,
	expectedVersion int64,
	revision entity.MaterialRevision,
	visibility entity.MaterialVisibility,
) (entity.Material, error) {
	result, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
		UPDATE materials
		SET current_draft_revision_id = $3, visibility = $4, version = version + 1,
		    status = CASE WHEN status = 'in_review' THEN 'draft' ELSE status END,
		    updated_at = clock_timestamp()
		WHERE id = $1 AND version = $2 AND status <> 'archived' AND deleted_at IS NULL`,
		materialID, expectedVersion, revision.ID, visibility,
	)
	if err != nil {
		return entity.Material{}, fmt.Errorf("advance material draft: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return entity.Material{}, entity.ErrMaterialConflict
	}
	return repository.Lock(ctx, materialID)
}

func (repository *ProductionRepository) LoadRevision(ctx context.Context, materialID, revisionID uuid.UUID) (entity.MaterialRevision, error) {
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		SELECT r.id, r.material_id, r.revision_number, r.title, r.description,
		       r.content_markdown, r.category_id, r.difficulty_id,
		       r.estimated_read_minutes, r.author_id, r.created_at, r.change_summary,
		       r.normalized_source_checksum, r.render_source_id,
		       r.render_source_kind, r.render_artifact_id,
		       COALESCE(jsonb_agg(rt.tag_id ORDER BY rt.display_order)
		           FILTER (WHERE rt.tag_id IS NOT NULL), '[]'::jsonb)
		FROM material_revisions r
		LEFT JOIN material_revision_tags rt ON rt.revision_id = r.id
		WHERE r.material_id = $1 AND r.id = $2
		GROUP BY r.id`, materialID, revisionID)
	if err != nil {
		return entity.MaterialRevision{}, fmt.Errorf("load material revision: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.MaterialRevision{}, entity.ErrMaterialNotFound
	}
	var revision entity.MaterialRevision
	var checksum []byte
	var kind string
	var tagJSON []byte
	if err := rows.Scan(
		&revision.ID, &revision.MaterialID, &revision.RevisionNumber,
		&revision.Title, &revision.Description, &revision.ContentMarkdown,
		&revision.CategoryID, &revision.DifficultyID, &revision.EstimatedReadMinutes,
		&revision.AuthorID, &revision.CreatedAt, &revision.ChangeSummary,
		&checksum, &revision.RenderSourceID, &kind, &revision.RenderArtifactID, &tagJSON,
	); err != nil {
		return entity.MaterialRevision{}, fmt.Errorf("scan material revision: %w", err)
	}
	if len(checksum) != 32 {
		return entity.MaterialRevision{}, entity.ErrMaterialInvalidCommand
	}
	copy(revision.NormalizedSourceChecksum[:], checksum)
	revision.RenderSourceKind = contentrender.SourceKind(kind)
	if err := json.Unmarshal(tagJSON, &revision.TagIDs); err != nil {
		return entity.MaterialRevision{}, entity.ErrMaterialInvalidCommand
	}
	if err := revision.Validate(); err != nil {
		return entity.MaterialRevision{}, err
	}
	return revision, rows.Err()
}

func (repository *ProductionRepository) Publish(
	ctx context.Context,
	materialID uuid.UUID,
	expectedVersion int64,
	revision entity.MaterialRevision,
	publishedAt time.Time,
	rendererVersion string,
	sanitizerVersion string,
) (entity.Material, error) {
	result, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
		UPDATE materials m
		SET published_revision_id = r.id,
		    published_render_source_id = r.render_source_id,
		    published_render_source_kind = r.render_source_kind,
		    published_source_checksum = r.normalized_source_checksum,
		    published_render_artifact_id = r.render_artifact_id,
		    published_at = $4, archived_at = NULL, status = 'published',
		    version = m.version + 1, updated_at = clock_timestamp()
		FROM material_revisions r
		JOIN content_render_artifacts a
		  ON a.id = r.render_artifact_id
		 AND a.render_source_id = r.render_source_id
		 AND a.render_source_kind = r.render_source_kind
		 AND a.source_checksum = r.normalized_source_checksum
		 AND a.status = 'ready'
		 AND a.renderer_version = $5
		 AND a.sanitizer_policy_version = $6
		WHERE m.id = $1 AND m.version = $2 AND m.status <> 'archived'
		  AND r.id = $3 AND r.material_id = m.id AND m.deleted_at IS NULL`,
		materialID, expectedVersion, revision.ID, publishedAt.UTC(),
		rendererVersion, sanitizerVersion,
	)
	if err != nil {
		return entity.Material{}, fmt.Errorf("publish material revision: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return entity.Material{}, entity.ErrMaterialConflict
	}
	return repository.Lock(ctx, materialID)
}

func (repository *ProductionRepository) Archive(ctx context.Context, materialID uuid.UUID, expectedVersion int64, archivedAt time.Time) (entity.Material, error) {
	result, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
		UPDATE materials SET status = 'archived', archived_at = $3,
		    version = version + 1, updated_at = clock_timestamp()
		WHERE id = $1 AND version = $2 AND status <> 'archived' AND deleted_at IS NULL`,
		materialID, expectedVersion, archivedAt.UTC(),
	)
	if err != nil {
		return entity.Material{}, fmt.Errorf("archive material: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return entity.Material{}, entity.ErrMaterialConflict
	}
	return repository.Lock(ctx, materialID)
}

func (repository *ProductionRepository) TransitionReview(ctx context.Context, materialID uuid.UUID, expectedVersion int64, state entity.MaterialState) (entity.Material, error) {
	if state != entity.MaterialDraft && state != entity.MaterialInReview {
		return entity.Material{}, entity.ErrMaterialInvalidCommand
	}
	result, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
		UPDATE materials SET status = $3, version = version + 1,
		    updated_at = clock_timestamp()
		WHERE id = $1 AND version = $2 AND status <> 'archived' AND deleted_at IS NULL`,
		materialID, expectedVersion, string(state),
	)
	if err != nil {
		return entity.Material{}, fmt.Errorf("transition material review: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return entity.Material{}, entity.ErrMaterialConflict
	}
	return repository.Lock(ctx, materialID)
}

func (repository *ProductionRepository) Rename(
	ctx context.Context,
	materialID uuid.UUID,
	expectedVersion int64,
	slug entity.MaterialSlug,
	retiredAt time.Time,
) (entity.Material, error) {
	locked, err := repository.Lock(ctx, materialID)
	if err != nil {
		return entity.Material{}, err
	}
	if locked.Version != expectedVersion || locked.Slug == slug.Slug {
		return entity.Material{}, entity.ErrMaterialConflict
	}
	result, err := repository.client.DB(ctx).Material.ExecContext(ctx, `
		WITH retired AS (
			UPDATE material_slugs
			SET state = 'retired', retired_at = $3
			WHERE id = $1 AND material_id = $2 AND state = 'current'
			RETURNING material_id
		), inserted AS (
			INSERT INTO material_slugs (id, material_id, slug, state, created_at)
			SELECT $4, retired.material_id, $5, 'current', $6
			FROM retired
			RETURNING id, material_id
		)
		UPDATE materials material
		SET current_slug_id = inserted.id, current_slug_state = 'current',
		    version = material.version + 1, updated_at = clock_timestamp()
		FROM inserted
		WHERE material.id = inserted.material_id AND material.id = $2
		  AND material.version = $7 AND material.deleted_at IS NULL`,
		locked.CurrentSlugID, materialID, retiredAt.UTC(), slug.ID,
		slug.Slug, slug.CreatedAt.UTC(), expectedVersion,
	)
	if err != nil {
		return entity.Material{}, mapSlugWriteError(err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return entity.Material{}, entity.ErrMaterialConflict
	}
	return repository.Lock(ctx, materialID)
}

type materialCursor struct {
	Sort  string    `json:"sort"`
	Score float64   `json:"score,omitempty"`
	Time  time.Time `json:"time,omitempty"`
	Count int64     `json:"count,omitempty"`
	ID    uuid.UUID `json:"id"`
}

func (repository *ProductionRepository) SearchPublic(ctx context.Context, query ports.SearchQuery) (ports.SearchResult, error) {
	cursor, err := decodeMaterialCursor(query.Cursor, query.Sort)
	if err != nil {
		return ports.SearchResult{}, err
	}
	arguments := []any{
		query.Authenticated, query.Text,
		contentrender.CurrentRendererVersion,
		contentrender.CurrentSanitizerPolicyVersion,
		nullableUUID(query.CategoryID), nullableUUID(query.DifficultyID),
		pq.Array(uuidStrings(query.TagIDs)), query.Limit + 1,
	}
	cursorPredicate := ""
	order := "p.published_at DESC, p.material_id DESC"
	switch query.Sort {
	case "relevance":
		order = "p.score DESC, p.material_id DESC"
		if cursor != nil {
			arguments = append(arguments, cursor.Score, cursor.ID)
			cursorPredicate = fmt.Sprintf(" AND (p.score, p.material_id) < ($%d, $%d)", len(arguments)-1, len(arguments))
		}
	case "popularity":
		order = "p.view_count DESC, p.material_id DESC"
		if cursor != nil {
			arguments = append(arguments, cursor.Count, cursor.ID)
			cursorPredicate = fmt.Sprintf(" AND (p.view_count, p.material_id) < ($%d, $%d)", len(arguments)-1, len(arguments))
		}
	default:
		if cursor != nil {
			arguments = append(arguments, cursor.Time, cursor.ID)
			cursorPredicate = fmt.Sprintf(" AND (p.published_at, p.material_id) < ($%d, $%d)", len(arguments)-1, len(arguments))
		}
	}
	statement := publicProjectionCTE() + `
		SELECT ` + publicProjectionColumns("p") + `
		FROM projection p
		WHERE ($2 = '' OR p.document @@ websearch_to_tsquery('simple', $2))
		  AND ($5::uuid IS NULL OR p.category_id = $5)
		  AND ($6::uuid IS NULL OR p.difficulty_id = $6)
		  AND (cardinality($7::uuid[]) = 0 OR p.tag_ids @> $7::uuid[])` +
		cursorPredicate + ` ORDER BY ` + order + ` LIMIT $8`
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return ports.SearchResult{}, fmt.Errorf("search public materials: %w", err)
	}
	defer func() { _ = rows.Close() }()
	records := make([]entity.PublicMaterial, 0, query.Limit+1)
	scores := make([]float64, 0, query.Limit+1)
	for rows.Next() {
		record, score, err := scanPublicMaterial(rows)
		if err != nil {
			return ports.SearchResult{}, err
		}
		records, scores = append(records, record), append(scores, score)
	}
	if err := rows.Err(); err != nil {
		return ports.SearchResult{}, err
	}
	result := ports.SearchResult{Records: records}
	if len(records) > query.Limit {
		last, score := records[query.Limit-1], scores[query.Limit-1]
		result.Records = records[:query.Limit]
		result.NextCursor, err = encodeMaterialCursor(materialCursor{
			Sort: query.Sort, Score: score, Time: last.PublishedAt,
			Count: last.ViewCount, ID: last.MaterialID,
		})
	}
	return result, err
}

func (repository *ProductionRepository) SearchAdmin(ctx context.Context, query ports.SearchQuery) (ports.SearchResult, error) {
	cursor, err := decodeMaterialCursor(query.Cursor, query.Sort)
	if err != nil {
		return ports.SearchResult{}, err
	}
	var cursorTime any
	var cursorID any
	if cursor != nil {
		cursorTime = cursor.Time.UTC()
		cursorID = cursor.ID
	}
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		SELECT m.id, s.slug, r.title, r.description, ''::text AS sanitized_html,
		       r.category_id, c.name, r.difficulty_id, d.name, d.level,
		       COALESCE(tags.tags_json, '[]'::jsonb), r.author_id, u.username,
		       r.estimated_read_minutes, COALESCE(m.published_at, r.created_at),
		       m.view_count, m.version, r.id, r.revision_number, a.id,
		       a.renderer_version, a.sanitizer_policy_version, m.visibility,
		       0::float8, m.status, ''::text AS content_markdown,
		       m.created_at, m.updated_at,
		       m.published_revision_id, published.revision_number
		FROM materials m
		JOIN material_slugs s ON s.id = m.current_slug_id AND s.material_id = m.id AND s.state = 'current'
		JOIN material_revisions r ON r.id = m.current_draft_revision_id AND r.material_id = m.id
		JOIN content_render_artifacts a ON a.id = r.render_artifact_id
		 AND a.render_source_id = r.render_source_id
		 AND a.source_checksum = r.normalized_source_checksum
		JOIN material_categories c ON c.id = r.category_id
		JOIN difficulties d ON d.id = r.difficulty_id
		JOIN users u ON u.id = r.author_id
		LEFT JOIN material_revisions published
		  ON published.id = m.published_revision_id
		 AND published.material_id = m.id
		CROSS JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object('id', t.id, 'name', t.name) ORDER BY rt.display_order) AS tags_json
			FROM material_revision_tags rt JOIN tags t ON t.id = rt.tag_id
			WHERE rt.revision_id = r.id
		) tags
		WHERE m.deleted_at IS NULL
		  AND ($1 = '' OR to_tsvector('simple', r.title || ' ' || r.description)
		       @@ websearch_to_tsquery('simple', $1))
		  AND ($2::uuid IS NULL OR r.category_id = $2)
		  AND ($3::uuid IS NULL OR r.difficulty_id = $3)
		  AND (cardinality($4::uuid[]) = 0 OR NOT EXISTS (
		    SELECT 1 FROM unnest($4::uuid[]) required_tag
		    WHERE NOT EXISTS (
		      SELECT 1 FROM material_revision_tags mrt
		      WHERE mrt.revision_id = r.id AND mrt.tag_id = required_tag
		    )
		  ))
		  AND ($5::text IS NULL OR m.status = $5)
		  AND ($6::timestamptz IS NULL OR m.updated_at < $6
		       OR (m.updated_at = $6 AND m.id < $7))
		ORDER BY m.updated_at DESC, m.id DESC
		LIMIT $8`, query.Text, nullableUUID(query.CategoryID),
		nullableUUID(query.DifficultyID), pq.Array(uuidStrings(query.TagIDs)),
		nullableMaterialState(query.Status), cursorTime, cursorID, query.Limit+1)
	if err != nil {
		return ports.SearchResult{}, fmt.Errorf("search admin materials: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := ports.SearchResult{Records: make([]entity.PublicMaterial, 0, query.Limit+1)}
	updatedTimes := make([]time.Time, 0, query.Limit+1)
	for rows.Next() {
		var status, markdown string
		var createdAt, updatedAt time.Time
		var publishedRevisionID uuid.NullUUID
		var publishedRevisionNumber sql.NullInt64
		record, _, err := scanPublicMaterialWithSuffix(rows, func() []any {
			return []any{
				&status, &markdown, &createdAt, &updatedAt,
				&publishedRevisionID, &publishedRevisionNumber,
			}
		})
		if err != nil {
			return ports.SearchResult{}, err
		}
		record.Status = entity.MaterialState(status)
		record.ContentMarkdown = markdown
		record.CreatedAt = createdAt
		record.UpdatedAt = updatedAt
		setPublishedRevision(
			&record,
			publishedRevisionID,
			publishedRevisionNumber,
		)
		result.Records = append(result.Records, record)
		updatedTimes = append(updatedTimes, updatedAt)
	}
	if err := rows.Err(); err != nil {
		return ports.SearchResult{}, err
	}
	if len(result.Records) > query.Limit {
		lastIndex := query.Limit - 1
		result.NextCursor, err = encodeMaterialCursor(materialCursor{
			Sort: query.Sort, Time: updatedTimes[lastIndex],
			ID: result.Records[lastIndex].MaterialID,
		})
		result.Records = result.Records[:query.Limit]
	}
	return result, err
}

func (repository *ProductionRepository) ResolvePublicSlug(ctx context.Context, slug string, authenticated bool) (entity.SlugResolution, error) {
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, publicProjectionCTE()+`
		SELECT `+publicProjectionColumns("p")+`, requested.state
		FROM material_slugs requested
		JOIN projection p ON p.material_id = requested.material_id
		WHERE requested.slug = $5`, authenticated, "",
		contentrender.CurrentRendererVersion,
		contentrender.CurrentSanitizerPolicyVersion,
		slug)
	if err != nil {
		return entity.SlugResolution{}, fmt.Errorf("resolve material slug: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.SlugResolution{}, entity.ErrMaterialNotFound
	}
	var state string
	material, _, err := scanPublicMaterialWithSuffix(rows, func() []any {
		return []any{&state}
	})
	if err != nil {
		return entity.SlugResolution{}, err
	}
	return entity.SlugResolution{
		Material: material, Redirect: state == "retired", CanonicalSlug: material.Slug,
	}, nil
}

func (repository *ProductionRepository) RelatedPublic(ctx context.Context, materialID uuid.UUID, authenticated bool, limit int) ([]entity.PublicMaterial, error) {
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, publicProjectionCTE()+`
		, target AS (
			SELECT category_id, difficulty_id, tag_ids FROM projection WHERE material_id = $5
		)
		SELECT `+publicProjectionColumns("p")+`
		FROM projection p CROSS JOIN target t
		WHERE p.material_id <> $5
		ORDER BY (
			CASE WHEN p.category_id = t.category_id THEN 4 ELSE 0 END +
			CASE WHEN p.difficulty_id = t.difficulty_id THEN 2 ELSE 0 END +
			(SELECT COUNT(*) FROM unnest(p.tag_ids) candidate
			 WHERE candidate = ANY(t.tag_ids)) * 3
		) DESC, p.published_at DESC, p.material_id DESC
		LIMIT $6`, authenticated, "",
		contentrender.CurrentRendererVersion,
		contentrender.CurrentSanitizerPolicyVersion,
		materialID, limit)
	if err != nil {
		return nil, fmt.Errorf("query related materials: %w", err)
	}
	defer func() { _ = rows.Close() }()
	records := make([]entity.PublicMaterial, 0, limit)
	for rows.Next() {
		record, _, err := scanPublicMaterial(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (repository *ProductionRepository) ListRevisions(ctx context.Context, materialID uuid.UUID, before int64, limit int) ([]entity.MaterialRevisionSummary, error) {
	if limit < 1 || limit > 101 {
		return nil, entity.ErrMaterialInvalidCommand
	}
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		SELECT r.id, r.material_id, r.revision_number, r.title, r.description,
		       r.category_id, r.difficulty_id, r.estimated_read_minutes,
		       r.author_id, r.created_at, r.change_summary,
		       r.normalized_source_checksum, r.render_source_id,
		       r.render_artifact_id,
		       COALESCE(jsonb_agg(rt.tag_id ORDER BY rt.display_order)
		         FILTER (WHERE rt.tag_id IS NOT NULL), '[]'::jsonb)
		FROM material_revisions r
		LEFT JOIN material_revision_tags rt ON rt.revision_id = r.id
		WHERE r.material_id = $1
		  AND ($2::bigint = 0 OR r.revision_number < $2)
		GROUP BY r.id
		ORDER BY r.revision_number DESC
		LIMIT $3`, materialID, before, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	revisions := make([]entity.MaterialRevisionSummary, 0, limit)
	for rows.Next() {
		var revision entity.MaterialRevisionSummary
		var checksum []byte
		var tagJSON []byte
		if err := rows.Scan(
			&revision.ID, &revision.MaterialID, &revision.RevisionNumber,
			&revision.Title, &revision.Description, &revision.CategoryID,
			&revision.DifficultyID, &revision.EstimatedReadMinutes,
			&revision.AuthorID, &revision.CreatedAt, &revision.ChangeSummary,
			&checksum, &revision.RenderSourceID, &revision.RenderArtifactID,
			&tagJSON,
		); err != nil {
			return nil, err
		}
		if len(checksum) != 32 ||
			json.Unmarshal(tagJSON, &revision.TagIDs) != nil {
			return nil, entity.ErrMaterialInvalidCommand
		}
		copy(revision.NormalizedSourceChecksum[:], checksum)
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func (repository *ProductionRepository) RecordDailyView(
	ctx context.Context,
	materialID uuid.UUID,
	day time.Time,
	fingerprint [32]byte,
	authenticated bool,
) (entity.DailyViewReceipt, error) {
	recordID := uuid.NewSHA1(uuid.NameSpaceOID, fingerprint[:])
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		WITH visible AS (
			SELECT id
			FROM materials
			WHERE id = $1 AND status = 'published' AND deleted_at IS NULL
			  AND (visibility = 'public' OR ($4 AND visibility = 'authenticated'))
		), accepted AS (
			INSERT INTO material_view_dailies (
				id, material_id, view_day, viewer_fingerprint, created_at
			)
			SELECT $5, visible.id, $2::date, $3, clock_timestamp()
			FROM visible
			ON CONFLICT (material_id, view_day, viewer_fingerprint) DO NOTHING
			RETURNING id, material_id, view_day
		)
		SELECT id, material_id, view_day, true FROM accepted
		UNION ALL
		SELECT receipt.id, receipt.material_id, receipt.view_day, false
		FROM material_view_dailies receipt
		JOIN visible ON visible.id = receipt.material_id
		WHERE receipt.material_id = $1 AND receipt.view_day = $2::date
		  AND receipt.viewer_fingerprint = $3
		  AND NOT EXISTS (SELECT 1 FROM accepted)
		LIMIT 1`,
		materialID, day.UTC(), fingerprint[:], authenticated, recordID)
	if err != nil {
		return entity.DailyViewReceipt{}, fmt.Errorf("record material daily view: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var receipt entity.DailyViewReceipt
	if !rows.Next() {
		return entity.DailyViewReceipt{}, entity.ErrMaterialNotFound
	}
	if err := rows.Scan(
		&receipt.ID, &receipt.MaterialID, &receipt.ViewDay, &receipt.Inserted,
	); err != nil {
		return entity.DailyViewReceipt{}, err
	}
	return receipt, rows.Err()
}

func (repository *ProductionRepository) ApplyDailyViewProjection(
	ctx context.Context,
	receiptID uuid.UUID,
	materialID uuid.UUID,
) (bool, error) {
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		WITH accepted AS (
			UPDATE material_view_dailies
			SET projected_at = clock_timestamp()
			WHERE id = $1 AND material_id = $2 AND projected_at IS NULL
			RETURNING material_id
		), advanced AS (
			UPDATE materials m
			SET view_count = m.view_count + 1
			FROM accepted a
			WHERE m.id = a.material_id
			RETURNING m.id
		)
		SELECT EXISTS (SELECT 1 FROM advanced)`,
		receiptID, materialID)
	if err != nil {
		return false, fmt.Errorf("project material daily view: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var applied bool
	if !rows.Next() {
		return false, sql.ErrNoRows
	}
	if err := rows.Scan(&applied); err != nil {
		return false, err
	}
	return applied, rows.Err()
}

func (repository *ProductionRepository) ReconcileDailyViewProjection(
	ctx context.Context,
	limit int,
) (int, error) {
	if limit < 1 || limit > 1000 {
		return 0, entity.ErrMaterialInvalidCommand
	}
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id, material_id
			FROM material_view_dailies
			WHERE projected_at IS NULL
			ORDER BY view_day, id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		), accepted AS (
			UPDATE material_view_dailies receipt
			SET projected_at = clock_timestamp()
			FROM candidates candidate
			WHERE receipt.id = candidate.id
			  AND receipt.material_id = candidate.material_id
			  AND receipt.projected_at IS NULL
			RETURNING receipt.material_id
		), deltas AS (
			SELECT material_id, COUNT(*)::bigint AS view_delta
			FROM accepted
			GROUP BY material_id
		), advanced AS (
			UPDATE materials material
			SET view_count = material.view_count + delta.view_delta
			FROM deltas delta
			WHERE material.id = delta.material_id
			RETURNING delta.view_delta
		)
		SELECT COALESCE(SUM(view_delta), 0)::bigint FROM advanced`,
		limit)
	if err != nil {
		return 0, fmt.Errorf("reconcile material daily views: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var reconciled int64
	if !rows.Next() {
		return 0, sql.ErrNoRows
	}
	if err := rows.Scan(&reconciled); err != nil {
		return 0, err
	}
	if reconciled < 0 || reconciled > int64(limit) {
		return 0, entity.ErrMaterialInvalidCommand
	}
	return int(reconciled), rows.Err()
}

func (repository *ProductionRepository) LoadCommandReceipt(
	ctx context.Context,
	commandID uuid.UUID,
) (entity.MaterialCommandReceipt, bool, error) {
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		SELECT id, event_type, aggregate_type, aggregate_id, aggregate_version, payload, occurred_at
		FROM outbox_events
		WHERE id = $1`,
		commandID)
	if err != nil {
		return entity.MaterialCommandReceipt{}, false, fmt.Errorf(
			"load material command receipt: %w", err,
		)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.MaterialCommandReceipt{}, false, rows.Err()
	}
	var receipt entity.MaterialCommandReceipt
	if err := rows.Scan(
		&receipt.EventID, &receipt.EventType, &receipt.AggregateType, &receipt.AggregateID,
		&receipt.Version, &receipt.Payload, &receipt.OccurredAt,
	); err != nil {
		return entity.MaterialCommandReceipt{}, false, err
	}
	return receipt, true, rows.Err()
}

func (repository *ProductionRepository) LoadAdminProjection(
	ctx context.Context,
	materialID uuid.UUID,
) (entity.PublicMaterial, error) {
	rows, err := repository.client.DB(ctx).Material.QueryContext(ctx, `
		SELECT m.id, s.slug, r.title, r.description, a.sanitized_html,
		       r.category_id, c.name, r.difficulty_id, d.name, d.level,
		       COALESCE(tags.tags_json, '[]'::jsonb), r.author_id, u.username,
		       r.estimated_read_minutes, COALESCE(m.published_at, r.created_at),
		       m.view_count, m.version, r.id, r.revision_number, a.id,
		       a.renderer_version, a.sanitizer_policy_version, m.visibility,
		       0::float8, m.status, r.content_markdown, m.created_at, m.updated_at,
		       m.published_revision_id, published.revision_number
		FROM materials m
		JOIN material_slugs s
		  ON s.id = m.current_slug_id AND s.material_id = m.id
		 AND s.state = 'current'
		JOIN material_revisions r
		  ON r.id = m.current_draft_revision_id AND r.material_id = m.id
		JOIN content_render_artifacts a ON a.id = r.render_artifact_id
		 AND a.render_source_id = r.render_source_id
		 AND a.render_source_kind = r.render_source_kind
		 AND a.source_checksum = r.normalized_source_checksum
		JOIN material_categories c ON c.id = r.category_id
		JOIN difficulties d ON d.id = r.difficulty_id
		JOIN users u ON u.id = r.author_id
		LEFT JOIN material_revisions published
		  ON published.id = m.published_revision_id
		 AND published.material_id = m.id
		CROSS JOIN LATERAL (
			SELECT COALESCE(jsonb_agg(
				jsonb_build_object('id', t.id, 'name', t.name)
				ORDER BY rt.display_order
			) FILTER (WHERE t.id IS NOT NULL), '[]'::jsonb) AS tags_json
			FROM material_revision_tags rt
			JOIN tags t ON t.id = rt.tag_id
			WHERE rt.revision_id = r.id
		) tags
		WHERE m.id = $1 AND m.deleted_at IS NULL`,
		materialID)
	if err != nil {
		return entity.PublicMaterial{}, fmt.Errorf(
			"load admin material projection: %w", err,
		)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.PublicMaterial{}, entity.ErrMaterialNotFound
	}
	var status, markdown string
	var createdAt, updatedAt time.Time
	var publishedRevisionID uuid.NullUUID
	var publishedRevisionNumber sql.NullInt64
	material, _, err := scanPublicMaterialWithSuffix(rows, func() []any {
		return []any{
			&status, &markdown, &createdAt, &updatedAt,
			&publishedRevisionID, &publishedRevisionNumber,
		}
	})
	if err != nil {
		return entity.PublicMaterial{}, err
	}
	material.Status = entity.MaterialState(status)
	material.ContentMarkdown = markdown
	material.CreatedAt = createdAt
	material.UpdatedAt = updatedAt
	setPublishedRevision(
		&material,
		publishedRevisionID,
		publishedRevisionNumber,
	)
	return material, rows.Err()
}

func setPublishedRevision(
	material *entity.PublicMaterial,
	publishedRevisionID uuid.NullUUID,
	publishedRevisionNumber sql.NullInt64,
) {
	if material == nil || !publishedRevisionID.Valid ||
		!publishedRevisionNumber.Valid || publishedRevisionNumber.Int64 < 1 {
		return
	}
	value := publishedRevisionID.UUID
	material.PublishedRevisionID = &value
	number := publishedRevisionNumber.Int64
	material.PublishedRevisionNumber = &number
}

func publicProjectionCTE() string {
	return `
		WITH projection AS (
			SELECT m.id AS material_id, current_slug.slug, r.title, r.description,
			       a.sanitized_html, r.category_id, c.name AS category_name,
			       r.difficulty_id, d.name AS difficulty_name, d.level AS difficulty_level,
			       tags.tags_json, tags.tag_ids, r.author_id, u.username AS author_name,
			       r.estimated_read_minutes, m.published_at, m.view_count, m.version,
			       r.id AS revision_id, r.revision_number, a.id AS artifact_id,
			       a.renderer_version, a.sanitizer_policy_version, m.visibility,
			       to_tsvector('simple', r.title || ' ' || r.description) AS document,
			       CASE WHEN $2 = '' THEN 0
			            ELSE ts_rank_cd(
			              to_tsvector('simple', r.title || ' ' || r.description),
			              websearch_to_tsquery('simple', $2)
			            ) END AS score
			FROM materials m
			JOIN material_slugs current_slug ON current_slug.id = m.current_slug_id
			 AND current_slug.material_id = m.id AND current_slug.state = 'current'
			JOIN material_revisions r ON r.id = m.published_revision_id AND r.material_id = m.id
			JOIN content_render_artifacts a ON a.id = m.published_render_artifact_id
			 AND a.render_source_id = m.published_render_source_id
			 AND a.render_source_kind = m.published_render_source_kind
			 AND a.source_checksum = m.published_source_checksum
			 AND a.status = 'ready'
			 AND a.renderer_version = $3
			 AND a.sanitizer_policy_version = $4
			JOIN material_categories c ON c.id = r.category_id AND c.deleted_at IS NULL
			JOIN difficulties d ON d.id = r.difficulty_id AND d.deleted_at IS NULL
			JOIN users u ON u.id = r.author_id AND u.deleted_at IS NULL
			CROSS JOIN LATERAL (
				SELECT COALESCE(jsonb_agg(
					jsonb_build_object('id', t.id, 'name', t.name)
					ORDER BY rt.display_order
				) FILTER (WHERE t.id IS NOT NULL), '[]'::jsonb) AS tags_json,
				COALESCE(array_agg(t.id ORDER BY rt.display_order)
					FILTER (WHERE t.id IS NOT NULL), ARRAY[]::uuid[]) AS tag_ids
				FROM material_revision_tags rt
				JOIN tags t ON t.id = rt.tag_id AND t.deleted_at IS NULL
				WHERE rt.revision_id = r.id
			) tags
			WHERE m.status = 'published' AND m.published_revision_id IS NOT NULL
			  AND m.deleted_at IS NULL
			  AND (m.visibility = 'public' OR ($1 AND m.visibility = 'authenticated'))
		)`
}

func publicProjectionColumns(prefix string) string {
	columns := []string{
		"material_id", "slug", "title", "description", "sanitized_html",
		"category_id", "category_name", "difficulty_id", "difficulty_name",
		"difficulty_level", "tags_json", "author_id", "author_name",
		"estimated_read_minutes", "published_at", "view_count", "version",
		"revision_id", "revision_number", "artifact_id", "renderer_version",
		"sanitizer_policy_version", "visibility", "score",
	}
	for index := range columns {
		columns[index] = prefix + "." + columns[index]
	}
	return strings.Join(columns, ", ")
}

type rowScanner interface{ Scan(...any) error }

func scanPublicMaterial(scanner rowScanner) (entity.PublicMaterial, float64, error) {
	return scanPublicMaterialWithSuffix(scanner, nil)
}

func scanPublicMaterialWithSuffix(scanner rowScanner, suffix func() []any) (entity.PublicMaterial, float64, error) {
	var material entity.PublicMaterial
	var tagsJSON []byte
	var visibility string
	var score float64
	destinations := []any{
		&material.MaterialID, &material.Slug, &material.Title, &material.Description,
		&material.SanitizedHTML, &material.CategoryID, &material.CategoryName,
		&material.DifficultyID, &material.DifficultyName, &material.DifficultyLevel,
		&tagsJSON, &material.AuthorID, &material.AuthorName,
		&material.EstimatedReadMinutes, &material.PublishedAt, &material.ViewCount,
		&material.Version, &material.RevisionID, &material.RevisionNumber,
		&material.RenderArtifactID, &material.RendererVersion,
		&material.SanitizerPolicyVersion, &visibility, &score,
	}
	if suffix != nil {
		destinations = append(destinations, suffix()...)
	}
	if err := scanner.Scan(destinations...); err != nil {
		return entity.PublicMaterial{}, 0, fmt.Errorf("scan public material: %w", err)
	}
	if err := json.Unmarshal(tagsJSON, &material.Tags); err != nil {
		return entity.PublicMaterial{}, 0, entity.ErrMaterialInvalidCommand
	}
	material.Visibility = entity.MaterialVisibility(visibility)
	return material, score, nil
}

func scanAggregate(rows *sql.Rows) (entity.Material, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.Material{}, err
		}
		return entity.Material{}, entity.ErrMaterialNotFound
	}
	var material entity.Material
	var draftID uuid.UUID
	var publishedRevision, publishedArtifact uuid.NullUUID
	var publishedAt, archivedAt sql.NullTime
	if err := rows.Scan(
		&material.ID, &material.CurrentSlugID, &material.Slug, &material.AuthorID,
		&material.Status, &material.Visibility, &draftID, &publishedRevision,
		&publishedArtifact, &material.ViewCount, &material.Version,
		&publishedAt, &archivedAt, &material.CreatedAt, &material.UpdatedAt,
	); err != nil {
		return entity.Material{}, err
	}
	material.CurrentDraftID = draftID
	if publishedRevision.Valid {
		value := publishedRevision.UUID
		material.PublishedRevisionID = &value
	}
	if publishedArtifact.Valid {
		value := publishedArtifact.UUID
		material.PublishedArtifactID = &value
	}
	if publishedAt.Valid {
		value := publishedAt.Time.UTC()
		material.PublishedAt = &value
	}
	if archivedAt.Valid {
		value := archivedAt.Time.UTC()
		material.ArchivedAt = &value
	}
	return material, rows.Err()
}

func encodeMaterialCursor(cursor materialCursor) (string, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeMaterialCursor(value, sort string) (*materialCursor, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > 512 {
		return nil, entity.ErrMaterialInvalidCommand
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	var cursor materialCursor
	if json.Unmarshal(decoded, &cursor) != nil || cursor.Sort != sort || cursor.ID == uuid.Nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	if cursor.Time.IsZero() || cursor.Count < 0 || cursor.Score < 0 ||
		(sort != "relevance" && sort != "published" &&
			sort != "popularity" && sort != "updated") {
		return nil, entity.ErrMaterialInvalidCommand
	}
	return &cursor, nil
}

func mapSlugWriteError(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
		return errors.Join(entity.ErrMaterialSlugConflict, err)
	}
	return fmt.Errorf("write material slug: %w", err)
}

func nullableUUID(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return value.String()
}

func nullableMaterialState(value *entity.MaterialState) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func uuidStrings(values []uuid.UUID) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}
