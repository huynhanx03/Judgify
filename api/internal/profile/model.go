package profile

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrProfileNotProvisioned = errors.New("profile: game projection not provisioned")

const (
	MaximumTraits          = 4
	MaximumElements        = 256
	MaximumLevels          = 256
	MaximumRanks           = 256
	MaximumDifficultyStats = 1_024
	MaximumTagStats        = 1_024
	MaximumTagElements     = 4_096
)

type Stats struct {
	TotalExp         int64
	Rating           int
	TotalSubmissions int
	AcceptedCount    int
}

type Trait struct {
	ID                uuid.UUID
	Code              string
	Type              string
	Name              string
	RarityCode        string
	RarityName        string
	Description       string
	Slot              string
	SlotOrder         int
	EffectRevisionID  uuid.UUID
	EffectRevision    int64
	EffectChecksum    string
	EffectEffectiveAt time.Time
	Effect            TraitEffect
}

type TraitEffect struct {
	Version                    int
	Kind                       string
	Scope                      string
	ElementCodes               []string
	MultiplierDeltaBasisPoints *int64
	FlatBonus                  *int64
}

type RewardProfile struct {
	ID          uuid.UUID
	Revision    int64
	Checksum    string
	Source      string
	EffectiveAt time.Time
	Traits      []Trait
}

type ElementExp struct {
	Code string
	Name string
	Exp  int64
}

type Level struct {
	Name   string
	MinExp int64
}

type Rank struct {
	Name      string
	MinRating int
}

type DifficultyStat struct {
	Name        string
	Level       int
	SolvedCount int
}

type ElementBrief struct {
	Code string
	Name string
}

type TagStat struct {
	Name        string
	SolvedCount int
	Elements    []ElementBrief
}

type Projection struct {
	Stats           Stats
	RewardProfile   RewardProfile
	Elements        []ElementExp
	Levels          []Level
	Ranks           []Rank
	DifficultyStats []DifficultyStat
	TagStats        []TagStat
}

type Source interface {
	Load(context.Context, uuid.UUID) (Projection, error)
}
