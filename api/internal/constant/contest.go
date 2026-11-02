package constant

import "time"

const (
	ContestTopicJudge = "contest-judge"

	ContestObjContest        = "contest"
	ContestObjContestProblem = "contest problem"
	ContestObjRegistration   = "registration"

	ContestStatusDraft    = "draft"
	ContestStatusUpcoming = "upcoming"
	ContestStatusRunning  = "running"
	ContestStatusEnded    = "ended"

	ContestOrchestratorTickInterval = 30 * time.Second
)
