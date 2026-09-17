package usecase

// Contest content prepares bounded Markdown before entering an aggregate
// transaction. Only renderer-owned safe artifacts cross public boundaries;
// requester-authored Markdown remains restricted to owner and jury projections.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/authctx"
	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	contestAnnouncementPublishedV1   = contestcontract.EventAnnouncementPublishedV1
	contestAnnouncementWithdrawnV1   = contestcontract.EventAnnouncementWithdrawnV1
	contestClarificationSubmittedV1  = contestcontract.EventClarificationSubmittedV1
	contestClarificationAnsweredV1   = contestcontract.EventClarificationAnsweredV1
	contestClarificationClosedV1     = contestcontract.EventClarificationClosedV1
	contestClarificationWithdrawnV1  = contestcontract.EventClarificationWithdrawnV1
	maximumContentResultProjection   = 1 << 20
	scheduledAnnouncementAuditReason = "scheduled publication"
)

type ContentDependencies struct {
	Transactions platformtx.UnitOfWork
	Repository   ports.ContentRepository
	Renderer     *contentrender.Renderer
	Content      contentrender.Repository
	Audit        audit.Appender
	Outbox       eventing.OutboxAppender
}

type contentService struct {
	dependencies ContentDependencies
}

func NewContentService(
	dependencies ContentDependencies,
) (ports.ContentService, error) {
	if nilContestDependency(dependencies.Transactions) ||
		nilContestDependency(dependencies.Repository) ||
		dependencies.Renderer == nil ||
		nilContestDependency(dependencies.Content) ||
		nilContestDependency(dependencies.Audit) ||
		nilContestDependency(dependencies.Outbox) {
		return nil, ports.ErrInvalidContestContentCommand
	}
	return &contentService{dependencies: dependencies}, nil
}

var _ ports.ContentService = (*contentService)(nil)

func (service *contentService) CreateAnnouncement(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.CreateAnnouncementRequest,
) (*contract.AnnouncementResponse, error) {
	if request == nil ||
		contestID == uuid.Nil ||
		actorID == uuid.Nil ||
		request.CommandID == uuid.Nil ||
		entity.ValidateContestContentReason(request.Reason, true) != nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	audience := entity.AnnouncementAudience(request.Audience)
	if !audience.Valid() {
		return nil, ports.ErrInvalidContestContentCommand
	}

	metadata := contentCommandMetadata{
		Kind:      ports.ContentCommandAnnouncementCreate,
		ContestID: contestID,
		CommandID: request.CommandID,
	}
	return executeContentCommand(ctx, metadata, func(ctx context.Context) (*contract.AnnouncementResponse, error) {
		sessionID, err := contentActorSession(ctx, actorID)
		if err != nil {
			return nil, err
		}
		receipt, err := newContentReceipt(
			contestID,
			ports.ContentCommandAnnouncementCreate,
			request.CommandID,
			contentCommandFingerprint{
				Audience:     request.Audience,
				Title:        request.Title,
				Markdown:     request.Markdown,
				ScheduledFor: request.ScheduledFor,
				Reason:       request.Reason,
			},
			actorID,
			sessionID,
		)
		if err != nil {
			return nil, err
		}
		announcementID, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		prepared, err := service.prepareRevision(
			ctx,
			announcementID,
			actorID,
			entity.AnnouncementKind,
			request.Title,
			request.Markdown,
		)
		if err != nil {
			return nil, err
		}

		var response *contract.AnnouncementResponse
		err = service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
			reservation, reserveErr := service.dependencies.Repository.ReserveCommand(txCtx, receipt)
			if reserveErr != nil {
				return reserveErr
			}
			if reservation.ExactReplay {
				response, reserveErr = replayAnnouncementResult(reservation.Receipt)
				return reserveErr
			}

			revision, persistErr := service.persistPreparedRevision(txCtx, prepared)
			if persistErr != nil {
				return persistErr
			}
			view, createErr := service.dependencies.Repository.CreateAnnouncement(
				txCtx,
				ports.AnnouncementDraftCommand{
					ContestID:      contestID,
					AnnouncementID: announcementID,
					Audience:       audience,
					ScheduledFor:   request.ScheduledFor,
					Revision:       revision,
					ActorUserID:    actorID,
					ActorSessionID: &sessionID,
					CommandID:      request.CommandID,
					Reason:         request.Reason,
				},
			)
			if createErr != nil {
				return createErr
			}
			response = announcementAdminDTO(view)
			if completeErr := completeAnnouncementCommand(
				txCtx,
				service.dependencies.Repository,
				&receipt,
				view,
				response,
			); completeErr != nil {
				return completeErr
			}
			return service.auditAnnouncementDraft(
				txCtx,
				actorID,
				sessionID,
				"contest.announcement.draft_created.v1",
				view,
				request.Reason,
			)
		})
		return response, err
	})
}

