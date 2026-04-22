package entity

import "time"

// Contest represents a programming contest.
type Contest struct {
	ID              int
	Title           string
	Description     string
	StartTime       time.Time
	EndTime         time.Time
	Status          string // draft, upcoming, running, ended
	AuthorID        int
	MaxParticipants int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
