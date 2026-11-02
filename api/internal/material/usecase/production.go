package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	materialcontract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type ProductionDependencies struct {
	Repository ports.MaterialEditorialRepository
	Content    contentrender.Repository
	Renderer   *contentrender.Renderer
	Journal    platformmutation.Executor
	Now        func() time.Time
	NewID      func() (uuid.UUID, error)
}

type productionService struct {
	repository ports.MaterialEditorialRepository
	content    contentrender.Repository
	renderer   *contentrender.Renderer
	journal    platformmutation.Executor
	now        func() time.Time
	newID      func() (uuid.UUID, error)
}

func NewProductionService(dependencies ProductionDependencies) (ports.MaterialEditorialService, error) {
	if dependencies.Repository == nil || dependencies.Content == nil ||
		dependencies.Renderer == nil || dependencies.Journal == nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	if dependencies.Now == nil {
		dependencies.Now = func() time.Time { return time.Now().UTC() }
	}
	if dependencies.NewID == nil {
		dependencies.NewID = uuid.NewV7
	}
	return &productionService{
		repository: dependencies.Repository, content: dependencies.Content,
		renderer: dependencies.Renderer, journal: dependencies.Journal,
		now: dependencies.Now, newID: dependencies.NewID,
	}, nil
}

func (service *productionService) CreateDraft(ctx context.Context, command ports.CreateDraftCommand) (*contract.MaterialMutationReceipt, error) {
	slug, err := entity.NormalizeSlug(command.Slug)
	if err != nil || command.CommandID == uuid.Nil || command.MaterialID == uuid.Nil || command.AuthorID == uuid.Nil ||
		!command.Visibility.Valid() {
		return nil, entity.ErrMaterialInvalidCommand
	}
	reason, err := entity.RequireReason(command.ChangeSummary)
	if err != nil {
		return nil, err
	}
	requestHash, err := materialMutationRequestHash(
		materialcontract.EventDraftCreatedV1,
		struct {
			Slug         string                    `json:"slug"`
			Title        string                    `json:"title"`
			Description  string                    `json:"description"`
			Content      []byte                    `json:"content"`
			CategoryID   uuid.UUID                 `json:"category_id"`
			DifficultyID uuid.UUID                 `json:"difficulty_id"`
			TagIDs       []uuid.UUID               `json:"tag_ids"`
			AuthorID     uuid.UUID                 `json:"author_id"`
			Visibility   entity.MaterialVisibility `json:"visibility"`
			Reason       string                    `json:"reason"`
		}{
			Slug: slug, Title: command.Title, Description: command.Description,
			Content: command.Content, CategoryID: command.CategoryID,
			DifficultyID: command.DifficultyID, TagIDs: command.TagIDs,
			AuthorID: command.AuthorID, Visibility: command.Visibility, Reason: reason,
		},
	)
	if err != nil {
		return nil, err
	}
	if replay, found, replayErr := service.loadMaterialMutationReceipt(
		ctx, command.CommandID, materialcontract.EventDraftCreatedV1,
		uuid.Nil, 2, requestHash, true,
	); replayErr != nil || found {
		return replay, replayErr
	}
	err = service.journal.Execute(ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		now := service.now().UTC()
		slugID, revisionID, sourceID, identityErr := service.threeIDs()
		if identityErr != nil {
			return platformmutation.Command{}, identityErr
		}
		material := entity.Material{
			ID: command.MaterialID, CurrentSlugID: slugID, AuthorID: command.AuthorID,
			CurrentDraftID: revisionID,
			Status:         string(entity.MaterialDraft), Visibility: string(command.Visibility),
			Version: 1, CreatedAt: now, UpdatedAt: now,
		}
		slugRecord := entity.MaterialSlug{
			ID: slugID, MaterialID: command.MaterialID, Slug: slug,
			State: "current", CreatedAt: now,
		}
		if err := service.repository.CreateIdentity(txCtx, material, slugRecord); err != nil {
			return platformmutation.Command{}, err
		}
		revision, artifact, revisionErr := service.buildRevision(
			txCtx, revisionInput{
				revisionID: revisionID, sourceID: sourceID, materialID: command.MaterialID,
				revisionNumber: 1, title: command.Title, description: command.Description,
				content: command.Content, categoryID: command.CategoryID,
				difficultyID: command.DifficultyID, tagIDs: command.TagIDs,
				authorID: command.AuthorID, reason: reason, createdAt: now,
			},
		)
		if revisionErr != nil {
			return platformmutation.Command{}, revisionErr
		}
		revision.RenderArtifactID = artifact.ID
		if err := service.repository.StoreRevision(txCtx, revision); err != nil {
			return platformmutation.Command{}, err
		}
		material, err = service.repository.AdvanceDraft(txCtx, command.MaterialID, 1, revision, command.Visibility)
		if err != nil {
			return platformmutation.Command{}, err
		}
		mutation := materialMutation(
			materialcontract.EventDraftCreatedV1, material, reason,
			map[string]any{},
			materialEvidence(material, revision),
		)
		mutation.EventID = command.CommandID
		mutation.Payload["request_hash"] = requestHash
		return mutation, nil
	})
	if err != nil {
		if replay, found, replayErr := service.loadMaterialMutationReceipt(
			ctx, command.CommandID, materialcontract.EventDraftCreatedV1,
			uuid.Nil, 2, requestHash, true,
		); replayErr != nil || found {
			return replay, replayErr
		}
		return nil, err
	}
	receipt, found, err := service.loadMaterialMutationReceipt(
		ctx, command.CommandID, materialcontract.EventDraftCreatedV1,
		uuid.Nil, 2, requestHash, false,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, entity.ErrMaterialConflict
	}
	return receipt, nil
}

