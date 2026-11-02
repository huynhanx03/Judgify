// Package cache binds domain cache ports to the shared cache facade.
package cache

import commoncache "github.com/huynhanx03/go-common/pkg/common/cache"

// Store is the shared cache coordinator supplied by infrastructure.
type Store = *commoncache.Cache

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

// New builds domain cache adapters over one shared cache coordinator.
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
