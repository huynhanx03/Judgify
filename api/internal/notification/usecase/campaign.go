package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/authctx"
	notificationdb "github.com/huynhanx03/judgify/internal/notification/adapters/driven/db"
	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	entity "github.com/huynhanx03/judgify/internal/notification/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	identityports "github.com/huynhanx03/judgify/internal/ports"
)

type CampaignServiceConfig struct {
	MaximumRecipients       int
	ConfirmationTTL         time.Duration
	RecentAuthenticationTTL time.Duration
}

type CampaignDependencies struct {
	Repository   ports.CampaignRepository
	Transactions platformtx.UnitOfWork
	Content      contentrender.Repository
	Renderer     *contentrender.Renderer
	Audience     identityports.CampaignAudienceParticipant
	Audit        audit.Appender
	Outbox       eventing.OutboxAppender
	Operations   ports.OperationReader
	Confirmation *CampaignConfirmationProtector
	Capabilities ports.CampaignCapabilityChecker
}

type campaignService struct {
	repository   ports.CampaignRepository
	transactions platformtx.UnitOfWork
	content      contentrender.Repository
	renderer     *contentrender.Renderer
	audience     identityports.CampaignAudienceParticipant
	audit        audit.Appender
	outbox       eventing.OutboxAppender
	operations   ports.OperationReader
	confirmation *CampaignConfirmationProtector
	capabilities ports.CampaignCapabilityChecker
	config       CampaignServiceConfig
	now          func() time.Time
	newID        func() (uuid.UUID, error)
}

func NewCampaignService(
	dependencies CampaignDependencies,
	config CampaignServiceConfig,
) (ports.CampaignService, error) {
	if dependencies.Repository == nil || dependencies.Transactions == nil ||
		dependencies.Content == nil || dependencies.Renderer == nil ||
		dependencies.Audience == nil || dependencies.Audit == nil ||
		dependencies.Outbox == nil || dependencies.Operations == nil ||
		dependencies.Confirmation == nil || dependencies.Capabilities == nil {
		return nil, ports.ErrInvalidRequest
	}
	if config.MaximumRecipients == 0 {
		config.MaximumRecipients = contract.DefaultMaximumCampaignRecipients
	}
	if config.ConfirmationTTL == 0 {
		config.ConfirmationTTL = 5 * time.Minute
	}
	if config.RecentAuthenticationTTL == 0 {
		config.RecentAuthenticationTTL = 10 * time.Minute
	}
	if config.MaximumRecipients < 1 ||
		config.MaximumRecipients > 1_000_000 ||
		config.ConfirmationTTL < time.Minute ||
		config.ConfirmationTTL > 15*time.Minute ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, ports.ErrInvalidRequest
	}
	return &campaignService{
		repository:   dependencies.Repository,
		transactions: dependencies.Transactions,
		content:      dependencies.Content,
		renderer:     dependencies.Renderer,
		audience:     dependencies.Audience,
		audit:        dependencies.Audit,
		outbox:       dependencies.Outbox,
		operations:   dependencies.Operations,
		confirmation: dependencies.Confirmation,
		capabilities: dependencies.Capabilities,
		config:       config,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.NewV7,
	}, nil
}

var _ ports.CampaignService = (*campaignService)(nil)