func (service *productionService) SaveDraft(ctx context.Context, command ports.MaterialSaveDraftCommand) (*contract.MaterialMutationReceipt, error) {
	if command.CommandID == uuid.Nil || command.MaterialID == uuid.Nil || command.AuthorID == uuid.Nil ||
		command.ExpectedVersion < 1 || !command.Visibility.Valid() {
		return nil, entity.ErrMaterialInvalidCommand
	}
	reason, err := entity.RequireReason(command.ChangeSummary)
	if err != nil {
		return nil, err
	}
	requestHash, err := materialMutationRequestHash(
		materialcontract.EventDraftSavedV1,
		struct {
			MaterialID      uuid.UUID                 `json:"material_id"`
			ExpectedVersion int64                     `json:"expected_version"`
			Title           string                    `json:"title"`
			Description     string                    `json:"description"`
			Content         []byte                    `json:"content"`
			CategoryID      uuid.UUID                 `json:"category_id"`
			DifficultyID    uuid.UUID                 `json:"difficulty_id"`
			TagIDs          []uuid.UUID               `json:"tag_ids"`
			AuthorID        uuid.UUID                 `json:"author_id"`
			Visibility      entity.MaterialVisibility `json:"visibility"`
			Reason          string                    `json:"reason"`
		}{
			MaterialID: command.MaterialID, ExpectedVersion: command.ExpectedVersion,
			Title: command.Title, Description: command.Description, Content: command.Content,
			CategoryID: command.CategoryID, DifficultyID: command.DifficultyID,
			TagIDs: command.TagIDs, AuthorID: command.AuthorID,
			Visibility: command.Visibility, Reason: reason,
		},
	)
	if err != nil {
		return nil, err
	}
	if replay, found, replayErr := service.loadMaterialMutationReceipt(
		ctx, command.CommandID, materialcontract.EventDraftSavedV1,
		command.MaterialID, command.ExpectedVersion+1, requestHash, true,
	); replayErr != nil || found {
		return replay, replayErr
	}
	err = service.journal.Execute(ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		locked, err := service.repository.Lock(txCtx, command.MaterialID)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if locked.Version != command.ExpectedVersion || locked.State() == entity.MaterialArchived {
			return platformmutation.Command{}, entity.ErrMaterialConflict
		}
		number, err := service.repository.NextRevisionNumber(txCtx, command.MaterialID)
		if err != nil {
			return platformmutation.Command{}, err
		}
		revisionID, sourceID, err := service.twoIDs()
		if err != nil {
			return platformmutation.Command{}, err
		}
		revision, artifact, err := service.buildRevision(txCtx, revisionInput{
			revisionID: revisionID, sourceID: sourceID, materialID: command.MaterialID,
			revisionNumber: number, title: command.Title, description: command.Description,
			content: command.Content, categoryID: command.CategoryID,
			difficultyID: command.DifficultyID, tagIDs: command.TagIDs,
			authorID: command.AuthorID, reason: reason, createdAt: service.now().UTC(),
		})
		if err != nil {
			return platformmutation.Command{}, err
		}
		revision.RenderArtifactID = artifact.ID
		if err := service.repository.StoreRevision(txCtx, revision); err != nil {
			return platformmutation.Command{}, err
		}
		updated, err := service.repository.AdvanceDraft(
			txCtx, command.MaterialID, command.ExpectedVersion, revision, command.Visibility,
		)
		if err != nil {
			return platformmutation.Command{}, err
		}
		mutation := materialMutation(
			materialcontract.EventDraftSavedV1, updated, reason,
			map[string]any{"version": locked.Version, "revision_id": locked.CurrentDraftID.String()},
			materialEvidence(updated, revision),
		)
		mutation.EventID = command.CommandID
		mutation.Payload["request_hash"] = requestHash
		return mutation, nil
	})
	if err != nil {
		if replay, found, replayErr := service.loadMaterialMutationReceipt(
			ctx, command.CommandID, materialcontract.EventDraftSavedV1,
			command.MaterialID, command.ExpectedVersion+1, requestHash, true,
		); replayErr != nil || found {
			return replay, replayErr
		}
		return nil, err
	}
	receipt, found, err := service.loadMaterialMutationReceipt(
		ctx, command.CommandID, materialcontract.EventDraftSavedV1,
		command.MaterialID, command.ExpectedVersion+1, requestHash, false,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, entity.ErrMaterialConflict
	}
	return receipt, nil
}

