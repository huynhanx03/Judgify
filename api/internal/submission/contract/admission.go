package contract

// JudgeBacklogAdmissionAdvisoryLock serializes every database transition that
// increases ready/retry_wait work with submission intake. Decreasing backlog
// does not require this lock.
const JudgeBacklogAdmissionAdvisoryLock int64 = 7415107

// SolveProjectionAdvisorySeed is the stable hash seed used to serialize the
// canonical solve projection even before its user/problem row exists.
const SolveProjectionAdvisorySeed int64 = 7415108

// ContestStandingAdvisorySeed serializes standing reconstruction by the
// stable contest/user pair before a standing row necessarily exists.
const ContestStandingAdvisorySeed int64 = 7415109
