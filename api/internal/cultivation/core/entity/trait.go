package entity

import "time"

// TraitEffect defines buff configuration stored in metadata JSONB.
type TraitEffect struct {
	Type           string   `json:"type"`            // exp_multiplier, cooldown_reduce, tribulation_boost
	Value          float64  `json:"value"`            // 1.5 = x1.5, 0.5 = -50%
	TargetScope    string   `json:"target_scope"`     // element, all, system
	TargetElements []string `json:"target_elements"`  // ["fire","water"] when scope=element
	DailyLimit     *int     `json:"daily_limit"`      // nil = unlimited
}

// Trait represents a root bone or talent definition.
type Trait struct {
	ID          int                    `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	RarityID    int                    `json:"rarity_id"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}