func (service *campaignService) Create(
	ctx context.Context,
	actorID uuid.UUID,
	request *contract.CreateCampaignRequest,
) (*contract.CampaignResponse, error) {
	if service == nil || ctx == nil || actorID == uuid.Nil || request == nil ||
		!contract.ValidCampaignReason(request.Reason) {
		return nil, campaignError(ports.ErrInvalidRequest)
	}
	audience, err := audienceFromRequest(request.Audience)
	if err != nil {
		return nil, campaignError(err)
	}
	campaignID, revisionID, sourceID, err := service.threeIDs()
	if err != nil {
		return nil, campaignError(err)
	}
	prepared, source, artifact, err := service.prepareRevision(
		ctx, campaignID, revisionID, sourceID, 1, actorID,
		request.Title, request.BodyMarkdown, request.ActionPath, audience,
	)
	if err != nil {
		return nil, campaignError(err)
	}
	now := prepared.CreatedAt
	campaign := entity.Campaign{
		ID: campaignID, Status: contract.CampaignStatusDraft,
		CurrentDraftRevisionID: revisionID, Version: 1,
		CreatedBy: actorID, CreatedAt: now, UpdatedAt: now,
	}
	ctx = correlation.EnsureContext(ctx)
	var view entity.CampaignView
	err = service.transactions.Within(ctx, func(txCtx context.Context) error {
		persisted, persistErr := service.persistPreparedRevision(txCtx, prepared, source, artifact)
		if persistErr != nil {
			return persistErr
		}
		view, persistErr = service.repository.Create(txCtx, campaign, persisted)
		if persistErr != nil {
			return persistErr
		}
		return service.appendCampaignEvent(
			txCtx, actorID, "notification_campaign.create",
			contract.EventCampaignDraft, view, request.Reason,
			map[string]any{},
			map[string]any{
				"status":        view.Campaign.Status,
				"version":       view.Campaign.Version,
				"revision_id":   view.Revision.ID.String(),
				"audience_kind": view.Revision.Audience.Kind,
			},
		)
	})
	if err != nil {
		return nil, campaignError(err)
	}
	logger.FromContext(ctx).Info(
		"notification campaign draft created",
		zap.String("campaign_id", view.Campaign.ID.String()),
		zap.Int64("campaign_version", view.Campaign.Version),
		zap.String("audience_kind", view.Revision.Audience.Kind),
	)
	return service.response(ctx, view)
}

func (service *campaignService) Revise(
	ctx context.Context,
	actorID uuid.UUID,
	request *contract.ReviseCampaignRequest,
) (*contract.CampaignResponse, error) {
	if service == nil || ctx == nil || actorID == uuid.Nil || request == nil ||
		request.ID == uuid.Nil || request.ExpectedVersion < 1 ||
		!contract.ValidCampaignReason(request.Reason) {
		return nil, campaignError(ports.ErrInvalidRequest)
	}
	audience, err := audienceFromRequest(request.Audience)
	if err != nil {
		return nil, campaignError(err)
	}
	revisionID, sourceID, err := service.twoIDs()
	if err != nil {
		return nil, campaignError(err)
	}
	// Revision number is assigned under the campaign lock. Rendering is pure
	// and source-bound, so it remains outside the transaction.
	prepared, source, artifact, err := service.prepareRevision(
		ctx, request.ID, revisionID, sourceID, 1, actorID,
		request.Title, request.BodyMarkdown, request.ActionPath, audience,
	)
	if err != nil {
		return nil, campaignError(err)
	}
	ctx = correlation.EnsureContext(ctx)
	var view entity.CampaignView
	err = service.transactions.Within(ctx, func(txCtx context.Context) error {
		before, lockErr := service.repository.Lock(txCtx, request.ID)
		if lockErr != nil {
			return lockErr
		}
		if before.Campaign.Status != contract.CampaignStatusDraft {
			return ports.ErrCampaignNotDraft
		}
		if before.Campaign.Version != request.ExpectedVersion {
			return ports.ErrCampaignConflict
		}
		prepared.RevisionNumber = before.Revision.RevisionNumber + 1
		persisted, persistErr := service.persistPreparedRevision(txCtx, prepared, source, artifact)
		if persistErr != nil {
			return persistErr
		}
		view, persistErr = service.repository.AddRevision(
			txCtx, persisted, request.ExpectedVersion, prepared.CreatedAt,
		)
		if persistErr != nil {
			return persistErr
		}
		return service.appendCampaignEvent(
			txCtx, actorID, "notification_campaign.revise",
			contract.EventCampaignDraft, view, request.Reason,
			map[string]any{
				"version":     before.Campaign.Version,
				"revision_id": before.Revision.ID.String(),
			},
			map[string]any{
				"version":       view.Campaign.Version,
				"revision_id":   view.Revision.ID.String(),
				"audience_kind": view.Revision.Audience.Kind,
			},
		)
	})
	if err != nil {
		return nil, campaignError(err)
	}
	logger.FromContext(ctx).Info(
		"notification campaign draft revised",
		zap.String("campaign_id", view.Campaign.ID.String()),
		zap.Int64("campaign_version", view.Campaign.Version),
		zap.Int64("revision_number", view.Revision.RevisionNumber),
	)
	return service.response(ctx, view)
}

