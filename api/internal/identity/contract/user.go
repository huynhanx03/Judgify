package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type GetProfileRequest struct{}

type GetProfileAttributesRequest struct{}

// ProfileAttributeMutation is an optimistic, typed mutation against one
// reviewed active definition revision. Value and Unset are mutually
// exclusive; zero ExpectedValueVersion asserts that no value currently
// exists.
type ProfileAttributeMutation struct {
	DefinitionID                 uuid.UUID
	ExpectedDefinitionRevisionID uuid.UUID
	ExpectedValueVersion         int64
	Value                        json.RawMessage
	Unset                        bool
}

type UpdateProfileAttributesRequest struct {
	Mutations []ProfileAttributeMutation
}

type ProfileAttributeValueResponse struct {
	DefinitionID         uuid.UUID
	DefinitionRevisionID uuid.UUID
	Key                  string
	Kind                 string
	RequiredOnOnboarding bool
	DataType             string
	Label                string
	Description          string
	Visibility           string
	UserEditable         bool
	DisplayOrder         int
	Validation           json.RawMessage
	HasValue             bool
	Value                json.RawMessage
	ValueVersion         int64
}

// ProfileAttributesResponse keeps the established fixed projection beside
// the schema-driven value plane so existing session/profile consumers do not
// need to understand custom attributes.
type ProfileAttributesResponse struct {
	Profile    ProfileAttrs
	Attributes []ProfileAttributeValueResponse
}

// ProfileAttrs is the fixed compatibility projection consumed by the profile
// enricher and browser-session bootstrap.
type ProfileAttrs struct {
	ID        uuid.UUID
	Username  string
	FirstName string
	LastName  string
	Gender    int
	Birthday  string
	JoinedAt  string
}

// ---- Full profile response (GET /users/profile) ----

const (
	ProfileStateReady         = "ready"
	ProfileStateUnprovisioned = "unprovisioned"
)

// ProfileResponse is the rich response returned by GET /users/profile.
type ProfileResponse struct {
	ProfileState string
	ID           uuid.UUID
	Username     string
	FirstName    string
	LastName     string
	Gender       int
	Birthday     string
	JoinedAt     string
	Cultivation  *CultivationInfo
	ProblemStats *ProblemStats
}

// CultivationInfo holds all cultivation-related data for a user.
type CultivationInfo struct {
	TotalExp      int64
	Rating        int
	RewardProfile *RewardProfileInfo
	Level         LevelInfo
	Rank          RankInfo
	RootBone      *TraitInfo
	Talents       []TraitInfo
	Elements      []ElementExp
}

// ProblemStats holds aggregated submission + solve stats for a user.
type ProblemStats struct {
	TotalSubmissions int
	AcceptedCount    int
	ByDifficulty     []DiffStat
	ByTag            []TagStat
}

// DiffStat is per-difficulty solved count.
type DiffStat struct {
	Name        string
	Level       int
	SolvedCount int
}

// TagStat is per-tag solved count with element display.
type TagStat struct {
	Name        string
	SolvedCount int
	Elements    []ElementBrief
}

// ElementBrief is a minimal element for display.
type ElementBrief struct {
	Code string
	Name string
}

// LevelInfo holds current level name and progress toward the next level.
type LevelInfo struct {
	Name          string
	NextName      string
	TierIndex     int
	NextTierIndex int
	Progress      float64
	ExpToNext     int64
}

// RankInfo holds current rank name and progress toward the next rank.
type RankInfo struct {
	Name          string
	NextName      string
	TierIndex     int
	NextTierIndex int
	Progress      float64
	RatingToNext  int
}

// TraitInfo is a flattened trait for profile display.
type TraitInfo struct {
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
	EffectEffectiveAt string
	Effect            TraitEffectInfo
}

type TraitEffectInfo struct {
	Version                    int
	Kind                       string
	Scope                      string
	ElementCodes               []string
	MultiplierDeltaBasisPoints *int64
	FlatBonus                  *int64
}

type RewardProfileInfo struct {
	ID          uuid.UUID
	Revision    int64
	Checksum    string
	Source      string
	EffectiveAt string
}

// ElementExp holds per-element EXP for the radar chart.
type ElementExp struct {
	Code string
	Name string
	Exp  int64
}

// UserResponse represents the user data returned to clients.
type UserResponse struct {
	ID                    uuid.UUID
	Username              string
	Status                string
	Version               int64
	SuspendedAt           *time.Time
	Roles                 []string
	AuthorizationRevision int64
	CreatedAt             string
	UpdatedAt             string
}