func (service *contentService) EditAnnouncement(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.EditAnnouncementRequest,
) (*contract.AnnouncementResponse, error) {
	if request == nil ||
		contestID == uuid.Nil ||
		actorID == uuid.Nil ||
		request.AnnouncementID == uuid.Nil ||
		request.CommandID == uuid.Nil ||
		request.ExpectedVersion < 1 ||
		entity.ValidateContestContentReason(request.Reason, true) != nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	audience := entity.AnnouncementAudience(request.Audience)
	if !audience.Valid() {
		return nil, ports.ErrInvalidContestContentCommand
	}

	metadata := contentCommandMetadata{
		Kind:            ports.ContentCommandAnnouncementEdit,
		ContestID:       contestID,
		ContentID:       request.AnnouncementID,
		CommandID:       request.CommandID,
		ExpectedVersion: request.ExpectedVersion,
	}
	return executeContentCommand(ctx, metadata, func(ctx context.Context) (*contract.AnnouncementResponse, error) {
		sessionID, err := contentActorSession(ctx, actorID)
		if err != nil {
			return nil, err
		}
		receipt, err := newContentReceipt(
			contestID,
			ports.ContentCommandAnnouncementEdit,
			request.CommandID,
			contentCommandFingerprint{
				ContentID:       &request.AnnouncementID,
				ExpectedVersion: request.ExpectedVersion,
				Audience:        request.Audience,
				Title:           request.Title,
				Markdown:        request.Markdown,
				ScheduledFor:    request.ScheduledFor,
				Reason:          request.Reason,
			},
			actorID,
			sessionID,
		)
		if err != nil {
			return nil, err
		}
		prepared, err := service.prepareRevision(
			ctx,
			request.AnnouncementID,
			actorID,
			entity.AnnouncementKind,
			request.Title,
			request.Markdown,
		)
		if err != nil {
			return nil, err
		}

		var response *contract.AnnouncementResponse
		err = service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
			reservation, reserveErr := service.dependencies.Repository.ReserveCommand(txCtx, receipt)
			if reserveErr != nil {
				return reserveErr
			}
			if reservation.ExactReplay {
				response, reserveErr = replayAnnouncementResult(reservation.Receipt)
				return reserveErr
			}

			revision, persistErr := service.persistPreparedRevision(txCtx, prepared)
			if persistErr != nil {
				return persistErr
			}
			view, editErr := service.dependencies.Repository.EditAnnouncement(
				txCtx,
				ports.AnnouncementDraftCommand{
					ContestID:       contestID,
					AnnouncementID:  request.AnnouncementID,
					ExpectedVersion: request.ExpectedVersion,
					Audience:        audience,
					ScheduledFor:    request.ScheduledFor,
					Revision:        revision,
					ActorUserID:     actorID,
					ActorSessionID:  &sessionID,
					CommandID:       request.CommandID,
					Reason:          request.Reason,
				},
			)
			if editErr != nil {
				return editErr
			}
			response = announcementAdminDTO(view)
			if completeErr := completeAnnouncementCommand(
				txCtx,
				service.dependencies.Repository,
				&receipt,
				view,
				response,
			); completeErr != nil {
				return completeErr
			}
			return service.auditAnnouncementDraft(
				txCtx,
				actorID,
				sessionID,
				"contest.announcement.draft_updated.v1",
				view,
				request.Reason,
			)
		})
		return response, err
	})
}

func (service *contentService) PublishAnnouncement(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.AnnouncementCommandRequest,
) (*contract.AnnouncementResponse, error) {
	return service.changeAnnouncementState(
		ctx,
		contestID,
		actorID,
		request,
		true,
	)
}

func (service *contentService) WithdrawAnnouncement(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.AnnouncementCommandRequest,
) (*contract.AnnouncementResponse, error) {
	return service.changeAnnouncementState(
		ctx,
		contestID,
		actorID,
		request,
		false,
	)
}

func (service *contentService) changeAnnouncementState(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.AnnouncementCommandRequest,
	publish bool,
) (*contract.AnnouncementResponse, error) {
	if request == nil ||
		contestID == uuid.Nil ||
		actorID == uuid.Nil ||
		request.AnnouncementID == uuid.Nil ||
		request.CommandID == uuid.Nil ||
		request.ExpectedVersion < 1 ||
		entity.ValidateContestContentReason(request.Reason, true) != nil {
		return nil, ports.ErrInvalidContestContentCommand
	}

	kind := ports.ContentCommandAnnouncementWithdraw
	eventType := contestAnnouncementWithdrawnV1
	if publish {
		kind = ports.ContentCommandAnnouncementPublish
		eventType = contestAnnouncementPublishedV1
	}
	metadata := contentCommandMetadata{
		Kind:            kind,
		ContestID:       contestID,
		ContentID:       request.AnnouncementID,
		CommandID:       request.CommandID,
		ExpectedVersion: request.ExpectedVersion,
	}
	return executeContentCommand(ctx, metadata, func(ctx context.Context) (*contract.AnnouncementResponse, error) {
		sessionID, err := contentActorSession(ctx, actorID)
		if err != nil {
			return nil, err
		}
		receipt, err := newContentReceipt(
			contestID,
			kind,
			request.CommandID,
			contentCommandFingerprint{
				ContentID:       &request.AnnouncementID,
				ExpectedVersion: request.ExpectedVersion,
				Reason:          request.Reason,
			},
			actorID,
			sessionID,
		)
		if err != nil {
			return nil, err
		}

		var response *contract.AnnouncementResponse
		err = service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
			reservation, reserveErr := service.dependencies.Repository.ReserveCommand(txCtx, receipt)
			if reserveErr != nil {
				return reserveErr
			}
			if reservation.ExactReplay {
				response, reserveErr = replayAnnouncementResult(reservation.Receipt)
				return reserveErr
			}

			command := ports.AnnouncementStateCommand{
				ContestID:       contestID,
				AnnouncementID:  request.AnnouncementID,
				ExpectedVersion: request.ExpectedVersion,
				ActorUserID:     actorID,
				ActorSessionID:  &sessionID,
				CommandID:       request.CommandID,
				Reason:          request.Reason,
			}
			var view ports.AnnouncementView
			if publish {
				view, reserveErr = service.dependencies.Repository.PublishAnnouncement(txCtx, command)
			} else {
				view, reserveErr = service.dependencies.Repository.WithdrawAnnouncement(txCtx, command)
			}
			if reserveErr != nil {
				return reserveErr
			}
			if publish {
				response = announcementPublicDTO(view)
			} else {
				response = announcementAdminDTO(view)
			}
			if completeErr := completeAnnouncementCommand(
				txCtx,
				service.dependencies.Repository,
				&receipt,
				view,
				response,
			); completeErr != nil {
				return completeErr
			}

			payload := safeAnnouncementPayload(view)
			if !publish {
				payload = map[string]any{
					"contest_id":      contestID.String(),
					"announcement_id": view.Announcement.ID.String(),
					"version":         view.Announcement.Version,
					"audience":        string(view.Announcement.Audience),
				}
			}
			return service.journal(
				txCtx,
				actorID,
				&sessionID,
				eventType,
				contestcontract.AggregateAnnouncement,
				view.Announcement.ID,
				view.Announcement.Version,
				request.Reason,
				payload,
			)
		})
		return response, err
	})
}

