package constant

import "time"

const (
	CultivationObjElement = "element"
	CultivationObjLevel   = "level"
	CultivationObjRank    = "rank"
	CultivationObjRarity  = "rarity"

	CultivationCacheTTL = 10 * time.Minute

	CultivationLevelByIDKeyPrefix   = "judgify:v1:cultivation:level:id:"
	CultivationLevelAllKey          = "judgify:v1:cultivation:level:all"
	CultivationElementByIDKeyPrefix = "judgify:v1:cultivation:element:id:"
	CultivationElementAllKey        = "judgify:v1:cultivation:element:all"
	CultivationRankByIDKeyPrefix    = "judgify:v1:cultivation:rank:id:"
	CultivationRankAllKey           = "judgify:v1:cultivation:rank:all"
	CultivationRarityByIDKeyPrefix  = "judgify:v1:cultivation:rarity:id:"
	CultivationRarityAllKey         = "judgify:v1:cultivation:rarity:all"
	CultivationTraitByIDKeyPrefix   = "judgify:v1:cultivation:trait:id:"
	CultivationTraitAllKey          = "judgify:v1:cultivation:trait:all"
)
