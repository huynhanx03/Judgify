package constant

// MQ topics for contest judge pipeline.
const (
	TopicContestJudge = "contest-judge"
)

// Object names for the contest domain.
const (
	ObjContest      = "contest"
	ObjRegistration = "registration"
)

// Contest statuses.
const (
	StatusDraft    = "draft"
	StatusUpcoming = "upcoming"
	StatusRunning  = "running"
	StatusEnded    = "ended"
)

// Orchestrator tick interval.
const OrchestratorTickInterval = 30 * 1e9 // 30 seconds in nanoseconds