func (service *contentService) SubmitClarification(
	ctx context.Context,
	contestID uuid.UUID,
	requesterID uuid.UUID,
	request *contract.SubmitClarificationRequest,
) (*contract.ClarificationResponse, error) {
	if request == nil ||
		contestID == uuid.Nil ||
		requesterID == uuid.Nil ||
		request.CommandID == uuid.Nil {
		return nil, ports.ErrInvalidContestContentCommand
	}

	metadata := contentCommandMetadata{
		Kind:      ports.ContentCommandClarificationSubmit,
		ContestID: contestID,
		CommandID: request.CommandID,
	}
	return executeContentCommand(ctx, metadata, func(ctx context.Context) (*contract.ClarificationResponse, error) {
		sessionID, err := contentActorSession(ctx, requesterID)
		if err != nil {
			return nil, err
		}
		receipt, err := newContentReceipt(
			contestID,
			ports.ContentCommandClarificationSubmit,
			request.CommandID,
			contentCommandFingerprint{
				ContestProblemID: request.ContestProblemID,
				Markdown:         request.QuestionMarkdown,
			},
			requesterID,
			sessionID,
		)
		if err != nil {
			return nil, err
		}
		clarificationID, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		prepared, err := service.prepareRevision(
			ctx,
			clarificationID,
			requesterID,
			entity.ClarificationQuestionKind,
			"",
			request.QuestionMarkdown,
		)
		if err != nil {
			return nil, err
		}

		var response *contract.ClarificationResponse
		err = service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
			reservation, reserveErr := service.dependencies.Repository.ReserveCommand(txCtx, receipt)
			if reserveErr != nil {
				return reserveErr
			}
			if reservation.ExactReplay {
				response, reserveErr = replayClarificationResult(reservation.Receipt)
				return reserveErr
			}

			revision, persistErr := service.persistPreparedRevision(txCtx, prepared)
			if persistErr != nil {
				return persistErr
			}
			view, submitErr := service.dependencies.Repository.SubmitClarification(
				txCtx,
				ports.ClarificationSubmitCommand{
					ContestID:        contestID,
					ClarificationID:  clarificationID,
					RequesterID:      requesterID,
					ContestProblemID: request.ContestProblemID,
					Question:         revision,
					ActorSessionID:   &sessionID,
					CommandID:        request.CommandID,
				},
			)
			if submitErr != nil {
				return submitErr
			}
			response = clarificationOwnDTO(view)
			if completeErr := completeClarificationCommand(
				txCtx,
				service.dependencies.Repository,
				&receipt,
				view,
				revision.ID,
				response,
			); completeErr != nil {
				return completeErr
			}

			// Raw question text, requester identity, and previews never enter
			// the integration event or general audit payload.
			return service.journal(
				txCtx,
				requesterID,
				&sessionID,
				contestClarificationSubmittedV1,
				contestcontract.AggregateClarification,
				clarificationID,
				view.Clarification.Version,
				"",
				map[string]any{
					"contest_id":         contestID.String(),
					"clarification_id":   clarificationID.String(),
					"version":            view.Clarification.Version,
					"contest_problem_id": optionalID(request.ContestProblemID),
				},
			)
		})
		return response, err
	})
}

func (service *contentService) AnswerClarification(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.AnswerClarificationRequest,
) (*contract.ClarificationResponse, error) {
	if request == nil ||
		contestID == uuid.Nil ||
		actorID == uuid.Nil ||
		request.ClarificationID == uuid.Nil ||
		request.CommandID == uuid.Nil ||
		request.ExpectedVersion < 1 ||
		entity.ValidateContestContentReason(request.Reason, true) != nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	audience := entity.ClarificationAudience(request.Audience)
	if !audience.Valid() ||
		(audience == entity.ClarificationAudienceParticipants &&
			request.PublicSummaryMarkdown == nil) {
		return nil, ports.ErrInvalidContestContentCommand
	}

	metadata := contentCommandMetadata{
		Kind:            ports.ContentCommandClarificationAnswer,
		ContestID:       contestID,
		ContentID:       request.ClarificationID,
		CommandID:       request.CommandID,
		ExpectedVersion: request.ExpectedVersion,
	}
	return executeContentCommand(ctx, metadata, func(ctx context.Context) (*contract.ClarificationResponse, error) {
		sessionID, err := contentActorSession(ctx, actorID)
		if err != nil {
			return nil, err
		}
		receipt, err := newContentReceipt(
			contestID,
			ports.ContentCommandClarificationAnswer,
			request.CommandID,
			contentCommandFingerprint{
				ContentID:             &request.ClarificationID,
				ExpectedVersion:       request.ExpectedVersion,
				Audience:              request.Audience,
				Markdown:              request.AnswerMarkdown,
				PublicSummaryMarkdown: request.PublicSummaryMarkdown,
				Reason:                request.Reason,
			},
			actorID,
			sessionID,
		)
		if err != nil {
			return nil, err
		}
		answer, err := service.prepareRevision(
			ctx,
			request.ClarificationID,
			actorID,
			entity.ClarificationAnswerKind,
			"",
			request.AnswerMarkdown,
		)
		if err != nil {
			return nil, err
		}
		var summary *preparedContentRevision
		if request.PublicSummaryMarkdown != nil {
			prepared, prepareErr := service.prepareRevision(
				ctx,
				request.ClarificationID,
				actorID,
				entity.ClarificationPublicSummaryKind,
				"",
				*request.PublicSummaryMarkdown,
			)
			if prepareErr != nil {
				return nil, prepareErr
			}
			summary = &prepared
		}

		var response *contract.ClarificationResponse
		err = service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
			reservation, reserveErr := service.dependencies.Repository.ReserveCommand(txCtx, receipt)
			if reserveErr != nil {
				return reserveErr
			}
			if reservation.ExactReplay {
				response, reserveErr = replayClarificationResult(reservation.Receipt)
				return reserveErr
			}

			answerRevision, persistErr := service.persistPreparedRevision(txCtx, answer)
			if persistErr != nil {
				return persistErr
			}
			var summaryRevision *entity.ContentRevision
			if summary != nil {
				value, summaryErr := service.persistPreparedRevision(txCtx, *summary)
				if summaryErr != nil {
					return summaryErr
				}
				summaryRevision = &value
			}
			view, answerErr := service.dependencies.Repository.AnswerClarification(
				txCtx,
				ports.ClarificationAnswerCommand{
					ContestID:       contestID,
					ClarificationID: request.ClarificationID,
					ExpectedVersion: request.ExpectedVersion,
					Audience:        audience,
					PublicSummary:   summaryRevision,
					Answer:          answerRevision,
					ActorUserID:     actorID,
					ActorSessionID:  &sessionID,
					CommandID:       request.CommandID,
					Reason:          request.Reason,
				},
			)
			if answerErr != nil {
				return answerErr
			}
			response = clarificationAdminDTO(view)
			if completeErr := completeClarificationCommand(
				txCtx,
				service.dependencies.Repository,
				&receipt,
				view,
				answerRevision.ID,
				response,
			); completeErr != nil {
				return completeErr
			}
			return service.journal(
				txCtx,
				actorID,
				&sessionID,
				contestClarificationAnsweredV1,
				contestcontract.AggregateClarification,
				view.Clarification.ID,
				view.Clarification.Version,
				request.Reason,
				safeClarificationPayload(view),
			)
		})
		return response, err
	})
}