func (service *campaignService) Get(
	ctx context.Context,
	campaignID uuid.UUID,
) (*contract.CampaignResponse, error) {
	if service == nil || ctx == nil || campaignID == uuid.Nil {
		return nil, campaignError(ports.ErrInvalidRequest)
	}
	view, err := service.repository.Get(ctx, campaignID)
	if err != nil {
		return nil, campaignError(err)
	}
	return service.response(ctx, view)
}

func (service *campaignService) List(
	ctx context.Context,
	request *contract.CampaignListRequest,
) (*contract.CampaignListResponse, error) {
	if service == nil || ctx == nil {
		return nil, campaignError(ports.ErrInvalidRequest)
	}
	query := ports.CampaignListQuery{Limit: contract.DefaultCampaignPageSize}
	if request != nil {
		query.Status = request.Status
		if request.Limit > 0 {
			query.Limit = request.Limit
		}
		if request.Cursor != "" {
			cursor, err := notificationdb.DecodeCampaignCursor(request.Cursor)
			if err != nil {
				return nil, campaignError(err)
			}
			query.Cursor = &cursor
		}
	}
	page, err := service.repository.List(ctx, query)
	if err != nil {
		return nil, campaignError(err)
	}
	result := &contract.CampaignListResponse{Items: make([]contract.CampaignResponse, 0, len(page.Items))}
	for _, view := range page.Items {
		value, responseErr := service.response(ctx, view)
		if responseErr != nil {
			return nil, responseErr
		}
		result.Items = append(result.Items, *value)
	}
	if page.Next != nil {
		encoded, encodeErr := notificationdb.EncodeCampaignCursor(*page.Next)
		if encodeErr != nil {
			return nil, campaignError(encodeErr)
		}
		result.NextCursor = &encoded
	}
	logger.FromContext(ctx).Debug(
		"notification campaigns listed",
		zap.Int("campaign_count", len(result.Items)),
		zap.String("status_filter", query.Status),
	)
	return result, nil
}

