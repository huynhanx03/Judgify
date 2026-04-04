package dto

type UpdateProfileRequest struct {
	FirstName string `json:"first_name" validate:"required,max=50"`
	LastName  string `json:"last_name" validate:"required,max=50"`
	Gender    int    `json:"gender" validate:"oneof=0 1 2"`
	Birthday  string `json:"birthday" validate:"required,datetime=2006-01-02"`
}

type GetProfileRequest struct{}

// ProfileAttrs is the internal profile data used by the enricher pipeline and UpdateProfile.
type ProfileAttrs struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Gender    int    `json:"gender"`
	Birthday  string `json:"birthday,omitempty"`
	JoinedAt  string `json:"joined_at,omitempty"`
}

type CreateUserRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=50"`
	Password  string `json:"password" validate:"required,min=6"`
	RoleID    int    `json:"role_id"`
	FirstName string `json:"first_name" validate:"required,max=50"`
	LastName  string `json:"last_name" validate:"required,max=50"`
	Gender    int    `json:"gender" validate:"oneof=0 1 2"`
	Birthday  string `json:"birthday" validate:"required,datetime=2006-01-02"`
}

// ---- Full profile response (GET /users/profile) ----

// ProfileResponse is the rich response returned by GET /users/profile.
type ProfileResponse struct {
	Username     string          `json:"username"`
	FirstName    string          `json:"first_name"`
	LastName     string          `json:"last_name"`
	Gender       int             `json:"gender"`
	Birthday     string          `json:"birthday,omitempty"`
	JoinedAt     string          `json:"joined_at"`
	Cultivation  CultivationInfo `json:"cultivation"`
	ProblemStats ProblemStats    `json:"problem_stats"`
}

// CultivationInfo holds all cultivation-related data for a user.
type CultivationInfo struct {
	TotalExp int64        `json:"total_exp"`
	Rating   int          `json:"rating"`
	Level    LevelInfo    `json:"level"`
	Rank     RankInfo     `json:"rank"`
	RootBone *TraitInfo   `json:"root_bone,omitempty"`
	Talents  []TraitInfo  `json:"talents"`
	Elements []ElementExp `json:"elements"`
}

// ProblemStats holds aggregated submission + solve stats for a user.
type ProblemStats struct {
	TotalSubmissions int        `json:"total_submissions"`
	AcceptedCount    int        `json:"accepted_count"`
	ByDifficulty     []DiffStat `json:"by_difficulty"`
	ByTag            []TagStat  `json:"by_tag"`
}

// DiffStat is per-difficulty solved count.
type DiffStat struct {
	Name        string `json:"name"`
	Level       int    `json:"level"`
	SolvedCount int    `json:"solved_count"`
}

// TagStat is per-tag solved count with element display.
type TagStat struct {
	Name        string         `json:"name"`
	SolvedCount int            `json:"solved_count"`
	Elements    []ElementBrief `json:"elements"`
}

// ElementBrief is a minimal element for display.
type ElementBrief struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// LevelInfo holds current level name and progress toward the next level.
type LevelInfo struct {
	Name          string  `json:"name"`
	NextName      string  `json:"next_name,omitempty"`       // empty if at max level
	TierIndex     int     `json:"tier_index"`                // 0-based index for color palette
	NextTierIndex int     `json:"next_tier_index,omitempty"` // 0 if at max level
	Progress      float64 `json:"progress"`                  // 0–100
	ExpToNext     int64   `json:"exp_to_next"`               // 0 if at max level
}

// RankInfo holds current rank name and progress toward the next rank.
type RankInfo struct {
	Name          string  `json:"name"`
	NextName      string  `json:"next_name,omitempty"`       // empty if at max rank
	TierIndex     int     `json:"tier_index"`                // 0-based index for color palette
	NextTierIndex int     `json:"next_tier_index,omitempty"` // 0 if at max rank
	Progress      float64 `json:"progress"`                  // 0–100
	RatingToNext  int     `json:"rating_to_next"`            // 0 if at max rank
}

// TraitInfo is a flattened trait for profile display.
type TraitInfo struct {
	Name        string                 `json:"name"`
	RarityCode  string                 `json:"rarity_code,omitempty"`
	RarityName  string                 `json:"rarity_name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ElementExp holds per-element EXP for the radar chart.
type ElementExp struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Exp  int64  `json:"exp"`
}

// DeleteUserRequest represents the request to delete a user.
type DeleteUserRequest struct {
	ID int `uri:"id" validate:"required"`
}

// UserResponse represents the user data returned to clients.
type UserResponse struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	RoleID    int    `json:"role_id"`
	RoleName  string `json:"role_name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UpdateUserRequest represents the request to update a user's role.
type UpdateUserRequest struct {
	ID     int `json:"-" uri:"id" validate:"required"`
	RoleID int `json:"role_id" validate:"required"`
}
