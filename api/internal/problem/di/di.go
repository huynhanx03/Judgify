package di

import (
	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/problem/core/service"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

// ProblemContainer holds all dependencies for the problem domain.
type ProblemContainer struct {
	ProblemHandlerGroup *http.ProblemHandlerGroup

	ProblemRepo  ports.ProblemRepository
	TestCaseRepo ports.TestCaseRepository
	TagRepo      ports.TagRepository

	ProblemService  ports.ProblemService
	TestCaseService ports.TestCaseService
	TagService      ports.TagService
}

// NewProblemContainer creates a new ProblemContainer.
func NewProblemContainer() *ProblemContainer {
	client := global.EntClient

	// Repositories
	tagRepo := db.NewTagRepository(client)
	problemRepo := db.NewProblemRepository(client)
	testCaseRepo := db.NewTestCaseRepository(client)

	// Services
	tagService := service.NewTagService(tagRepo)
	problemService := service.NewProblemService(problemRepo, tagRepo)
	testCaseService := service.NewTestCaseService(testCaseRepo, problemRepo)

	// Handlers
	problemHandlerGroup := &http.ProblemHandlerGroup{
		ProblemHandler:  http.NewProblemHandler(problemService),
		TestCaseHandler: http.NewTestCaseHandler(testCaseService),
		TagHandler:      http.NewTagHandler(tagService),
	}

	return &ProblemContainer{
		ProblemHandlerGroup: problemHandlerGroup,

		ProblemRepo:  problemRepo,
		TestCaseRepo: testCaseRepo,
		TagRepo:      tagRepo,

		ProblemService:  problemService,
		TestCaseService: testCaseService,
		TagService:      tagService,
	}
}
