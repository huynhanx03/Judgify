// Package cache owns Judgify's cache policy adapters. It delegates storage,
// eviction, and singleflight mechanics to go-common; use cases only depend on
// their narrow cache ports.
package cache

import commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

// Store is the process-local cache capability supplied by infrastructure.
// Keeping the alias here prevents use cases from importing go-common directly.
type Store = commoncache.LocalCache[string, any]

// Registry is built once at composition time. It is a DI convenience only:
// services receive their specific cache port, never this whole registry.
type Registry struct {
	ProblemTag                  *Tag
	ProblemDifficulty           *Difficulty
	MaterialCategory            *MaterialCategory
	CultivationLevel            *Level
	CultivationElement          *Element
	CultivationRank             *Rank
	CultivationRarity           *Rarity
	CultivationTrait            *Trait
	IdentityRole                *Role
	IdentityAttributeDefinition *AttributeDefinition
}

// New builds all L1 cache adapters from one shared process-local store.
func New(store Store) Registry {
	return Registry{
		ProblemTag:                  NewTag(store),
		ProblemDifficulty:           NewDifficulty(store),
		MaterialCategory:            NewMaterialCategory(store),
		CultivationLevel:            NewLevel(store),
		CultivationElement:          NewElement(store),
		CultivationRank:             NewRank(store),
		CultivationRarity:           NewRarity(store),
		CultivationTrait:            NewTrait(store),
		IdentityRole:                NewRole(store),
		IdentityAttributeDefinition: NewAttributeDefinition(store),
	}
}
