package dto

// RankingEntry represents a single entry in the ranking list.
type RankingEntry struct {
	Rank      int    `json:"rank"`
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	Rating    int    `json:"rating"`
	RankTitle string `json:"rank_title,omitempty"`
	TotalExp  int64  `json:"total_exp"`
	LevelName string `json:"level_name,omitempty"`
	Level     int    `json:"level,omitempty"`
}

// GetTopRankingRequest represents request for top ranking.
type GetTopRankingRequest struct {
	Limit int `form:"limit" validate:"min=1,max=100"`
}