func (service *productionService) PreviewLifecycle(
	ctx context.Context,
	command ports.PreviewMaterialLifecycleCommand,
) (ports.MaterialLifecyclePreview, error) {
	if err := validateLifecycleCommand(
		command.Action,
		command.MaterialID,
		command.TargetRevisionID,
		command.ExpectedVersion,
	); err != nil {
		return ports.MaterialLifecyclePreview{}, err
	}
	current, err := service.repository.LoadAggregate(ctx, command.MaterialID)
	if err != nil {
		return ports.MaterialLifecyclePreview{}, err
	}
	if current.Version != command.ExpectedVersion {
		return ports.MaterialLifecyclePreview{}, entity.ErrMaterialConflict
	}
	if err := validateLifecycleTransition(
		current,
		command.Action,
		command.TargetRevisionID,
	); err != nil {
		return ports.MaterialLifecyclePreview{}, err
	}
	before, err := service.currentLifecycleProjection(ctx, current)
	if err != nil {
		return ports.MaterialLifecyclePreview{}, err
	}
	after := before
	after.Version = current.Version + 1
	var targetID *uuid.UUID
	if command.Action == ports.MaterialLifecycleArchive {
		after.Status = entity.MaterialArchived
	} else if !lifecycleRequiresTargetRevision(command.Action) {
		targetStatus := lifecycleTargetStatus(command.Action)
		if targetStatus == "" {
			return ports.MaterialLifecyclePreview{}, entity.ErrMaterialInvalidCommand
		}
		after.Status = targetStatus
	} else {
		revision, artifact, revisionErr := service.validatedLifecycleRevision(
			ctx,
			command.MaterialID,
			command.TargetRevisionID,
			true,
		)
		if revisionErr != nil {
			return ports.MaterialLifecyclePreview{}, revisionErr
		}
		if targetErr := validateLifecycleTarget(
			command.Action,
			before,
			revision,
		); targetErr != nil {
			return ports.MaterialLifecyclePreview{}, targetErr
		}
		after = lifecycleProjection(current, revision, artifact)
		after.Version = current.Version + 1
		after.Status = entity.MaterialPublished
		targetID = uuidPointer(revision.ID)
	}
	return ports.MaterialLifecyclePreview{
		Action:           command.Action,
		MaterialID:       command.MaterialID,
		TargetRevisionID: targetID,
		Before:           before,
		After:            after,
	}, nil
}

