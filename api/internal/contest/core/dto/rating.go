package dto

// RatingChangeResponse represents a user's rating change after a contest.
type RatingChangeResponse struct {
	UserID     int `json:"user_id"`
	Username   string `json:"username"`
	OldRating  int `json:"old_rating"`
	NewRating  int `json:"new_rating"`
	Rank       int `json:"rank"`
	Delta      int `json:"delta"`
}

// GetContestRatingChangesRequest represents request to get rating changes for a contest.
type GetContestRatingChangesRequest struct {
	ContestID int `uri:"id" validate:"required"`
}
