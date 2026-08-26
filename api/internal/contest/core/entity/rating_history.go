package entity

// RatingHistory represents a rating change record after contest ends.
type RatingHistory struct {
	ID           int
	UserID       int
	Username     string
	ContestID    int
	OldRating    int
	NewRating    int
	RankPosition int
}
