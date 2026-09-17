package di

import (
	"context"
	"fmt"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	judgedb "github.com/huynhanx03/judgify/internal/judge/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	submissiondb "github.com/huynhanx03/judgify/internal/submission/adapters/driven/db"
	submissionhttp "github.com/huynhanx03/judgify/internal/submission/adapters/driver/http"
	service "github.com/huynhanx03/judgify/internal/submission/usecase"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// SubmissionContainer holds read models and the canonical durable command
// boundary. API composition deliberately contains no Forge or Docker object.
type SubmissionContainer struct {
	SubmissionHandlerGroup *submissionhttp.SubmissionHandlerGroup
	SubmissionRepo         ports.SubmissionRepository
	SubmissionService      ports.SubmissionService
	SubmissionCommands     ports.SubmissionCommandService
	SubmissionOperations   ports.SubmissionOperations
}

// IntakeParticipants are transaction participants owned by other domains.
// The application composition root constructs them and injects only their
// narrow workflow contracts, keeping Submission DI independent of foreign
// persistence adapters.
type IntakeParticipants struct {
	Identity intake.IdentityEligibility
	Contest  intake.ContestReservation
	Problem  intake.ProblemResolver
}

func NewSubmissionContainer(
	client *internalent.EntClient,
	config settings.Judge,
	releaseID string,
	approvedReleaseManifestChecksum string,
	participants IntakeParticipants,
) (*SubmissionContainer, error) {
	if client == nil || client.Client() == nil {
		return nil, fmt.Errorf("initialize submission: database unavailable")
	}
	if releaseID == "" || releaseID != config.ReleaseID {
		return nil, fmt.Errorf("initialize submission: release binding mismatch")
	}
	transactions := platformtx.NewEntUnitOfWork(client.Client())
	outbox := eventing.NewOutbox()
	auditAppender := audit.NewRepository()

	gate, err := submissiondb.NewIntakeGate(
		client,
		config.MaximumActiveSubmissionsPerUser,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize submission admission: %w", err)
	}
	var capabilityResolver intake.CapabilityResolver
	switch config.APIAuthoringCatalogMode {
	case settings.JudgeAPICatalogDevelopment:
		if approvedReleaseManifestChecksum != "" {
			return nil, fmt.Errorf("initialize submission: development catalog carried an executable binding")
		}
		capabilityResolver = unavailableCapabilityResolver{}
	case settings.JudgeAPICatalogApprovedRelease:
		if approvedReleaseManifestChecksum == "" {
			return nil, fmt.Errorf("initialize submission: approved release binding unavailable")
		}
		capabilityResolver, err = judgedb.NewSubmissionCapabilityResolver(
			client,
			judgedb.SubmissionCapabilityConfig{
				ReleaseID:                       releaseID,
				ApprovedReleaseManifestChecksum: approvedReleaseManifestChecksum,
				MaximumReadyBacklog:             config.MaximumReadyBacklog,
				MaximumOldestReadyAge:           config.MaximumOldestReadyAge,
				MinimumFreeDiskBytes:            config.MinimumFreeDiskBytes,
				AdmissionDiskPath:               config.AdmissionDiskPath,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"initialize submission capability admission: %w",
				err,
			)
		}
	default:
		return nil, fmt.Errorf("initialize submission: unknown authoring catalog mode")
	}
	runtimeCatalog, err := judgedb.NewPublicRuntimeCatalog(
		client,
		config.ReleaseID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"initialize public runtime catalog: %w",
			err,
		)
	}
	clock, err := submissiondb.NewIntakeDatabaseClock(client)
	if err != nil {
		return nil, fmt.Errorf(
			"initialize submission database clock: %w",
			err,
		)
	}
	writer, err := submissiondb.NewDurableIntakeWriter(
		client,
		outbox,
		auditAppender,
		config.MaximumAttempts,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize submission writer: %w", err)
	}
	workflow, err := intake.New(intake.Dependencies{
		Transactions: transactions,
		Gate:         gate,
		Identity:     participants.Identity,
		Admission:    gate,
		Contest:      participants.Contest,
		Problem:      participants.Problem,
		Capability:   capabilityResolver,
		Clock:        clock,
		Writer:       writer,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize submission workflow: %w", err)
	}
	commandService, err := service.NewSubmissionCommandService(workflow)
	if err != nil {
		return nil, fmt.Errorf(
			"initialize submission command service: %w",
			err,
		)
	}
	operationsRepository, err := submissiondb.NewSubmissionOperationsRepository(
		client,
		transactions,
		outbox,
		auditAppender,
		config.MaximumAttempts,
	)
	if err != nil {
		return nil, fmt.Errorf("initialize submission operations repository: %w", err)
	}
	operationsService, err := service.NewJudgeOperations(operationsRepository)
	if err != nil {
		return nil, fmt.Errorf("initialize submission operations service: %w", err)
	}

	repository := submissiondb.NewSubmissionRepository(client)
	readService := service.NewSubmissionService(repository)
	handlerGroup := &submissionhttp.SubmissionHandlerGroup{
		SubmissionHandler: submissionhttp.NewSubmissionHandlerWithOperations(
			readService,
			commandService,
			operationsService,
		),
		RuntimeCatalog: runtimeCatalog,
	}
	return &SubmissionContainer{
		SubmissionHandlerGroup: handlerGroup,
		SubmissionRepo:         repository,
		SubmissionService:      readService,
		SubmissionCommands:     commandService,
		SubmissionOperations:   operationsService,
	}, nil
}

// unavailableCapabilityResolver keeps the development authoring catalog
// metadata-only. It lets the API serve authoring screens without allowing a
// submission to claim that an unapproved runtime can execute it.
type unavailableCapabilityResolver struct{}

func (unavailableCapabilityResolver) Resolve(
	ctx context.Context,
	_ intake.CapabilityRequest,
) (intake.CapabilitySnapshot, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return intake.CapabilitySnapshot{}, err
		}
	}
	return intake.CapabilitySnapshot{}, intake.ErrCapabilityUnavailable
}

var _ intake.CapabilityResolver = unavailableCapabilityResolver{}
