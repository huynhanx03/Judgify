package db

import (
	"errors"

	"github.com/google/uuid"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

var ErrInvalidStoredTraitEffect = errors.New("cultivation storage: invalid trait effect revision")

func ToTraitEffectRevisionEntity(
	record *generate.TraitEffectRevision,
) (entity.TraitEffectRevision, error) {
	if record == nil || record.SchemaVersion != entity.TraitEffectSchemaVersion ||
		len(record.Checksum) != 32 {
		return entity.TraitEffectRevision{}, ErrInvalidStoredTraitEffect
	}
	var effect entity.TraitEffectSpec
	if err := commonjson.UnmarshalStrictLimit(
		record.Effect,
		&effect,
		entity.MaximumTraitEffectBytes,
	); err != nil {
		return entity.TraitEffectRevision{}, ErrInvalidStoredTraitEffect
	}
	var targetElementIDs []uuid.UUID
	if err := commonjson.UnmarshalStrictLimit(
		record.TargetElementIds,
		&targetElementIDs,
		entity.MaximumTraitEffectBytes,
	); err != nil {
		return entity.TraitEffectRevision{}, ErrInvalidStoredTraitEffect
	}
	value := entity.TraitEffectRevision{
		ID: record.ID, TraitID: record.TraitID, Revision: record.Revision,
		Effect: effect, TargetElementIDs: targetElementIDs,
		EffectiveAt: record.EffectiveAt.UTC(),
	}
	copy(value.Checksum[:], record.Checksum)
	if !value.Valid() {
		return entity.TraitEffectRevision{}, ErrInvalidStoredTraitEffect
	}
	return value, nil
}