func (service *productionService) ApplyLifecycle(
	ctx context.Context,
	command ports.ApplyMaterialLifecycleCommand,
) (*contract.MaterialLifecycleReceipt, error) {
	if err := validateLifecycleCommand(
		command.Action,
		command.MaterialID,
		command.TargetRevisionID,
		command.ExpectedVersion,
	); err != nil || command.CommandID == uuid.Nil ||
		command.RequestHash == ([sha256.Size]byte{}) {
		return nil, entity.ErrMaterialInvalidCommand
	}
	reason, err := entity.RequireReason(command.Reason)
	if err != nil {
		return nil, err
	}
	requestHash := hex.EncodeToString(command.RequestHash[:])
	if receipt, found, receiptErr := service.loadLifecycleReceipt(
		ctx,
		command,
		requestHash,
		true,
	); receiptErr != nil || found {
		return receipt, receiptErr
	}
	err = service.journal.Execute(
		ctx,
		func(txCtx context.Context) (platformmutation.Command, error) {
			locked, lockErr := service.repository.Lock(txCtx, command.MaterialID)
			if lockErr != nil {
				return platformmutation.Command{}, lockErr
			}
			if locked.Version != command.ExpectedVersion {
				return platformmutation.Command{}, entity.ErrMaterialConflict
			}
			if transitionErr := validateLifecycleTransition(
				locked,
				command.Action,
				command.TargetRevisionID,
			); transitionErr != nil {
				return platformmutation.Command{}, transitionErr
			}
			before, projectionErr := service.currentLifecycleProjection(
				txCtx,
				locked,
			)
			if projectionErr != nil {
				return platformmutation.Command{}, projectionErr
			}
			updated := entity.Material{}
			after := before
			after.Version = locked.Version + 1
			if command.Action == ports.MaterialLifecycleArchive {
				updated, projectionErr = service.repository.Archive(
					txCtx,
					command.MaterialID,
					command.ExpectedVersion,
					service.now().UTC(),
				)
				after.Status = entity.MaterialArchived
			} else if !lifecycleRequiresTargetRevision(command.Action) {
				targetStatus := lifecycleTargetStatus(command.Action)
				updated, projectionErr = service.repository.TransitionReview(
					txCtx, command.MaterialID, command.ExpectedVersion, targetStatus,
				)
				after.Status = targetStatus
			} else {
				revision, artifact, revisionErr := service.validatedLifecycleRevision(
					txCtx,
					command.MaterialID,
					command.TargetRevisionID,
					true,
				)
				if revisionErr != nil {
					return platformmutation.Command{}, revisionErr
				}
				if targetErr := validateLifecycleTarget(
					command.Action,
					before,
					revision,
				); targetErr != nil {
					return platformmutation.Command{}, targetErr
				}
				updated, projectionErr = service.repository.Publish(
					txCtx,
					command.MaterialID,
					command.ExpectedVersion,
					revision,
					service.now().UTC(),
					contentrender.CurrentRendererVersion,
					contentrender.CurrentSanitizerPolicyVersion,
				)
				after = lifecycleProjection(locked, revision, artifact)
				after.Version = locked.Version + 1
				after.Status = entity.MaterialPublished
			}
			if projectionErr != nil {
				return platformmutation.Command{}, projectionErr
			}
			if updated.ID != command.MaterialID ||
				updated.Version != command.ExpectedVersion+1 ||
				updated.State() != after.Status {
				return platformmutation.Command{}, entity.ErrMaterialConflict
			}
			eventType, eventErr := materialLifecycleEvent(command.Action)
			if eventErr != nil {
				return platformmutation.Command{}, eventErr
			}
			mutation := materialMutation(
				eventType,
				updated,
				reason,
				lifecycleProjectionDocument(before),
				lifecycleProjectionDocument(after),
			)
			mutation.EventID = command.CommandID
			mutation.Payload["request_hash"] = requestHash
			return mutation, nil
		},
	)
	if err != nil {
		if receipt, found, receiptErr := service.loadLifecycleReceipt(
			ctx,
			command,
			requestHash,
			true,
		); receiptErr != nil || found {
			return receipt, receiptErr
		}
		return nil, err
	}
	receipt, found, err := service.loadLifecycleReceipt(
		ctx,
		command,
		requestHash,
		false,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, entity.ErrMaterialConflict
	}
	return receipt, nil
}

type lifecycleReceiptPayload struct {
	MaterialID  uuid.UUID                                    `json:"material_id"`
	Version     int64                                        `json:"version"`
	After       contract.MaterialLifecycleProjectionResponse `json:"after"`
	RequestHash string                                       `json:"request_hash"`
}

