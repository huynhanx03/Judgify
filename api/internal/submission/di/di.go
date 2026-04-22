package di

import (
	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/pkg/mq/forge"

	"github.com/huynhanx03/judgify/internal/submission/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/submission/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/submission/core/service"
	"github.com/huynhanx03/judgify/internal/submission/ports"
)

// SubmissionContainer holds all dependencies for the submission domain.
type SubmissionContainer struct {
	SubmissionHandlerGroup *http.SubmissionHandlerGroup
	SubmissionRepo         ports.SubmissionRepository
	SubmissionService      ports.SubmissionService
}

// NewSubmissionContainer creates a new SubmissionContainer.
func NewSubmissionContainer(producer *forge.Producer, contestProducer *forge.Producer) *SubmissionContainer {
	client := global.EntClient

	// Repository
	submissionRepo := db.NewSubmissionRepository(client)

	// Service
	submissionService := service.NewSubmissionService(submissionRepo, producer, contestProducer)

	// Handler
	submissionHandlerGroup := &http.SubmissionHandlerGroup{
		SubmissionHandler: http.NewSubmissionHandler(submissionService),
	}

	return &SubmissionContainer{
		SubmissionHandlerGroup: submissionHandlerGroup,
		SubmissionRepo:         submissionRepo,
		SubmissionService:      submissionService,
	}
}
