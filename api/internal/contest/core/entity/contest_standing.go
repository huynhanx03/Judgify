package entity

// ContestStanding represents a user's standing in a contest.
type ContestStanding struct {
	ID             int
	ContestID      int
	UserID         int
	SolvedCount    int
	Penalty        int
	ProblemResults map[string]any
}