func (service *productionService) loadLifecycleReceipt(
	ctx context.Context,
	command ports.ApplyMaterialLifecycleCommand,
	requestHash string,
	replayed bool,
) (*contract.MaterialLifecycleReceipt, bool, error) {
	receipt, found, err := service.repository.LoadCommandReceipt(
		ctx,
		command.CommandID,
	)
	if err != nil || !found {
		return nil, found, err
	}
	eventType, eventErr := materialLifecycleEvent(command.Action)
	if eventErr != nil {
		return nil, false, eventErr
	}
	var payload lifecycleReceiptPayload
	if receipt.EventID != command.CommandID ||
		receipt.EventType != eventType ||
		receipt.AggregateType != materialAggregate ||
		receipt.AggregateID != command.MaterialID ||
		receipt.Version != command.ExpectedVersion+1 ||
		receipt.OccurredAt.IsZero() ||
		json.Unmarshal(receipt.Payload, &payload) != nil ||
		payload.MaterialID != command.MaterialID ||
		payload.Version != receipt.Version ||
		payload.After.Version != receipt.Version ||
		payload.RequestHash != requestHash ||
		!validLifecycleReceiptProjection(payload.After) ||
		(command.Action == ports.MaterialLifecycleArchive &&
			payload.After.Status != string(entity.MaterialArchived)) ||
		(command.Action == ports.MaterialLifecycleSubmitReview &&
			payload.After.Status != string(entity.MaterialInReview)) ||
		(command.Action == ports.MaterialLifecycleReturnToDraft &&
			payload.After.Status != string(entity.MaterialDraft)) ||
		(lifecycleRequiresTargetRevision(command.Action) &&
			(payload.After.Status != string(entity.MaterialPublished) ||
				payload.After.RevisionID == nil ||
				*payload.After.RevisionID != command.TargetRevisionID)) {
		return nil, false, entity.ErrMaterialCommandConflict
	}
	return &contract.MaterialLifecycleReceipt{
		MaterialID:       command.MaterialID,
		Action:           string(command.Action),
		CommandID:        command.CommandID,
		EventID:          receipt.EventID,
		Version:          receipt.Version,
		After:            payload.After,
		CommittedAt:      receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func validLifecycleReceiptProjection(
	projection contract.MaterialLifecycleProjectionResponse,
) bool {
	if !validLifecycleChecksum(projection.SourceChecksum) ||
		projection.Version < 1 ||
		!entity.MaterialState(projection.Status).Valid() ||
		!entity.MaterialVisibility(projection.Visibility).Valid() ||
		strings.TrimSpace(projection.Slug) == "" ||
		projection.RevisionID == nil || *projection.RevisionID == uuid.Nil ||
		projection.RevisionNumber < 1 || strings.TrimSpace(projection.Title) == "" ||
		projection.CategoryID == uuid.Nil || projection.DifficultyID == uuid.Nil ||
		projection.RenderSourceID == uuid.Nil ||
		projection.RenderArtifactID == uuid.Nil ||
		strings.TrimSpace(projection.RendererVersion) == "" ||
		strings.TrimSpace(projection.SanitizerPolicyVersion) == "" ||
		len(projection.TagIDs) > entity.MaximumMaterialTags {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(projection.TagIDs))
	for _, tagID := range projection.TagIDs {
		if tagID == uuid.Nil {
			return false
		}
		if _, duplicate := seen[tagID]; duplicate {
			return false
		}
		seen[tagID] = struct{}{}
	}
	return true
}

func validLifecycleChecksum(value string) bool {
	checksum, err := hex.DecodeString(value)
	if err != nil || len(checksum) != sha256.Size {
		return false
	}
	for _, current := range checksum {
		if current != 0 {
			return true
		}
	}
	return false
}

func (service *productionService) currentLifecycleProjection(
	ctx context.Context,
	material entity.Material,
) (ports.MaterialLifecycleProjection, error) {
	revisionID := material.CurrentDraftID
	if material.State() == entity.MaterialPublished &&
		material.PublishedRevisionID != nil {
		revisionID = *material.PublishedRevisionID
	}
	if revisionID == uuid.Nil || material.Version < 1 ||
		material.ID == uuid.Nil || strings.TrimSpace(material.Slug) == "" ||
		!material.State().Valid() ||
		!entity.MaterialVisibility(material.Visibility).Valid() {
		return ports.MaterialLifecycleProjection{}, entity.ErrMaterialInvalidCommand
	}
	revision, artifact, err := service.validatedLifecycleRevision(
		ctx,
		material.ID,
		revisionID,
		false,
	)
	if err != nil {
		return ports.MaterialLifecycleProjection{}, err
	}
	return lifecycleProjection(material, revision, artifact), nil
}

func (service *productionService) validatedLifecycleRevision(
	ctx context.Context,
	materialID uuid.UUID,
	revisionID uuid.UUID,
	requireCurrentPipeline bool,
) (entity.MaterialRevision, contentrender.Artifact, error) {
	revision, err := service.repository.LoadRevision(
		ctx,
		materialID,
		revisionID,
	)
	if err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	artifact, err := loadValidatedRevisionArtifact(
		ctx,
		service.content,
		revision,
		requireCurrentPipeline,
	)
	if err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	return revision, artifact, nil
}

func lifecycleProjection(
	material entity.Material,
	revision entity.MaterialRevision,
	artifact contentrender.Artifact,
) ports.MaterialLifecycleProjection {
	return ports.MaterialLifecycleProjection{
		Version:                material.Version,
		Status:                 material.State(),
		Visibility:             entity.MaterialVisibility(material.Visibility),
		Slug:                   material.Slug,
		RevisionID:             uuidPointer(revision.ID),
		RevisionNumber:         revision.RevisionNumber,
		Title:                  revision.Title,
		CategoryID:             revision.CategoryID,
		DifficultyID:           revision.DifficultyID,
		TagIDs:                 append([]uuid.UUID(nil), revision.TagIDs...),
		RenderSourceID:         revision.RenderSourceID,
		RenderArtifactID:       revision.RenderArtifactID,
		SourceChecksum:         hex.EncodeToString(revision.NormalizedSourceChecksum[:]),
		RendererVersion:        artifact.RendererVersion,
		SanitizerPolicyVersion: artifact.SanitizerPolicyVersion,
	}
}

func validateLifecycleCommand(
	action ports.MaterialLifecycleAction,
	materialID uuid.UUID,
	targetRevisionID uuid.UUID,
	expectedVersion int64,
) error {
	if !action.Valid() || materialID == uuid.Nil || expectedVersion < 1 ||
		(!lifecycleRequiresTargetRevision(action) !=
			(targetRevisionID == uuid.Nil)) {
		return entity.ErrMaterialInvalidCommand
	}
	return nil
}

func validateLifecycleTransition(
	material entity.Material,
	action ports.MaterialLifecycleAction,
	targetRevisionID uuid.UUID,
) error {
	switch action {
	case ports.MaterialLifecyclePublish:
		if material.State() == entity.MaterialArchived ||
			targetRevisionID != material.CurrentDraftID ||
			(material.PublishedRevisionID != nil &&
				*material.PublishedRevisionID == targetRevisionID) {
			return entity.ErrMaterialInvalidLifecycle
		}
	case ports.MaterialLifecycleArchive:
		if material.State() != entity.MaterialPublished ||
			material.PublishedRevisionID == nil {
			return entity.ErrMaterialInvalidLifecycle
		}
	case ports.MaterialLifecycleSubmitReview:
		if material.State() != entity.MaterialDraft || targetRevisionID != uuid.Nil {
			return entity.ErrMaterialInvalidLifecycle
		}
	case ports.MaterialLifecycleReturnToDraft:
		if material.State() != entity.MaterialInReview || targetRevisionID != uuid.Nil {
			return entity.ErrMaterialInvalidLifecycle
		}
	case ports.MaterialLifecycleRollback:
		if material.State() != entity.MaterialPublished ||
			material.PublishedRevisionID == nil ||
			*material.PublishedRevisionID == targetRevisionID ||
			material.CurrentDraftID == targetRevisionID {
			return entity.ErrMaterialInvalidLifecycle
		}
	default:
		return entity.ErrMaterialInvalidCommand
	}
	return nil
}

func validateLifecycleTarget(
	action ports.MaterialLifecycleAction,
	before ports.MaterialLifecycleProjection,
	target entity.MaterialRevision,
) error {
	if before.RevisionNumber < 1 || target.RevisionNumber < 1 {
		return entity.ErrMaterialInvalidLifecycle
	}
	switch action {
	case ports.MaterialLifecyclePublish:
		if before.Status == entity.MaterialPublished &&
			target.RevisionNumber <= before.RevisionNumber {
			return entity.ErrMaterialInvalidLifecycle
		}
	case ports.MaterialLifecycleRollback:
		if target.RevisionNumber >= before.RevisionNumber {
			return entity.ErrMaterialInvalidLifecycle
		}
	default:
		return entity.ErrMaterialInvalidCommand
	}
	return nil
}

func materialLifecycleEvent(
	action ports.MaterialLifecycleAction,
) (string, error) {
	switch action {
	case ports.MaterialLifecyclePublish:
		return materialcontract.EventPublishedV1, nil
	case ports.MaterialLifecycleArchive:
		return materialcontract.EventArchivedV1, nil
	case ports.MaterialLifecycleRollback:
		return materialcontract.EventRolledBackV1, nil
	case ports.MaterialLifecycleSubmitReview:
		return materialcontract.EventReviewSubmittedV1, nil
	case ports.MaterialLifecycleReturnToDraft:
		return materialcontract.EventReviewReturnedV1, nil
	default:
		return "", entity.ErrMaterialInvalidCommand
	}
}

func lifecycleRequiresTargetRevision(action ports.MaterialLifecycleAction) bool {
	return action == ports.MaterialLifecyclePublish || action == ports.MaterialLifecycleRollback
}

func lifecycleTargetStatus(action ports.MaterialLifecycleAction) entity.MaterialState {
	switch action {
	case ports.MaterialLifecycleSubmitReview:
		return entity.MaterialInReview
	case ports.MaterialLifecycleReturnToDraft:
		return entity.MaterialDraft
	default:
		return ""
	}
}

func lifecycleProjectionDocument(
	projection ports.MaterialLifecycleProjection,
) map[string]any {
	return map[string]any{
		"version":                  projection.Version,
		"status":                   string(projection.Status),
		"visibility":               string(projection.Visibility),
		"slug":                     projection.Slug,
		"revision_id":              uuidString(projection.RevisionID),
		"revision_number":          projection.RevisionNumber,
		"title":                    projection.Title,
		"category_id":              projection.CategoryID.String(),
		"difficulty_id":            projection.DifficultyID.String(),
		"tag_ids":                  uuidValues(projection.TagIDs),
		"render_source_id":         projection.RenderSourceID.String(),
		"render_artifact_id":       projection.RenderArtifactID.String(),
		"source_checksum":          projection.SourceChecksum,
		"renderer_version":         projection.RendererVersion,
		"sanitizer_policy_version": projection.SanitizerPolicyVersion,
	}
}

func uuidValues(values []uuid.UUID) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}

func uuidPointer(value uuid.UUID) *uuid.UUID {
	result := value
	return &result
}

func (service *productionService) Rename(ctx context.Context, command ports.RenameMaterialCommand) (*contract.MaterialMutationReceipt, error) {
	slug, err := entity.NormalizeSlug(command.Slug)
	if err != nil || command.CommandID == uuid.Nil || command.ActorID == uuid.Nil ||
		command.MaterialID == uuid.Nil || command.ExpectedVersion < 1 {
		return nil, entity.ErrMaterialInvalidCommand
	}
	reason, err := entity.RequireReason(command.Reason)
	if err != nil {
		return nil, err
	}
	requestHash, err := materialMutationRequestHash(
		materialcontract.EventSlugChangedV1,
		struct {
			MaterialID      uuid.UUID `json:"material_id"`
			ActorID         uuid.UUID `json:"actor_id"`
			Slug            string    `json:"slug"`
			ExpectedVersion int64     `json:"expected_version"`
			Reason          string    `json:"reason"`
		}{
			MaterialID: command.MaterialID, ActorID: command.ActorID, Slug: slug,
			ExpectedVersion: command.ExpectedVersion, Reason: reason,
		},
	)
	if err != nil {
		return nil, err
	}
	if replay, found, replayErr := service.loadMaterialMutationReceipt(
		ctx, command.CommandID, materialcontract.EventSlugChangedV1,
		command.MaterialID, command.ExpectedVersion+1, requestHash, true,
	); replayErr != nil || found {
		return replay, replayErr
	}
	err = service.journal.Execute(ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		locked, err := service.repository.Lock(txCtx, command.MaterialID)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if locked.Version != command.ExpectedVersion ||
			locked.State() == entity.MaterialArchived {
			return platformmutation.Command{}, entity.ErrMaterialConflict
		}
		id, err := service.newID()
		if err != nil {
			return platformmutation.Command{}, fmt.Errorf("create material slug identity: %w", err)
		}
		updated, err := service.repository.Rename(txCtx, command.MaterialID, command.ExpectedVersion, entity.MaterialSlug{
			ID: id, MaterialID: command.MaterialID, Slug: slug, State: "current", CreatedAt: service.now().UTC(),
		}, service.now().UTC())
		if err != nil {
			return platformmutation.Command{}, err
		}
		mutation := materialMutation(
			materialcontract.EventSlugChangedV1, updated, reason,
			map[string]any{"version": locked.Version, "slug": locked.Slug},
			map[string]any{"version": updated.Version, "slug": updated.Slug},
		)
		mutation.EventID = command.CommandID
		mutation.Payload["request_hash"] = requestHash
		return mutation, nil
	})
	if err != nil {
		if replay, found, replayErr := service.loadMaterialMutationReceipt(
			ctx, command.CommandID, materialcontract.EventSlugChangedV1,
			command.MaterialID, command.ExpectedVersion+1, requestHash, true,
		); replayErr != nil || found {
			return replay, replayErr
		}
		return nil, err
	}
	receipt, found, err := service.loadMaterialMutationReceipt(
		ctx, command.CommandID, materialcontract.EventSlugChangedV1,
		command.MaterialID, command.ExpectedVersion+1, requestHash, false,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, entity.ErrMaterialConflict
	}
	return receipt, nil
}

