package db

import (
	"context"
	stdsql "database/sql"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

type TraitEffectRepository struct {
	client *dbent.EntClient
	newID  func() (uuid.UUID, error)
}

func NewTraitEffectRepository(client *dbent.EntClient) (*TraitEffectRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, entity.ErrInvalidTraitEffect
	}
	return &TraitEffectRepository{client: client, newID: uuid.NewV7}, nil
}

var _ ports.TraitEffectRepository = (*TraitEffectRepository)(nil)

func (repository *TraitEffectRepository) LockTimeline(ctx context.Context) error {
	if repository == nil || repository.client == nil || ctx == nil {
		return entity.ErrInvalidTraitEffect
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	if err := acquireRewardTimelineLock(
		ctx, repository.client.DB(ctx).TraitEffectRevision,
	); err != nil {
		return fmt.Errorf("lock trait reward timeline: %w", err)
	}
	return nil
}

func (repository *TraitEffectRepository) Publish(
	ctx context.Context,
	traitID uuid.UUID,
	traitVersion int64,
	effect entity.TraitEffectSpec,
) (entity.TraitEffectRevision, error) {
	if repository == nil || repository.client == nil || repository.newID == nil ||
		ctx == nil || traitID == uuid.Nil || traitVersion < 1 {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	normalized, err := entity.NormalizeTraitEffectSpec(effect)
	if err != nil {
		return entity.TraitEffectRevision{}, err
	}
	if err := repository.LockTimeline(ctx); err != nil {
		return entity.TraitEffectRevision{}, err
	}
	executor := repository.client.DB(ctx).TraitEffectRevision
	var currentRevision stdsql.NullInt64
	if err := querySingleRow(ctx, executor, `
		SELECT MAX(revision)
		FROM trait_effect_revisions
		WHERE trait_id = $1`, traitID).Scan(&currentRevision); err != nil {
		return entity.TraitEffectRevision{}, fmt.Errorf("read trait effect revision: %w", err)
	}
	if (!currentRevision.Valid && traitVersion != 1) ||
		(currentRevision.Valid && currentRevision.Int64 != traitVersion-1) {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	targetIDs, err := resolveTraitEffectElements(ctx, executor, normalized.ElementCodes)
	if err != nil {
		return entity.TraitEffectRevision{}, err
	}
	effectiveAt, err := rewardTimelineNow(ctx, executor)
	if err != nil {
		return entity.TraitEffectRevision{}, fmt.Errorf("read trait effect effective time: %w", err)
	}
	id, err := repository.newID()
	if err != nil || id == uuid.Nil {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	revision, err := entity.NewTraitEffectRevision(
		id, traitID, traitVersion, normalized, targetIDs, effectiveAt,
	)
	if err != nil {
		return entity.TraitEffectRevision{}, err
	}
	effectDocument, err := stdjson.Marshal(revision.Effect)
	if err != nil {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	// An effect with no element scope carries no target IDs, and a nil slice
	// would marshal to null. The column is NOT NULL and constrained to a JSON
	// array, so an empty scope must still be written as an empty array.
	targetElementIDs := revision.TargetElementIDs
	if targetElementIDs == nil {
		targetElementIDs = []uuid.UUID{}
	}
	targetDocument, err := stdjson.Marshal(targetElementIDs)
	if err != nil {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	if _, err := executor.ExecContext(ctx, `
		INSERT INTO trait_effect_revisions (
			id, trait_id, revision, schema_version, effect,
			target_element_ids, checksum, effective_at, created_at
		) VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,$8,$8)`,
		revision.ID, revision.TraitID, revision.Revision,
		entity.TraitEffectSchemaVersion, effectDocument, targetDocument,
		revision.Checksum[:], revision.EffectiveAt,
	); err != nil {
		return entity.TraitEffectRevision{}, fmt.Errorf("append trait effect revision: %w", err)
	}
	return revision, nil
}

func (repository *TraitEffectRepository) Current(
	ctx context.Context,
	traitID uuid.UUID,
) (entity.TraitEffectRevision, error) {
	return repository.load(ctx, traitID, time.Time{})
}

func (repository *TraitEffectRepository) EffectiveAt(
	ctx context.Context,
	traitID uuid.UUID,
	at time.Time,
) (entity.TraitEffectRevision, error) {
	if at.IsZero() {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	return repository.load(ctx, traitID, at.UTC())
}

func (repository *TraitEffectRepository) load(
	ctx context.Context,
	traitID uuid.UUID,
	at time.Time,
) (entity.TraitEffectRevision, error) {
	if repository == nil || repository.client == nil || ctx == nil || traitID == uuid.Nil {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	statement := `
		SELECT id, trait_id, revision, schema_version, effect,
			target_element_ids, checksum, effective_at
		FROM trait_effect_revisions
		WHERE trait_id = $1`
	arguments := []any{traitID}
	if !at.IsZero() {
		statement += ` AND effective_at <= $2`
		arguments = append(arguments, at.UTC())
	}
	statement += ` ORDER BY effective_at DESC, revision DESC, id DESC LIMIT 1`
	return scanTraitEffectRevision(
		querySingleRow(
			ctx, repository.client.DB(ctx).TraitEffectRevision, statement, arguments...,
		).Scan,
	)
}

type traitEffectElementQueryer interface {
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
}

func resolveTraitEffectElements(
	ctx context.Context,
	queryer traitEffectElementQueryer,
	codes []string,
) ([]uuid.UUID, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(codes))
	arguments := make([]any, len(codes))
	for index, code := range codes {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		arguments[index] = code
	}
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, code
		FROM elements
		WHERE deleted_at IS NULL AND code IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY code
		FOR SHARE`, arguments...)
	if err != nil {
		return nil, fmt.Errorf("resolve trait effect elements: %w", err)
	}
	defer func() { _ = rows.Close() }()
	resolved := make(map[string]uuid.UUID, len(codes))
	for rows.Next() {
		var id uuid.UUID
		var code string
		if err := rows.Scan(&id, &code); err != nil || id == uuid.Nil {
			return nil, entity.ErrInvalidTraitEffect
		}
		resolved[code] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(resolved) != len(codes) {
		return nil, entity.ErrInvalidTraitEffect
	}
	result := make([]uuid.UUID, len(codes))
	for index, code := range codes {
		id, found := resolved[code]
		if !found {
			return nil, entity.ErrInvalidTraitEffect
		}
		result[index] = id
	}
	return result, nil
}

func scanTraitEffectRevision(
	scan func(...any) error,
) (entity.TraitEffectRevision, error) {
	var value entity.TraitEffectRevision
	var schemaVersion int
	var effectDocument, targetDocument, checksum []byte
	if err := scan(
		&value.ID, &value.TraitID, &value.Revision, &schemaVersion,
		&effectDocument, &targetDocument, &checksum, &value.EffectiveAt,
	); err != nil {
		if errors.Is(err, stdsql.ErrNoRows) {
			return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
		}
		return entity.TraitEffectRevision{}, err
	}
	if schemaVersion != entity.TraitEffectSchemaVersion || len(checksum) != 32 ||
		len(effectDocument) == 0 || len(effectDocument) > entity.MaximumTraitEffectBytes ||
		len(targetDocument) == 0 || len(targetDocument) > entity.MaximumTraitEffectBytes {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	if err := commonjson.UnmarshalStrictLimit(
		effectDocument, &value.Effect, entity.MaximumTraitEffectBytes,
	); err != nil {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	if err := commonjson.UnmarshalStrictLimit(
		targetDocument, &value.TargetElementIDs, entity.MaximumTraitEffectBytes,
	); err != nil {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	copy(value.Checksum[:], checksum)
	value.EffectiveAt = value.EffectiveAt.UTC()
	if !value.Valid() {
		return entity.TraitEffectRevision{}, entity.ErrInvalidTraitEffect
	}
	return value, nil
}