func (service *campaignService) DryRun(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	request *contract.DryRunCampaignRequest,
) (*contract.CampaignDryRunResponse, error) {
	if service == nil || ctx == nil || actorID == uuid.Nil || sessionID == uuid.Nil ||
		request == nil || request.ID == uuid.Nil || request.ExpectedVersion < 1 {
		return nil, campaignError(ports.ErrInvalidRequest)
	}
	now := service.now().UTC()
	if request.ScheduledFor != nil {
		scheduledFor := request.ScheduledFor.UTC()
		if !scheduledFor.After(now) ||
			scheduledFor.After(now.Add(contract.MaximumCampaignScheduleAhead)) {
			return nil, campaignError(ports.ErrInvalidRequest)
		}
	}
	token, nonceHash, err := service.confirmation.Issue()
	if err != nil {
		return nil, campaignError(err)
	}
	ctx = correlation.EnsureContext(ctx)
	var (
		view         entity.CampaignView
		snapshot     entity.AudienceSnapshot
		confirmation entity.CampaignConfirmation
	)
	err = service.transactions.Within(ctx, func(txCtx context.Context) error {
		var lockErr error
		view, lockErr = service.repository.Lock(txCtx, request.ID)
		if lockErr != nil {
			return lockErr
		}
		if view.Campaign.Status != contract.CampaignStatusDraft {
			return ports.ErrCampaignNotDraft
		}
		if view.Campaign.Version != request.ExpectedVersion {
			return ports.ErrCampaignConflict
		}
		if authorizationErr := service.capabilities.RequireAudienceCapability(
			txCtx,
			view.Revision.Audience.Kind,
		); authorizationErr != nil {
			return authorizationErr
		}
		if view.Revision.Audience.Kind != contract.AudienceExplicitUsers {
			if authErr := service.audience.RequireRecentReauthentication(
				txCtx, actorID, sessionID,
				now.Add(-service.config.RecentAuthenticationTTL),
			); authErr != nil {
				return mapAudienceError(authErr)
			}
		}
		identitySnapshot, snapshotErr := service.audience.SnapshotCampaignAudience(
			txCtx,
			identityAudience(view.Revision.Audience),
			service.config.MaximumRecipients,
			now,
		)
		if snapshotErr != nil {
			return mapAudienceError(snapshotErr)
		}
		members := make([]entity.AudienceMember, len(identitySnapshot.Members))
		for index, member := range identitySnapshot.Members {
			members[index] = entity.AudienceMember{
				UserID: member.UserID, Username: member.Username,
			}
		}
		snapshot, snapshotErr = entity.NewAudienceSnapshot(members, identitySnapshot.At)
		if snapshotErr != nil {
			return snapshotErr
		}
		confirmationID, idErr := service.newID()
		if idErr != nil {
			return idErr
		}
		confirmation = entity.CampaignConfirmation{
			ID: confirmationID, NonceHash: nonceHash,
			ActorID: actorID, SessionID: sessionID,
			CampaignID:               view.Campaign.ID,
			CampaignRevisionID:       view.Revision.ID,
			RenderSourceID:           view.Revision.BodyRenderSourceID,
			RenderSourceKind:         view.Revision.BodyRenderSourceKind,
			NormalizedSourceChecksum: view.Revision.NormalizedSourceChecksum,
			RenderArtifactID:         view.Revision.BodyRenderArtifactID,
			AudienceFingerprint:      snapshot.Fingerprint,
			ExpectedCampaignVersion:  view.Campaign.Version,
			ScheduledFor:             cloneCampaignTime(request.ScheduledFor),
			DryRunEstimatedAt:        snapshot.EstimatedAt,
			DryRunRecipientCount:     int64(len(snapshot.Members)),
			IssuedAt:                 now,
			ExpiresAt:                now.Add(service.config.ConfirmationTTL),
		}
		if storeErr := service.repository.StoreConfirmation(txCtx, confirmation); storeErr != nil {
			return storeErr
		}
		actor := actorID
		session := sessionID
		_, auditErr := service.audit.Append(txCtx, audit.Entry{
			ActorUserID: &actor, ActorSessionID: &session,
			Action:     "notification_campaign.dry_run",
			Resource:   contract.CampaignAggregateType,
			ResourceID: view.Campaign.ID.String(),
			Reason:     "review notification campaign audience",
			Before: map[string]any{
				"campaign_version": view.Campaign.Version,
			},
			After: map[string]any{
				"campaign_version": view.Campaign.Version,
				"revision_id":      view.Revision.ID.String(),
				"audience_kind":    view.Revision.Audience.Kind,
				"recipient_count":  len(snapshot.Members),
				"expires_at":       confirmation.ExpiresAt,
			},
		})
		return auditErr
	})
	if err != nil {
		return nil, campaignError(err)
	}
	sampleSize := min(len(snapshot.Members), contract.MaximumCampaignSampleSize)
	sample := make([]contract.CampaignAudienceMemberResponse, sampleSize)
	for index := range sampleSize {
		sample[index] = contract.CampaignAudienceMemberResponse{
			UserID:   snapshot.Members[index].UserID,
			Username: snapshot.Members[index].Username,
		}
	}
	logger.FromContext(ctx).Info(
		"notification campaign dry run confirmed",
		zap.String("campaign_id", view.Campaign.ID.String()),
		zap.String("revision_id", view.Revision.ID.String()),
		zap.Int("recipient_count", len(snapshot.Members)),
		zap.Time("confirmation_expires_at", confirmation.ExpiresAt),
	)
	return &contract.CampaignDryRunResponse{
		CampaignID:            view.Campaign.ID,
		RevisionID:            view.Revision.ID,
		CampaignVersion:       view.Campaign.Version,
		RecipientCount:        int64(len(snapshot.Members)),
		Sample:                sample,
		EstimatedAt:           snapshot.EstimatedAt,
		ConfirmationToken:     token,
		ConfirmationExpiresAt: confirmation.ExpiresAt,
		ScheduledFor:          cloneCampaignTime(request.ScheduledFor),
	}, nil
}