func (service *contentService) CloseClarification(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.ClarificationCommandRequest,
) (*contract.ClarificationResponse, error) {
	return service.changeClarificationState(
		ctx,
		contestID,
		actorID,
		request,
		true,
	)
}

func (service *contentService) WithdrawClarification(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.ClarificationCommandRequest,
) (*contract.ClarificationResponse, error) {
	return service.changeClarificationState(
		ctx,
		contestID,
		actorID,
		request,
		false,
	)
}

func (service *contentService) changeClarificationState(
	ctx context.Context,
	contestID uuid.UUID,
	actorID uuid.UUID,
	request *contract.ClarificationCommandRequest,
	closeClarification bool,
) (*contract.ClarificationResponse, error) {
	if request == nil ||
		contestID == uuid.Nil ||
		actorID == uuid.Nil ||
		request.ClarificationID == uuid.Nil ||
		request.CommandID == uuid.Nil ||
		request.ExpectedVersion < 1 ||
		entity.ValidateContestContentReason(request.Reason, true) != nil {
		return nil, ports.ErrInvalidContestContentCommand
	}

	kind := ports.ContentCommandClarificationWithdraw
	eventType := contestClarificationWithdrawnV1
	if closeClarification {
		kind = ports.ContentCommandClarificationClose
		eventType = contestClarificationClosedV1
	}
	metadata := contentCommandMetadata{
		Kind:            kind,
		ContestID:       contestID,
		ContentID:       request.ClarificationID,
		CommandID:       request.CommandID,
		ExpectedVersion: request.ExpectedVersion,
	}
	return executeContentCommand(ctx, metadata, func(ctx context.Context) (*contract.ClarificationResponse, error) {
		sessionID, err := contentActorSession(ctx, actorID)
		if err != nil {
			return nil, err
		}
		receipt, err := newContentReceipt(
			contestID,
			kind,
			request.CommandID,
			contentCommandFingerprint{
				ContentID:       &request.ClarificationID,
				ExpectedVersion: request.ExpectedVersion,
				Reason:          request.Reason,
			},
			actorID,
			sessionID,
		)
		if err != nil {
			return nil, err
		}

		var response *contract.ClarificationResponse
		err = service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
			reservation, reserveErr := service.dependencies.Repository.ReserveCommand(txCtx, receipt)
			if reserveErr != nil {
				return reserveErr
			}
			if reservation.ExactReplay {
				response, reserveErr = replayClarificationResult(reservation.Receipt)
				return reserveErr
			}

			command := ports.ClarificationStateCommand{
				ContestID:       contestID,
				ClarificationID: request.ClarificationID,
				ExpectedVersion: request.ExpectedVersion,
				ActorUserID:     actorID,
				ActorSessionID:  &sessionID,
				CommandID:       request.CommandID,
				Reason:          request.Reason,
			}
			var view ports.ClarificationView
			if closeClarification {
				view, reserveErr = service.dependencies.Repository.CloseClarification(txCtx, command)
			} else {
				view, reserveErr = service.dependencies.Repository.WithdrawClarification(txCtx, command)
			}
			if reserveErr != nil {
				return reserveErr
			}
			if closeClarification {
				response = clarificationAdminDTO(view)
			} else {
				response = clarificationOwnDTO(view)
			}
			revisionID := uuid.Nil
			if view.Question != nil {
				revisionID = view.Question.ID
			}
			if completeErr := completeClarificationCommand(
				txCtx,
				service.dependencies.Repository,
				&receipt,
				view,
				revisionID,
				response,
			); completeErr != nil {
				return completeErr
			}
			return service.journal(
				txCtx,
				actorID,
				&sessionID,
				eventType,
				contestcontract.AggregateClarification,
				view.Clarification.ID,
				view.Clarification.Version,
				request.Reason,
				safeClarificationPayload(view),
			)
		})
		return response, err
	})
}

func (service *contentService) ListPublicAnnouncements(
	ctx context.Context,
	request *contract.ContestContentListRequest,
) (*contract.ContestContentPage[contract.AnnouncementResponse], error) {
	return service.listAnnouncements(ctx, uuid.Nil, request, announcementVisibilityPublic)
}

