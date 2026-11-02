package db

import (
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToTraitEntity converts Ent Trait model to domain entity.
func ToTraitEntity(m *generate.Trait) *entity.Trait {
	if m == nil {
		return nil
	}
	t := &entity.Trait{
		ID:           m.ID,
		Code:         m.Code,
		Type:         string(m.Type),
		Name:         m.Name,
		RarityID:     m.RarityID,
		Description:  m.Description,
		DisplayOrder: m.DisplayOrder,
		Active:       m.Active,
		Version:      m.Version,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
	if revisions, err := m.Edges.EffectRevisionsOrErr(); err == nil && len(revisions) == 1 {
		if revision, mapErr := ToTraitEffectRevisionEntity(revisions[0]); mapErr == nil {
			t.EffectRevision = &revision
		}
	}
	if r, err := m.Edges.RarityOrErr(); err == nil && r != nil {
		t.Rarity = &entity.TraitRarity{
			ID:     r.ID,
			Name:   r.Name,
			Code:   r.Code,
			Weight: r.Weight,
		}
	}
	return t
}