func (service *campaignService) ListRecipients(
	ctx context.Context,
	request *contract.CampaignRecipientListRequest,
) (*contract.CampaignRecipientListResponse, error) {
	if service == nil || ctx == nil || request == nil || request.ID == uuid.Nil {
		return nil, campaignError(ports.ErrInvalidRequest)
	}
	query := ports.RecipientListQuery{
		CampaignID: request.ID, Status: request.Status,
		Limit: contract.DefaultCampaignPageSize,
	}
	if request.Limit > 0 {
		query.Limit = request.Limit
	}
	if request.Cursor != "" {
		cursor, err := notificationdb.DecodeRecipientCursor(request.Cursor)
		if err != nil {
			return nil, campaignError(err)
		}
		query.Cursor = &cursor
	}
	page, err := service.repository.ListRecipients(ctx, query)
	if err != nil {
		return nil, campaignError(err)
	}
	response := &contract.CampaignRecipientListResponse{
		Items: make([]contract.CampaignRecipientResponse, len(page.Items)),
	}
	for index, item := range page.Items {
		response.Items[index] = contract.CampaignRecipientResponse{
			UserID: item.UserID, Status: item.Status,
			NotificationID: cloneCampaignUUID(item.NotificationID),
			SkipCode:       cloneCampaignString(item.SkipCode),
			FailureCode:    cloneCampaignString(item.FailureCode),
			AttemptCount:   item.AttemptCount,
			LastAttemptAt:  cloneCampaignTime(item.LastAttemptAt),
			ProcessedAt:    cloneCampaignTime(item.ProcessedAt),
		}
	}
	if page.Next != nil {
		encoded, encodeErr := notificationdb.EncodeRecipientCursor(*page.Next)
		if encodeErr != nil {
			return nil, campaignError(encodeErr)
		}
		response.NextCursor = &encoded
	}
	return response, nil
}

func (service *campaignService) prepareRevision(
	ctx context.Context,
	campaignID uuid.UUID,
	revisionID uuid.UUID,
	sourceID uuid.UUID,
	revisionNumber int64,
	authorID uuid.UUID,
	title string,
	body string,
	actionPath *string,
	audience entity.AudienceSelector,
) (entity.CampaignRevision, contentrender.Source, contentrender.Artifact, error) {
	if !contract.ValidCampaignTitle(title) || !contract.ValidCampaignBody(body) ||
		audience.Validate() != nil ||
		(actionPath != nil && contract.ValidateActionPath(*actionPath) != nil) {
		return entity.CampaignRevision{}, contentrender.Source{}, contentrender.Artifact{}, ports.ErrInvalidRequest
	}
	renderRequest, err := contentrender.NewRenderRequest(
		sourceID, contentrender.NotificationCampaignBody, []byte(body),
	)
	if err != nil {
		return entity.CampaignRevision{}, contentrender.Source{}, contentrender.Artifact{}, err
	}
	artifact, err := service.renderer.Render(ctx, renderRequest)
	if err != nil {
		return entity.CampaignRevision{}, contentrender.Source{}, contentrender.Artifact{}, err
	}
	if artifact.Status != contentrender.StatusReady {
		return entity.CampaignRevision{}, contentrender.Source{}, contentrender.Artifact{}, ports.ErrInvalidRequest
	}
	now := service.now().UTC()
	artifact.CreatedBy = &authorID
	artifact.CreatedAt = now
	source := contentrender.Source{
		ID: sourceID, OwnerRevisionID: revisionID,
		Kind:      contentrender.NotificationCampaignBody,
		Checksum:  renderRequest.SourceChecksum,
		CreatedBy: &authorID, CreatedAt: now,
	}
	revision := entity.CampaignRevision{
		ID: revisionID, CampaignID: campaignID,
		RevisionNumber: revisionNumber,
		Title:          title, BodyMarkdown: string(renderRequest.NormalizedMarkdown),
		BodyRenderSourceID:       sourceID,
		BodyRenderSourceKind:     string(contentrender.NotificationCampaignBody),
		ActionPath:               cloneCampaignString(actionPath),
		Audience:                 audience,
		NormalizedSourceChecksum: renderRequest.SourceChecksum,
		AuthorID:                 authorID, CreatedAt: now,
	}
	return revision, source, artifact, nil
}

func (service *campaignService) persistPreparedRevision(
	ctx context.Context,
	revision entity.CampaignRevision,
	source contentrender.Source,
	artifact contentrender.Artifact,
) (entity.CampaignRevision, error) {
	persistedSource, err := service.content.RegisterSource(ctx, source)
	if err != nil {
		return entity.CampaignRevision{}, err
	}
	persistedArtifact, err := service.content.StoreArtifact(ctx, persistedSource, artifact)
	if err != nil {
		return entity.CampaignRevision{}, err
	}
	revision.BodyRenderArtifactID = persistedArtifact.ID
	revision.SanitizedHTML = persistedArtifact.SanitizedHTML
	if err := revision.Validate(); err != nil {
		return entity.CampaignRevision{}, err
	}
	return revision, nil
}