func (service *contentService) ListParticipantAnnouncements(
	ctx context.Context,
	actorID uuid.UUID,
	request *contract.ContestContentListRequest,
) (*contract.ContestContentPage[contract.AnnouncementResponse], error) {
	return service.listAnnouncements(ctx, actorID, request, announcementVisibilityParticipant)
}

func (service *contentService) ListAdminAnnouncements(
	ctx context.Context,
	request *contract.ContestContentListRequest,
) (*contract.ContestContentPage[contract.AnnouncementResponse], error) {
	return service.listAnnouncements(ctx, uuid.Nil, request, announcementVisibilityAdmin)
}

func (service *contentService) GetAdminAnnouncement(
	ctx context.Context,
	contestID uuid.UUID,
	announcementID uuid.UUID,
) (*contract.AnnouncementResponse, error) {
	view, err := service.dependencies.Repository.GetAdminAnnouncement(
		ctx,
		contestID,
		announcementID,
	)
	if err != nil {
		return nil, err
	}
	return announcementAdminDTO(view), nil
}

type announcementVisibility uint8

const (
	announcementVisibilityPublic announcementVisibility = iota + 1
	announcementVisibilityParticipant
	announcementVisibilityAdmin
)

func (service *contentService) listAnnouncements(
	ctx context.Context,
	actorID uuid.UUID,
	request *contract.ContestContentListRequest,
	visibility announcementVisibility,
) (*contract.ContestContentPage[contract.AnnouncementResponse], error) {
	if request == nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	if visibility == announcementVisibilityAdmin {
		if request.Status != "" &&
			!entity.AnnouncementStatus(request.Status).Valid() {
			return nil, ports.ErrInvalidContestContentCommand
		}
	} else if request.Status != "" {
		return nil, ports.ErrInvalidContestContentCommand
	}
	query, err := contentQuery(request, actorID)
	if err != nil {
		return nil, err
	}
	var (
		items []ports.AnnouncementView
		next  *ports.ContentCursor
	)
	switch visibility {
	case announcementVisibilityPublic:
		items, next, err = service.dependencies.Repository.ListPublicAnnouncements(ctx, query)
	case announcementVisibilityParticipant:
		items, next, err = service.dependencies.Repository.ListParticipantAnnouncements(ctx, query)
	case announcementVisibilityAdmin:
		items, next, err = service.dependencies.Repository.ListAdminAnnouncements(ctx, query)
	default:
		return nil, ports.ErrInvalidContestContentCommand
	}
	if err != nil {
		return nil, err
	}
	page := &contract.ContestContentPage[contract.AnnouncementResponse]{
		Items: make([]contract.AnnouncementResponse, 0, len(items)),
	}
	for _, item := range items {
		if visibility == announcementVisibilityAdmin {
			page.Items = append(page.Items, *announcementAdminDTO(item))
		} else {
			page.Items = append(page.Items, *announcementPublicDTO(item))
		}
	}
	page.NextCursor, err = encodeNext(next)
	return page, err
}

func (service *contentService) ListParticipantClarifications(
	ctx context.Context,
	actorID uuid.UUID,
	request *contract.ContestContentListRequest,
) (*contract.ContestContentPage[contract.ClarificationResponse], error) {
	if request == nil || request.Status != "" {
		return nil, ports.ErrInvalidContestContentCommand
	}
	query, err := contentQuery(request, actorID)
	if err != nil {
		return nil, err
	}
	items, next, err := service.dependencies.Repository.ListParticipantClarifications(ctx, query)
	if err != nil {
		return nil, err
	}
	page := &contract.ContestContentPage[contract.ClarificationResponse]{
		Items: make([]contract.ClarificationResponse, 0, len(items)),
	}
	for _, item := range items {
		page.Items = append(page.Items, sharedClarificationDTO(item))
	}
	page.NextCursor, err = encodeNext(next)
	return page, err
}

func (service *contentService) ListOwnClarifications(
	ctx context.Context,
	actorID uuid.UUID,
	request *contract.ContestContentListRequest,
) (*contract.ContestContentPage[contract.ClarificationResponse], error) {
	return service.listPrivateClarifications(ctx, actorID, request, true)
}

func (service *contentService) ListAdminClarifications(
	ctx context.Context,
	request *contract.ContestContentListRequest,
) (*contract.ContestContentPage[contract.ClarificationResponse], error) {
	return service.listPrivateClarifications(ctx, uuid.Nil, request, false)
}

