package constant

import "time"

const (
	ProblemObjProblem    = "problem"
	ProblemObjTag        = "tag"
	ProblemObjTestCase   = "test case"
	ProblemObjDifficulty = "difficulty"

	// ProblemCacheTTL is the read-through cache lifetime for lookup services.
	ProblemCacheTTL = 10 * time.Minute

	ProblemTagByIDKeyPrefix        = "judgify:v1:problem:tag:id:"
	ProblemTagAllKey               = "judgify:v1:problem:tag:all"
	ProblemDifficultyByIDKeyPrefix = "judgify:v1:problem:difficulty:id:"
	ProblemDifficultyAllKey        = "judgify:v1:problem:difficulty:all"
)