type materialMutationReceiptPayload struct {
	MaterialID  uuid.UUID `json:"material_id"`
	Version     int64     `json:"version"`
	RequestHash string    `json:"request_hash"`
}

func (service *productionService) loadMaterialMutationReceipt(
	ctx context.Context,
	commandID uuid.UUID,
	eventType string,
	expectedMaterialID uuid.UUID,
	expectedVersion int64,
	requestHash string,
	replayed bool,
) (*contract.MaterialMutationReceipt, bool, error) {
	if commandID == uuid.Nil || strings.TrimSpace(eventType) == "" ||
		expectedVersion < 1 || len(requestHash) != sha256.Size*2 {
		return nil, false, entity.ErrMaterialInvalidCommand
	}
	receipt, found, err := service.repository.LoadCommandReceipt(ctx, commandID)
	if err != nil || !found {
		return nil, found, err
	}
	var payload materialMutationReceiptPayload
	if receipt.EventID != commandID || receipt.EventType != eventType ||
		receipt.AggregateType != materialAggregate ||
		receipt.AggregateID == uuid.Nil || receipt.Version != expectedVersion ||
		receipt.OccurredAt.IsZero() ||
		json.Unmarshal(receipt.Payload, &payload) != nil ||
		payload.MaterialID != receipt.AggregateID || payload.Version != receipt.Version ||
		payload.RequestHash != requestHash ||
		(expectedMaterialID != uuid.Nil && receipt.AggregateID != expectedMaterialID) {
		return nil, false, entity.ErrMaterialCommandConflict
	}
	return &contract.MaterialMutationReceipt{
		MaterialID:       receipt.AggregateID,
		Action:           receipt.EventType,
		CommandID:        commandID,
		EventID:          receipt.EventID,
		Version:          receipt.Version,
		CommittedAt:      receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func materialMutationRequestHash(eventType string, document any) (string, error) {
	if strings.TrimSpace(eventType) == "" || document == nil {
		return "", entity.ErrMaterialInvalidCommand
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return "", entity.ErrMaterialInvalidCommand
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte("judgify/material/mutation-idempotency/v1"))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(eventType))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(encoded)
	return hex.EncodeToString(digest.Sum(nil)), nil
}

type revisionInput struct {
	revisionID, sourceID, materialID uuid.UUID
	revisionNumber                   int64
	title, description               string
	content                          []byte
	categoryID, difficultyID         uuid.UUID
	tagIDs                           []uuid.UUID
	authorID                         uuid.UUID
	reason                           string
	createdAt                        time.Time
}

func (service *productionService) buildRevision(ctx context.Context, input revisionInput) (entity.MaterialRevision, contentrender.Artifact, error) {
	if err := entity.ValidateRevisionInput(input.title, input.description, input.content, input.tagIDs); err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	normalized, err := contentrender.NormalizeMarkdown(input.content)
	if err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	source := contentrender.Source{
		ID: input.sourceID, OwnerRevisionID: input.revisionID,
		Kind: contentrender.MaterialBody, Checksum: normalized.Checksum,
		CreatedBy: &input.authorID, CreatedAt: input.createdAt,
	}
	source, err = service.content.RegisterSource(ctx, source)
	if err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	artifact, err := service.renderer.Render(ctx, contentrender.RenderRequest{
		SourceID: source.ID, SourceKind: source.Kind, SourceChecksum: source.Checksum,
		NormalizedMarkdown: normalized.Bytes,
	})
	if err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	if artifact.Status != contentrender.StatusReady {
		return entity.MaterialRevision{}, contentrender.Artifact{}, entity.ErrMaterialArtifactRejected
	}
	artifact.CreatedBy = &input.authorID
	artifact.CreatedAt = input.createdAt
	artifact, err = service.content.StoreArtifact(ctx, source, artifact)
	if err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	revision := entity.MaterialRevision{
		ID: input.revisionID, MaterialID: input.materialID,
		RevisionNumber: input.revisionNumber, Title: strings.TrimSpace(input.title),
		Description:     strings.TrimSpace(input.description),
		ContentMarkdown: append([]byte(nil), normalized.Bytes...),
		CategoryID:      input.categoryID, DifficultyID: input.difficultyID,
		EstimatedReadMinutes: entity.EstimatedReadMinutes(normalized.Bytes),
		AuthorID:             input.authorID, CreatedAt: input.createdAt,
		ChangeSummary: input.reason, NormalizedSourceChecksum: normalized.Checksum,
		RenderSourceID: source.ID, RenderSourceKind: source.Kind,
		RenderArtifactID: artifact.ID, TagIDs: append([]uuid.UUID(nil), input.tagIDs...),
	}
	if err := revision.Validate(); err != nil {
		return entity.MaterialRevision{}, contentrender.Artifact{}, err
	}
	return revision, artifact, nil
}

func (service *productionService) twoIDs() (uuid.UUID, uuid.UUID, error) {
	first, err := service.newID()
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	second, err := service.newID()
	return first, second, err
}

func (service *productionService) threeIDs() (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	first, second, err := service.twoIDs()
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	third, err := service.newID()
	return first, second, third, err
}

func materialMutation(action string, material entity.Material, reason string, before, after map[string]any) platformmutation.Command {
	return platformmutation.Command{
		Action: action, Resource: materialAggregate, ResourceID: material.ID.String(),
		Reason: reason, Before: before, After: after, EventType: action,
		AggregateType: materialAggregate, AggregateID: material.ID,
		AggregateVersion: material.Version,
		Payload:          map[string]any{"material_id": material.ID.String(), "version": material.Version, "after": after},
	}
}

func materialEvidence(material entity.Material, revision entity.MaterialRevision) map[string]any {
	return map[string]any{
		"material_id": material.ID.String(), "version": material.Version,
		"status": material.Status, "visibility": material.Visibility,
		"revision_id": revision.ID.String(), "revision_number": revision.RevisionNumber,
		"render_source_id":   revision.RenderSourceID.String(),
		"render_artifact_id": revision.RenderArtifactID.String(),
	}
}

func uuidString(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

var _ ports.MaterialEditorialService = (*productionService)(nil)