func (service *contentService) listPrivateClarifications(
	ctx context.Context,
	actorID uuid.UUID,
	request *contract.ContestContentListRequest,
	own bool,
) (*contract.ContestContentPage[contract.ClarificationResponse], error) {
	if request == nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	if own {
		if request.Status != "" {
			return nil, ports.ErrInvalidContestContentCommand
		}
	} else if request.Status != "" &&
		!entity.ClarificationStatus(request.Status).Valid() {
		return nil, ports.ErrInvalidContestContentCommand
	}
	query, err := contentQuery(request, actorID)
	if err != nil {
		return nil, err
	}
	var (
		items []ports.ClarificationView
		next  *ports.ContentCursor
	)
	if own {
		items, next, err = service.dependencies.Repository.ListOwnClarifications(ctx, query)
	} else {
		items, next, err = service.dependencies.Repository.ListAdminClarifications(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	page := &contract.ContestContentPage[contract.ClarificationResponse]{
		Items: make([]contract.ClarificationResponse, 0, len(items)),
	}
	for _, item := range items {
		if own {
			page.Items = append(page.Items, *clarificationOwnDTO(item))
		} else {
			page.Items = append(page.Items, *clarificationAdminDTO(item))
		}
	}
	page.NextCursor, err = encodeNext(next)
	return page, err
}

func (service *contentService) GetOwnClarification(
	ctx context.Context,
	contestID uuid.UUID,
	clarificationID uuid.UUID,
	actorID uuid.UUID,
) (*contract.ClarificationResponse, error) {
	view, err := service.dependencies.Repository.GetOwnClarification(
		ctx,
		contestID,
		clarificationID,
		actorID,
	)
	if err != nil {
		return nil, err
	}
	return clarificationOwnDTO(view), nil
}

func (service *contentService) GetAdminClarification(
	ctx context.Context,
	contestID uuid.UUID,
	clarificationID uuid.UUID,
) (*contract.ClarificationResponse, error) {
	view, err := service.dependencies.Repository.GetAdminClarification(
		ctx,
		contestID,
		clarificationID,
	)
	if err != nil {
		return nil, err
	}
	return clarificationAdminDTO(view), nil
}

func (service *contentService) ListRevisions(
	ctx context.Context,
	owner entity.ContentOwnerType,
	ownerID uuid.UUID,
) ([]contract.ContentRevisionResponse, error) {
	values, err := service.dependencies.Repository.ListRevisions(ctx, owner, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]contract.ContentRevisionResponse, 0, len(values))
	for _, value := range values {
		result = append(result, contract.ContentRevisionResponse{
			ID:             value.ID,
			ContentKind:    string(value.Kind),
			RevisionNumber: value.RevisionNumber,
			Title:          value.Title,
			Markdown:       value.Markdown,
			AuthorID:       value.AuthorID,
			CreatedAt:      value.CreatedAt,
		})
	}
	return result, nil
}

func (service *contentService) PublishDueAnnouncements(
	ctx context.Context,
	limit int,
) (int, error) {
	ctx = correlation.EnsureContext(ctx)
	due, err := service.dependencies.Repository.FindDueAnnouncements(ctx, limit)
	if err != nil {
		return 0, err
	}
	published := 0
	for _, item := range due {
		commandID, idErr := uuid.NewV7()
		if idErr != nil {
			return published, idErr
		}
		itemContext := logger.WithFields(
			ctx,
			logger.String("contest_id", item.ContestID.String()),
			logger.String("announcement_id", item.AnnouncementID.String()),
			logger.String("expected_version", strconv.FormatInt(item.Version, 10)),
			logger.String("command_id", commandID.String()),
		)
		publishErr := service.dependencies.Transactions.Within(
			itemContext,
			func(txCtx context.Context) error {
				view, commandErr := service.dependencies.Repository.PublishAnnouncement(
					txCtx,
					ports.AnnouncementStateCommand{
						ContestID:       item.ContestID,
						AnnouncementID:  item.AnnouncementID,
						ExpectedVersion: item.Version,
						CommandID:       commandID,
						Reason:          scheduledAnnouncementAuditReason,
						System:          true,
					},
				)
				if commandErr != nil {
					return commandErr
				}
				return service.journal(
					txCtx,
					uuid.Nil,
					nil,
					contestAnnouncementPublishedV1,
					contestcontract.AggregateAnnouncement,
					view.Announcement.ID,
					view.Announcement.Version,
					scheduledAnnouncementAuditReason,
					safeAnnouncementPayload(view),
				)
			},
		)
		if publishErr != nil {
			if errors.Is(publishErr, entity.ErrContestContentSchedulePending) ||
				errors.Is(publishErr, entity.ErrContestContentVersionConflict) {
				continue
			}
			logger.FromContext(itemContext).Warn(
				"scheduled contest announcement publication failed",
				zap.Error(publishErr),
			)
			return published, publishErr
		}
		published++
	}
	if published > 0 {
		logger.FromContext(ctx).Info(
			"scheduled contest announcements published",
			zap.Int("published_count", published),
		)
	}
	return published, nil
}

type preparedContentRevision struct {
	revision entity.ContentRevision
	source   contentrender.Source
	artifact contentrender.Artifact
}

func (service *contentService) prepareRevision(
	ctx context.Context,
	ownerID uuid.UUID,
	authorID uuid.UUID,
	kind entity.ContentKind,
	title string,
	markdown string,
) (preparedContentRevision, error) {
	sourceKind, valid := contentSourceKind(kind)
	if ownerID == uuid.Nil || authorID == uuid.Nil || !valid {
		return preparedContentRevision{}, ports.ErrInvalidContestContentCommand
	}
	revisionID, err := uuid.NewV7()
	if err != nil {
		return preparedContentRevision{}, err
	}
	sourceID, err := uuid.NewV7()
	if err != nil {
		return preparedContentRevision{}, err
	}
	renderRequest, err := contentrender.NewRenderRequest(
		sourceID,
		sourceKind,
		[]byte(markdown),
	)
	if err != nil {
		return preparedContentRevision{},
			fmt.Errorf("prepare contest content render: %w", err)
	}
	createdAt := time.Now().UTC()
	revision := entity.ContentRevision{
		ID:               revisionID,
		OwnerType:        contentOwnerType(kind),
		OwnerID:          ownerID,
		Kind:             kind,
		RevisionNumber:   1,
		Title:            titlePointer(kind, title),
		Markdown:         string(renderRequest.NormalizedMarkdown),
		AuthorID:         authorID,
		RenderSourceID:   sourceID,
		RenderSourceKind: string(sourceKind),
		SourceChecksum:   renderRequest.SourceChecksum,
		// A temporary non-zero identity lets the entity validate the authored
		// draft before the append-only repository allocates the artifact ID.
		RenderArtifactID: sourceID,
		CreatedAt:        createdAt,
	}
	if err := revision.Validate(); err != nil {
		return preparedContentRevision{}, ports.ErrInvalidContestContentCommand
	}
	revision.RenderArtifactID = uuid.Nil

	artifact, err := service.dependencies.Renderer.Render(ctx, renderRequest)
	if err != nil {
		return preparedContentRevision{}, err
	}
	if artifact.Status != contentrender.StatusReady {
		return preparedContentRevision{}, entity.ErrContestContentNotReady
	}
	artifact.CreatedBy = &authorID
	artifact.CreatedAt = createdAt
	source := contentrender.Source{
		ID:              sourceID,
		OwnerRevisionID: revisionID,
		Kind:            sourceKind,
		Checksum:        renderRequest.SourceChecksum,
		CreatedBy:       &authorID,
		CreatedAt:       createdAt,
	}
	return preparedContentRevision{
		revision: revision,
		source:   source,
		artifact: artifact,
	}, nil
}

func (service *contentService) persistPreparedRevision(
	ctx context.Context,
	prepared preparedContentRevision,
) (entity.ContentRevision, error) {
	source, err := service.dependencies.Content.RegisterSource(ctx, prepared.source)
	if err != nil {
		return entity.ContentRevision{}, err
	}
	artifact, err := service.dependencies.Content.StoreArtifact(
		ctx,
		source,
		prepared.artifact,
	)
	if err != nil {
		return entity.ContentRevision{}, err
	}
	if artifact.ID == uuid.Nil || artifact.Status != contentrender.StatusReady {
		return entity.ContentRevision{}, entity.ErrContestContentNotReady
	}
	revision := prepared.revision
	revision.RenderSourceID = source.ID
	revision.RenderSourceKind = string(source.Kind)
	revision.SourceChecksum = source.Checksum
	revision.RenderArtifactID = artifact.ID
	return revision, nil
}

func (service *contentService) journal(
	ctx context.Context,
	actor uuid.UUID,
	session *uuid.UUID,
	eventType string,
	aggregate string,
	aggregateID uuid.UUID,
	version int64,
	reason string,
	payload map[string]any,
) error {
	eventID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	occurredAt := time.Now().UTC()
	enrichedPayload := make(map[string]any, len(payload)+2)
	for key, value := range payload {
		enrichedPayload[key] = value
	}
	enrichedPayload["event_id"] = eventID
	enrichedPayload["occurred_at"] = occurredAt
	payloadBytes, err := json.Marshal(enrichedPayload)
	if err != nil {
		return fmt.Errorf("encode contest content event payload: %w", err)
	}
	var actorPointer *uuid.UUID
	if actor != uuid.Nil {
		actorCopy := actor
		actorPointer = &actorCopy
	}
	if _, err = service.dependencies.Audit.Append(ctx, audit.Entry{
		ID:             eventID,
		ActorUserID:    actorPointer,
		ActorSessionID: session,
		Action:         eventType,
		Resource:       aggregate,
		ResourceID:     aggregateID.String(),
		Reason:         reason,
		Before:         map[string]any{},
		After:          enrichedPayload,
		CorrelationID:  correlation.FromContext(ctx),
		OccurredAt:     occurredAt,
	}); err != nil {
		return err
	}
	return service.dependencies.Outbox.Append(ctx, eventing.Event{
		ID:               eventID,
		Type:             eventType,
		AggregateType:    aggregate,
		AggregateID:      aggregateID,
		AggregateVersion: version,
		CorrelationID:    correlation.FromContext(ctx),
		Payload:          payloadBytes,
		OccurredAt:       occurredAt,
	})
}

func (service *contentService) auditAnnouncementDraft(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	action string,
	view ports.AnnouncementView,
	reason string,
) error {
	auditID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = service.dependencies.Audit.Append(ctx, audit.Entry{
		ID:             auditID,
		ActorUserID:    &actorID,
		ActorSessionID: &sessionID,
		Action:         action,
		Resource:       contestcontract.AggregateAnnouncement,
		ResourceID:     view.Announcement.ID.String(),
		Reason:         reason,
		Before:         map[string]any{},
		After:          safeAnnouncementPayload(view),
		CorrelationID:  correlation.FromContext(ctx),
		OccurredAt:     time.Now().UTC(),
	})
	return err
}

type contentCommandFingerprint struct {
	ContentID             *uuid.UUID `json:"content_id,omitempty"`
	ContestProblemID      *uuid.UUID `json:"contest_problem_id,omitempty"`
	ExpectedVersion       int64      `json:"expected_version,omitempty"`
	Audience              string     `json:"audience,omitempty"`
	Title                 string     `json:"title,omitempty"`
	Markdown              string     `json:"markdown,omitempty"`
	PublicSummaryMarkdown *string    `json:"public_summary_markdown,omitempty"`
	ScheduledFor          *time.Time `json:"scheduled_for,omitempty"`
	Reason                string     `json:"reason,omitempty"`
}

func newContentReceipt(
	contestID uuid.UUID,
	kind ports.ContentCommandKind,
	commandID uuid.UUID,
	fingerprint contentCommandFingerprint,
	actorID uuid.UUID,
	sessionID uuid.UUID,
) (ports.ContentCommandReceipt, error) {
	if contestID == uuid.Nil ||
		!kind.Valid() ||
		commandID == uuid.Nil ||
		actorID == uuid.Nil ||
		sessionID == uuid.Nil {
		return ports.ContentCommandReceipt{},
			ports.ErrInvalidContestContentCommand
	}
	encoded, err := json.Marshal(struct {
		Kind        ports.ContentCommandKind  `json:"kind"`
		ContestID   uuid.UUID                 `json:"contest_id"`
		Fingerprint contentCommandFingerprint `json:"fingerprint"`
	}{
		Kind:        kind,
		ContestID:   contestID,
		Fingerprint: fingerprint,
	})
	if err != nil {
		return ports.ContentCommandReceipt{},
			fmt.Errorf("encode contest content command: %w", err)
	}
	return ports.ContentCommandReceipt{
		ContestID:      contestID,
		Kind:           kind,
		IdempotencyKey: commandID,
		RequestHash:    sha256.Sum256(encoded),
		ActorUserID:    actorID,
		ActorSessionID: sessionID,
	}, nil
}

func completeAnnouncementCommand(
	ctx context.Context,
	repository ports.ContentRepository,
	receipt *ports.ContentCommandReceipt,
	view ports.AnnouncementView,
	response *contract.AnnouncementResponse,
) error {
	if receipt == nil || response == nil ||
		view.Announcement.ID != response.ID ||
		view.Announcement.ContestID != response.ContestID ||
		view.Announcement.Version != response.Version ||
		view.Revision.ID == uuid.Nil {
		return ports.ErrInvalidContestContentCommand
	}
	revisionID := view.Revision.ID
	return completeContentCommand(
		ctx,
		repository,
		receipt,
		entity.ContentOwnerAnnouncement,
		response.ID,
		&revisionID,
		response.Version,
		response,
	)
}

func completeClarificationCommand(
	ctx context.Context,
	repository ports.ContentRepository,
	receipt *ports.ContentCommandReceipt,
	view ports.ClarificationView,
	resultRevisionID uuid.UUID,
	response *contract.ClarificationResponse,
) error {
	if receipt == nil || response == nil ||
		view.Clarification.ID != response.ID ||
		view.Clarification.ContestID != response.ContestID ||
		view.Clarification.Version != response.Version ||
		resultRevisionID == uuid.Nil {
		return ports.ErrInvalidContestContentCommand
	}
	return completeContentCommand(
		ctx,
		repository,
		receipt,
		entity.ContentOwnerClarification,
		response.ID,
		&resultRevisionID,
		response.Version,
		response,
	)
}

func completeContentCommand(
	ctx context.Context,
	repository ports.ContentRepository,
	receipt *ports.ContentCommandReceipt,
	owner entity.ContentOwnerType,
	contentID uuid.UUID,
	revisionID *uuid.UUID,
	version int64,
	response any,
) error {
	if receipt == nil || repository == nil || response == nil {
		return ports.ErrInvalidContestContentCommand
	}
	projection, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode contest content result projection: %w", err)
	}
	if len(projection) < 2 || len(projection) > maximumContentResultProjection {
		return ports.ErrInvalidContestContentCommand
	}
	receipt.ResultOwner = owner
	receipt.ResultContentID = contentID
	receipt.ResultRevisionID = revisionID
	receipt.ResultVersion = version
	receipt.ResultProjection = projection
	return repository.CompleteCommand(ctx, *receipt)
}