func (service *campaignService) appendCampaignEvent(
	ctx context.Context,
	actorID uuid.UUID,
	action string,
	eventType string,
	view entity.CampaignView,
	reason string,
	before map[string]any,
	after map[string]any,
) error {
	eventID, err := service.newID()
	if err != nil || eventID == uuid.Nil {
		return fmt.Errorf("create campaign event identity: %w", err)
	}
	sessionID, sessionOK := campaignSession(ctx)
	if !sessionOK {
		return ports.ErrInvalidRequest
	}
	event := contract.CampaignLifecycleEvent{
		CampaignID: view.Campaign.ID,
		Status:     view.Campaign.Status,
		Version:    view.Campaign.Version,
	}
	if err := event.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode campaign event: %w", err)
	}
	actor := actorID
	session := sessionID
	if _, err := service.audit.Append(ctx, audit.Entry{
		ID: eventID, ActorUserID: &actor, ActorSessionID: &session,
		Action: action, Resource: contract.CampaignAggregateType,
		ResourceID: view.Campaign.ID.String(), Reason: reason,
		Before: before, After: after,
	}); err != nil {
		return fmt.Errorf("append campaign audit: %w", err)
	}
	return service.outbox.Append(ctx, eventing.Event{
		ID: eventID, Type: eventType,
		AggregateType:    contract.CampaignAggregateType,
		AggregateID:      view.Campaign.ID,
		AggregateVersion: view.Campaign.Version,
		Payload:          payload,
		OccurredAt:       service.now().UTC(),
	})
}

func (service *campaignService) response(
	ctx context.Context,
	view entity.CampaignView,
) (*contract.CampaignResponse, error) {
	var projection *operation.Operation
	if view.Campaign.OperationID != nil {
		value, err := service.operations.Get(ctx, *view.Campaign.OperationID)
		if err != nil {
			return nil, campaignError(err)
		}
		projection = &value
	}
	return CampaignResponse(view, projection), nil
}

// CampaignResponse is shared with the schedule workflow so both HTTP paths
// expose the same effective status projection.
func CampaignResponse(
	view entity.CampaignView,
	projection *operation.Operation,
) *contract.CampaignResponse {
	status := view.Campaign.Status
	if projection != nil && status != contract.CampaignStatusCancelled &&
		status != contract.CampaignStatusCompleted &&
		status != contract.CampaignStatusFailed {
		switch projection.Status {
		case operation.StatusCancelled:
			status = contract.CampaignStatusCancelled
		case operation.StatusFailed:
			status = contract.CampaignStatusFailed
		case operation.StatusCompleted:
			if view.Counts.Failed > 0 {
				status = contract.CampaignStatusFailed
			} else {
				status = contract.CampaignStatusCompleted
			}
		case operation.StatusRunning:
			status = contract.CampaignStatusSending
		}
	}
	response := &contract.CampaignResponse{
		ID: view.Campaign.ID, Status: status, Version: view.Campaign.Version,
		Revision: contract.CampaignRevisionResponse{
			ID:             view.Revision.ID,
			RevisionNumber: view.Revision.RevisionNumber,
			Title:          view.Revision.Title,
			BodyMarkdown:   view.Revision.BodyMarkdown,
			SanitizedHTML:  view.Revision.SanitizedHTML,
			ActionPath:     cloneCampaignString(view.Revision.ActionPath),
			Audience: contract.CampaignAudienceResponse{
				Kind:          view.Revision.Audience.Kind,
				RoleID:        cloneCampaignUUID(view.Revision.Audience.RoleID),
				ExplicitUsers: append([]uuid.UUID(nil), view.Revision.Audience.UserIDs...),
			},
			AuthorID:  view.Revision.AuthorID,
			CreatedAt: view.Revision.CreatedAt.UTC(),
		},
		Recipients: contract.CampaignRecipientCountsResponse{
			Total: view.Counts.Total, Pending: view.Counts.Pending,
			Created: view.Counts.Created, Skipped: view.Counts.Skipped,
			Failed: view.Counts.Failed,
		},
		ScheduledFor:    cloneCampaignTime(view.Campaign.ScheduledFor),
		LastFailureCode: cloneCampaignString(view.Campaign.LastFailureCode),
		CreatedBy:       view.Campaign.CreatedBy,
		ScheduledBy:     cloneCampaignUUID(view.Campaign.ScheduledBy),
		CancelledBy:     cloneCampaignUUID(view.Campaign.CancelledBy),
		CancelReason:    cloneCampaignString(view.Campaign.CancelReason),
		CreatedAt:       view.Campaign.CreatedAt.UTC(),
		UpdatedAt:       view.Campaign.UpdatedAt.UTC(),
		ScheduledAt:     cloneCampaignTime(view.Campaign.ScheduledAt),
		StartedAt:       cloneCampaignTime(view.Campaign.StartedAt),
		CompletedAt:     cloneCampaignTime(view.Campaign.CompletedAt),
		CancelledAt:     cloneCampaignTime(view.Campaign.CancelledAt),
	}
	if projection != nil {
		response.Operation = &contract.CampaignOperationResponse{
			ID: projection.ID, Status: string(projection.Status),
			Processed: projection.Processed, Succeeded: projection.Succeeded,
			Failed: projection.Failed, Total: projection.Total,
			Version:         projection.Version,
			RequestedAction: projection.RequestedAction,
			LastErrorCode:   projection.LastErrorCode,
		}
	}
	return response
}

