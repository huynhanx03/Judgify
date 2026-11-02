package http

type rankingPageRequest struct {
	Cursor string `form:"cursor" validate:"omitempty,max=1024"`
	Limit  int    `form:"limit" validate:"omitempty,min=1,max=100"`
}

type updateRankingPrivacyRequest struct {
	Visibility      string `json:"visibility" validate:"required,oneof=public anonymous hidden"`
	ExpectedVersion int64  `json:"expected_version" validate:"min=1"`
	Reason          string `json:"reason" validate:"required,min=3,max=512"`
}

type rankingEntryResponse struct {
	Rank        int64   `json:"rank"`
	PublicID    string  `json:"public_id"`
	DisplayName *string `json:"display_name,omitempty"`
	DisplayMode string  `json:"display_mode"`
	Rating      int     `json:"rating"`
	RankTitle   string  `json:"rank_title,omitempty"`
	TotalExp    int64   `json:"total_exp"`
	LevelName   string  `json:"level_name,omitempty"`
	Level       int     `json:"level,omitempty"`
}

type rankingPageResponse struct {
	Entries            []*rankingEntryResponse `json:"entries"`
	NextCursor         string                  `json:"next_cursor,omitempty"`
	ProjectionRevision int64                   `json:"projection_revision"`
}

type rankingPrivacyResponse struct {
	Visibility         string `json:"visibility"`
	Version            int64  `json:"version"`
	ProjectionRevision int64  `json:"projection_revision"`
	Replayed           bool   `json:"replayed,omitempty"`
}