func replayAnnouncementResult(
	receipt ports.ContentCommandReceipt,
) (*contract.AnnouncementResponse, error) {
	response, err := decodeContentResult[contract.AnnouncementResponse](receipt)
	if err != nil ||
		receipt.ResultOwner != entity.ContentOwnerAnnouncement ||
		response.ID != receipt.ResultContentID ||
		response.ContestID != receipt.ContestID ||
		response.Version != receipt.ResultVersion {
		return nil, ports.ErrInvalidContestContentCommand
	}
	return response, nil
}

func replayClarificationResult(
	receipt ports.ContentCommandReceipt,
) (*contract.ClarificationResponse, error) {
	response, err := decodeContentResult[contract.ClarificationResponse](receipt)
	if err != nil ||
		receipt.ResultOwner != entity.ContentOwnerClarification ||
		response.ID != receipt.ResultContentID ||
		response.ContestID != receipt.ContestID ||
		response.Version != receipt.ResultVersion {
		return nil, ports.ErrInvalidContestContentCommand
	}
	return response, nil
}

func decodeContentResult[T any](
	receipt ports.ContentCommandReceipt,
) (*T, error) {
	if len(receipt.ResultProjection) < 2 ||
		len(receipt.ResultProjection) > maximumContentResultProjection {
		return nil, ports.ErrInvalidContestContentCommand
	}
	var response T
	if err := json.Unmarshal(receipt.ResultProjection, &response); err != nil {
		return nil, ports.ErrInvalidContestContentCommand
	}
	return &response, nil
}