func audienceFromRequest(request contract.CampaignAudienceRequest) (entity.AudienceSelector, error) {
	return entity.CanonicalAudience(request.Kind, request.RoleID, request.UserIDs)
}

func identityAudience(value entity.AudienceSelector) identityports.CampaignAudienceSelector {
	return identityports.CampaignAudienceSelector{
		Kind:    value.Kind,
		RoleID:  cloneCampaignUUID(value.RoleID),
		UserIDs: append([]uuid.UUID(nil), value.UserIDs...),
	}
}

func mapAudienceError(err error) error {
	switch {
	case errors.Is(err, identityports.IdentityErrCampaignAudienceTooLarge):
		return ports.ErrCampaignAudienceTooLarge
	case errors.Is(err, identityports.ErrCampaignRecentAuthenticationRequired):
		return ports.ErrCampaignReauthenticationRequired
	case errors.Is(err, identityports.ErrCampaignRoleNotFound),
		errors.Is(err, identityports.ErrInvalidCampaignAudience):
		return ports.ErrInvalidRequest
	default:
		return err
	}
}

func campaignError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return err
	}
	switch {
	case errors.Is(err, ports.ErrInvalidRequest),
		errors.Is(err, ports.ErrInvalidCursor),
		errors.Is(err, contract.ErrInvalidCampaign),
		errors.Is(err, ErrInvalidCampaignConfirmationProtector),
		errors.Is(err, ports.ErrCampaignConfirmationInvalid),
		errors.Is(err, ports.ErrCampaignConfirmationExpired):
		return apperr.New(apperr.CodeBadRequest, "invalid notification campaign request", err)
	case errors.Is(err, ports.ErrCampaignNotFound):
		return apperr.New(apperr.CodeNotFound, "notification campaign not found", err)
	case errors.Is(err, ports.ErrCampaignConflict),
		errors.Is(err, ports.ErrCampaignNotDraft),
		errors.Is(err, ports.ErrCampaignNotCancellable),
		errors.Is(err, ports.ErrCampaignAudienceChanged),
		errors.Is(err, operation.ErrConflict):
		return apperr.New(apperr.CodeConflict, "notification campaign changed; review again", err)
	case errors.Is(err, ports.ErrCampaignAudienceTooLarge):
		return apperr.New(apperr.CodeBadRequest, "notification campaign audience exceeds the configured limit", err)
	case errors.Is(err, ports.ErrCampaignReauthenticationRequired),
		errors.Is(err, ports.ErrCampaignPermissionDenied):
		return apperr.New(apperr.CodeForbidden, "recent authentication or permission required", err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return apperr.New(apperr.CodeDatabaseError, "notification campaign service unavailable", err)
	}
}

func MapCampaignError(err error) error { return campaignError(err) }

func campaignSession(ctx context.Context) (uuid.UUID, bool) {
	// Kept here to avoid making repository/audit code aware of HTTP.
	return authctx.SessionID(ctx)
}

func (service *campaignService) twoIDs() (uuid.UUID, uuid.UUID, error) {
	first, err := service.newID()
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	second, err := service.newID()
	return first, second, err
}

func (service *campaignService) threeIDs() (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	first, second, err := service.twoIDs()
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	third, err := service.newID()
	return first, second, third, err
}

func cloneCampaignUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneCampaignString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneCampaignTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
