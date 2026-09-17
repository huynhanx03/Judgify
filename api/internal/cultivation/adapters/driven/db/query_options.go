package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate/element"
	"github.com/huynhanx03/judgify/internal/ent/generate/level"
	"github.com/huynhanx03/judgify/internal/ent/generate/rank"
	"github.com/huynhanx03/judgify/internal/ent/generate/rarity"
	"github.com/huynhanx03/judgify/internal/ent/generate/trait"
	"github.com/huynhanx03/judgify/internal/ent/generate/usertrait"
)

var (
	elementQueryColumns = []string{
		element.FieldID,
		element.FieldCreatedAt,
		element.FieldUpdatedAt,
		element.FieldName,
		element.FieldCode,
	}
	levelQueryColumns = []string{
		level.FieldID,
		level.FieldCreatedAt,
		level.FieldUpdatedAt,
		level.FieldName,
		level.FieldMinExp,
	}
	rankQueryColumns = []string{
		rank.FieldID,
		rank.FieldCreatedAt,
		rank.FieldUpdatedAt,
		rank.FieldName,
		rank.FieldMinRating,
	}
	rarityQueryColumns = []string{
		rarity.FieldID,
		rarity.FieldCreatedAt,
		rarity.FieldUpdatedAt,
		rarity.FieldName,
		rarity.FieldCode,
		rarity.FieldWeight,
	}
	traitQueryColumns = []string{
		trait.FieldID,
		trait.FieldCreatedAt,
		trait.FieldUpdatedAt,
		trait.FieldType,
		trait.FieldName,
		trait.FieldRarityID,
	}
	userTraitQueryColumns = []string{
		usertrait.FieldID,
		usertrait.FieldCreatedAt,
		usertrait.FieldUpdatedAt,
		usertrait.FieldUserID,
		usertrait.FieldTraitID,
	}
)