func contentSourceKind(
	kind entity.ContentKind,
) (contentrender.SourceKind, bool) {
	switch kind {
	case entity.AnnouncementKind:
		return contentrender.ContestAnnouncement, true
	case entity.ClarificationQuestionKind,
		entity.ClarificationPublicSummaryKind,
		entity.ClarificationAnswerKind:
		return contentrender.ContestClarification, true
	default:
		return "", false
	}
}

func contentOwnerType(kind entity.ContentKind) entity.ContentOwnerType {
	if kind == entity.AnnouncementKind {
		return entity.ContentOwnerAnnouncement
	}
	return entity.ContentOwnerClarification
}

func titlePointer(kind entity.ContentKind, value string) *string {
	if kind != entity.AnnouncementKind {
		return nil
	}
	copy := strings.Clone(value)
	return &copy
}

func optionalID(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return value.String()
}

func contentActorSession(
	ctx context.Context,
	actorID uuid.UUID,
) (uuid.UUID, error) {
	contextActor, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !actorOK ||
		!sessionOK ||
		actorID == uuid.Nil ||
		contextActor != actorID ||
		sessionID == uuid.Nil {
		return uuid.Nil, ports.ErrInvalidContestContentCommand
	}
	return sessionID, nil
}

type contentCommandMetadata struct {
	Kind            ports.ContentCommandKind
	ContestID       uuid.UUID
	ContentID       uuid.UUID
	CommandID       uuid.UUID
	ExpectedVersion int64
}

func executeContentCommand[T any](
	ctx context.Context,
	metadata contentCommandMetadata,
	execute func(context.Context) (*T, error),
) (*T, error) {
	ctx = correlation.EnsureContext(ctx)
	fields := []logger.Field{
		logger.String("command_kind", string(metadata.Kind)),
		logger.String("contest_id", metadata.ContestID.String()),
		logger.String("command_id", metadata.CommandID.String()),
	}
	if metadata.ContentID != uuid.Nil {
		fields = append(fields, logger.String("content_id", metadata.ContentID.String()))
	}
	if metadata.ExpectedVersion > 0 {
		fields = append(fields, logger.String("expected_version", strconv.FormatInt(metadata.ExpectedVersion, 10)))
	}
	ctx = logger.WithFields(ctx, fields...)
	log := logger.FromContext(ctx)
	log.Debug("contest content command started")
	response, err := execute(ctx)
	if err != nil {
		log.Warn(
			"contest content command failed",
			zap.Error(err),
		)
		return nil, err
	}
	log.Info("contest content command committed")
	return response, nil
}
